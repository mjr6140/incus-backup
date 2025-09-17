package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"incus-backup/src/cli"
)

func TestPruneCmd_RemovesOldSnapshots(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "instances", "default", "web", "20240101T010101Z"))
	mustMkdirAll(t, filepath.Join(root, "instances", "default", "web", "20240202T020202Z"))

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"prune", "instances", "--target", "dir:" + root, "--keep", "1", "-y"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("prune command failed: %v; stderr=%s", err, errBuf.String())
	}

	if _, err := os.Stat(filepath.Join(root, "instances", "default", "web", "20240101T010101Z")); !os.IsNotExist(err) {
		t.Fatalf("expected oldest snapshot removed; stat err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "instances", "default", "web", "20240202T020202Z")); err != nil {
		t.Fatalf("expected newest snapshot retained; stat err=%v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("delete")) {
		t.Fatalf("expected delete preview in output; got:\n%s", out.String())
	}
}

func TestPruneCmd_DryRunDoesNotDelete(t *testing.T) {
	root := t.TempDir()
	mustMkdirAll(t, filepath.Join(root, "instances", "default", "db", "20240101T010101Z"))
	mustMkdirAll(t, filepath.Join(root, "instances", "default", "db", "20240202T020202Z"))

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"prune", "instances", "--target", "dir:" + root, "--keep", "1", "--dry-run"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("prune command failed: %v; stderr=%s", err, errBuf.String())
	}

	for _, ts := range []string{"20240101T010101Z", "20240202T020202Z"} {
		if _, err := os.Stat(filepath.Join(root, "instances", "default", "db", ts)); err != nil {
			t.Fatalf("expected snapshot %s to remain after dry-run: %v", ts, err)
		}
	}
	if !bytes.Contains(out.Bytes(), []byte("delete")) {
		t.Fatalf("expected preview of deletions even in dry-run; got:\n%s", out.String())
	}
}
