package cli

import (
    "fmt"
    "io"
    "os"

    "github.com/spf13/cobra"
)

// NewRootCmd returns the root cobra command for the incus-backup CLI.
func NewRootCmd(stdout, stderr io.Writer) *cobra.Command {
    cmd := &cobra.Command{
        Use:   "incus-backup",
        Short: "Back up and restore Incus instances, volumes, images, and config",
        SilenceUsage:  true,
        SilenceErrors: true,
    }

    cmd.SetOut(stdout)
    cmd.SetErr(stderr)

    // Global flags (log level to be added later; safety flags now)
    addGlobalFlags(cmd)

    // Subcommands
    cmd.AddCommand(newVersionCmd(stdout))
    cmd.AddCommand(newListCmd(stdout, stderr))

    return cmd
}

// Execute runs the CLI with the process stdio.
func Execute() int {
    root := NewRootCmd(os.Stdout, os.Stderr)
    if err := root.Execute(); err != nil {
        // cobra already wrote the error to stderr if appropriate
        // return non-zero exit code
        fmt.Fprintln(os.Stderr, err)
        return 1
    }
    return 0
}
