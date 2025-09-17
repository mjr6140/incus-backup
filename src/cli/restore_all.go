package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	cfg "incus-backup/src/backup/config"
	ibak "incus-backup/src/backup/instances"
	vbak "incus-backup/src/backup/volumes"
	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	"incus-backup/src/safety"
	"incus-backup/src/target"
)

func newRestoreAllCmd(stdout, stderr io.Writer) *cobra.Command {
	var project, version string
	var replace, skipExisting, applyConfig bool
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Restore config (optional apply), all volumes, and all instances",
		RunE: func(cmd *cobra.Command, args []string) error {
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return fmt.Errorf("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}
			if tgt.Scheme == "restic" {
				return restoreAllFromRestic(cmd, tgt, project, version, replace, skipExisting, applyConfig, stdout)
			}
			if tgt.Scheme != "dir" {
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}
			client, err := incusapi.ConnectLocal()
			if err != nil {
				return err
			}

			// Load config desired/current and plans
			cfgDir, err := resolveConfigSnapshotDir(tgt, version)
			if err != nil {
				return err
			}
			desiredProjects, _ := loadProjects(filepath.Join(cfgDir, "projects.json"))
			desiredNetworks, _ := loadNetworks(filepath.Join(cfgDir, "networks.json"))
			desiredPools, _ := loadStoragePools(filepath.Join(cfgDir, "storage_pools.json"))
			currentProjects, _ := client.ListProjects()
			currentNetworks, _ := client.ListNetworks()
			currentPools, _ := client.ListStoragePools()
			pplan := cfg.BuildProjectsPlan(currentProjects, desiredProjects)
			nplan := cfg.BuildNetworksPlan(currentNetworks, desiredNetworks)
			splan := cfg.BuildStoragePoolsPlan(currentPools, desiredPools)

			// Collect volumes
			var volItems [][2]string
			baseV := filepath.Join(tgt.DirPath, "volumes", project)
			if entries, err := os.ReadDir(baseV); err == nil {
				for _, p := range entries {
					if !p.IsDir() || strings.HasPrefix(p.Name(), ".") {
						continue
					}
					namesDir := filepath.Join(baseV, p.Name())
					if names, err := os.ReadDir(namesDir); err == nil {
						for _, n := range names {
							if n.IsDir() && !strings.HasPrefix(n.Name(), ".") {
								volItems = append(volItems, [2]string{p.Name(), n.Name()})
							}
						}
					}
				}
			}
			sort.Slice(volItems, func(i, j int) bool {
				if volItems[i][0] == volItems[j][0] {
					return volItems[i][1] < volItems[j][1]
				}
				return volItems[i][0] < volItems[j][0]
			})

			// Collect instances
			var instNames []string
			baseI := filepath.Join(tgt.DirPath, "instances", project)
			if entries, err := os.ReadDir(baseI); err == nil {
				for _, e := range entries {
					if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
						instNames = append(instNames, e.Name())
					}
				}
			}
			sort.Strings(instNames)

			// Preview tables
			fmt.Fprintln(stdout, "Config preview")
			renderProjectsPlan(stdout, pplan)
			renderNetworksPlan(stdout, nplan)
			renderStoragePoolsPlan(stdout, splan)

			// Volumes preview
			type vrow struct{ Action, Project, Pool, Name, Version string }
			var vrows []vrow
			for _, it := range volItems {
				pool, name := it[0], it[1]
				snapDir, err := resolveVolumeSnapshotDir(tgt, project, pool, name, version)
				if err != nil {
					return err
				}
				exists, _ := client.VolumeExists(project, pool, name)
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
				vrows = append(vrows, vrow{Action: action, Project: project, Pool: pool, Name: name, Version: filepath.Base(snapDir)})
			}
			tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTION\tPROJECT\tPOOL\tNAME\tVERSION")
			for _, r := range vrows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Pool, r.Name, r.Version)
			}
			_ = tw.Flush()

			// Instances preview
			type irow struct{ Action, Project, Name, Version string }
			var irows []irow
			for _, name := range instNames {
				snapDir, err := resolveInstanceSnapshotDir(tgt, project, name, version)
				if err != nil {
					return err
				}
				exists, _ := client.InstanceExists(project, name)
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
				irows = append(irows, irow{Action: action, Project: project, Name: name, Version: filepath.Base(snapDir)})
			}
			tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ACTION\tPROJECT\tNAME\tVERSION")
			for _, r := range irows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Name, r.Version)
			}
			_ = tw.Flush()

			opts := getSafetyOptions(cmd)
			if opts.DryRun {
				return nil
			}

			// Confirm
			ok, err := safety.Confirm(opts, os.Stdin, stdout, fmt.Sprintf("Apply restore for config (apply=%v), %d volumes, %d instances?", applyConfig, len(volItems), len(instNames)))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}

			// Apply config (optional)
			if applyConfig {
				// Vol deletions gated by --force already in restore_config
				renderProjectsPlan(stdout, pplan)
				renderNetworksPlan(stdout, nplan)
				renderStoragePoolsPlan(stdout, splan)
				var buf strings.Builder
				enc := json.NewEncoder(&buf)
				enc.SetIndent("", "  ")
				// Apply
				if sum, err := cfg.ApplyStoragePoolsPlan(client, splan, opts.Force); err != nil {
					return err
				} else {
					fmt.Fprintln(stdout, sum)
				}
				if sum, err := cfg.ApplyNetworksPlan(client, nplan, opts.Force); err != nil {
					return err
				} else {
					fmt.Fprintln(stdout, sum)
				}
				if sum, err := cfg.ApplyProjectsPlan(client, pplan); err != nil {
					return err
				} else {
					fmt.Fprintln(stdout, sum)
				}
			}

			// Volumes apply
			for i, it := range volItems {
				pool, name := it[0], it[1]
				snapDir, err := resolveVolumeSnapshotDir(tgt, project, pool, name, version)
				if err != nil {
					return err
				}
				exists, _ := client.VolumeExists(project, pool, name)
				fmt.Fprintf(stdout, "[vol %d/%d] %s/%s\n", i+1, len(volItems), pool, name)
				if exists {
					if skipExisting {
						fmt.Fprintf(stdout, "[vol %d/%d] skip existing\n", i+1, len(volItems))
						continue
					}
					if err := client.DeleteVolume(project, pool, name); err != nil {
						return err
					}
				}
				if err := vbak.RestoreVolume(client, snapDir, project, pool, name, stdout); err != nil {
					return err
				}
			}

			// Instances apply
			for i, name := range instNames {
				snapDir, err := resolveInstanceSnapshotDir(tgt, project, name, version)
				if err != nil {
					return err
				}
				exists, _ := client.InstanceExists(project, name)
				fmt.Fprintf(stdout, "[inst %d/%d] %s\n", i+1, len(instNames), name)
				if exists {
					if skipExisting {
						fmt.Fprintf(stdout, "[inst %d/%d] skip existing\n", i+1, len(instNames))
						continue
					}
					_ = client.StopInstance(project, name, true)
					if err := client.DeleteInstance(project, name); err != nil {
						return err
					}
				}
				if err := ibak.RestoreInstance(client, snapDir, project, name, stdout); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&project, "project", "default", "Incus project")
	cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest per item)")
	cmd.Flags().BoolVar(&replace, "replace", false, "Replace existing resources if they exist")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Skip resources that already exist")
	cmd.Flags().BoolVar(&applyConfig, "apply-config", false, "Apply declarative config changes from backup")
	return cmd
}

func restoreAllFromRestic(cmd *cobra.Command, tgt target.Target, project, version string, replace, skipExisting, applyConfig bool, stdout io.Writer) error {
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
	client, err := incusapi.ConnectLocal()
	if err != nil {
		return err
	}

	configData, err := cfg.LoadSnapshotRestic(ctx, info, tgt.Value, version)
	if err != nil {
		return err
	}
	currentProjects, err := client.ListProjects()
	if err != nil {
		return err
	}
	projectPlan := cfg.BuildProjectsPlan(currentProjects, configData.Projects)
	currentNetworks, err := client.ListNetworks()
	if err != nil {
		return err
	}
	networkPlan := cfg.BuildNetworksPlan(currentNetworks, configData.Networks)
	currentPools, err := client.ListStoragePools()
	if err != nil {
		return err
	}
	poolPlan := cfg.BuildStoragePoolsPlan(currentPools, configData.StoragePools)

	volItems, err := volumeItemsFromArgs(ctx, info, tgt.Value, project, nil)
	if err != nil {
		return err
	}
	for i := range volItems {
		it := &volItems[i]
		snap, err := findVolumeSnapshot(ctx, info, tgt.Value, project, it.pool, it.name, version)
		if err != nil {
			return err
		}
		it.snapshot = snap
		exists, err := client.VolumeExists(project, it.pool, it.name)
		if err != nil {
			return err
		}
		it.exists = exists
	}

	type instItem struct {
		name     string
		snapshot restic.Snapshot
		exists   bool
	}
	var instItems []instItem
	instNames, err := listInstanceNames(ctx, info, tgt.Value, project)
	if err != nil {
		return err
	}
	for _, name := range instNames {
		snap, err := findInstanceSnapshot(ctx, info, tgt.Value, project, name, version)
		if err != nil {
			return err
		}
		exists, err := client.InstanceExists(project, name)
		if err != nil {
			return err
		}
		instItems = append(instItems, instItem{name: name, snapshot: snap, exists: exists})
	}

	fmt.Fprintln(stdout, "Config preview")
	renderProjectsPlan(stdout, projectPlan)
	renderNetworksPlan(stdout, networkPlan)
	renderStoragePoolsPlan(stdout, poolPlan)

	type vrow struct{ Action, Project, Pool, Name, Version string }
	var vrows []vrow
	for _, it := range volItems {
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
		vrows = append(vrows, vrow{Action: action, Project: project, Pool: it.pool, Name: it.name, Version: volumeSnapshotTimestamp(it.snapshot)})
	}
	tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tPROJECT\tPOOL\tNAME\tVERSION")
	for _, r := range vrows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Pool, r.Name, r.Version)
	}
	_ = tw.Flush()

	type irow struct{ Action, Project, Name, Version string }
	var irows []irow
	for _, it := range instItems {
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
		irows = append(irows, irow{Action: action, Project: project, Name: it.name, Version: snapshotTimestamp(it.snapshot)})
	}
	tw = tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "ACTION\tPROJECT\tNAME\tVERSION")
	for _, r := range irows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Action, r.Project, r.Name, r.Version)
	}
	_ = tw.Flush()

	opts := getSafetyOptions(cmd)
	if opts.DryRun {
		return nil
	}

	ok, err := safety.Confirm(opts, cmd.InOrStdin(), stdout, fmt.Sprintf("Apply restore for config (apply=%v), %d volumes, %d instances?", applyConfig, len(volItems), len(instItems)))
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	if applyConfig {
		renderProjectsPlan(stdout, projectPlan)
		renderNetworksPlan(stdout, networkPlan)
		renderStoragePoolsPlan(stdout, poolPlan)

		if sum, err := cfg.ApplyStoragePoolsPlan(client, poolPlan, opts.Force); err != nil {
			return err
		} else if sum != "" {
			fmt.Fprintln(stdout, sum)
		}
		if sum, err := cfg.ApplyNetworksPlan(client, networkPlan, opts.Force); err != nil {
			return err
		} else if sum != "" {
			fmt.Fprintln(stdout, sum)
		}
		if sum, err := cfg.ApplyProjectsPlan(client, projectPlan); err != nil {
			return err
		} else if sum != "" {
			fmt.Fprintln(stdout, sum)
		}
	}

	for i, it := range volItems {
		fmt.Fprintf(stdout, "[vol %d/%d] %s/%s\n", i+1, len(volItems), it.pool, it.name)
		if it.exists {
			if skipExisting {
				fmt.Fprintf(stdout, "[vol %d/%d] skip existing\n", i+1, len(volItems))
				continue
			}
			if err := client.DeleteVolume(project, it.pool, it.name); err != nil {
				return err
			}
		}
		if err := vbak.RestoreVolumeRestic(ctx, info, tgt.Value, it.snapshot, client, project, it.pool, it.name, stdout); err != nil {
			return err
		}
	}

	for i, it := range instItems {
		fmt.Fprintf(stdout, "[inst %d/%d] %s\n", i+1, len(instItems), it.name)
		if it.exists {
			if skipExisting {
				fmt.Fprintf(stdout, "[inst %d/%d] skip existing\n", i+1, len(instItems))
				continue
			}
			_ = client.StopInstance(project, it.name, true)
			if err := client.DeleteInstance(project, it.name); err != nil {
				return err
			}
		}
		if err := ibak.RestoreInstanceRestic(ctx, info, tgt.Value, it.snapshot, client, project, it.name, it.name, stdout); err != nil {
			return err
		}
	}

	return nil
}
