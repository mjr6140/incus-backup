package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	cfg "incus-backup/src/backup/config"
	ibak "incus-backup/src/backup/instances"
	vbak "incus-backup/src/backup/volumes"
	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	"incus-backup/src/target"
)

func newBackupAllCmd(stdout, stderr io.Writer) *cobra.Command {
	var project string
	var optimized bool
	var noSnapshot bool
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Back up config, all custom volumes, and all instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return fmt.Errorf("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}

			client, err := incusapi.ConnectLocal()
			if err != nil {
				return err
			}

			resticMode := tgt.Scheme == "restic"
			var (
				info      restic.BinaryInfo
				resticCtx context.Context
			)
			if resticMode {
				info, err = checkResticBinary(cmd, true)
				if err != nil {
					return err
				}
				resticCtx = cmd.Context()
				if resticCtx == nil {
					resticCtx = context.Background()
				}
			}

			switch tgt.Scheme {
			case "dir":
				fmt.Fprintln(stdout, "[1/3] Backing up config")
				if _, err := cfg.BackupAll(client, tgt.DirPath, time.Now()); err != nil {
					return err
				}
				fmt.Fprintln(stdout, "[1/3] Done config")
			case "restic":
				fmt.Fprintln(stdout, "[1/3] Backing up config")
				if _, err := cfg.BackupAllRestic(resticCtx, info, tgt.Value, client, time.Now(), stdout); err != nil {
					return err
				}
				fmt.Fprintln(stdout, "[1/3] Done config")
			default:
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}

			// Volumes (all)
			vols, err := client.ListCustomVolumes(project)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "[2/3] Backing up volumes (count=%d)\n", len(vols))
			for i, v := range vols {
				fmt.Fprintf(stdout, "  [%d/%d] %s/%s\n", i+1, len(vols), v.Pool, v.Name)
				if resticMode {
					if _, err := vbak.BackupVolumeRestic(resticCtx, info, tgt.Value, client, project, v.Pool, v.Name, optimized, !noSnapshot, time.Now(), stdout); err != nil {
						return err
					}
				} else {
					if _, err := vbak.BackupVolume(client, tgt.DirPath, project, v.Pool, v.Name, optimized, !noSnapshot, time.Now(), stdout); err != nil {
						return err
					}
				}
			}
			fmt.Fprintln(stdout, "[2/3] Done volumes")

			// Instances (all)
			insts, err := client.ListInstances(project)
			if err != nil {
				return err
			}
			fmt.Fprintf(stdout, "[3/3] Backing up instances (count=%d)\n", len(insts))
			for i, in := range insts {
				fmt.Fprintf(stdout, "  [%d/%d] %s\n", i+1, len(insts), in.Name)
				if resticMode {
					if _, err := ibak.BackupInstanceRestic(resticCtx, info, tgt.Value, client, project, in.Name, optimized, !noSnapshot, time.Now(), stdout); err != nil {
						return err
					}
				} else {
					if _, err := ibak.BackupInstance(client, tgt.DirPath, project, in.Name, optimized, !noSnapshot, time.Now(), stdout); err != nil {
						return err
					}
				}
			}
			fmt.Fprintln(stdout, "[3/3] Done instances")
			return nil
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&project, "project", "default", "Incus project")
	cmd.Flags().BoolVar(&optimized, "optimized", false, "Use storage-optimized export format")
	cmd.Flags().BoolVar(&noSnapshot, "no-snapshot", false, "Do not create a temporary snapshot before export")
	return cmd
}
