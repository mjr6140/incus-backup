package target_test

import (
    "strings"
    "testing"

    "incus-backup/src/target"
)

func TestParse_Dir_OK(t *testing.T) {
    got, err := target.Parse("dir:/mnt/nas/sysbackup/incus")
    if err != nil {
        t.Fatalf("Parse error: %v", err)
    }
    if got.Scheme != "dir" {
        t.Fatalf("scheme = %q, want dir", got.Scheme)
    }
    if got.DirPath == "" || !strings.HasPrefix(got.DirPath, "/mnt/") {
        t.Fatalf("DirPath = %q, want absolute under /mnt", got.DirPath)
    }
}

func TestParse_Dir_Root_OK(t *testing.T) {
    got, err := target.Parse("dir:/")
    if err != nil {
        t.Fatalf("Parse error: %v", err)
    }
    if got.DirPath != "/" {
        t.Fatalf("DirPath = %q, want /", got.DirPath)
    }
}

func TestParse_Invalid_Empty(t *testing.T) {
    if _, err := target.Parse(""); err == nil {
        t.Fatalf("expected error for empty target")
    }
}

func TestParse_Invalid_NoScheme(t *testing.T) {
    if _, err := target.Parse("/var/backups"); err == nil {
        t.Fatalf("expected error for missing scheme")
    }
}

func TestParse_Invalid_UnsupportedScheme(t *testing.T) {
    if _, err := target.Parse("s3:/repo"); err == nil {
        t.Fatalf("expected error for unsupported scheme")
    }
}

func TestParse_Restic_OK(t *testing.T) {
    got, err := target.Parse("restic:/var/backups/repo")
    if err != nil {
        t.Fatalf("Parse error: %v", err)
    }
    if got.Scheme != "restic" {
        t.Fatalf("scheme = %q, want restic", got.Scheme)
    }
    if got.Value != "/var/backups/repo" {
        t.Fatalf("value = %q, want /var/backups/repo", got.Value)
    }
}

func TestParse_Dir_Relative_Invalid(t *testing.T) {
    if _, err := target.Parse("dir:relative/path"); err == nil {
        t.Fatalf("expected error for relative path")
    }
}
