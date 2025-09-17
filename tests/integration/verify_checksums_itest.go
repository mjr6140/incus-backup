//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"incus-backup/src/cli"
)

func TestVerifyDetectsCorruptedSnapshot(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	proj := "itest-verify-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	inst := "iverify"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })

	root := t.TempDir()

	// Create a marker file so the backup contains data.
	run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo verify-ok > /root/marker.txt")

	// Run backup all to capture instance + config state.
	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"backup", "all", "--project", proj, "--target", "dir:" + root})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("backup all failed: %v; stderr=%s", err, errBuf.String())
		}
	}

	// Initial verify should report ok status.
	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"verify", "all", "--target", "dir:" + root})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("verify all failed: %v; stderr=%s", err, errBuf.String())
		}
		if !strings.Contains(out.String(), "ok") {
			t.Fatalf("expected ok status; got:\n%s", out.String())
		}
	}

	instSnap := latestInstanceSnapshot(t, root, proj, inst)
	manifestPath := filepath.Join(instSnap, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte("corrupted"), 0o644); err != nil {
		t.Fatalf("corrupt manifest: %v", err)
	}

	// Verify again and expect mismatch with per-file detail.
	{
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"verify", "instances", "--target", "dir:" + root, "--output", "json"})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("verify instances failed: %v; stderr=%s", err, errBuf.String())
		}

		var results []struct {
			Project string `json:"project"`
			Name    string `json:"name"`
			Status  string `json:"status"`
			Files   []struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			} `json:"files"`
		}
		if err := json.Unmarshal(out.Bytes(), &results); err != nil {
			t.Fatalf("unmarshal verify results: %v\n%s", err, out.String())
		}
		if len(results) == 0 {
			t.Fatalf("expected at least one verify result")
		}
		var found bool
		for _, r := range results {
			if r.Project == proj && r.Name == inst {
				if r.Status != "mismatch" {
					t.Fatalf("expected mismatch status; got %s", r.Status)
				}
				for _, f := range r.Files {
					if f.Name == "manifest.json" {
						if f.Status != "mismatch" {
							t.Fatalf("expected manifest mismatch detail; got %s", f.Status)
						}
						found = true
						break
					}
				}
			}
		}
		if !found {
			t.Fatalf("did not find mismatch detail for manifest.json: %+v", results)
		}
	}
}

func latestInstanceSnapshot(t *testing.T, root, project, name string) string {
	t.Helper()
	base := filepath.Join(root, "instances", project, name)
	entries, err := os.ReadDir(base)
	if err != nil {
		t.Fatalf("read snapshots for %s/%s: %v", project, name, err)
	}
	var timestamps []string
	for _, e := range entries {
		if e.IsDir() {
			timestamps = append(timestamps, e.Name())
		}
	}
	if len(timestamps) == 0 {
		t.Fatalf("no snapshots recorded for %s/%s", project, name)
	}
	sort.Strings(timestamps)
	return filepath.Join(base, timestamps[len(timestamps)-1])
}
