package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	vol "incus-backup/src/backup/volumes"
	"incus-backup/src/incusapi"
	"incus-backup/src/safety"
	"incus-backup/src/target"
)

func newRestoreVolumesCmd(stdout, stderr io.Writer) *cobra.Command {
	var project, version string
	var replace, skipExisting bool
	cmd := &cobra.Command{
		Use:   "volumes [POOL/NAME ...]",
		Short: "Restore custom volumes (all or selected)",
		RunE: func(cmd *cobra.Command, args []string) error {
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return errors.New("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}

			client, err := incusapi.ConnectLocal()
			if err != nil {
				return err
			}

			if tgt.Scheme == "restic" {
				return restoreVolumesFromRestic(cmd, client, tgt, project, version, args, replace, skipExisting, stdout)
			}
			if tgt.Scheme != "dir" {
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}

			var items [][2]string
			if len(args) == 0 {
				base := filepath.Join(tgt.DirPath, "volumes", project)
				pools, err := os.ReadDir(base)
				if err != nil {
					return fmt.Errorf("scan pools under %s: %w", base, err)
				}
				for _, p := range pools {
					if !p.IsDir() || strings.HasPrefix(p.Name(), ".") {
						continue
					}
					names, err := os.ReadDir(filepath.Join(base, p.Name()))
					if err != nil {
						return err
					}
					for _, n := range names {
						if n.IsDir() && !strings.HasPrefix(n.Name(), ".") {
							items = append(items, [2]string{p.Name(), n.Name()})
						}
					}
				}
			} else {
				for _, a := range args {
					var pool, name string
					if parts := strings.SplitN(a, "/", 2); len(parts) == 2 {
						pool, name = parts[0], parts[1]
					}
					if pool == "" || name == "" {
						return fmt.Errorf("invalid volume spec %q (expected POOL/NAME)", a)
					}
					items = append(items, [2]string{pool, name})
				}
			}
			sort.Slice(items, func(i, j int) bool {
				if items[i][0] == items[j][0] {
					return items[i][1] < items[j][1]
				}
				return items[i][0] < items[j][0]
			})
			if len(items) == 0 {
				return nil
			}

			type row struct{ Action, Project, Pool, Name, TargetName, Version string }
			var rows []row
			for _, it := range items {
				pool, name := it[0], it[1]
				snapDir, err := resolveVolumeSnapshotDir(tgt, project, pool, name, version)
				if err != nil {
					return err
				}
				exists, err := client.VolumeExists(project, pool, name)
				if err != nil {
					return err
				}
				action := "create"
				if exists {
					action = "conflict"
					if replace {
						action = "replace"
					}
					if skipExisting {
						action = "skip"
					}
				}
				rows = append(rows, row{Action: action, Project: project, Pool: pool, Name: name, TargetName: name, Version: filepath.Base(snapDir)})
			}
			tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTION\tPROJECT\tPOOL\tNAME\tTARGET_NAME\tVERSION")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Pool, r.Name, r.TargetName, r.Version)
			}
			_ = tw.Flush()

			opts := getSafetyOptions(cmd)
			if opts.DryRun {
				return nil
			}

			if !(replace || skipExisting) {
				ok, err := safety.Confirm(opts, os.Stdin, stdout, fmt.Sprintf("Apply restore for %d volumes?", len(items)))
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}

			for i, it := range items {
				pool, name := it[0], it[1]
				snapDir, err := resolveVolumeSnapshotDir(tgt, project, pool, name, version)
				if err != nil {
					return err
				}
				exists, err := client.VolumeExists(project, pool, name)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "[%d/%d] Restoring volume %s/%s (project %s)\n", i+1, len(items), pool, name, project)
				if exists {
					if skipExisting {
						fmt.Fprintf(stdout, "[%d/%d] Skip existing %s/%s\n", i+1, len(items), pool, name)
						continue
					}
					if err := client.DeleteVolume(project, pool, name); err != nil {
						return err
					}
				}
				if err := vol.RestoreVolume(client, snapDir, project, pool, name, stdout); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "[%d/%d] Done %s/%s\n", i+1, len(items), pool, name)
			}
			return nil
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&project, "project", "default", "Incus project")
	cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest per volume)")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing volumes if they exist")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip volumes that already exist")
	return cmd
}
