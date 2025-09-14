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

func TestInstanceBackupAndRestore_Alpine(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" { t.Skip("INCUS_TESTS=1 not set") }

    proj := "itest-inst-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func(){ _ = exec.Command("incus", "project", "delete", proj).Run() })

    instName := "i1"
    // Launch a tiny container in the project
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

    // Delete the instance
    run(t, "incus", "--project", proj, "delete", "--force", instName)

    // Restore the instance from latest snapshot
    {
        var out, errb bytes.Buffer
        cmd := cli.NewRootCmd(&out, &errb)
        cmd.SetArgs([]string{"restore", "instance", instName, "--project", proj, "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore: %v; stderr=%s", err, errb.String())
        }
    }

    // Ensure instance exists again
    got := run(t, "incus", "--project", proj, "list", "-c", "n", "--format", "csv")
    if !strings.Contains(got, instName) {
        t.Fatalf("expected restored instance %s listed; got %s", instName, got)
    }
}

func run(t *testing.T, name string, args ...string) string {
    t.Helper()
    cmd := exec.Command(name, args...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        t.Fatalf("%s %v: %v\n%s", name, args, err, string(out))
    }
    return string(out)
}

