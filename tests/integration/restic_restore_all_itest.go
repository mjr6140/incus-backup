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

func TestResticRestoreAll(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	restictest.RequireBinary(t)
	repoParent := t.TempDir()
	repo := restictest.InitRepo(t, repoParent)
	t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

	proj := "itest-restic-all-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	instName := "rall-inst"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", instName)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", instName).Run() })
	run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "echo restic-restore-all > /root/marker.txt")

	pool := "default"
	volName := "rall-vol"
	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volName, "size=16MiB")
	t.Cleanup(func() {
		_ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volName).Run()
	})
	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, instName, "/mnt/rall")
	run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "echo restic-volume-all > /mnt/rall/marker.txt")
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, instName)

	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"backup", "all", "--project", proj, "--target", "restic:" + repo})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("restic backup all failed: %v; stderr=%s", err, errBuf.String())
		}
	}

	run(t, "incus", "--project", proj, "delete", "--force", instName)
	run(t, "incus", "--project", proj, "storage", "volume", "delete", pool, volName)

	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"restore", "all", "--project", proj, "--target", "restic:" + repo, "--yes"})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("restic restore all failed: %v; stderr=%s", err, errBuf.String())
		}
	}

	output := run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "cat /root/marker.txt")
	if !strings.Contains(output, "restic-restore-all") {
		t.Fatalf("instance marker missing after restore; got %q", output)
	}

	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, instName, "/mnt/rall")
	volContent := run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "cat /mnt/rall/marker.txt")
	if !strings.Contains(volContent, "restic-volume-all") {
		t.Fatalf("volume marker missing after restore; got %q", volContent)
	}
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, instName)
}
