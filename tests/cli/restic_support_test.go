package cli_test

import (
	"context"
	"strings"
	"testing"

	backendrestic "incus-backup/src/backend/restic"
	"incus-backup/src/cli"
	"incus-backup/src/restic"
)

func TestResticTarget_ListSuccess(t *testing.T) {
	restore := cli.SetResticDetectorForTest(func(context.Context) (restic.BinaryInfo, error) {
		return restic.BinaryInfo{Path: "/bin/echo", Version: "0.18.2"}, nil
	})
	defer restore()
	resetSnaps := backendrestic.SetListSnapshotsForTest(func(context.Context, restic.BinaryInfo, string, []string) ([]restic.Snapshot, error) {
		return nil, nil
	})
	defer resetSnaps()
	resetCfg := backendrestic.SetListConfigTimestampsForTest(func(context.Context, restic.BinaryInfo, string) ([]string, error) {
		return nil, nil
	})
	defer resetCfg()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"list", "all", "--target", "restic:/repo", "--output", "json"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("expected list to succeed, got %v", err)
	}
	if warning := errBuf.String(); warning != "" {
		t.Fatalf("expected no warning for supported version; got %q", warning)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Fatalf("expected empty JSON array output, got %q", out.String())
	}
}

func TestResticTarget_VersionPromptAbort(t *testing.T) {
	restore := cli.SetResticDetectorForTest(func(context.Context) (restic.BinaryInfo, error) {
		return restic.BinaryInfo{Path: "/bin/restic", Version: "0.17.0"}, nil
	})
	defer restore()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"list", "all", "--target", "restic:/repo"})
	cmd.SetIn(strings.NewReader("n\n"))
	if _, err := cmd.ExecuteC(); err == nil || !strings.Contains(err.Error(), "aborted: restic version is below supported minimum") {
		t.Fatalf("expected abort error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Warning: restic 0.17.0 detected") {
		t.Fatalf("expected warning message, got %q", errBuf.String())
	}
}

func TestResticTarget_VersionPromptAutoYes(t *testing.T) {
	restore := cli.SetResticDetectorForTest(func(context.Context) (restic.BinaryInfo, error) {
		return restic.BinaryInfo{Path: "/bin/echo", Version: "0.17.0"}, nil
	})
	defer restore()
	resetSnaps := backendrestic.SetListSnapshotsForTest(func(context.Context, restic.BinaryInfo, string, []string) ([]restic.Snapshot, error) {
		return nil, nil
	})
	defer resetSnaps()
	resetCfg := backendrestic.SetListConfigTimestampsForTest(func(context.Context, restic.BinaryInfo, string) ([]string, error) {
		return nil, nil
	})
	defer resetCfg()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"list", "all", "--target", "restic:/repo", "--yes"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("expected list to succeed after --yes, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Warning: restic 0.17.0 detected") {
		t.Fatalf("expected warning message, got %q", errBuf.String())
	}
}
