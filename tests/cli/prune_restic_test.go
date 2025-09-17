package cli_test

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"incus-backup/src/cli"
	"incus-backup/src/restic"
)

func TestResticPrunePreviewDryRun(t *testing.T) {
	bin := restic.BinaryInfo{Path: "/bin/echo", Version: restic.RequiredVersion}

	restoreDetector := cli.SetResticDetectorForTest(func(context.Context) (restic.BinaryInfo, error) {
		return bin, nil
	})
	defer restoreDetector()

	restoreList := cli.SetResticPruneListSnapshotsForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, tags []string) ([]restic.Snapshot, error) {
		for _, tag := range tags {
			if tag == "type=instance" {
				return []restic.Snapshot{
					snapshotWithTags("old-data", map[string]string{"type": "instance", "part": "data", "project": "alpha", "name": "vm1", "timestamp": "20240101T000000Z"}),
					snapshotWithTags("old-manifest", map[string]string{"type": "instance", "part": "manifest", "project": "alpha", "name": "vm1", "timestamp": "20240101T000000Z"}),
					snapshotWithTags("new-data", map[string]string{"type": "instance", "part": "data", "project": "alpha", "name": "vm1", "timestamp": "20240102T000000Z"}),
				}, nil
			}
		}
		return nil, nil
	})
	defer restoreList()

	calledForget := false
	restoreForget := cli.SetResticPruneForgetForTest(func(context.Context, restic.BinaryInfo, string, []string, bool) error {
		calledForget = true
		return nil
	})
	defer restoreForget()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"prune", "instances", "--target", "restic:/repo", "--keep", "1", "--dry-run"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("prune failed: %v", err)
	}
	if calledForget {
		t.Fatalf("forget should not be called during dry-run")
	}
	if !strings.Contains(out.String(), "20240101T000000Z") {
		t.Fatalf("expected preview to include old timestamp, got %q", out.String())
	}
}

func TestResticPruneDeletesOldSnapshots(t *testing.T) {
	bin := restic.BinaryInfo{Path: "/bin/echo", Version: restic.RequiredVersion}

	restoreDetector := cli.SetResticDetectorForTest(func(context.Context) (restic.BinaryInfo, error) {
		return bin, nil
	})
	defer restoreDetector()

	restoreList := cli.SetResticPruneListSnapshotsForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, tags []string) ([]restic.Snapshot, error) {
		for _, tag := range tags {
			if tag == "type=instance" {
				return []restic.Snapshot{
					snapshotWithTags("old-data", map[string]string{"type": "instance", "part": "data", "project": "alpha", "name": "vm1", "timestamp": "20240101T000000Z"}),
					snapshotWithTags("old-manifest", map[string]string{"type": "instance", "part": "manifest", "project": "alpha", "name": "vm1", "timestamp": "20240101T000000Z"}),
					snapshotWithTags("old-checksums", map[string]string{"type": "instance", "part": "checksums", "project": "alpha", "name": "vm1", "timestamp": "20240101T000000Z"}),
					snapshotWithTags("new-data", map[string]string{"type": "instance", "part": "data", "project": "alpha", "name": "vm1", "timestamp": "20240102T000000Z"}),
					snapshotWithTags("new-manifest", map[string]string{"type": "instance", "part": "manifest", "project": "alpha", "name": "vm1", "timestamp": "20240102T000000Z"}),
					snapshotWithTags("new-checksums", map[string]string{"type": "instance", "part": "checksums", "project": "alpha", "name": "vm1", "timestamp": "20240102T000000Z"}),
				}, nil
			}
		}
		return nil, nil
	})
	defer restoreList()

	var receivedIDs []string
	var receivedPrune bool
	restoreForget := cli.SetResticPruneForgetForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, ids []string, prune bool) error {
		receivedIDs = append([]string(nil), ids...)
		receivedPrune = prune
		return nil
	})
	defer restoreForget()

	var out, errBuf strings.Builder
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"prune", "instances", "--target", "restic:/repo", "--keep", "1", "--yes"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("prune failed: %v\nstderr=%s", err, errBuf.String())
	}
	if len(receivedIDs) != 3 {
		t.Fatalf("expected 3 snapshot IDs to prune, got %d (%v)", len(receivedIDs), receivedIDs)
	}
	if !receivedPrune {
		t.Fatalf("expected prune flag to be true")
	}
}

func snapshotWithTags(id string, tags map[string]string) restic.Snapshot {
	var tagList []string
	for k, v := range tags {
		tagList = append(tagList, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(tagList)
	return restic.Snapshot{ID: id, Time: time.Unix(0, 0), Tags: tagList}
}
