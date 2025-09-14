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

// Restores to a new instance name when the original still exists.
func TestInstanceRestore_TargetName(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-tname-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    instName := "i1"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", instName)
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", instName).Run() })

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

    // Restore to a new name while original exists
    copyName := instName + "-copy"
    t.Cleanup(func(){ _ = exec.Command("incus", "--project", proj, "delete", "--force", copyName).Run() })
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "instance", instName, "--project", proj, "--target", "dir:" + root, "--target-name", copyName})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore --target-name: %v; stderr=%s", err, errb.String())
        }
    }

    // Ensure both names exist
    got := run(t, "incus", "--project", proj, "list", "-c", "n", "--format", "csv")
    if !strings.Contains(got, instName) || !strings.Contains(got, copyName) {
        t.Fatalf("expected both %s and %s listed; got %s", instName, copyName, got)
    }
}

