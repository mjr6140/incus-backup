//go:build integration

package restictest

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"incus-backup/src/restic"
)

const TestPassword = "incus-itest"

// RequireBinary ensures restic is available and meets the minimum supported version.
func RequireBinary(t testing.TB) restic.BinaryInfo {
	t.Helper()
	info, err := restic.Detect(context.Background())
	if err != nil {
		t.Fatalf("restic detection failed: %v", err)
	}
	if !restic.IsCompatible(info.Version) {
		t.Fatalf("restic version %s is below required %s", info.Version, restic.RequiredVersion)
	}
	return info
}

// InitRepo creates a new restic repository under dir (usually t.TempDir()).
func InitRepo(t testing.TB, dir string) string {
	t.Helper()
	RequireBinary(t)
	repo := filepath.Join(dir, "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir restic repo: %v", err)
	}
	cmd := exec.Command("restic", "init")
	cmd.Env = append(os.Environ(), fmt.Sprintf("RESTIC_PASSWORD=%s", TestPassword), fmt.Sprintf("RESTIC_REPOSITORY=%s", repo))
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restic init failed: %v\n%s", err, string(out))
	}
	return repo
}

// Command returns an exec.Cmd pre-configured with repository/password environment.
func Command(repo string, name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RESTIC_PASSWORD=%s", TestPassword),
		fmt.Sprintf("RESTIC_REPOSITORY=%s", repo),
	)
	return cmd
}
