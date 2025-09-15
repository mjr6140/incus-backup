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

// Restores a volume with --replace and verifies original data is restored.
func TestVolumeRestore_Replace(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-volrepl-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    pool := "default"
    volName := "vrepl"
    run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volName, "size=16MiB")
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volName).Run() })

    inst := "ivolrepl"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })

    // Attach and write original marker
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
    run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo original > /mnt/vol/marker.txt")
    run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, inst)

    root := t.TempDir()
    // Backup the volume
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"backup", "volumes", pool+"/"+volName, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("backup volumes: %v; stderr=%s", err, errb.String()) }
    }

    // Change content so we can verify replace actually restores the old data
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
    run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo changed > /mnt/vol/marker.txt")
    run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, inst)

    // Restore with --replace (ensure not attached while deleting)
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "volume", pool+"/"+volName, "--project", proj, "--target", "dir:" + root, "--replace", "-y"})
        if _, err := cmd.ExecuteC(); err != nil { t.Fatalf("restore volume --replace: %v; stderr=%s", err, errb.String()) }
    }

    // Attach and verify original marker restored
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
    out := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /mnt/vol/marker.txt || true")
    if !strings.Contains(out, "original") {
        t.Fatalf("expected original marker restored; got: %q", out)
    }
}

