package config

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
)

type SnapshotData struct {
	Timestamp    string
	Manifest     Manifest
	Projects     []incusapi.Project
	Profiles     []incusapi.Profile
	Networks     []incusapi.Network
	StoragePools []incusapi.StoragePool
}

func BackupAllRestic(ctx context.Context, bin restic.BinaryInfo, repo string, client incusapi.Client, now time.Time, progressOut io.Writer) (string, error) {
	if err := restic.EnsureRepository(ctx, bin, repo); err != nil {
		return "", err
	}

	ts := now.UTC().Format("20060102T150405Z")
	entries := make([]struct {
		name string
		hash string
	}, 0, 4)

	if data, hash, err := marshalProjects(client); err != nil {
		return "", err
	} else if err := restic.BackupBytes(ctx, bin, repo, configFileName("projects.json"), tagsConfig(ts, "projects"), data, progressOut); err != nil {
		return "", err
	} else {
		entries = append(entries, struct{ name, hash string }{"projects", hash})
	}

	if data, hash, err := marshalProfiles(client); err != nil {
		return "", err
	} else if err := restic.BackupBytes(ctx, bin, repo, configFileName("profiles.json"), tagsConfig(ts, "profiles"), data, progressOut); err != nil {
		return "", err
	} else {
		entries = append(entries, struct{ name, hash string }{"profiles", hash})
	}

	if data, hash, err := marshalNetworks(client); err != nil {
		return "", err
	} else if err := restic.BackupBytes(ctx, bin, repo, configFileName("networks.json"), tagsConfig(ts, "networks"), data, progressOut); err != nil {
		return "", err
	} else {
		entries = append(entries, struct{ name, hash string }{"networks", hash})
	}

	if data, hash, err := marshalStoragePools(client); err != nil {
		return "", err
	} else if err := restic.BackupBytes(ctx, bin, repo, configFileName("storage_pools.json"), tagsConfig(ts, "storage_pools"), data, progressOut); err != nil {
		return "", err
	} else {
		entries = append(entries, struct{ name, hash string }{"storage_pools", hash})
	}

	manifest := Manifest{Type: "config", CreatedAt: now.UTC()}
	for _, entry := range entries {
		manifest.Includes = append(manifest.Includes, entry.name)
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	manifestHash := hashBytes(manifestBytes)
	if err := restic.BackupBytes(ctx, bin, repo, configFileName("manifest.json"), tagsConfig(ts, "manifest"), manifestBytes, progressOut); err != nil {
		return "", err
	}

	checksums := buildChecksums(entries, manifestHash)
	if err := restic.BackupBytes(ctx, bin, repo, configFileName("checksums.txt"), tagsConfig(ts, "checksums"), []byte(checksums), progressOut); err != nil {
		return "", err
	}

	return ts, nil
}

func LoadSnapshotRestic(ctx context.Context, bin restic.BinaryInfo, repo string, version string) (SnapshotData, error) {
	var snapshot SnapshotData

	manifestSnap, err := findConfigSnapshot(ctx, bin, repo, version, "manifest")
	if err != nil {
		return snapshot, err
	}
	ts := manifestSnap.TagMap()["timestamp"]
	if ts == "" {
		ts = manifestSnap.Time.UTC().Format("20060102T150405Z")
	}
	snapshot.Timestamp = ts

	manifestBytes, err := dumpConfigFile(ctx, bin, repo, manifestSnap, configFileName("manifest.json"))
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(manifestBytes, &snapshot.Manifest); err != nil {
		return snapshot, err
	}

	if snapshot.Projects, err = loadProjectsRestic(ctx, bin, repo, ts); err != nil {
		return snapshot, err
	}
	if snapshot.Profiles, err = loadProfilesRestic(ctx, bin, repo, ts); err != nil {
		return snapshot, err
	}
	if snapshot.Networks, err = loadNetworksRestic(ctx, bin, repo, ts); err != nil {
		return snapshot, err
	}
	if snapshot.StoragePools, err = loadStoragePoolsRestic(ctx, bin, repo, ts); err != nil {
		return snapshot, err
	}

	return snapshot, nil
}

func ListResticConfigTimestamps(ctx context.Context, bin restic.BinaryInfo, repo string) ([]string, error) {
	snaps, err := restic.ListSnapshots(ctx, bin, repo, []string{"type=config", "part=manifest"})
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	var ts []string
	for _, snap := range snaps {
		value := snap.TagMap()["timestamp"]
		if value == "" {
			value = snap.Time.UTC().Format("20060102T150405Z")
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		ts = append(ts, value)
	}
	sort.Strings(ts)
	return ts, nil
}

func marshalProjects(client incusapi.Client) ([]byte, string, error) {
	projects, err := client.ListProjects()
	if err != nil {
		return nil, "", err
	}
	sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return data, hashBytes(data), nil
}

func marshalProfiles(client incusapi.Client) ([]byte, string, error) {
	profiles, err := client.ListProfiles()
	if err != nil {
		return nil, "", err
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return data, hashBytes(data), nil
}

func marshalNetworks(client incusapi.Client) ([]byte, string, error) {
	networks, err := client.ListNetworks()
	if err != nil {
		return nil, "", err
	}
	sort.Slice(networks, func(i, j int) bool { return networks[i].Name < networks[j].Name })
	data, err := json.MarshalIndent(networks, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return data, hashBytes(data), nil
}

func marshalStoragePools(client incusapi.Client) ([]byte, string, error) {
	pools, err := client.ListStoragePools()
	if err != nil {
		return nil, "", err
	}
	sort.Slice(pools, func(i, j int) bool { return pools[i].Name < pools[j].Name })
	data, err := json.MarshalIndent(pools, "", "  ")
	if err != nil {
		return nil, "", err
	}
	return data, hashBytes(data), nil
}

func buildChecksums(entries []struct{ name, hash string }, manifestHash string) string {
	var lines []string
	for _, entry := range entries {
		file := fmt.Sprintf("%s.json", entry.name)
		lines = append(lines, fmt.Sprintf("%s  %s", entry.hash, file))
	}
	lines = append(lines, fmt.Sprintf("%s  manifest.json", manifestHash))
	return strings.Join(lines, "\n") + "\n"
}

func configFileName(file string) string { return file }

func tagsConfig(ts, part string) []string {
	return []string{
		"type=config",
		"schema=v1",
		fmt.Sprintf("timestamp=%s", ts),
		fmt.Sprintf("part=%s", part),
	}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func findConfigSnapshot(ctx context.Context, bin restic.BinaryInfo, repo, version, part string) (restic.Snapshot, error) {
	tags := []string{"type=config", fmt.Sprintf("part=%s", part)}
	if version != "" {
		tags = append(tags, fmt.Sprintf("timestamp=%s", version))
	}
	snaps, err := restic.ListSnapshots(ctx, bin, repo, tags)
	if err != nil {
		return restic.Snapshot{}, err
	}
	if len(snaps) == 0 {
		if version == "" {
			return restic.Snapshot{}, fmt.Errorf("no config snapshots found")
		}
		return restic.Snapshot{}, fmt.Errorf("no config snapshot found for timestamp %s", version)
	}
	if version != "" {
		for i := len(snaps) - 1; i >= 0; i-- {
			if snaps[i].TagMap()["timestamp"] == version {
				return snaps[i], nil
			}
		}
		return restic.Snapshot{}, fmt.Errorf("config snapshot with timestamp %s not found", version)
	}
	return snaps[len(snaps)-1], nil
}

func dumpConfigFile(ctx context.Context, bin restic.BinaryInfo, repo string, snap restic.Snapshot, path string) ([]byte, error) {
	var buf bytes.Buffer
	if err := restic.Dump(ctx, bin, repo, snap.ID, path, &buf, nil); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func loadProjectsRestic(ctx context.Context, bin restic.BinaryInfo, repo, ts string) ([]incusapi.Project, error) {
	snap, err := findConfigSnapshot(ctx, bin, repo, ts, "projects")
	if err != nil {
		return nil, err
	}
	data, err := dumpConfigFile(ctx, bin, repo, snap, configFileName("projects.json"))
	if err != nil {
		return nil, err
	}
	var out []incusapi.Project
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadProfilesRestic(ctx context.Context, bin restic.BinaryInfo, repo, ts string) ([]incusapi.Profile, error) {
	snap, err := findConfigSnapshot(ctx, bin, repo, ts, "profiles")
	if err != nil {
		return nil, err
	}
	data, err := dumpConfigFile(ctx, bin, repo, snap, configFileName("profiles.json"))
	if err != nil {
		return nil, err
	}
	var out []incusapi.Profile
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadNetworksRestic(ctx context.Context, bin restic.BinaryInfo, repo, ts string) ([]incusapi.Network, error) {
	snap, err := findConfigSnapshot(ctx, bin, repo, ts, "networks")
	if err != nil {
		return nil, err
	}
	data, err := dumpConfigFile(ctx, bin, repo, snap, configFileName("networks.json"))
	if err != nil {
		return nil, err
	}
	var out []incusapi.Network
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadStoragePoolsRestic(ctx context.Context, bin restic.BinaryInfo, repo, ts string) ([]incusapi.StoragePool, error) {
	snap, err := findConfigSnapshot(ctx, bin, repo, ts, "storage_pools")
	if err != nil {
		return nil, err
	}
	data, err := dumpConfigFile(ctx, bin, repo, snap, configFileName("storage_pools.json"))
	if err != nil {
		return nil, err
	}
	var out []incusapi.StoragePool
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
