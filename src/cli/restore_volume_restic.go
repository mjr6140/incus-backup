package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	vol "incus-backup/src/backup/volumes"
	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	"incus-backup/src/safety"
	"incus-backup/src/target"

	"github.com/spf13/cobra"
)

func restoreVolumeFromRestic(cmd *cobra.Command, client incusapi.Client, tgt target.Target, project, pool, name, version, targetName string, replace, skipExisting bool, stdout io.Writer) error {
	info, err := checkResticBinary(cmd, true)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := restic.EnsureRepository(ctx, info, tgt.Value); err != nil {
		return err
	}

	snap, err := findVolumeSnapshot(ctx, info, tgt.Value, project, pool, name, version)
	if err != nil {
		return err
	}

	destName := targetName
	if destName == "" {
		destName = name
	}
	exists, err := client.VolumeExists(project, pool, destName)
	if err != nil {
		return err
	}

	action := "create"
	if exists {
		action = "conflict"
		if replace {
			action = "replace"
		}
		if skipExisting {
			action = "skip"
		}
	}

	renderVolumeRestorePreview(stdout, []volumePreviewRow{{
		Action:     action,
		Project:    project,
		Pool:       pool,
		Name:       name,
		TargetName: destName,
		Version:    volumeSnapshotTimestamp(snap),
	}})

	opts := getSafetyOptions(cmd)
	if opts.DryRun {
		return nil
	}

	if exists {
		if skipExisting {
			return nil
		}
		if !replace {
			ok, err := safety.Confirm(opts, cmd.InOrStdin(), stdout, fmt.Sprintf("Volume %s/%s exists. Replace it?", pool, destName))
			if err != nil || !ok {
				return err
			}
		}
		if err := client.DeleteVolume(project, pool, destName); err != nil {
			return err
		}
	}

	return vol.RestoreVolumeRestic(ctx, info, tgt.Value, snap, client, project, pool, destName, stdout)
}

func restoreVolumesFromRestic(cmd *cobra.Command, client incusapi.Client, tgt target.Target, project, version string, args []string, replace, skipExisting bool, stdout io.Writer) error {
	info, err := checkResticBinary(cmd, true)
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if err := restic.EnsureRepository(ctx, info, tgt.Value); err != nil {
		return err
	}

	items, err := volumeItemsFromArgs(ctx, info, tgt.Value, project, args)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	type row struct{ Action, Project, Pool, Name, TargetName, Version string }
	var rows []row
	for i := range items {
		it := &items[i]
		snap, err := findVolumeSnapshot(ctx, info, tgt.Value, project, it.pool, it.name, version)
		if err != nil {
			return err
		}
		exists, err := client.VolumeExists(project, it.pool, it.name)
		if err != nil {
			return err
		}
		action := "create"
		if exists {
			action = "conflict"
			if replace {
				action = "replace"
			}
			if skipExisting {
				action = "skip"
			}
		}
		rows = append(rows, row{Action: action, Project: project, Pool: it.pool, Name: it.name, TargetName: it.name, Version: volumeSnapshotTimestamp(snap)})
		it.snapshot = snap
		it.exists = exists
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tPROJECT\tPOOL\tNAME\tTARGET_NAME\tVERSION")
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Pool, r.Name, r.TargetName, r.Version)
	}
	_ = tw.Flush()

	opts := getSafetyOptions(cmd)
	if opts.DryRun {
		return nil
	}

	if !(replace || skipExisting) {
		ok, err := safety.Confirm(opts, cmd.InOrStdin(), stdout, fmt.Sprintf("Apply restore for %d volumes?", len(items)))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	for i, it := range items {
		fmt.Fprintf(stdout, "[%d/%d] Restoring volume %s/%s\n", i+1, len(items), it.pool, it.name)
		if it.exists {
			if skipExisting {
				fmt.Fprintf(stdout, "[%d/%d] Skip existing %s/%s\n", i+1, len(items), it.pool, it.name)
				continue
			}
			if err := client.DeleteVolume(project, it.pool, it.name); err != nil {
				return err
			}
		}
		if err := vol.RestoreVolumeRestic(ctx, info, tgt.Value, it.snapshot, client, project, it.pool, it.name, stdout); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "[%d/%d] Done %s/%s\n", i+1, len(items), it.pool, it.name)
	}
	return nil
}

type volumeItem struct {
	pool     string
	name     string
	snapshot restic.Snapshot
	exists   bool
}

func volumeItemsFromArgs(ctx context.Context, bin restic.BinaryInfo, repo, project string, args []string) ([]volumeItem, error) {
	var out []volumeItem
	if len(args) == 0 {
		snaps, err := restic.ListSnapshots(ctx, bin, repo, []string{"type=volume", "part=data", fmt.Sprintf("project=%s", project)})
		if err != nil {
			return nil, err
		}
		seen := map[string]struct{}{}
		for _, snap := range snaps {
			tags := snap.TagMap()
			pool := tags["pool"]
			name := tags["name"]
			if pool == "" || name == "" {
				continue
			}
			key := pool + "/" + name
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, volumeItem{pool: pool, name: name})
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].pool == out[j].pool {
				return out[i].name < out[j].name
			}
			return out[i].pool < out[j].pool
		})
		return out, nil
	}

	for _, arg := range args {
		parts := strings.SplitN(arg, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid volume spec %q (expected POOL/NAME)", arg)
		}
		out = append(out, volumeItem{pool: parts[0], name: parts[1]})
	}
	return out, nil
}

func findVolumeSnapshot(ctx context.Context, bin restic.BinaryInfo, repo, project, pool, name, desiredTs string) (restic.Snapshot, error) {
	tags := []string{
		"type=volume",
		"part=data",
		fmt.Sprintf("project=%s", project),
		fmt.Sprintf("pool=%s", pool),
		fmt.Sprintf("name=%s", name),
	}
	snaps, err := restic.ListSnapshots(ctx, bin, repo, tags)
	if err != nil {
		return restic.Snapshot{}, err
	}
	if len(snaps) == 0 {
		return restic.Snapshot{}, fmt.Errorf("no restic snapshots found for volume %s/%s", pool, name)
	}
	if desiredTs != "" {
		for _, s := range snaps {
			if volumeSnapshotTimestamp(s) == desiredTs {
				return s, nil
			}
		}
		return restic.Snapshot{}, fmt.Errorf("restic snapshot with timestamp %s not found for volume %s/%s", desiredTs, pool, name)
	}
	return snaps[len(snaps)-1], nil
}

func volumeSnapshotTimestamp(snap restic.Snapshot) string {
	if ts := snap.TagMap()["timestamp"]; ts != "" {
		return ts
	}
	return snap.Time.UTC().Format("20060102T150405Z")
}
