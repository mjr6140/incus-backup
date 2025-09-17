package cli

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	vol "incus-backup/src/backup/volumes"
	"incus-backup/src/incusapi"
	"incus-backup/src/target"
	"strings"
)

func newBackupVolumesCmd(stdout, stderr io.Writer) *cobra.Command {
	var project string
	var optimized bool
	var noSnapshot bool
	cmd := &cobra.Command{
		Use:   "volumes [POOL/NAME ...]",
		Short: "Back up custom volumes (all or selected)",
		RunE: func(cmd *cobra.Command, args []string) error {
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return fmt.Errorf("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}
			if tgt.Scheme == "restic" {
				return resticNotImplemented(cmd)
			}
			if tgt.Scheme != "dir" {
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}
			client, err := incusapi.ConnectLocal()
			if err != nil {
				return err
			}
			var items [][2]string // pool, name
			if len(args) == 0 {
				vols, err := client.ListCustomVolumes(project)
				if err != nil {
					return err
				}
				for _, v := range vols {
					items = append(items, [2]string{v.Pool, v.Name})
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
			total := len(items)
			for i, it := range items {
				pool, name := it[0], it[1]
				fmt.Fprintf(stdout, "[%d/%d] Backing up volume %s/%s (project %s)\n", i+1, total, pool, name, project)
				if _, err := vol.BackupVolume(client, tgt.DirPath, project, pool, name, optimized, !noSnapshot, time.Now(), stdout); err != nil {
					return err
				}
				fmt.Fprintf(stdout, "[%d/%d] Done %s/%s\n", i+1, total, pool, name)
			}
			return nil
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&project, "project", "default", "Incus project")
	cmd.Flags().BoolVar(&optimized, "optimized", false, "Use storage-optimized export format")
	cmd.Flags().BoolVar(&noSnapshot, "no-snapshot", false, "Do not create a temporary snapshot before export")
	return cmd
}
