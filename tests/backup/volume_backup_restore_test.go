package backup_test

import (
    "os"
    "path/filepath"
    "testing"
    "time"

    vol "incus-backup/src/backup/volumes"
    "incus-backup/src/incusapi"
)

func TestVolumeBackupAndRestore_WithFakeClient(t *testing.T) {
    root := t.TempDir()
    fake := incusapi.NewFake()
    if fake.Volumes["default"] == nil { fake.Volumes["default"] = map[string]map[string][]byte{} }
    if fake.Volumes["default"]["pool"] == nil { fake.Volumes["default"]["pool"] = map[string][]byte{} }
    fake.Volumes["default"]["pool"]["vol1"] = []byte("VOL-DATA")

    now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
    dir, err := vol.BackupVolume(fake, root, "default", "pool", "vol1", false, true, now, nil)
    if err != nil { t.Fatalf("backup volume: %v", err) }
    for _, f := range []string{"volume.tar.xz", "manifest.json", "checksums.txt"} {
        if _, err := os.Stat(filepath.Join(dir, f)); err != nil { t.Fatalf("missing %s: %v", f, err) }
    }
    // Restore to a new name
    if err := vol.RestoreVolume(fake, dir, "default", "pool", "vol1-copy", nil); err != nil { t.Fatalf("restore volume: %v", err) }
    if got := string(fake.Volumes["default"]["pool"]["vol1-copy"]); got != "VOL-DATA" { t.Fatalf("unexpected restored content: %q", got) }
}

