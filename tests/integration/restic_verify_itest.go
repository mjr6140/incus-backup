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

type verifyResultJSON struct {
	Type      string              `json:"type"`
	Project   string              `json:"project,omitempty"`
	Pool      string              `json:"pool,omitempty"`
	Name      string              `json:"name,omitempty"`
	Timestamp string              `json:"timestamp"`
	Status    string              `json:"status"`
	Files     []map[string]string `json:"files"`
}

func TestResticVerifyCommand(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	restictest.RequireBinary(t)
	repoParent := t.TempDir()
	repo := restictest.InitRepo(t, repoParent)
	t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

	proj := "itest-restic-verify-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	inst := "rvinst"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })
	run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo verify > /root/payload.txt")

	pool := "default"
	vol := "rvvol"
	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, vol, "size=16MiB")
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, vol).Run() })
	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, vol, inst, "/mnt/vol")
	run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo restic > /mnt/vol/payload.txt")
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, vol, inst)

	var out, errBuf bytes.Buffer

	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "config", "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("backup config failed: %v; stderr=%s", err, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "instances", inst, "--project", proj, "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("backup instance failed: %v; stderr=%s", err, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"backup", "volumes", pool + "/" + vol, "--project", proj, "--target", "restic:" + repo})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("backup volume failed: %v; stderr=%s", err, errBuf.String())
	}

	out.Reset()
	errBuf.Reset()
	cmd = cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"verify", "--target", "restic:" + repo, "--output", "json"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("verify failed: %v; stderr=%s", err, errBuf.String())
	}

	var results []verifyResultJSON
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("parse verify json: %v\n%s", err, out.String())
	}

	ensure := func(cond bool, format string, args ...interface{}) {
		if !cond {
			t.Fatalf(format, args...)
		}
	}

	ensure(len(results) >= 3, "expected at least three results, got %d", len(results))
	found := map[string]bool{}
	for _, res := range results {
		switch res.Type {
		case backendpkg.KindConfig:
			if res.Status == "ok" {
				found["config"] = true
			}
		case backendpkg.KindInstance:
			if res.Project == proj && res.Name == inst && res.Status == "ok" {
				found["instance"] = true
			}
		case backendpkg.KindVolume:
			if res.Project == proj && res.Pool == pool && res.Name == vol && res.Status == "ok" {
				found["volume"] = true
			}
		}
	}

	ensure(found["config"], "missing config verification result")
	ensure(found["instance"], "missing instance verification result")
	ensure(found["volume"], "missing volume verification result")
}
