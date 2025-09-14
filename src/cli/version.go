package cli

import (
    "fmt"
    "io"

    "github.com/spf13/cobra"

    "incus-backup/src/version"
)

func newVersionCmd(stdout io.Writer) *cobra.Command {
    return &cobra.Command{
        Use:   "version",
        Short: "Print the version number",
        Run: func(cmd *cobra.Command, args []string) {
            fmt.Fprintln(stdout, version.Version)
        },
    }
}

