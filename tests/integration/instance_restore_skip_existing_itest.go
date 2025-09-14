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

// When --skip-existing is used, restore should not modify an existing instance.
func TestInstanceRestore_SkipExisting(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-skip-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    instName := "i1"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", instName)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", instName).Run() })

    // Write a marker after backup so we can detect whether restore changed the instance.
    root := t.TempDir()

    // Initial backup
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"backup", "instances", instName, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("backup: %v; stderr=%s", err, errb.String())
        }
    }

    // Modify the instance (create marker)
    run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "echo changed > /root/marker.txt")

    // Attempt restore with --skip-existing. Should succeed and not modify instance.
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "instance", instName, "--project", proj, "--target", "dir:" + root, "--skip-existing"})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore --skip-existing: %v; stderr=%s", err, errb.String())
        }
    }

    // Verify marker still present (restore did not overwrite)
    out := run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "cat /root/marker.txt || true")
    if !strings.Contains(out, "changed") {
        t.Fatalf("expected marker unchanged by skip-existing; got: %q", out)
    }
}

