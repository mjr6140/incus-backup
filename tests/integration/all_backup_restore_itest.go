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

// End-to-end: backup all → delete resources → restore all → verify markers.
func TestBackupAllAndRestoreAll_EndToEnd(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-all-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    pool := "default"
    inst := "iall"
    vol := "vall"

    // Create resources
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })
    run(t, "incus", "--project", proj, "storage", "volume", "create", pool, vol, "size=16MiB")
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, vol).Run() })

    // Write instance and volume markers
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

    // Restore all (apply config too, though config drift is minimal here)
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "all", "--project", proj, "--target", "dir:" + root, "--apply-config", "-y"})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("restore all: %v; stderr=%s", err, errb.String()) }
    }

    // Verify instance marker restored
    out1 := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /root/inst_marker.txt || true")
    if !strings.Contains(out1, "inst-ok") { t.Fatalf("expected inst marker restored; got: %q", out1) }

    // Verify volume marker restored (attach to instance)
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, vol, inst, "/mnt/vol")
    out2 := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /mnt/vol/vol_marker.txt || true")
    if !strings.Contains(out2, "vol-ok") { t.Fatalf("expected vol marker restored; got: %q", out2) }
}

