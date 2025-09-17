package restic

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"incus-backup/src/backend"
	cfg "incus-backup/src/backup/config"
	"incus-backup/src/restic"
)

type listSnapshotsFunc func(context.Context, restic.BinaryInfo, string, []string) ([]restic.Snapshot, error)
type listConfigTimestampsFunc func(context.Context, restic.BinaryInfo, string) ([]string, error)

type Backend struct {
	ctx  context.Context
	bin  restic.BinaryInfo
	repo string
}

var listSnapshots listSnapshotsFunc = restic.ListSnapshots
var listConfigTimestamps listConfigTimestampsFunc = cfg.ListResticConfigTimestamps

func New(ctx context.Context, bin restic.BinaryInfo, repo string) (*Backend, error) {
	if bin.Path == "" {
		return nil, errors.New("restic binary info is required")
	}
	if repo == "" {
		return nil, errors.New("restic repository must not be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return &Backend{ctx: ctx, bin: bin, repo: repo}, nil
}

func (b *Backend) List(kind string) ([]backend.Entry, error) {
	kinds := []string{backend.KindInstance, backend.KindVolume, backend.KindImage, backend.KindConfig}
	if kind != "" && kind != backend.KindAll {
		kinds = []string{kind}
	}

	var entries []backend.Entry
	for _, k := range kinds {
		switch k {
		case backend.KindInstance:
			e, err := b.listInstances()
			if err != nil {
				return nil, err
			}
			entries = append(entries, e...)
		case backend.KindVolume:
			e, err := b.listVolumes()
			if err != nil {
				return nil, err
			}
			entries = append(entries, e...)
		case backend.KindImage:
			e, err := b.listImages()
			if err != nil {
				return nil, err
			}
			entries = append(entries, e...)
		case backend.KindConfig:
			e, err := b.listConfig()
			if err != nil {
				return nil, err
			}
			entries = append(entries, e...)
		default:
			return nil, fmt.Errorf("restic backend: unsupported kind %s", k)
		}
	}

	sort.Slice(entries, func(i, j int) bool {
		a, b := entries[i], entries[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Project != b.Project {
			return a.Project < b.Project
		}
		if a.Pool != b.Pool {
			return a.Pool < b.Pool
		}
		if a.Name != b.Name {
			return a.Name < b.Name
		}
		if a.Fingerprint != b.Fingerprint {
			return a.Fingerprint < b.Fingerprint
		}
		return a.Timestamp < b.Timestamp
	})

	if len(entries) == 0 {
		return []backend.Entry{}, nil
	}

	return entries, nil
}

func (b *Backend) listInstances() ([]backend.Entry, error) {
	snaps, err := listSnapshots(b.ctx, b.bin, b.repo, []string{"type=instance", "part=manifest"})
	if err != nil {
		return nil, err
	}
	var entries []backend.Entry
	seen := map[string]struct{}{}
	for _, snap := range snaps {
		tags := snap.TagMap()
		project := tags["project"]
		name := tags["name"]
		if project == "" || name == "" {
			continue
		}
		ts := snapshotTimestamp(snap)
		key := project + "\x00" + name + "\x00" + ts
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, backend.Entry{
			Type:      "instance",
			Project:   project,
			Name:      name,
			Timestamp: ts,
			Path:      snap.ID,
		})
	}
	return entries, nil
}

func (b *Backend) listVolumes() ([]backend.Entry, error) {
	snaps, err := listSnapshots(b.ctx, b.bin, b.repo, []string{"type=volume", "part=manifest"})
	if err != nil {
		return nil, err
	}
	var entries []backend.Entry
	seen := map[string]struct{}{}
	for _, snap := range snaps {
		tags := snap.TagMap()
		project := tags["project"]
		pool := tags["pool"]
		name := tags["name"]
		if project == "" || pool == "" || name == "" {
			continue
		}
		ts := snapshotTimestamp(snap)
		key := project + "\x00" + pool + "\x00" + name + "\x00" + ts
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, backend.Entry{
			Type:      "volume",
			Project:   project,
			Pool:      pool,
			Name:      name,
			Timestamp: ts,
			Path:      snap.ID,
		})
	}
	return entries, nil
}

func (b *Backend) listImages() ([]backend.Entry, error) {
	return nil, nil
}

func (b *Backend) listConfig() ([]backend.Entry, error) {
	timestamps, err := listConfigTimestamps(b.ctx, b.bin, b.repo)
	if err != nil {
		return nil, err
	}
	entries := make([]backend.Entry, 0, len(timestamps))
	for _, ts := range timestamps {
		entries = append(entries, backend.Entry{
			Type:      "config",
			Timestamp: ts,
		})
	}
	return entries, nil
}

func snapshotTimestamp(snap restic.Snapshot) string {
	if ts := snap.TagMap()["timestamp"]; ts != "" {
		return ts
	}
	return snap.Time.UTC().Format("20060102T150405Z")
}

// SetListSnapshotsForTest allows tests to stub out restic snapshot listing.
func SetListSnapshotsForTest(fn listSnapshotsFunc) func() {
	prev := listSnapshots
	listSnapshots = fn
	return func() { listSnapshots = prev }
}

// SetListConfigTimestampsForTest allows tests to stub config timestamp enumeration.
func SetListConfigTimestampsForTest(fn listConfigTimestampsFunc) func() {
	prev := listConfigTimestamps
	listConfigTimestamps = fn
	return func() { listConfigTimestamps = prev }
}
