package cli

import (
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/spf13/cobra"

    inst "incus-backup/src/backup/instances"
    "incus-backup/src/incusapi"
    "incus-backup/src/target"
)

func newRestoreInstanceCmd(stdout, stderr io.Writer) *cobra.Command {
    var project, version, targetName string
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
            return inst.RestoreInstance(client, snapDir, project, targetName)
        },
    }
    cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
    cmd.Flags().StringVar(&project, "project", "default", "Incus project")
    cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest)")
    cmd.Flags().StringVar(&targetName, "target-name", "", "Optional new name for the restored instance")
    return cmd
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

