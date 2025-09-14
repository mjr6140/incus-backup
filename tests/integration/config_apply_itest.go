//go:build integration

package integration

import (
    "bytes"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "testing"
    "time"

    "incus-backup/src/cli"
    "incus-backup/src/incusapi"
)

// TestConfigRestoreApply_Projects creates a project, backs up config, deletes
// the project, then applies restore and verifies the project is recreated.
func TestConfigRestoreApply_Projects(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" {
        t.Skip("INCUS_TESTS=1 not set; skipping integration test")
    }

    client, err := incusapi.ConnectLocal()
    if err != nil {
        t.Fatalf("connect incus: %v", err)
    }

    name := "itest-apply-" + time.Now().UTC().Format("20060102T150405")
    if err := client.CreateProject(name, map[string]string{"features.images": "true"}); err != nil {
        t.Fatalf("create project: %v", err)
    }
    // Ensure cleanup if apply path fails
    t.Cleanup(func() { _ = client.DeleteProject(name) })

    root := t.TempDir()

    // Backup config
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"backup", "config", "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("backup config: %v; stderr=%s", err, stderr.String())
        }
    }
    // Validate snapshot exists
    cfgDir := filepath.Join(root, "config")
    entries, err := os.ReadDir(cfgDir)
    if err != nil { t.Fatalf("read config dir: %v", err) }
    var snaps []string
    for _, e := range entries {
        if e.IsDir() && !strings.HasPrefix(e.Name(), ".") { snaps = append(snaps, e.Name()) }
    }
    if len(snaps) == 0 { t.Fatalf("no config snapshots under %s", cfgDir) }
    sort.Strings(snaps)
    latest := snaps[len(snaps)-1]
    for _, f := range []string{"projects.json", "manifest.json", "checksums.txt"} {
        if _, err := os.Stat(filepath.Join(cfgDir, latest, f)); err != nil {
            t.Fatalf("missing %s: %v", f, err)
        }
    }

    // Delete the project to create drift
    if err := client.DeleteProject(name); err != nil {
        t.Fatalf("delete project: %v", err)
    }

    // Apply restore (non-interactive)
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"restore", "config", "--target", "dir:" + root, "--apply", "-y"})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore config --apply: %v; stderr=%s", err, stderr.String())
        }
        if !strings.Contains(out.String(), "projects: created=") {
            t.Fatalf("expected apply summary in output; got: %s", out.String())
        }
    }

    // Verify project exists again and config was applied
    projects, err := client.ListProjects()
    if err != nil { t.Fatalf("list projects: %v", err) }
    var cfg map[string]string
    for _, p := range projects { if p.Name == name { cfg = p.Config; break } }
    if cfg == nil { t.Fatalf("expected project %s to be recreated", name) }
    if cfg["features.images"] != "true" {
        t.Fatalf("expected features.images=true after apply; got %v", cfg)
    }
}
