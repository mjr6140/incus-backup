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

	"github.com/spf13/cobra"

	cfg "incus-backup/src/backup/config"
	"incus-backup/src/incusapi"
	"incus-backup/src/safety"
	"incus-backup/src/target"
)

type configSnapshot struct {
	Timestamp    string
	Projects     []incusapi.Project
	Profiles     []incusapi.Profile
	Networks     []incusapi.Network
	StoragePools []incusapi.StoragePool
}

func newRestoreConfigCmd(stdout, stderr io.Writer) *cobra.Command {
	var version, output string
	var apply bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Preview or apply declarative config from a backup",
		RunE: func(cmd *cobra.Command, args []string) error {
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return fmt.Errorf("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}

			snap, err := loadConfigSnapshot(cmd, tgt, version)
			if err != nil {
				return err
			}

			client, err := incusapi.ConnectLocal()
			if err != nil {
				return err
			}
			currentProjects, err := client.ListProjects()
			if err != nil {
				return err
			}
			projectPlan := cfg.BuildProjectsPlan(currentProjects, snap.Projects)

			currentNetworks, err := client.ListNetworks()
			if err != nil {
				return err
			}
			networkPlan := cfg.BuildNetworksPlan(currentNetworks, snap.Networks)

			currentPools, err := client.ListStoragePools()
			if err != nil {
				return err
			}
			poolPlan := cfg.BuildStoragePoolsPlan(currentPools, snap.StoragePools)

			if !apply {
				switch output {
				case "json":
					enc := json.NewEncoder(stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(struct {
						Projects     cfg.ProjectPlan     `json:"projects"`
						Networks     cfg.NetworkPlan     `json:"networks"`
						StoragePools cfg.StoragePoolPlan `json:"storage_pools"`
					}{projectPlan, networkPlan, poolPlan})
				case "table", "":
					renderProjectsPlan(stdout, projectPlan)
					renderNetworksPlan(stdout, networkPlan)
					renderStoragePoolsPlan(stdout, poolPlan)
					return nil
				default:
					return fmt.Errorf("unsupported --output: %s", output)
				}
			}

			opts := getSafetyOptions(cmd)
			if opts.DryRun {
				renderProjectsPlan(stdout, projectPlan)
				renderNetworksPlan(stdout, networkPlan)
				renderStoragePoolsPlan(stdout, poolPlan)
				return nil
			}

			renderProjectsPlan(stdout, projectPlan)
			renderNetworksPlan(stdout, networkPlan)
			renderStoragePoolsPlan(stdout, poolPlan)

			var buf strings.Builder
			buf.WriteString("Apply config changes? (networks/storage pools may disrupt running workloads)\n")
			buf.WriteString(fmt.Sprintf("Projects => Create: %d, Update: %d, Delete: %d\n", len(projectPlan.ToCreate), len(projectPlan.ToUpdate), len(projectPlan.ToDelete)))
			buf.WriteString(fmt.Sprintf("Networks => Create: %d, Update: %d, Delete: %d\n", len(networkPlan.ToCreate), len(networkPlan.ToUpdate), len(networkPlan.ToDelete)))
			buf.WriteString(fmt.Sprintf("Storage Pools => Create: %d, Update: %d, Delete: %d\n", len(poolPlan.ToCreate), len(poolPlan.ToUpdate), len(poolPlan.ToDelete)))
			ok, err := safety.Confirm(opts, os.Stdin, stdout, buf.String())
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}

			spCreated, spUpdated, spDeleted := 0, 0, 0
			for _, p := range poolPlan.ToCreate {
				fmt.Fprintf(stdout, "[storage] create %s\n", p.Name)
				if err := client.CreateStoragePool(p); err != nil {
					return err
				}
				spCreated++
			}
			for _, u := range poolPlan.ToUpdate {
				fmt.Fprintf(stdout, "[storage] update %s\n", u.Name)
				if err := client.UpdateStoragePool(incusapi.StoragePool{Name: u.Name, Config: u.DesiredConf}); err != nil {
					return err
				}
				spUpdated++
			}
			if opts.Force {
				for _, p := range poolPlan.ToDelete {
					fmt.Fprintf(stdout, "[storage] delete %s\n", p.Name)
					if err := client.DeleteStoragePool(p.Name); err != nil {
						return err
					}
					spDeleted++
				}
			}
			fmt.Fprintf(stdout, "storage_pools: created=%d updated=%d deleted=%d\n", spCreated, spUpdated, spDeleted)

			netCreated, netUpdated, netDeleted := 0, 0, 0
			for _, n := range networkPlan.ToCreate {
				fmt.Fprintf(stdout, "[networks] create %s\n", n.Name)
				if err := client.CreateNetwork(n); err != nil {
					return err
				}
				netCreated++
			}
			for _, u := range networkPlan.ToUpdate {
				fmt.Fprintf(stdout, "[networks] update %s\n", u.Name)
				if err := client.UpdateNetwork(incusapi.Network{Name: u.Name, Config: u.DesiredConf}); err != nil {
					return err
				}
				netUpdated++
			}
			if opts.Force {
				for _, n := range networkPlan.ToDelete {
					fmt.Fprintf(stdout, "[networks] delete %s\n", n.Name)
					if err := client.DeleteNetwork(n.Name); err != nil {
						return err
					}
					netDeleted++
				}
			}
			fmt.Fprintf(stdout, "networks: created=%d updated=%d deleted=%d\n", netCreated, netUpdated, netDeleted)

			prCreated, prUpdated, prDeleted := 0, 0, 0
			for _, p := range projectPlan.ToCreate {
				fmt.Fprintf(stdout, "[projects] create %s\n", p.Name)
				if err := client.CreateProject(p.Name, p.Config); err != nil {
					return err
				}
				prCreated++
			}
			for _, u := range projectPlan.ToUpdate {
				fmt.Fprintf(stdout, "[projects] update %s\n", u.Name)
				if err := client.UpdateProject(u.Name, u.Desired); err != nil {
					return err
				}
				prUpdated++
			}
			for _, p := range projectPlan.ToDelete {
				fmt.Fprintf(stdout, "[projects] delete %s\n", p.Name)
				if err := client.DeleteProject(p.Name); err != nil {
					return err
				}
				prDeleted++
			}
			fmt.Fprintf(stdout, "projects: created=%d updated=%d deleted=%d\n", prCreated, prUpdated, prDeleted)
			return nil
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVar(&version, "version", "", "Snapshot timestamp (default: latest)")
	cmd.Flags().BoolVar(&apply, "apply", false, "Apply changes (default: preview)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json")
	return cmd
}

func loadConfigSnapshot(cmd *cobra.Command, tgt target.Target, version string) (configSnapshot, error) {
	switch tgt.Scheme {
	case "dir":
		dir, err := resolveConfigSnapshotDir(tgt, version)
		if err != nil {
			return configSnapshot{}, err
		}
		return loadConfigSnapshotDir(dir)
	case "restic":
		info, err := checkResticBinary(cmd, true)
		if err != nil {
			return configSnapshot{}, err
		}
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		data, err := cfg.LoadSnapshotRestic(ctx, info, tgt.Value, version)
		if err != nil {
			return configSnapshot{}, err
		}
		return configSnapshot{
			Timestamp:    data.Timestamp,
			Projects:     data.Projects,
			Profiles:     data.Profiles,
			Networks:     data.Networks,
			StoragePools: data.StoragePools,
		}, nil
	default:
		return configSnapshot{}, fmt.Errorf("unsupported backend: %s", tgt.Scheme)
	}
}

func loadConfigSnapshotDir(dir string) (configSnapshot, error) {
	projects, err := loadProjects(filepath.Join(dir, "projects.json"))
	if err != nil {
		return configSnapshot{}, err
	}
	profiles, err := loadProfiles(filepath.Join(dir, "profiles.json"))
	if err != nil {
		return configSnapshot{}, err
	}
	networks, err := loadNetworks(filepath.Join(dir, "networks.json"))
	if err != nil {
		return configSnapshot{}, err
	}
	pools, err := loadStoragePools(filepath.Join(dir, "storage_pools.json"))
	if err != nil {
		return configSnapshot{}, err
	}
	return configSnapshot{
		Timestamp:    filepath.Base(dir),
		Projects:     projects,
		Profiles:     profiles,
		Networks:     networks,
		StoragePools: pools,
	}, nil
}

func resolveConfigSnapshotDir(tgt target.Target, version string) (string, error) {
	base := filepath.Join(tgt.DirPath, "config")
	if version != "" {
		return filepath.Join(base, version), nil
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return "", err
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}
	if len(names) == 0 {
		return "", fmt.Errorf("no config snapshots found under %s", base)
	}
	sort.Strings(names)
	return filepath.Join(base, names[len(names)-1]), nil
}

func renderProjectsPlan(w io.Writer, p cfg.ProjectPlan) {
	fmt.Fprintf(w, "Config preview (projects)\n")
	fmt.Fprintf(w, "Create: %d\n", len(p.ToCreate))
	for _, c := range p.ToCreate {
		fmt.Fprintf(w, "  + %s\n", c.Name)
	}
	fmt.Fprintf(w, "Update: %d\n", len(p.ToUpdate))
	for _, u := range p.ToUpdate {
		fmt.Fprintf(w, "  ~ %s\n", u.Name)
	}
	fmt.Fprintf(w, "Delete: %d\n", len(p.ToDelete))
	for _, d := range p.ToDelete {
		fmt.Fprintf(w, "  - %s\n", d.Name)
	}
}

func renderNetworksPlan(w io.Writer, p cfg.NetworkPlan) {
	fmt.Fprintf(w, "Config preview (networks)\n")
	fmt.Fprintf(w, "Create: %d\n", len(p.ToCreate))
	for _, c := range p.ToCreate {
		fmt.Fprintf(w, "  + %s\n", c.Name)
	}
	fmt.Fprintf(w, "Update: %d\n", len(p.ToUpdate))
	for _, u := range p.ToUpdate {
		fmt.Fprintf(w, "  ~ %s\n", u.Name)
	}
	fmt.Fprintf(w, "Delete: %d\n", len(p.ToDelete))
	for _, d := range p.ToDelete {
		fmt.Fprintf(w, "  - %s\n", d.Name)
	}
}

func renderStoragePoolsPlan(w io.Writer, p cfg.StoragePoolPlan) {
	fmt.Fprintf(w, "Config preview (storage pools)\n")
	fmt.Fprintf(w, "Create: %d\n", len(p.ToCreate))
	for _, c := range p.ToCreate {
		fmt.Fprintf(w, "  + %s\n", c.Name)
	}
	fmt.Fprintf(w, "Update: %d\n", len(p.ToUpdate))
	for _, u := range p.ToUpdate {
		fmt.Fprintf(w, "  ~ %s\n", u.Name)
	}
	fmt.Fprintf(w, "Delete: %d\n", len(p.ToDelete))
	for _, d := range p.ToDelete {
		fmt.Fprintf(w, "  - %s\n", d.Name)
	}
}

func loadProjects(path string) ([]incusapi.Project, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []incusapi.Project
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadProfiles(path string) ([]incusapi.Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []incusapi.Profile
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadNetworks(path string) ([]incusapi.Network, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []incusapi.Network
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func loadStoragePools(path string) ([]incusapi.StoragePool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []incusapi.StoragePool
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
