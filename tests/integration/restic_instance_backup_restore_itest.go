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
    restictest "incus-backup/tests/internal/restictest"
)

func TestResticInstanceBackupAndRestore(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" {
        t.Skip("INCUS_TESTS=1 not set")
    }

    restictest.RequireBinary(t)
    repoParent := t.TempDir()
    repo := restictest.InitRepo(t, repoParent)
    t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

    proj := "itest-restic-inst-" + time.Now().UTC().Format("20060102T150405")
    run(t, "incus", "project", "create", proj)
    t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

    instName := "ri1"
    run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", instName)
    t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", instName).Run() })
    run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "echo restic-ok > /root/marker.txt")

    var out, errBuf bytes.Buffer
    cmd := cli.NewRootCmd(&out, &errBuf)
    cmd.SetArgs([]string{"backup", "instances", instName, "--project", proj, "--target", "restic:" + repo})
    if _, err := cmd.ExecuteC(); err != nil {
        t.Fatalf("restic backup failed: %v; stderr=%s", err, errBuf.String())
    }

    run(t, "incus", "--project", proj, "delete", "--force", instName)

    out.Reset()
    errBuf.Reset()
    cmd = cli.NewRootCmd(&out, &errBuf)
    cmd.SetArgs([]string{"restore", "instance", instName, "--project", proj, "--target", "restic:" + repo})
    if _, err := cmd.ExecuteC(); err != nil {
        t.Fatalf("restic restore failed: %v; stderr=%s", err, errBuf.String())
    }

    output := run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "cat /root/marker.txt")
    if !strings.Contains(output, "restic-ok") {
        t.Fatalf("marker missing after restore; got %q", output)
    }
}

