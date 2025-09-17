package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"incus-backup/src/backend"
	"incus-backup/src/restic"
)

type resticPruneCandidate struct {
	Type      string
	Project   string
	Pool      string
	Name      string
	Timestamp string
	Snapshots []restic.Snapshot
}

type resticListForPruneFunc func(context.Context, restic.BinaryInfo, string, []string) ([]restic.Snapshot, error)
type resticForgetFunc func(context.Context, restic.BinaryInfo, string, []string, bool) error

type resourceKey struct {
	Project string
	Pool    string
	Name    string
}

var listSnapshotsForPrune resticListForPruneFunc = restic.ListSnapshots
var forgetSnapshotsFunc resticForgetFunc = restic.ForgetSnapshots

func planResticPrune(ctx context.Context, bin restic.BinaryInfo, repo, kind string, keep int) ([]resticPruneCandidate, error) {
	if keep <= 0 {
		return nil, fmt.Errorf("keep must be > 0")
	}
	allKinds := kind == "" || kind == backend.KindAll
	var candidates []resticPruneCandidate
	if allKinds || kind == backend.KindInstance {
		inst, err := collectResticInstanceVersions(ctx, bin, repo)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, pruneByKeep(inst, keep)...)
	}
	if allKinds || kind == backend.KindVolume {
		vols, err := collectResticVolumeVersions(ctx, bin, repo)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, pruneByKeep(vols, keep)...)
	}
	if allKinds || kind == backend.KindConfig {
		cfgGroups, err := collectResticConfigVersions(ctx, bin, repo)
		if err != nil {
			return nil, err
		}
		candidates = append(candidates, pruneByKeep(cfgGroups, keep)...)
	}
	if kind == backend.KindImage {
		return nil, fmt.Errorf("restic prune: images not implemented")
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
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
		return a.Timestamp < b.Timestamp
	})
	return candidates, nil
}

func collectResticInstanceVersions(ctx context.Context, bin restic.BinaryInfo, repo string) (map[resourceKey][]resticPruneCandidate, error) {
	snaps, err := listSnapshotsForPrune(ctx, bin, repo, []string{"type=instance"})
	if err != nil {
		return nil, err
	}
	grouped := map[resourceKey]map[string][]restic.Snapshot{}
	for _, snap := range snaps {
		tags := snap.TagMap()
		project := tags["project"]
		name := tags["name"]
		part := tags["part"]
		if project == "" || name == "" || part == "" {
			continue
		}
		ts := tags["timestamp"]
		if ts == "" {
			ts = snapshotTimestamp(snap)
		}
		key := resourceKey{Project: project, Name: name}
		buckets := grouped[key]
		if buckets == nil {
			buckets = map[string][]restic.Snapshot{}
			grouped[key] = buckets
		}
		if !snapshotExists(buckets[ts], snap.ID) {
			buckets[ts] = append(buckets[ts], snap)
		}
	}
	return convertGroupedToCandidates("instance", grouped), nil
}

func collectResticVolumeVersions(ctx context.Context, bin restic.BinaryInfo, repo string) (map[resourceKey][]resticPruneCandidate, error) {
	snaps, err := listSnapshotsForPrune(ctx, bin, repo, []string{"type=volume"})
	if err != nil {
		return nil, err
	}
	grouped := map[resourceKey]map[string][]restic.Snapshot{}
	for _, snap := range snaps {
		tags := snap.TagMap()
		project := tags["project"]
		pool := tags["pool"]
		name := tags["name"]
		part := tags["part"]
		if project == "" || pool == "" || name == "" || part == "" {
			continue
		}
		ts := tags["timestamp"]
		if ts == "" {
			ts = snapshotTimestamp(snap)
		}
		key := resourceKey{Project: project, Pool: pool, Name: name}
		buckets := grouped[key]
		if buckets == nil {
			buckets = map[string][]restic.Snapshot{}
			grouped[key] = buckets
		}
		if !snapshotExists(buckets[ts], snap.ID) {
			buckets[ts] = append(buckets[ts], snap)
		}
	}
	return convertGroupedToCandidates("volume", grouped), nil
}

func collectResticConfigVersions(ctx context.Context, bin restic.BinaryInfo, repo string) (map[resourceKey][]resticPruneCandidate, error) {
	snaps, err := listSnapshotsForPrune(ctx, bin, repo, []string{"type=config"})
	if err != nil {
		return nil, err
	}
	grouped := map[resourceKey]map[string][]restic.Snapshot{
		{}: {},
	}
	for _, snap := range snaps {
		tags := snap.TagMap()
		part := tags["part"]
		if part == "" {
			continue
		}
		ts := tags["timestamp"]
		if ts == "" {
			ts = snapshotTimestamp(snap)
		}
		buckets := grouped[resourceKey{}]
		if !snapshotExists(buckets[ts], snap.ID) {
			buckets[ts] = append(buckets[ts], snap)
		}
	}
	return convertGroupedToCandidates("config", grouped), nil
}

func convertGroupedToCandidates(kind string, grouped map[resourceKey]map[string][]restic.Snapshot) map[resourceKey][]resticPruneCandidate {
	out := make(map[resourceKey][]resticPruneCandidate, len(grouped))
	for key, versions := range grouped {
		var timestamps []string
		for ts := range versions {
			timestamps = append(timestamps, ts)
		}
		sort.Strings(timestamps)
		for _, ts := range timestamps {
			out[key] = append(out[key], resticPruneCandidate{
				Type:      kind,
				Project:   key.Project,
				Pool:      key.Pool,
				Name:      key.Name,
				Timestamp: ts,
				Snapshots: append([]restic.Snapshot(nil), versions[ts]...),
			})
		}
	}
	return out
}

func pruneByKeep(grouped map[resourceKey][]resticPruneCandidate, keep int) []resticPruneCandidate {
	var candidates []resticPruneCandidate
	for _, versions := range grouped {
		if len(versions) <= keep {
			continue
		}
		for _, candidate := range versions[:len(versions)-keep] {
			candidates = append(candidates, candidate)
		}
	}
	return candidates
}

func snapshotExists(snaps []restic.Snapshot, id string) bool {
	for _, s := range snaps {
		if s.ID == id {
			return true
		}
	}
	return false
}

func renderResticPrunePreview(w io.Writer, candidates []resticPruneCandidate) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tPROJECT\tPOOL\tNAME\tFINGERPRINT\tTIMESTAMP\tSNAPSHOTS")
	for _, c := range candidates {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d parts\n",
			c.Type,
			safePad(c.Project),
			safePad(c.Pool),
			safePad(c.Name),
			"",
			c.Timestamp,
			len(c.Snapshots),
		)
	}
	_ = tw.Flush()
}

func collectSnapshotIDs(candidates []resticPruneCandidate) []string {
	seen := make(map[string]struct{})
	var ids []string
	for _, c := range candidates {
		for _, snap := range c.Snapshots {
			if _, ok := seen[snap.ID]; ok {
				continue
			}
			seen[snap.ID] = struct{}{}
			ids = append(ids, snap.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

// SetResticPruneListSnapshotsForTest allows tests to override snapshot listing.
func SetResticPruneListSnapshotsForTest(fn resticListForPruneFunc) func() {
	prev := listSnapshotsForPrune
	listSnapshotsForPrune = fn
	return func() { listSnapshotsForPrune = prev }
}

// SetResticPruneForgetForTest allows tests to override restic forget operations.
func SetResticPruneForgetForTest(fn resticForgetFunc) func() {
	prev := forgetSnapshotsFunc
	forgetSnapshotsFunc = fn
	return func() { forgetSnapshotsFunc = prev }
}
