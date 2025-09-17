//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	backendpkg "incus-backup/src/backend"
	"incus-backup/src/cli"
	restictest "incus-backup/tests/internal/restictest"
)

func TestResticListCommand(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	restictest.RequireBinary(t)
	repoParent := t.TempDir()
	repo := restictest.InitRepo(t, repoParent)
	t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

	proj := "itest-restic-list-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	instName := "rlinst"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", instName)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", instName).Run() })

	pool := "default"
	volName := "rlvol"
	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, volName, "size=16MiB")
	t.Cleanup(func() {
		_ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, volName).Run()
	})
	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, volName, instName, "/mnt/vol")
	run(t, "incus", "--project", proj, "exec", instName, "--", "sh", "-lc", "echo restic-list > /mnt/vol/marker.txt")
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, volName, instName)

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "config", "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("restic config backup failed: %v; stderr=%s", err, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "instances", instName, "--project", proj, "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("restic instance backup failed: %v; stderr=%s", err, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "volumes", pool + "/" + volName, "--project", proj, "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("restic volume backup failed: %v; stderr=%s", err, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"list", "--target", "restic:" + repo, "--output", "json"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("restic list failed: %v; stderr=%s", err, errBuf.String())
	}

	var entries []backendpkg.Entry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("parse list output: %v\n%s", err, out.String())
	}

	var foundConfig, foundInstance, foundVolume bool
	for _, e := range entries {
		switch e.Type {
		case "config":
			foundConfig = true
		case "instance":
			if e.Project == proj && e.Name == instName {
				foundInstance = true
			}
		case "volume":
			if e.Project == proj && e.Pool == pool && e.Name == volName {
				foundVolume = true
			}
		}
	}

	if !foundConfig || !foundInstance || !foundVolume {
		t.Fatalf("missing expected entries: config=%v instance=%v volume=%v\nentries=%#v", foundConfig, foundInstance, foundVolume, entries)
	}
}
