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

func newRestoreVolumeCmd(stdout, stderr io.Writer) *cobra.Command {
	var project, version, targetName, pool string
	var replace, skipExisting bool
	cmd := &cobra.Command{
		Use:   "volume POOL/NAME",
		Short: "Restore a custom volume from backup",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var name string
			// Parse POOL/NAME safely (Go's fmt.Scanf doesn't support scansets like %[^/])
			if parts := strings.SplitN(args[0], "/", 2); len(parts) == 2 {
				pool, name = parts[0], parts[1]
			}
			if pool == "" || name == "" {
				return fmt.Errorf("invalid volume spec %q (expected POOL/NAME)", args[0])
			}
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
				return restoreVolumeFromRestic(cmd, client, tgt, project, pool, name, version, targetName, replace, skipExisting, stdout)
			}
			if tgt.Scheme != "dir" {
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}
			snapDir, err := resolveVolumeSnapshotDir(tgt, project, pool, name, version)
			if err != nil {
				return err
			}
			destName := targetName
			if destName == "" {
				destName = name
			}
			exists, err := client.VolumeExists(project, pool, destName)
			if err != nil {
				return err
			}
			// preview table
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
			renderVolumeRestorePreview(stdout, []volumePreviewRow{{Action: action, Project: project, Pool: pool, Name: name, TargetName: destName, Version: filepath.Base(snapDir)}})
			opts := getSafetyOptions(cmd)
			if opts.DryRun {
				return nil
			}
			if exists {
				if skipExisting {
					return nil
				}
				if !replace {
					ok, err := safety.Confirm(opts, os.Stdin, stdout, fmt.Sprintf("Volume %s/%s exists. Replace it?", pool, destName))
					if err != nil {
						return err
					}
					if !ok {
						return nil
					}
				}
				if err := client.DeleteVolume(project, pool, destName); err != nil {
					return err
				}
			}
			return vol.RestoreVolume(client, snapDir, project, pool, destName, stdout)
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&project, "project", "default", "Incus project")
	cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest)")
	cmd.Flags().StringVar(&targetName, "target-name", "", "Optional new name for the restored volume")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing volume if it exists")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip if the target volume already exists")
	return cmd
}

type volumePreviewRow struct{ Action, Project, Pool, Name, TargetName, Version string }

func renderVolumeRestorePreview(w io.Writer, rows []volumePreviewRow) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tPROJECT\tPOOL\tNAME\tTARGET_NAME\tVERSION")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Pool, r.Name, r.TargetName, r.Version)
	}
	_ = tw.Flush()
}

func resolveVolumeSnapshotDir(tgt target.Target, project, pool, name, version string) (string, error) {
	base := filepath.Join(tgt.DirPath, "volumes", project, pool, name)
	if version != "" {
		return filepath.Join(base, version), nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	var snaps []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			snaps = append(snaps, e.Name())
		}
	}
	if len(snaps) == 0 {
		return "", fmt.Errorf("no snapshots found under %s", base)
	}
	sort.Strings(snaps)
	return filepath.Join(base, snaps[len(snaps)-1]), nil
}
