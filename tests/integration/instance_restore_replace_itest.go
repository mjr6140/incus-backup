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

// Covers restoring over an existing instance using --replace.
func TestInstanceRestoreReplace_Alpine(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-repl-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    instName := "i1"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", instName)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", instName).Run() })

    // Create a marker file inside the instance
    run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "echo hello > /root/marker.txt")

    root := t.TempDir()

    // Backup the instance
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"backup", "instances", instName, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("backup: %v; stderr=%s", err, errb.String())
        }
    }

    // Remove the marker file to detect that restore replaced data
    run(t, "incus", "--project", proj, "exec", instName, "--", "rm", "-f", "/root/marker.txt")

    // Restore over the existing instance (replace)
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "instance", instName, "--project", proj, "--target", "dir:" + root, "--replace", "-y"})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore --replace: %v; stderr=%s", err, errb.String())
        }
    }

    // Verify the marker file is present again after restore
    out := run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "cat /root/marker.txt || true")
    if !strings.Contains(out, "hello") {
        t.Fatalf("expected restored marker file; got: %q", out)
    }
}

