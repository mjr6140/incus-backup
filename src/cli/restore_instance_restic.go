package cli

import (
	"context"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	inst "incus-backup/src/backup/instances"
	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	"incus-backup/src/safety"
	"incus-backup/src/target"

	"github.com/spf13/cobra"
)

func restoreInstanceFromRestic(cmd *cobra.Command, client incusapi.Client, tgt target.Target, project, name, version, targetName string, replace, skipExisting bool, stdout io.Writer) error {
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

	snap, err := findInstanceSnapshot(ctx, info, tgt.Value, project, name, version)
	if err != nil {
		return err
	}

	destName := targetName
	if destName == "" {
		destName = name
	}
	exists, err := client.InstanceExists(project, destName)
	if err != nil {
		return err
	}

	opts := getSafetyOptions(cmd)
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

	renderInstanceRestorePreview(stdout, []instancePreviewRow{{
		Action:     action,
		Project:    project,
		Name:       name,
		TargetName: destName,
		Version:    snapshotTimestamp(snap),
	}})

	if opts.DryRun {
		return nil
	}

	if exists {
		if skipExisting {
			return nil
		}
		if !replace {
			ok, err := safety.Confirm(opts, cmd.InOrStdin(), stdout, fmt.Sprintf("Instance %s already exists in project %s. Replace it?", destName, project))
			if err != nil || !ok {
				return err
			}
		}
		_ = client.StopInstance(project, destName, true)
		if err := client.DeleteInstance(project, destName); err != nil {
			return err
		}
	}

	return inst.RestoreInstanceRestic(ctx, info, tgt.Value, snap, client, project, name, destName, stdout)
}

func snapshotTimestamp(snap restic.Snapshot) string {
	if ts := snap.TagMap()["timestamp"]; ts != "" {
		return ts
	}
	return snap.Time.UTC().Format("20060102T150405Z")
}

func findInstanceSnapshot(ctx context.Context, bin restic.BinaryInfo, repo, project, name, desiredTs string) (restic.Snapshot, error) {
	tags := []string{
		"type=instance",
		"part=data",
		fmt.Sprintf("project=%s", project),
		fmt.Sprintf("name=%s", name),
	}
	snaps, err := restic.ListSnapshots(ctx, bin, repo, tags)
	if err != nil {
		return restic.Snapshot{}, err
	}
	if len(snaps) == 0 {
		return restic.Snapshot{}, fmt.Errorf("no restic snapshots found for instance %s", name)
	}
	if desiredTs != "" {
		for _, s := range snaps {
			if snapshotTimestamp(s) == desiredTs {
				return s, nil
			}
		}
		return restic.Snapshot{}, fmt.Errorf("restic snapshot with timestamp %s not found for instance %s", desiredTs, name)
	}
	return snaps[len(snaps)-1], nil
}

func restoreInstancesFromRestic(cmd *cobra.Command, client incusapi.Client, tgt target.Target, project, version string, namesArg []string, replace, skipExisting bool, stdout io.Writer) error {
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

	names := namesArg
	if len(names) == 0 {
		derived, err := listInstanceNames(ctx, info, tgt.Value, project)
		if err != nil {
			return err
		}
		names = derived
	}
	sort.Strings(names)
	if len(names) == 0 {
		return nil
	}

	type item struct {
		name     string
		snapshot restic.Snapshot
		exists   bool
	}
	var items []item
	for _, name := range names {
		snap, err := findInstanceSnapshot(ctx, info, tgt.Value, project, name, version)
		if err != nil {
			return err
		}
		exists, err := client.InstanceExists(project, name)
		if err != nil {
			return err
		}
		items = append(items, item{name: name, snapshot: snap, exists: exists})
	}

	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tPROJECT\tNAME\tTARGET_NAME\tVERSION")
	for _, it := range items {
		action := "create"
		if it.exists {
			action = "conflict"
			if replace {
				action = "replace"
			}
			if skipExisting {
				action = "skip"
			}
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", action, project, it.name, it.name, snapshotTimestamp(it.snapshot))
	}
	_ = tw.Flush()

	opts := getSafetyOptions(cmd)
	if opts.DryRun {
		return nil
	}

	if !(replace || skipExisting) {
		ok, err := safety.Confirm(opts, cmd.InOrStdin(), stdout, fmt.Sprintf("Apply restore for %d instances?", len(items)))
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
	}

	for i, it := range items {
		fmt.Fprintf(stdout, "[%d/%d] Restoring instance %s/%s\n", i+1, len(items), project, it.name)
		if it.exists {
			if skipExisting {
				fmt.Fprintf(stdout, "[%d/%d] Skip existing %s/%s\n", i+1, len(items), project, it.name)
				continue
			}
			_ = client.StopInstance(project, it.name, true)
			if err := client.DeleteInstance(project, it.name); err != nil {
				return err
			}
		}
		if err := inst.RestoreInstanceRestic(ctx, info, tgt.Value, it.snapshot, client, project, it.name, it.name, stdout); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "[%d/%d] Done %s/%s\n", i+1, len(items), project, it.name)
	}
	return nil
}

func listInstanceNames(ctx context.Context, bin restic.BinaryInfo, repo, project string) ([]string, error) {
	snaps, err := restic.ListSnapshots(ctx, bin, repo, []string{
		"type=instance",
		"part=data",
		fmt.Sprintf("project=%s", project),
	})
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, snap := range snaps {
		if name := snap.TagMap()["name"]; name != "" {
			seen[name] = struct{}{}
		}
	}
	var out []string
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
