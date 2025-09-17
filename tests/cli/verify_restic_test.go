package cli_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"
	"time"

	backendpkg "incus-backup/src/backend"
	"incus-backup/src/cli"
	"incus-backup/src/restic"
)

func TestCollectResticVerifyResults_InstanceOK(t *testing.T) {
	ctx := context.Background()
	bin := restic.BinaryInfo{Path: "/bin/echo", Version: restic.RequiredVersion}

	dataContent := []byte("instance-data")
	manifestContent := []byte("instance-manifest")

	restoreList := cli.SetResticVerifyListSnapshotsForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, tags []string) ([]restic.Snapshot, error) {
		for _, tag := range tags {
			if tag == "type=instance" {
				return []restic.Snapshot{
					{ID: "snap-data", Time: time.Unix(0, 0), Tags: []string{"type=instance", "part=data", "project=alpha", "name=vm1", "timestamp=20240101T000000Z"}},
					{ID: "snap-manifest", Time: time.Unix(0, 0), Tags: []string{"type=instance", "part=manifest", "project=alpha", "name=vm1", "timestamp=20240101T000000Z"}},
					{ID: "snap-checksums", Time: time.Unix(0, 0), Tags: []string{"type=instance", "part=checksums", "project=alpha", "name=vm1", "timestamp=20240101T000000Z"}},
				}, nil
			}
		}
		return nil, nil
	})
	defer restoreList()

	restoreDump := cli.SetResticVerifyDumpForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, snapshotID string, path string, w io.Writer, _ io.Writer) error {
		switch snapshotID {
		case "snap-data":
			if path != "export.tar.xz" {
				t.Fatalf("unexpected data path %s", path)
			}
			_, err := w.Write(dataContent)
			return err
		case "snap-manifest":
			if path != "manifest.json" {
				t.Fatalf("unexpected manifest path %s", path)
			}
			_, err := w.Write(manifestContent)
			return err
		case "snap-checksums":
			if path != "checksums.txt" {
				t.Fatalf("unexpected checksum path %s", path)
			}
			sums := []string{
				hexHash(dataContent) + "  export.tar.xz",
				hexHash(manifestContent) + "  manifest.json",
			}
			_, err := w.Write([]byte(strings.Join(sums, "\n") + "\n"))
			return err
		default:
			t.Fatalf("unexpected snapshot %s", snapshotID)
			return nil
		}
	})
	defer restoreDump()

	results, err := cli.CollectResticVerifyResultsForTest(ctx, bin, "repo", backendpkg.KindInstance)
	if err != nil {
		t.Fatalf("collect results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	res := results[0]
	if res.Status != "ok" {
		t.Fatalf("expected status ok, got %s", res.Status)
	}
	if len(res.Files) != 3 {
		t.Fatalf("expected 3 file entries, got %d", len(res.Files))
	}
	statuses := map[string]string{}
	for _, f := range res.Files {
		statuses[f.Name] = f.Status
	}
	if statuses["export.tar.xz"] != "ok" || statuses["manifest.json"] != "ok" || statuses["checksums.txt"] != "ok" {
		t.Fatalf("unexpected file statuses: %#v", statuses)
	}
}

func TestCollectResticVerifyResults_InstanceMismatch(t *testing.T) {
	ctx := context.Background()
	bin := restic.BinaryInfo{Path: "/bin/echo", Version: restic.RequiredVersion}

	restoreList := cli.SetResticVerifyListSnapshotsForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, tags []string) ([]restic.Snapshot, error) {
		for _, tag := range tags {
			if tag == "type=instance" {
				return []restic.Snapshot{
					{ID: "snap-data", Time: time.Unix(0, 0), Tags: []string{"type=instance", "part=data", "project=alpha", "name=vm1", "timestamp=20240101T000000Z"}},
					{ID: "snap-checksums", Time: time.Unix(0, 0), Tags: []string{"type=instance", "part=checksums", "project=alpha", "name=vm1", "timestamp=20240101T000000Z"}},
				}, nil
			}
		}
		return nil, nil
	})
	defer restoreList()

	restoreDump := cli.SetResticVerifyDumpForTest(func(_ context.Context, _ restic.BinaryInfo, _ string, snapshotID string, path string, w io.Writer, _ io.Writer) error {
		switch snapshotID {
		case "snap-data":
			if path != "export.tar.xz" {
				t.Fatalf("unexpected data path %s", path)
			}
			_, err := w.Write([]byte("data"))
			return err
		case "snap-checksums":
			sums := []string{"deadbeef  export.tar.xz"}
			_, err := w.Write([]byte(strings.Join(sums, "\n") + "\n"))
			return err
		default:
			t.Fatalf("unexpected snapshot %s", snapshotID)
			return nil
		}
	})
	defer restoreDump()

	results, err := cli.CollectResticVerifyResultsForTest(ctx, bin, "repo", backendpkg.KindInstance)
	if err != nil {
		t.Fatalf("collect results: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	res := results[0]
	if res.Status != "mismatch" {
		t.Fatalf("expected mismatch, got %s", res.Status)
	}
	if len(res.Files) != 2 {
		t.Fatalf("expected 2 file results, got %d", len(res.Files))
	}
	foundMismatch := false
	for _, f := range res.Files {
		if f.Name == "export.tar.xz" && f.Status == "mismatch" {
			foundMismatch = true
		}
	}
	if !foundMismatch {
		t.Fatalf("expected mismatch entry: %#v", res.Files)
	}
}

func hexHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
