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
    cmd.AddCommand(newRestoreInstanceCmd(stdout, stderr))
    cmd.AddCommand(newRestoreVolumeCmd(stdout, stderr))
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
            desiredProjects, err := loadProjects(filepath.Join(snapDir, "projects.json"))
            if err != nil { return err }

            client, err := incusapi.ConnectLocal()
            if err != nil { return err }
            current, err := client.ListProjects()
            if err != nil { return err }

            plan := cfg.BuildProjectsPlan(current, desiredProjects)
            // Also compute networks and storage pools plans
            desiredNetworks, _ := loadNetworks(filepath.Join(snapDir, "networks.json"))
            currentNetworks, _ := client.ListNetworks()
            nplan := cfg.BuildNetworksPlan(currentNetworks, desiredNetworks)

            desiredPools, _ := loadStoragePools(filepath.Join(snapDir, "storage_pools.json"))
            currentPools, _ := client.ListStoragePools()
            splan := cfg.BuildStoragePoolsPlan(currentPools, desiredPools)
            if !apply {
                switch output {
                case "json":
                    enc := json.NewEncoder(stdout)
                    enc.SetIndent("", "  ")
                    return enc.Encode(struct{
                        Projects cfg.ProjectPlan `json:"projects"`
                        Networks cfg.NetworkPlan `json:"networks"`
                        StoragePools cfg.StoragePoolPlan `json:"storage_pools"`
                    }{plan, nplan, splan})
                case "table", "":
                    renderProjectsPlan(stdout, plan)
                    renderNetworksPlan(stdout, nplan)
                    renderStoragePoolsPlan(stdout, splan)
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
                renderNetworksPlan(stdout, nplan)
                renderStoragePoolsPlan(stdout, splan)
                return nil
            }
            // Always echo a table-style preview before confirmation
            renderProjectsPlan(stdout, plan)
            renderNetworksPlan(stdout, nplan)
            renderStoragePoolsPlan(stdout, splan)
            // Prompt once with summary unless --yes
            var buf strings.Builder
            buf.WriteString("Apply config changes? (networks/storage pools may disrupt running workloads)\n")
            buf.WriteString(fmt.Sprintf("Projects => Create: %d, Update: %d, Delete: %d\n", len(plan.ToCreate), len(plan.ToUpdate), len(plan.ToDelete)))
            buf.WriteString(fmt.Sprintf("Networks => Create: %d, Update: %d, Delete: %d\n", len(nplan.ToCreate), len(nplan.ToUpdate), len(nplan.ToDelete)))
            buf.WriteString(fmt.Sprintf("Storage Pools => Create: %d, Update: %d, Delete: %d\n", len(splan.ToCreate), len(splan.ToUpdate), len(splan.ToDelete)))
            ok, err := safety.Confirm(opts, os.Stdin, stdout, buf.String())
            if err != nil { return err }
            if !ok { return nil }
            // Apply in order: storage pools, networks, projects
            // Only delete networks/pools when --force is set.
            // Print per-resource progress lines for consistency.
            // Storage pools
            spCreated, spUpdated, spDeleted := 0, 0, 0
            for _, p := range splan.ToCreate {
                fmt.Fprintf(stdout, "[storage] create %s\n", p.Name)
                if err := client.CreateStoragePool(p); err != nil { return err }
                spCreated++
            }
            for _, u := range splan.ToUpdate {
                fmt.Fprintf(stdout, "[storage] update %s\n", u.Name)
                if err := client.UpdateStoragePool(incusapi.StoragePool{Name: u.Name, Config: u.DesiredConf}); err != nil { return err }
                spUpdated++
            }
            if opts.Force {
                for _, p := range splan.ToDelete {
                    fmt.Fprintf(stdout, "[storage] delete %s\n", p.Name)
                    if err := client.DeleteStoragePool(p.Name); err != nil { return err }
                    spDeleted++
                }
            }
            fmt.Fprintf(stdout, "storage_pools: created=%d updated=%d deleted=%d\n", spCreated, spUpdated, spDeleted)

            // Networks
            netCreated, netUpdated, netDeleted := 0, 0, 0
            for _, n := range nplan.ToCreate {
                fmt.Fprintf(stdout, "[networks] create %s\n", n.Name)
                if err := client.CreateNetwork(n); err != nil { return err }
                netCreated++
            }
            for _, u := range nplan.ToUpdate {
                fmt.Fprintf(stdout, "[networks] update %s\n", u.Name)
                if err := client.UpdateNetwork(incusapi.Network{Name: u.Name, Config: u.DesiredConf}); err != nil { return err }
                netUpdated++
            }
            if opts.Force {
                for _, n := range nplan.ToDelete {
                    fmt.Fprintf(stdout, "[networks] delete %s\n", n.Name)
                    if err := client.DeleteNetwork(n.Name); err != nil { return err }
                    netDeleted++
                }
            }
            fmt.Fprintf(stdout, "networks: created=%d updated=%d deleted=%d\n", netCreated, netUpdated, netDeleted)

            // Projects
            prCreated, prUpdated, prDeleted := 0, 0, 0
            for _, p := range plan.ToCreate {
                fmt.Fprintf(stdout, "[projects] create %s\n", p.Name)
                if err := client.CreateProject(p.Name, p.Config); err != nil { return err }
                prCreated++
            }
            for _, u := range plan.ToUpdate {
                fmt.Fprintf(stdout, "[projects] update %s\n", u.Name)
                if err := client.UpdateProject(u.Name, u.Desired); err != nil { return err }
                prUpdated++
            }
            for _, p := range plan.ToDelete {
                fmt.Fprintf(stdout, "[projects] delete %s\n", p.Name)
                if err := client.DeleteProject(p.Name); err != nil { return err }
                prDeleted++
            }
            fmt.Fprintf(stdout, "projects: created=%d updated=%d deleted=%d\n", prCreated, prUpdated, prDeleted)
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

func loadNetworks(path string) ([]incusapi.Network, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    var out []incusapi.Network
    if err := json.Unmarshal(b, &out); err != nil { return nil, err }
    return out, nil
}

func loadStoragePools(path string) ([]incusapi.StoragePool, error) {
    b, err := os.ReadFile(path)
    if err != nil { return nil, err }
    var out []incusapi.StoragePool
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

func renderNetworksPlan(w io.Writer, p cfg.NetworkPlan) {
    fmt.Fprintf(w, "Config preview (networks)\n")
    fmt.Fprintf(w, "Create: %d\n", len(p.ToCreate))
    for _, c := range p.ToCreate { fmt.Fprintf(w, "  + %s\n", c.Name) }
    fmt.Fprintf(w, "Update: %d\n", len(p.ToUpdate))
    for _, u := range p.ToUpdate { fmt.Fprintf(w, "  ~ %s\n", u.Name) }
    fmt.Fprintf(w, "Delete: %d\n", len(p.ToDelete))
    for _, d := range p.ToDelete { fmt.Fprintf(w, "  - %s\n", d.Name) }
}

func renderStoragePoolsPlan(w io.Writer, p cfg.StoragePoolPlan) {
    fmt.Fprintf(w, "Config preview (storage pools)\n")
    fmt.Fprintf(w, "Create: %d\n", len(p.ToCreate))
    for _, c := range p.ToCreate { fmt.Fprintf(w, "  + %s\n", c.Name) }
    fmt.Fprintf(w, "Update: %d\n", len(p.ToUpdate))
    for _, u := range p.ToUpdate { fmt.Fprintf(w, "  ~ %s\n", u.Name) }
    fmt.Fprintf(w, "Delete: %d\n", len(p.ToDelete))
    for _, d := range p.ToDelete { fmt.Fprintf(w, "  - %s\n", d.Name) }
}
