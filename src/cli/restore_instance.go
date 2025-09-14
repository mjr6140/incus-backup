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

func newRestoreInstanceCmd(stdout, stderr io.Writer) *cobra.Command {
    var project, version, targetName string
    var replace bool
    var skipExisting bool
    cmd := &cobra.Command{
        Use:   "instance NAME",
        Short: "Restore a single instance from backup",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            name := args[0]
            tgtStr, _ := cmd.Flags().GetString("target")
            if tgtStr == "" { return errors.New("--target is required (e.g., dir:/path)") }
            tgt, err := target.Parse(tgtStr)
            if err != nil { return err }
            snapDir, err := resolveInstanceSnapshotDir(tgt, project, name, version)
            if err != nil { return err }
            client, err := incusapi.ConnectLocal()
            if err != nil { return err }
            destName := targetName
            if destName == "" { destName = name }
            exists, err := client.InstanceExists(project, destName)
            if err != nil { return err }
            opts := getSafetyOptions(cmd)
            if opts.DryRun {
                // Table-style preview
                action := "create"
                if exists {
                    action = "conflict"
                    if replace { action = "replace" }
                    if skipExisting { action = "skip" }
                }
                versionID := filepath.Base(snapDir)
                renderInstanceRestorePreview(stdout, []instancePreviewRow{{
                    Action:     action,
                    Project:    project,
                    Name:       name,
                    TargetName: destName,
                    Version:    versionID,
                }})
                return nil
            }
            if exists {
                if skipExisting { return nil }
                // If not replace, prompt user once
                if !replace {
                    var b strings.Builder
                    b.WriteString(fmt.Sprintf("Instance %s already exists in project %s. Replace it?\n", destName, project))
                    ok, err := safety.Confirm(opts, os.Stdin, stdout, b.String())
                    if err != nil { return err }
                    if !ok { return nil }
                }
                // Stop and delete existing (force when --force)
                _ = client.StopInstance(project, destName, true)
                if err := client.DeleteInstance(project, destName); err != nil { return err }
            }
            return inst.RestoreInstance(client, snapDir, project, destName)
        },
    }
    cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
    cmd.Flags().StringVar(&project, "project", "default", "Incus project")
    cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest)")
    cmd.Flags().StringVar(&targetName, "target-name", "", "Optional new name for the restored instance")
    cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing instance if it exists")
    cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip if the target instance already exists")
    return cmd
}

type instancePreviewRow struct {
    Action     string
    Project    string
    Name       string
    TargetName string
    Version    string
}

func renderInstanceRestorePreview(w io.Writer, rows []instancePreviewRow) {
    tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
    fmt.Fprintln(tw, "ACTION\tPROJECT\tNAME\tTARGET_NAME\tVERSION")
    for _, r := range rows {
        fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Name, r.TargetName, r.Version)
    }
    _ = tw.Flush()
}

func resolveInstanceSnapshotDir(tgt target.Target, project, name, version string) (string, error) {
    base := filepath.Join(tgt.DirPath, "instances", project, name)
    if version != "" { return filepath.Join(base, version), nil }
    entries, err := os.ReadDir(base)
    if err != nil { return "", err }
    var snaps []string
    for _, e := range entries { if e.IsDir() && !strings.HasPrefix(e.Name(), ".") { snaps = append(snaps, e.Name()) } }
    if len(snaps) == 0 { return "", fmt.Errorf("no snapshots found under %s", base) }
    sort.Strings(snaps)
    return filepath.Join(base, snaps[len(snaps)-1]), nil
}
