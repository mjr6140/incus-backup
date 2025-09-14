package backend_test

import (
    "os"
    "path/filepath"
    "testing"

    dir "incus-backup/src/backend/directory"
)

func TestDirectory_List_AllKinds(t *testing.T) {
    root := t.TempDir()

    mustMkdirAll(t, filepath.Join(root, "instances", "default", "vm1", "20250101T010101Z"))
    mustMkdirAll(t, filepath.Join(root, "volumes", "default", "pool1", "volA", "20250102T020202Z"))
    mustMkdirAll(t, filepath.Join(root, "images", "abc123", "20250103T030303Z"))
    mustMkdirAll(t, filepath.Join(root, "config", "20250104T040404Z"))

    b, err := dir.New(root)
    if err != nil {
        t.Fatalf("new backend: %v", err)
    }
    entries, err := b.List("")
    if err != nil {
        t.Fatalf("list: %v", err)
    }
    if len(entries) != 4 {
        t.Fatalf("got %d entries, want 4", len(entries))
    }
}

func mustMkdirAll(t *testing.T, path string) {
    t.Helper()
    if err := os.MkdirAll(path, 0o755); err != nil {
        t.Fatalf("mkdir -p %s: %v", path, err)
    }
}

