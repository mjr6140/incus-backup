package cli

import (
    "fmt"
    "io"
    "time"

    "github.com/spf13/cobra"

    inst "incus-backup/src/backup/instances"
    "incus-backup/src/incusapi"
    "incus-backup/src/target"
)

func newBackupInstancesCmd(stdout, stderr io.Writer) *cobra.Command {
    var project string
    var optimized bool
    var noSnapshot bool
    cmd := &cobra.Command{
        Use:   "instances [NAME...]",
        Short: "Back up instances (all or selected by name)",
        RunE: func(cmd *cobra.Command, args []string) error {
            tgtStr, _ := cmd.Flags().GetString("target")
            if tgtStr == "" { return fmt.Errorf("--target is required (e.g., dir:/path)") }
            tgt, err := target.Parse(tgtStr)
            if err != nil { return err }
            client, err := incusapi.ConnectLocal()
            if err != nil { return err }
            // If no args, list instances in project
            names := args
            if len(names) == 0 {
                insts, err := client.ListInstances(project)
                if err != nil { return err }
                for _, i := range insts { names = append(names, i.Name) }
            }
            total := len(names)
            for idx, name := range names {
                // Per-instance header to indicate progress across many instances
                fmt.Fprintf(stdout, "[%d/%d] Backing up instance %s/%s\n", idx+1, total, project, name)
                if _, err := inst.BackupInstance(client, tgt.DirPath, project, name, optimized, !noSnapshot, time.Now(), stdout); err != nil { return err }
                fmt.Fprintf(stdout, "[%d/%d] Done %s/%s\n", idx+1, total, project, name)
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
