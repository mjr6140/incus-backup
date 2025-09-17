package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	cfg "incus-backup/src/backup/config"
	"incus-backup/src/incusapi"
	"incus-backup/src/target"
)

func newBackupCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{Use: "backup", Short: "Create backups"}
	cmd.AddCommand(newBackupAllCmd(stdout, stderr))
	cmd.AddCommand(newBackupConfigCmd(stdout, stderr))
	cmd.AddCommand(newBackupInstancesCmd(stdout, stderr))
	cmd.AddCommand(newBackupVolumesCmd(stdout, stderr))
	return cmd
}

func newBackupConfigCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Back up declarative config (projects, etc.)",
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
			switch tgt.Scheme {
			case "dir":
				_, err = cfg.BackupAll(client, tgt.DirPath, time.Now())
				return err
			case "restic":
				info, err := checkResticBinary(cmd, true)
				if err != nil {
					return err
				}
				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				_, err = cfg.BackupAllRestic(ctx, info, tgt.Value, client, time.Now(), stdout)
				return err
			default:
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	return cmd
}
