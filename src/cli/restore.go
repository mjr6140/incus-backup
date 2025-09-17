package cli

import (
	"io"

	"github.com/spf13/cobra"
)

func newRestoreCmd(stdout, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restore",
		Short: "Restore from backups",
	}

	cmd.AddCommand(newRestoreAllCmd(stdout, stderr))
	cmd.AddCommand(newRestoreConfigCmd(stdout, stderr))
	cmd.AddCommand(newRestoreInstanceCmd(stdout, stderr))
	cmd.AddCommand(newRestoreInstancesCmd(stdout, stderr))
	cmd.AddCommand(newRestoreVolumeCmd(stdout, stderr))
	cmd.AddCommand(newRestoreVolumesCmd(stdout, stderr))

	return cmd
}
