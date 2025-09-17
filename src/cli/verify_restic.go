package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"incus-backup/src/backend"
	"incus-backup/src/restic"
)

type resticListSnapshotsFunc func(context.Context, restic.BinaryInfo, string, []string) ([]restic.Snapshot, error)
type resticDumpFunc func(context.Context, restic.BinaryInfo, string, string, string, io.Writer, io.Writer) error

var listSnapshotsRestic resticListSnapshotsFunc = restic.ListSnapshots
var dumpSnapshot resticDumpFunc = restic.Dump

func collectResticVerifyResults(ctx context.Context, bin restic.BinaryInfo, repo, kind string) ([]verifyResult, error) {
	var results []verifyResult
	appendResults := func(items []verifyResult, err error) error {
		if err != nil {
			return err
		}
		results = append(results, items...)
		return nil
	}
	switch kind {
	case backend.KindAll, "", backend.KindInstance, backend.KindVolume, backend.KindImage, backend.KindConfig:
		// handled below
	default:
		return nil, fmt.Errorf("restic verify: unsupported kind %s", kind)
	}
	if kind == backend.KindAll || kind == "" || kind == backend.KindInstance {
		if err := appendResults(verifyResticInstances(ctx, bin, repo)); err != nil {
			return nil, err
		}
	}
	if kind == backend.KindAll || kind == "" || kind == backend.KindVolume {
		if err := appendResults(verifyResticVolumes(ctx, bin, repo)); err != nil {
			return nil, err
		}
	}
	if kind == backend.KindAll || kind == "" || kind == backend.KindConfig {
		if err := appendResults(verifyResticConfig(ctx, bin, repo)); err != nil {
			return nil, err
		}
	}
	if kind == backend.KindImage {
		return nil, errors.New("restic verify: images backend not implemented")
	}

	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
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
	return results, nil
}

func verifyResticInstances(ctx context.Context, bin restic.BinaryInfo, repo string) ([]verifyResult, error) {
	snaps, err := listSnapshotsRestic(ctx, bin, repo, []string{"type=instance"})
	if err != nil {
		return nil, err
	}
	groups := make(map[string]*resticInstanceGroup)
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
		key := project + "\x00" + name + "\x00" + ts
		grp := groups[key]
		if grp == nil {
			grp = &resticInstanceGroup{
				Project: project,
				Name:    name,
				ts:      ts,
				parts:   map[string]restic.Snapshot{},
			}
			groups[key] = grp
		}
		grp.parts[part] = snap
	}
	var keys []string
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var results []verifyResult
	for _, k := range keys {
		grp := groups[k]
		res := verifyResult{Type: "instance", Project: grp.Project, Name: grp.Name, Timestamp: grp.ts}
		files, status := verifyInstanceGroup(ctx, bin, repo, grp)
		res.Files = files
		res.Status = status
		if dataSnap, ok := grp.parts["data"]; ok {
			res.Path = dataSnap.ID
		}
		results = append(results, res)
	}
	return results, nil
}

func verifyResticVolumes(ctx context.Context, bin restic.BinaryInfo, repo string) ([]verifyResult, error) {
	snaps, err := listSnapshotsRestic(ctx, bin, repo, []string{"type=volume"})
	if err != nil {
		return nil, err
	}
	groups := make(map[string]*resticVolumeGroup)
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
		key := project + "\x00" + pool + "\x00" + name + "\x00" + ts
		grp := groups[key]
		if grp == nil {
			grp = &resticVolumeGroup{
				Project: project,
				Pool:    pool,
				Name:    name,
				ts:      ts,
				parts:   map[string]restic.Snapshot{},
			}
			groups[key] = grp
		}
		grp.parts[part] = snap
	}
	var keys []string
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var results []verifyResult
	for _, k := range keys {
		grp := groups[k]
		res := verifyResult{Type: "volume", Project: grp.Project, Pool: grp.Pool, Name: grp.Name, Timestamp: grp.ts}
		files, status := verifyVolumeGroup(ctx, bin, repo, grp)
		res.Files = files
		res.Status = status
		if dataSnap, ok := grp.parts["data"]; ok {
			res.Path = dataSnap.ID
		}
		results = append(results, res)
	}
	return results, nil
}

func verifyResticConfig(ctx context.Context, bin restic.BinaryInfo, repo string) ([]verifyResult, error) {
	snaps, err := listSnapshotsRestic(ctx, bin, repo, []string{"type=config"})
	if err != nil {
		return nil, err
	}
	groups := make(map[string]*resticConfigGroup)
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
		grp := groups[ts]
		if grp == nil {
			grp = &resticConfigGroup{ts: ts, parts: map[string]restic.Snapshot{}}
			groups[ts] = grp
		}
		grp.parts[part] = snap
	}
	var timestamps []string
	for ts := range groups {
		timestamps = append(timestamps, ts)
	}
	sort.Strings(timestamps)
	var results []verifyResult
	for _, ts := range timestamps {
		grp := groups[ts]
		res := verifyResult{Type: "config", Timestamp: grp.ts}
		files, status := verifyConfigGroup(ctx, bin, repo, grp)
		res.Files = files
		res.Status = status
		if manifestSnap, ok := grp.parts["manifest"]; ok {
			res.Path = manifestSnap.ID
		}
		results = append(results, res)
	}
	return results, nil
}

type resticInstanceGroup struct {
	Project string
	Name    string
	ts      string
	parts   map[string]restic.Snapshot
}

type resticVolumeGroup struct {
	Project string
	Pool    string
	Name    string
	ts      string
	parts   map[string]restic.Snapshot
}

type resticConfigGroup struct {
	ts    string
	parts map[string]restic.Snapshot
}

func verifyInstanceGroup(ctx context.Context, bin restic.BinaryInfo, repo string, grp *resticInstanceGroup) ([]verifyFileResult, string) {
	checksumsSnap, ok := grp.parts["checksums"]
	if !ok {
		return []verifyFileResult{{Name: "checksums.txt", Status: "missing", Error: "restic snapshot missing"}}, "error"
	}
	checksumsPath := instanceChecksumsPath(grp.Project, grp.Name, grp.ts)
	entries, err := loadResticChecksums(ctx, bin, repo, checksumsSnap, checksumsPath)
	if err != nil {
		return []verifyFileResult{{Name: "checksums.txt", Status: "error", Error: err.Error()}}, "error"
	}
	files := []verifyFileResult{{Name: "checksums.txt", Status: "ok"}}
	hasMismatch := false
	hasError := false
	for _, entry := range entries {
		part := instancePartForFile(entry.Name)
		snap, ok := grp.parts[part]
		if !ok {
			hasError = true
			files = append(files, verifyFileResult{Name: entry.Name, Status: "missing", Expected: entry.Hash, Error: "restic snapshot missing"})
			continue
		}
		path := instanceFilePath(grp.Project, grp.Name, grp.ts, entry.Name)
		actual, err := hashResticFile(ctx, bin, repo, snap, path)
		if err != nil {
			hasError = true
			status := "error"
			if strings.Contains(err.Error(), "not found") {
				status = "missing"
			}
			files = append(files, verifyFileResult{Name: entry.Name, Status: status, Expected: entry.Hash, Error: err.Error()})
			continue
		}
		if strings.EqualFold(entry.Hash, actual) {
			files = append(files, verifyFileResult{Name: entry.Name, Status: "ok", Expected: entry.Hash, Actual: actual})
			continue
		}
		hasMismatch = true
		files = append(files, verifyFileResult{Name: entry.Name, Status: "mismatch", Expected: entry.Hash, Actual: actual})
	}
	switch {
	case hasError:
		return files, "error"
	case hasMismatch:
		return files, "mismatch"
	default:
		return files, "ok"
	}
}

func verifyVolumeGroup(ctx context.Context, bin restic.BinaryInfo, repo string, grp *resticVolumeGroup) ([]verifyFileResult, string) {
	checksumsSnap, ok := grp.parts["checksums"]
	if !ok {
		return []verifyFileResult{{Name: "checksums.txt", Status: "missing", Error: "restic snapshot missing"}}, "error"
	}
	checksumsPath := volumeChecksumsPath(grp.Project, grp.Pool, grp.Name, grp.ts)
	entries, err := loadResticChecksums(ctx, bin, repo, checksumsSnap, checksumsPath)
	if err != nil {
		return []verifyFileResult{{Name: "checksums.txt", Status: "error", Error: err.Error()}}, "error"
	}
	files := []verifyFileResult{{Name: "checksums.txt", Status: "ok"}}
	hasMismatch := false
	hasError := false
	for _, entry := range entries {
		part := volumePartForFile(entry.Name)
		snap, ok := grp.parts[part]
		if !ok {
			hasError = true
			files = append(files, verifyFileResult{Name: entry.Name, Status: "missing", Expected: entry.Hash, Error: "restic snapshot missing"})
			continue
		}
		path := volumeFilePath(grp.Project, grp.Pool, grp.Name, grp.ts, entry.Name)
		actual, err := hashResticFile(ctx, bin, repo, snap, path)
		if err != nil {
			hasError = true
			status := "error"
			if strings.Contains(err.Error(), "not found") {
				status = "missing"
			}
			files = append(files, verifyFileResult{Name: entry.Name, Status: status, Expected: entry.Hash, Error: err.Error()})
			continue
		}
		if strings.EqualFold(entry.Hash, actual) {
			files = append(files, verifyFileResult{Name: entry.Name, Status: "ok", Expected: entry.Hash, Actual: actual})
			continue
		}
		hasMismatch = true
		files = append(files, verifyFileResult{Name: entry.Name, Status: "mismatch", Expected: entry.Hash, Actual: actual})
	}
	switch {
	case hasError:
		return files, "error"
	case hasMismatch:
		return files, "mismatch"
	default:
		return files, "ok"
	}
}

func verifyConfigGroup(ctx context.Context, bin restic.BinaryInfo, repo string, grp *resticConfigGroup) ([]verifyFileResult, string) {
	checksumsSnap, ok := grp.parts["checksums"]
	if !ok {
		return []verifyFileResult{{Name: "checksums.txt", Status: "missing", Error: "restic snapshot missing"}}, "error"
	}
	checksumsPath := configChecksumsPath(grp.ts)
	entries, err := loadResticChecksums(ctx, bin, repo, checksumsSnap, checksumsPath)
	if err != nil {
		return []verifyFileResult{{Name: "checksums.txt", Status: "error", Error: err.Error()}}, "error"
	}
	files := []verifyFileResult{{Name: "checksums.txt", Status: "ok"}}
	hasMismatch := false
	hasError := false
	for _, entry := range entries {
		part := configPartForFile(entry.Name)
		snap, ok := grp.parts[part]
		if !ok {
			hasError = true
			files = append(files, verifyFileResult{Name: entry.Name, Status: "missing", Expected: entry.Hash, Error: "restic snapshot missing"})
			continue
		}
		path := configFilePath(grp.ts, entry.Name)
		actual, err := hashResticFile(ctx, bin, repo, snap, path)
		if err != nil {
			hasError = true
			status := "error"
			if strings.Contains(err.Error(), "not found") {
				status = "missing"
			}
			files = append(files, verifyFileResult{Name: entry.Name, Status: status, Expected: entry.Hash, Error: err.Error()})
			continue
		}
		if strings.EqualFold(entry.Hash, actual) {
			files = append(files, verifyFileResult{Name: entry.Name, Status: "ok", Expected: entry.Hash, Actual: actual})
			continue
		}
		hasMismatch = true
		files = append(files, verifyFileResult{Name: entry.Name, Status: "mismatch", Expected: entry.Hash, Actual: actual})
	}
	switch {
	case hasError:
		return files, "error"
	case hasMismatch:
		return files, "mismatch"
	default:
		return files, "ok"
	}
}

type checksumEntry struct {
	Name string
	Hash string
}

func loadResticChecksums(ctx context.Context, bin restic.BinaryInfo, repo string, snap restic.Snapshot, path string) ([]checksumEntry, error) {
	var buf bytes.Buffer
	if err := dumpSnapshot(ctx, bin, repo, snap.ID, path, &buf, nil); err != nil {
		return nil, err
	}
	lines := strings.Split(buf.String(), "\n")
	var entries []checksumEntry
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid checksum entry: %s", line)
		}
		entries = append(entries, checksumEntry{Name: parts[1], Hash: parts[0]})
	}
	return entries, nil
}

func hashResticFile(ctx context.Context, bin restic.BinaryInfo, repo string, snap restic.Snapshot, path string) (string, error) {
	h := sha256.New()
	if err := dumpSnapshot(ctx, bin, repo, snap.ID, path, h, nil); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func instanceChecksumsPath(string, string, string) string { return "checksums.txt" }

func instanceFilePath(_, _, _, file string) string { return file }

func instancePartForFile(file string) string {
	switch file {
	case "export.tar":
		return "data"
	case "manifest.json":
		return "manifest"
	default:
		return ""
	}
}

func volumeChecksumsPath(string, string, string, string) string { return "checksums.txt" }

func volumeFilePath(_, _, _, _, file string) string { return file }

func volumePartForFile(file string) string {
	switch file {
	case "volume.tar":
		return "data"
	case "manifest.json":
		return "manifest"
	default:
		return ""
	}
}

func configChecksumsPath(string) string { return "checksums.txt" }

func configFilePath(_, file string) string { return file }

func configPartForFile(file string) string {
	switch file {
	case "projects.json":
		return "projects"
	case "profiles.json":
		return "profiles"
	case "networks.json":
		return "networks"
	case "storage_pools.json":
		return "storage_pools"
	case "manifest.json":
		return "manifest"
	default:
		return ""
	}
}

// CollectResticVerifyResultsForTest allows tests to inject fake restic behaviours.
func CollectResticVerifyResultsForTest(ctx context.Context, bin restic.BinaryInfo, repo, kind string) ([]verifyResult, error) {
	return collectResticVerifyResults(ctx, bin, repo, kind)
}

// SetResticVerifyListSnapshotsForTest stubs the list snapshots helper within tests.
func SetResticVerifyListSnapshotsForTest(fn resticListSnapshotsFunc) func() {
	prev := listSnapshotsRestic
	listSnapshotsRestic = fn
	return func() { listSnapshotsRestic = prev }
}

// SetResticVerifyDumpForTest stubs the dump helper within tests.
func SetResticVerifyDumpForTest(fn resticDumpFunc) func() {
	prev := dumpSnapshot
	dumpSnapshot = fn
	return func() { dumpSnapshot = prev }
}
