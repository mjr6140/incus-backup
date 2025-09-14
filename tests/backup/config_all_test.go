package backup_test

import (
    "encoding/json"
    "os"
    "path/filepath"
    "testing"
    "time"

    cfg "incus-backup/src/backup/config"
    "incus-backup/src/incusapi"
)

func TestBackupAll_WritesProfiles(t *testing.T) {
    root := t.TempDir()
    fake := incusapi.NewFake()
    fake.ProfilesMap["default"] = incusapi.Profile{Name: "default", Description: "", Config: map[string]string{"limits.cpu": "1"}}

    now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
    dir, err := cfg.BackupAll(fake, root, now)
    if err != nil { t.Fatalf("backup all: %v", err) }
    // Files exist
    for _, f := range []string{"projects.json", "profiles.json", "manifest.json", "checksums.txt"} {
        if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
            t.Fatalf("missing %s: %v", f, err)
        }
    }
    // Profiles JSON contains our default profile
    var profiles []incusapi.Profile
    b, _ := os.ReadFile(filepath.Join(dir, "profiles.json"))
    if err := json.Unmarshal(b, &profiles); err != nil { t.Fatalf("unmarshal profiles: %v", err) }
    if len(profiles) != 1 || profiles[0].Name != "default" {
        t.Fatalf("unexpected profiles: %+v", profiles)
    }
}

