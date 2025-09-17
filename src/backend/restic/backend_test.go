package restic

import (
	"context"
	"testing"
	"time"

	backendpkg "incus-backup/src/backend"
	resticlib "incus-backup/src/restic"
)

func TestBackendListInstances(t *testing.T) {
	resetCfg := SetListConfigTimestampsForTest(func(context.Context, resticlib.BinaryInfo, string) ([]string, error) {
		return nil, nil
	})
	defer resetCfg()

	reset := SetListSnapshotsForTest(func(_ context.Context, _ resticlib.BinaryInfo, _ string, tags []string) ([]resticlib.Snapshot, error) {
		if hasTag(tags, "type=instance") {
			return []resticlib.Snapshot{
				{
					ID:   "snap-1",
					Time: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
					Tags: []string{"type=instance", "part=manifest", "project=alpha", "name=vm1", "timestamp=20240101T000000Z"},
				},
				{
					ID:   "snap-2",
					Time: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
					Tags: []string{"type=instance", "part=manifest", "project=alpha", "name=vm1"},
				},
			}, nil
		}
		return nil, nil
	})
	defer reset()

	bin := resticlib.BinaryInfo{Path: "/usr/bin/restic", Version: resticlib.RequiredVersion}
	b, err := New(context.Background(), bin, "repo")
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}

	entries, err := b.List(backendpkg.KindInstance)
	if err != nil {
		t.Fatalf("List instances: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	if entries[0].Project != "alpha" || entries[0].Name != "vm1" || entries[0].Timestamp != "20240101T000000Z" || entries[0].Path != "snap-1" {
		t.Fatalf("unexpected first entry: %#v", entries[0])
	}
	if entries[1].Timestamp != "20240102T000000Z" {
		t.Fatalf("expected fallback timestamp from snapshot time, got %q", entries[1].Timestamp)
	}
}

func TestBackendListVolumes(t *testing.T) {
	resetCfg := SetListConfigTimestampsForTest(func(context.Context, resticlib.BinaryInfo, string) ([]string, error) {
		return nil, nil
	})
	defer resetCfg()

	reset := SetListSnapshotsForTest(func(_ context.Context, _ resticlib.BinaryInfo, _ string, tags []string) ([]resticlib.Snapshot, error) {
		if hasTag(tags, "type=volume") {
			return []resticlib.Snapshot{
				{
					ID:   "vol-1",
					Time: time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC),
					Tags: []string{"type=volume", "part=manifest", "project=alpha", "pool=fast", "name=data", "timestamp=20240103T120000Z"},
				},
				{
					ID:   "vol-1-dup",
					Time: time.Date(2024, 1, 3, 12, 0, 0, 0, time.UTC),
					Tags: []string{"type=volume", "part=manifest", "project=alpha", "pool=fast", "name=data", "timestamp=20240103T120000Z"},
				},
			}, nil
		}
		return nil, nil
	})
	defer reset()

	bin := resticlib.BinaryInfo{Path: "/usr/bin/restic", Version: resticlib.RequiredVersion}
	b, err := New(context.Background(), bin, "repo")
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}

	entries, err := b.List(backendpkg.KindVolume)
	if err != nil {
		t.Fatalf("List volumes: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after dedupe, got %d", len(entries))
	}
	got := entries[0]
	if got.Project != "alpha" || got.Pool != "fast" || got.Name != "data" || got.Path != "vol-1" {
		t.Fatalf("unexpected volume entry: %#v", got)
	}
}

func TestBackendListConfig(t *testing.T) {
	reset := SetListConfigTimestampsForTest(func(context.Context, resticlib.BinaryInfo, string) ([]string, error) {
		return []string{"20240101T000000Z", "20240102T000000Z"}, nil
	})
	defer reset()

	resetSnaps := SetListSnapshotsForTest(func(context.Context, resticlib.BinaryInfo, string, []string) ([]resticlib.Snapshot, error) {
		return nil, nil
	})
	defer resetSnaps()

	bin := resticlib.BinaryInfo{Path: "/usr/bin/restic", Version: resticlib.RequiredVersion}
	b, err := New(context.Background(), bin, "repo")
	if err != nil {
		t.Fatalf("New backend: %v", err)
	}

	entries, err := b.List(backendpkg.KindConfig)
	if err != nil {
		t.Fatalf("List config: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 config entries, got %d", len(entries))
	}
	if entries[0].Timestamp != "20240101T000000Z" || entries[1].Timestamp != "20240102T000000Z" {
		t.Fatalf("unexpected config timestamps: %#v", entries)
	}
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}
