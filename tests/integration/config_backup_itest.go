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

// TestConfigBackupAndPreview exercises the CLI end-to-end for config backup and
// preview restore using a temporary Incus project. It requires INCUS_TESTS=1 and
// the containerized test harness or a local Incus daemon.
func TestConfigBackupAndPreview(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" {
        t.Skip("INCUS_TESTS=1 not set; skipping integration test")
    }

    client, err := incusapi.ConnectLocal()
    if err != nil {
        t.Fatalf("connect incus: %v", err)
    }

    name := "itest-config-" + time.Now().UTC().Format("20060102T150405")
    if err := client.CreateProject(name, map[string]string{}); err != nil {
        t.Fatalf("create project: %v", err)
    }
    t.Cleanup(func() { _ = client.DeleteProject(name) })

    root := t.TempDir()

    // Run: incus-backup backup config --target dir:<root>
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"backup", "config", "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("backup config: %v; stderr=%s", err, stderr.String())
        }
    }

    // Find latest config snapshot
    cfgDir := filepath.Join(root, "config")
    entries, err := os.ReadDir(cfgDir)
    if err != nil {
        t.Fatalf("read config dir: %v", err)
    }
    var snaps []string
    for _, e := range entries {
        if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
            snaps = append(snaps, e.Name())
        }
    }
    if len(snaps) == 0 {
        t.Fatalf("no config snapshots written under %s", cfgDir)
    }
    sort.Strings(snaps)
    latest := snaps[len(snaps)-1]
    snapPath := filepath.Join(cfgDir, latest)
    // Ensure expected files exist
    for _, f := range []string{"projects.json", "profiles.json", "manifest.json", "checksums.txt"} {
        if _, err := os.Stat(filepath.Join(snapPath, f)); err != nil {
            t.Fatalf("missing %s: %v", f, err)
        }
    }

    // Delete the project to create drift
    if err := client.DeleteProject(name); err != nil {
        t.Fatalf("delete project: %v", err)
    }
    // Now preview should propose creating it back.
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"restore", "config", "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore config preview: %v; stderr=%s", err, stderr.String())
        }
        s := out.String()
        if !strings.Contains(s, "Create:") || !strings.Contains(s, name) {
            t.Fatalf("expected preview to include create for %s; got:\n%s", name, s)
        }
    }
}
