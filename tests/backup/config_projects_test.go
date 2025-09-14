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

func TestBackupProjects_WritesFiles(t *testing.T) {
    root := t.TempDir()
    fake := incusapi.NewFake()
    _ = fake.CreateProject("alpha", map[string]string{"features.images": "true"})
    _ = fake.CreateProject("beta", nil)

    now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
    dir, err := cfg.BackupProjects(fake, root, now)
    if err != nil {
        t.Fatalf("backup: %v", err)
    }
    // Expect directory path matches our timestamp under config
    wantDir := filepath.Join(root, "config", "20250102T030405Z")
    if dir != wantDir {
        t.Fatalf("dir = %q, want %q", dir, wantDir)
    }
    // Files exist
    for _, f := range []string{"projects.json", "manifest.json", "checksums.txt"} {
        if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
            t.Fatalf("missing %s: %v", f, err)
        }
    }
    // Projects JSON contains our projects in sorted order
    var projects []incusapi.Project
    b, _ := os.ReadFile(filepath.Join(dir, "projects.json"))
    if err := json.Unmarshal(b, &projects); err != nil {
        t.Fatalf("unmarshal projects: %v", err)
    }
    if len(projects) != 2 || projects[0].Name != "alpha" || projects[1].Name != "beta" {
        t.Fatalf("unexpected projects: %+v", projects)
    }
}

