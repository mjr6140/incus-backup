package cli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"incus-backup/src/cli"
)

func TestVerifyCmd_TableIncludesFileDetails(t *testing.T) {
	root := t.TempDir()
	snapDir := filepath.Join(root, "instances", "default", "web", "20250101T010101Z")
	mustMkdirAll(t, snapDir)

	manifestSum := writeFileWithHash(t, filepath.Join(snapDir, "manifest.json"), "{\"name\":\"web\"}\n")
	dataSum := writeFileWithHash(t, filepath.Join(snapDir, "instance.tar.xz"), "binary-data")
	writeChecksums(t, filepath.Join(snapDir, "checksums.txt"), map[string]string{
		"manifest.json":   manifestSum,
		"instance.tar.xz": dataSum,
	})

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"verify", "instances", "--target", "dir:" + root})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("verify command failed: %v; stderr=%s", err, errBuf.String())
	}

	output := out.String()
	if !strings.Contains(output, "TYPE") || !strings.Contains(output, "STATUS") {
		t.Fatalf("expected table header in output; got:\n%s", output)
	}
	if !strings.Contains(output, "instance") || !strings.Contains(output, "default") {
		t.Fatalf("expected summary row for instance backup; got:\n%s", output)
	}
	if !strings.Contains(output, "- manifest.json: ok") {
		t.Fatalf("expected per-file ok detail; got:\n%s", output)
	}
}

func TestVerifyCmd_JSONReportsMismatchDetails(t *testing.T) {
	root := t.TempDir()
	snapDir := filepath.Join(root, "instances", "default", "db", "20250102T020202Z")
	mustMkdirAll(t, snapDir)

	manifestPath := filepath.Join(snapDir, "manifest.json")
	origSum := writeFileWithHash(t, manifestPath, "initial")
	dataSum := writeFileWithHash(t, filepath.Join(snapDir, "instance.tar.xz"), "xyz")
	writeChecksums(t, filepath.Join(snapDir, "checksums.txt"), map[string]string{
		"manifest.json":   origSum,
		"instance.tar.xz": dataSum,
	})

	// Corrupt the manifest after checksums are written to trigger mismatch.
	if err := os.WriteFile(manifestPath, []byte("mutated"), 0o644); err != nil {
		t.Fatalf("mutate manifest: %v", err)
	}

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"verify", "instances", "--target", "dir:" + root, "--output", "json"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("verify command failed: %v; stderr=%s", err, errBuf.String())
	}

	var results []struct {
		Status string `json:"status"`
		Files  []struct {
			Name     string `json:"name"`
			Status   string `json:"status"`
			Expected string `json:"expected"`
			Actual   string `json:"actual"`
			Error    string `json:"error"`
		} `json:"files"`
	}
	if err := json.Unmarshal(out.Bytes(), &results); err != nil {
		t.Fatalf("unmarshal verify json: %v\n%s", err, out.String())
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != "mismatch" {
		t.Fatalf("expected mismatch status, got %s", results[0].Status)
	}
	var found bool
	for _, f := range results[0].Files {
		if f.Name != "manifest.json" {
			continue
		}
		if f.Status != "mismatch" {
			t.Fatalf("expected mismatch detail, got %+v", f)
		}
		if f.Expected == f.Actual {
			t.Fatalf("expected differing hashes, got expected=%s actual=%s", f.Expected, f.Actual)
		}
		found = true
	}
	if !found {
		t.Fatalf("manifest.json detail missing: %+v", results[0].Files)
	}
}

func writeChecksums(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	var builder strings.Builder
	keys := make([]string, 0, len(entries))
	for k := range entries {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		fmt.Fprintf(&builder, "%s  %s\n", entries[name], name)
	}
	if err := os.WriteFile(path, []byte(builder.String()), 0o644); err != nil {
		t.Fatalf("write checksums: %v", err)
	}
}

func writeFileWithHash(t *testing.T, path, data string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for file: %v", err)
	}
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	sum := sha256.Sum256([]byte(data))
	return hex.EncodeToString(sum[:])
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir -p %s: %v", path, err)
	}
}
