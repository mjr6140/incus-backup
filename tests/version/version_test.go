package version_test

import (
    "testing"

    "incus-backup/src/version"
)

func TestVersionNonEmpty(t *testing.T) {
    if version.Version == "" {
        t.Fatalf("version string must not be empty")
    }
}

