package cli

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "github.com/spf13/cobra"

    cfg "incus-backup/src/backup/config"
    "incus-backup/src/safety"
    "incus-backup/src/incusapi"
    "incus-backup/src/target"
)

func newRestoreCmd(stdout, stderr io.Writer) *cobra.Command {
    cmd := &cobra.Command{Use: "restore", Short: "Restore from backups"}
    cmd.AddCommand(newRestoreConfigCmd(stdout, stderr))
    return cmd
}

func newRestoreConfigCmd(stdout, stderr io.Writer) *cobra.Command {
    var version, output string
    var apply bool
    cmd := &cobra.Command{
        Use:   "config",
        Short: "Preview or apply declarative config from a backup",
        RunE: func(cmd *cobra.Command, args []string) error {
            tgtStr, _ := cmd.Flags().GetString("target")
            if tgtStr == "" {
                return fmt.Errorf("--target is required (e.g., dir:/path)")
            }
            tgt, err := target.Parse(tgtStr)
            if err != nil { return err }
            if tgt.Scheme != "dir" { return fmt.Errorf("unsupported backend: %s", tgt.Scheme) }

            snapDir, err := resolveConfigSnapshotDir(tgt, version)
            if err != nil { return err }
            desired, err := loadProjects(filepath.Join(snapDir, "projects.json"))
            if err != nil { return err }

            client, err := incusapi.ConnectLocal()
            if err != nil { return err }
            current, err := client.ListProjects()
            if err != nil { return err }

            plan := cfg.BuildProjectsPlan(current, desired)
            if !apply {
                switch output {
                case "json":
                    enc := json.NewEncoder(stdout)
                    enc.SetIndent("", "  ")
                    return enc.Encode(plan)
                case "table", "":
                    renderProjectsPlan(stdout, plan)
                    return nil
                default:
                    return fmt.Errorf("unsupported --output: %s", output)
                }
            }
            // Apply mode
            opts := getSafetyOptions(cmd)
            if opts.DryRun {
                // print preview then exit
                renderProjectsPlan(stdout, plan)
                return nil
            }
            // Prompt once with summary unless --yes
            var buf strings.Builder
            buf.WriteString("Apply config changes for projects?\n")
            buf.WriteString(fmt.Sprintf("Create: %d, Update: %d, Delete: %d\n", len(plan.ToCreate), len(plan.ToUpdate), len(plan.ToDelete)))
            ok, err := safety.Confirm(opts, os.Stdin, stdout, buf.String())
            if err != nil { return err }
            if !ok { return nil }
            summary, err := cfg.ApplyProjectsPlan(client, plan)
            if err != nil { return err }
            fmt.Fprintln(stdout, summary)
            return nil
        },
    }
    cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
    cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest)")
    cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes (default: preview)")
    cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json")
    return cmd
}

func resolveConfigSnapshotDir(tgt target.Target, version string) (string, error) {
    base := filepath.Join(tgt.DirPath, "config")
    if version != "" {
        return filepath.Join(base, version), nil
    }
    // pick latest lexicographically
    entries, err := os.ReadDir(base)
    if err != nil { return "", err }
    var names []string
    for _, e := range entries {
        if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
            names = append(names, e.Name())
        }
    }
    if len(names) == 0 { return "", fmt.Errorf("no config snapshots found under %s", base) }
    sort.Strings(names)
    return filepath.Join(base, names[len(names)-1]), nil
}

func loadProjects(path string) ([]incusapi.Project, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    var out []incusapi.Project
    if err := json.Unmarshal(b, &out); err != nil { return nil, err }
    return out, nil
}

func renderProjectsPlan(w io.Writer, p cfg.ProjectPlan) {
    fmt.Fprintf(w, "Config preview (projects)\n")
    fmt.Fprintf(w, "Create: %d\n", len(p.ToCreate))
    for _, c := range p.ToCreate { fmt.Fprintf(w, "  + %s\n", c.Name) }
    fmt.Fprintf(w, "Update: %d\n", len(p.ToUpdate))
    for _, u := range p.ToUpdate { fmt.Fprintf(w, "  ~ %s\n", u.Name) }
    fmt.Fprintf(w, "Delete: %d\n", len(p.ToDelete))
    for _, d := range p.ToDelete { fmt.Fprintf(w, "  - %s\n", d.Name) }
}
