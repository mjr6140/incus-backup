//go:build integration

package integration

import (
    "bytes"
    "os"
    "os/exec"
    "strings"
    "testing"
    "time"

    "incus-backup/src/cli"
)

// Dry-run: backup all, delete resources, run restore all --dry-run, verify no changes.
func TestRestoreAll_DryRun_NoChanges(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-all-dry-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    pool := "default"
    inst := "idry"
    vol := "vdry"

    // Create instance + volume and write markers
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })
    run(t, "incus", "--project", proj, "storage", "volume", "create", pool, vol, "size=16MiB")
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, vol).Run() })
    run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo inst-ok > /root/inst_marker.txt")
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, vol, inst, "/mnt/vol")
    run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo vol-ok > /mnt/vol/vol_marker.txt")
    run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, vol, inst)

    // Backup all
    root := t.TempDir()
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"backup", "all", "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("backup all: %v; stderr=%s", err, errb.String()) }
    }

    // Delete resources
    run(t, "incus", "--project", proj, "delete", "--force", inst)
    run(t, "incus", "--project", proj, "storage", "volume", "delete", pool, vol)

    // Restore all in dry-run mode
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "all", "--project", proj, "--target", "dir:" + root, "--dry-run"})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("restore all --dry-run: %v; stderr=%s", err, errb.String()) }
        // Optionally assert preview contains headings
        s := out.String()
        if !strings.Contains(s, "Config preview") || !strings.Contains(s, "ACTION\tPROJECT") {
            t.Fatalf("expected preview tables in output; got: %s", s)
        }
    }

    // Verify instance still absent
    if _, err := exec.Command("incus", "--project", proj, "list", "-c", "n", "--format", "csv").CombinedOutput(); err == nil {
        t.Fatalf("expected no instances present after dry-run restore")
    }
    // Verify volume still absent
    if _, err := exec.Command("incus", "--project", proj, "storage", "volume", "show", pool, vol).CombinedOutput(); err == nil {
        t.Fatalf("expected no volume present after dry-run restore")
    }
}

