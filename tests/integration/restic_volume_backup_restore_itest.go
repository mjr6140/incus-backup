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

func TestResticVolumeBackupAndRestore(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	restictest.RequireBinary(t)
	repoParent := t.TempDir()
	repo := restictest.InitRepo(t, repoParent)
	t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

	proj := "itest-restic-vol-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	inst := "rv1"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })

	pool := "default"
	volName := "rv-data"
	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volName, "size=16MiB")
	t.Cleanup(func() {
		_ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volName).Run()
	})
	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
	run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo restic-vol > /mnt/vol/marker.txt")
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, inst)

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "volumes", pool + "/" + volName, "--project", proj, "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("restic volume backup failed: %v; stderr=%s", err, errBuf.String())
	}

	run(t, "incus", "--project", proj, "storage", "volume", "delete", pool, volName)

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"restore", "volume", pool + "/" + volName, "--project", proj, "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("restic volume restore failed: %v; stderr=%s", err, errBuf.String())
	}

	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, inst, "/mnt/vol")
	output := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "cat /mnt/vol/marker.txt")
	if !strings.Contains(output, "restic-vol") {
		t.Fatalf("restored volume missing marker; got %q", output)
	}
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, inst)
}
