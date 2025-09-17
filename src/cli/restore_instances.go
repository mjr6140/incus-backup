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

	inst "incus-backup/src/backup/instances"
	"incus-backup/src/incusapi"
	"incus-backup/src/safety"
	"incus-backup/src/target"
)

func newRestoreInstancesCmd(stdout, stderr io.Writer) *cobra.Command {
	var project, version string
	var replace, skipExisting bool
	cmd := &cobra.Command{
		Use:   "instances [NAME ...]",
		Short: "Restore one or more instances (or all if omitted)",
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
				return restoreInstancesFromRestic(cmd, client, tgt, project, version, args, replace, skipExisting, stdout)
			}

			if tgt.Scheme != "dir" {
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}

			var names []string
			if len(args) == 0 {
				instBase := filepath.Join(tgt.DirPath, "instances", project)
				entries, err := os.ReadDir(instBase)
				if err != nil {
					return fmt.Errorf("scan instances under %s: %w", instBase, err)
				}
				for _, e := range entries {
					if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
						names = append(names, e.Name())
					}
				}
			} else {
				names = append(names, args...)
			}
			sort.Strings(names)
			if len(names) == 0 {
				return nil
			}

			type row struct{ Action, Project, Name, TargetName, Version string }
			var rows []row
			for _, name := range names {
				destName := name
				snapDir, err := resolveInstanceSnapshotDir(tgt, project, name, version)
				if err != nil {
					return err
				}
				exists, err := client.InstanceExists(project, destName)
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
				rows = append(rows, row{Action: action, Project: project, Name: name, TargetName: destName, Version: filepath.Base(snapDir)})
			}

			tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTION\tPROJECT\tNAME\tTARGET_NAME\tVERSION")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Name, r.TargetName, r.Version)
			}
			_ = tw.Flush()

			opts := getSafetyOptions(cmd)
			if opts.DryRun {
				return nil
			}

			if !(replace || skipExisting) {
				ok, err := safety.Confirm(opts, os.Stdin, stdout, fmt.Sprintf("Apply restore for %d instances?", len(names)))
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
			}

			for i, name := range names {
				destName := name
				snapDir, err := resolveInstanceSnapshotDir(tgt, project, name, version)
				if err != nil {
					return err
				}
				exists, err := client.InstanceExists(project, destName)
				if err != nil {
					return err
				}
				fmt.Fprintf(stdout, "[%d/%d] Restoring instance %s/%s\n", i+1, len(names), project, destName)
				if exists {
					if skipExisting {
						fmt.Fprintf(stdout, "[%d/%d] Skip existing %s/%s\n", i+1, len(names), project, destName)
						continue
					}
					_ = client.StopInstance(project, destName, true)
					if err := client.DeleteInstance(project, destName); err != nil {
						return err
					}
				}
				if err := inst.RestoreInstance(client, snapDir, project, destName, stdout); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "[%d/%d] Done %s/%s\n", i+1, len(names), project, destName)
			}
			return nil
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&project, "project", "default", "Incus project")
	cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest per instance)")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing instances if they exist")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip instances that already exist")
	return cmd
}
