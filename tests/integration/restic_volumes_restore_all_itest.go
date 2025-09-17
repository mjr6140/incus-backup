//go:build integration

package integration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"incus-backup/src/cli"
	restictest "incus-backup/tests/internal/restictest"
)

func TestResticRestoreAllVolumesUsesCorrectSnapshots(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	restictest.RequireBinary(t)
	repoParent := t.TempDir()
	repo := restictest.InitRepo(t, repoParent)
	t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

	proj := "itest-restic-volall-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	inst := "rvall-host"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })

	pool := "default"
	volA := "rvall-a"
	volB := "rvall-b"

	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volA, "size=16MiB")
	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volB, "size=16MiB")
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volA).Run() })
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volB).Run() })

	writeMarker := func(vol, mount, marker string) {
		run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, vol, inst, mount)
		run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", fmt.Sprintf("echo %s > %s/marker.txt", marker, mount))
		run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, vol, inst)
	}

	writeMarker(volA, "/mnt/volA", "marker-A")
	writeMarker(volB, "/mnt/volB", "marker-B")

	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"backup", "volumes", pool + "/" + volA, pool + "/" + volB, "--project", proj, "--target", "restic:" + repo})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("restic backup volumes failed: %v; stderr=%s", err, errBuf.String())
		}
	}

	// Remove both volumes before restore.
	run(t, "incus", "--project", proj, "storage", "volume", "delete", pool, volA)
	run(t, "incus", "--project", proj, "storage", "volume", "delete", pool, volB)

	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"restore", "volumes", "--project", proj, "--target", "restic:" + repo, "--yes"})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("restic restore volumes failed: %v; stderr=%s", err, errBuf.String())
		}
	}

	checkMarker := func(vol, mount, marker string) {
		run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, vol, inst, mount)
		out := run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", fmt.Sprintf("cat %s/marker.txt", mount))
		if !strings.Contains(out, marker) {
			t.Fatalf("volume %s missing marker %s; got %q", vol, marker, out)
		}
		run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, vol, inst)
	}

	checkMarker(volA, "/mnt/volA", "marker-A")
	checkMarker(volB, "/mnt/volB", "marker-B")
}
