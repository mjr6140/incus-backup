package cli_test

import (
	"context"
	"strings"
	"testing"

	"incus-backup/src/cli"
	"incus-backup/src/restic"
)

func TestResticTarget_NotImplementedButDetectsBinary(t *testing.T) {
	restore := cli.SetResticDetectorForTest(func(context.Context) (restic.BinaryInfo, error) {
		return restic.BinaryInfo{Path: "/bin/restic", Version: "0.18.2"}, nil
	})
	defer restore()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"list", "all", "--target", "restic:/repo"})
	if _, err := cmd.ExecuteC(); err == nil || !strings.Contains(err.Error(), "restic backend is not implemented yet") {
		t.Fatalf("expected not implemented error, got %v", err)
	}
	if warning := errBuf.String(); warning != "" {
		t.Fatalf("expected no warning for supported version; got %q", warning)
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
		return restic.BinaryInfo{Path: "/bin/restic", Version: "0.17.0"}, nil
	})
	defer restore()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"list", "all", "--target", "restic:/repo", "--yes"})
	if _, err := cmd.ExecuteC(); err == nil || !strings.Contains(err.Error(), "restic backend is not implemented yet") {
		t.Fatalf("expected not implemented error, got %v", err)
	}
	if !strings.Contains(errBuf.String(), "Warning: restic 0.17.0 detected") {
		t.Fatalf("expected warning message, got %q", errBuf.String())
	}
}
