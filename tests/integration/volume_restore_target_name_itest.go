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

// Restores volume under a new name while original still exists; verifies both.
func TestVolumeRestore_TargetName(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-volname-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    pool := "default"
    v1 := "volsrc"
    v2 := "volcopy"
    run(t, "incus", "--project", proj, "storage", "volume", "create", pool, v1, "size=16MiB")
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, v1).Run() })

    inst := "ivol2"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })
    // Write marker to source
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, v1, inst, "/mnt/src")
    run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo copyme > /mnt/src/marker.txt")
    run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, v1, inst)

    root := t.TempDir()
    // Backup source volume
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"backup", "volumes", pool+"/"+v1, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("backup volumes: %v; stderr=%s", err, errb.String()) }
    }

    // Restore to a new name while original exists
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, v2).Run() })
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "volume", pool+"/"+v1, "--project", proj, "--target", "dir:" + root, "--target-name", v2})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("restore volume target-name: %v; stderr=%s", err, errb.String()) }
    }

    // Attach both and verify markers
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, v1, inst, "/mnt/src")
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, v2, inst, "/mnt/copy")
    out1 := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /mnt/src/marker.txt || true")
    out2 := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /mnt/copy/marker.txt || true")
    if !strings.Contains(out1, "copyme") || !strings.Contains(out2, "copyme") {
        t.Fatalf("expected both volumes to contain marker; got src=%q copy=%q", out1, out2)
    }
}

