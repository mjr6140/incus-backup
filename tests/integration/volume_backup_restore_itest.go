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

// Creates a custom volume, writes a marker via an Alpine instance, backs up,
// deletes the volume, restores it, and verifies the marker content.
func TestVolumeBackupAndRestore_RoundTrip(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-vol-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    // Use default pool name "default" which is present in our container init.
    pool := "default"
    volName := "v1"

    // Create a small volume and an instance to attach it to.
    run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volName, "size=16MiB")
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volName).Run() })

    inst := "ivol"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })

    // Attach, write marker, detach
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
    run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo hello-volume > /mnt/vol/marker.txt")
    run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, inst)

    root := t.TempDir()
    // Backup the volume via CLI
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"backup", "volumes", pool+"/"+volName, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("backup volumes: %v; stderr=%s", err, errb.String())
        }
    }

    // Delete the volume
    run(t, "incus", "--project", proj, "storage", "volume", "delete", pool, volName)

    // Restore the volume via CLI
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "volume", pool+"/"+volName, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore volume: %v; stderr=%s", err, errb.String())
        }
    }

    // Attach restored volume and verify marker
    run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
    out := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /mnt/vol/marker.txt || true")
    if !strings.Contains(out, "hello-volume") {
        t.Fatalf("expected marker restored; got: %q", out)
    }
}

