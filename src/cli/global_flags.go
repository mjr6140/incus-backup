package cli

import (
    "github.com/spf13/cobra"
    "incus-backup/src/safety"
)

// addGlobalFlags adds persistent safety-related flags to the root command.
func addGlobalFlags(cmd *cobra.Command) {
    cmd.PersistentFlags().Bool("dry-run", false, "Show planned actions without making changes")
    cmd.PersistentFlags().BoolP("yes", "y", false, "Assume 'yes' to prompts and run non-interactively")
    cmd.PersistentFlags().Bool("force", false, "Force potentially dangerous operations (implies --yes in some cases)")
}

// getSafetyOptions reads global flags into a safety.Options struct.
func getSafetyOptions(cmd *cobra.Command) safety.Options {
    dry, _ := cmd.Root().PersistentFlags().GetBool("dry-run")
    yes, _ := cmd.Root().PersistentFlags().GetBool("yes")
    force, _ := cmd.Root().PersistentFlags().GetBool("force")
    return safety.Options{DryRun: dry, Yes: yes, Force: force}
}

