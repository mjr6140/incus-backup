package backup_test

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    inst "incus-backup/src/backup/instances"
    "incus-backup/src/incusapi"
)

func TestInstanceBackupAndRestore_WithFakeClient(t *testing.T) {
    root := t.TempDir()
    fake := incusapi.NewFake()
    // Seed fake export content for project default, instance web
    if fake.Instances["default"] == nil { fake.Instances["default"] = map[string][]byte{} }
    fake.Instances["default"]["web"] = []byte("EXPORT-WEB")

    now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
    dir, err := inst.BackupInstance(fake, root, "default", "web", false, true, now, nil)
    if err != nil { t.Fatalf("backup instance: %v", err) }

    // Files exist
    for _, f := range []string{"export.tar.xz", "manifest.json", "checksums.txt"} {
        if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
            t.Fatalf("missing %s: %v", f, err)
        }
    }

    // Restore to a new name
    if err := inst.RestoreInstance(fake, dir, "default", "web-restored", nil); err != nil {
        t.Fatalf("restore instance: %v", err)
    }
    // Verify fake has restored content
    if got := string(fake.Instances["default"]["web-restored"]); got != "EXPORT-WEB" {
        t.Fatalf("unexpected restored content: %q", got)
    }
}
