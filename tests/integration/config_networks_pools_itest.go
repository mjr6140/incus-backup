//go:build integration

package integration

import (
    "bytes"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "testing"
    "time"

    "incus-backup/src/cli"
    "incus-backup/src/incusapi"
)

func TestConfigBackupPreviewAndApply_NetworksAndPools(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" {
        t.Skip("INCUS_TESTS=1 not set; skipping integration test")
    }

    client, err := incusapi.ConnectLocal()
    if err != nil { t.Fatalf("connect incus: %v", err) }

    ts := time.Now().UTC().Format("20060102T150405")
    netName := "itestbr-" + ts
    poolName := "itestpool-" + ts

    // Create isolated resources
    if err := client.CreateNetwork(incusapi.Network{Name: netName, Type: "bridge", Config: map[string]string{"ipv4.address": "auto"}}); err != nil {
        t.Fatalf("create network: %v", err)
    }
    t.Cleanup(func() { _ = client.DeleteNetwork(netName) })

    if err := client.CreateStoragePool(incusapi.StoragePool{Name: poolName, Driver: "dir"}); err != nil {
        t.Fatalf("create storage pool: %v", err)
    }
    t.Cleanup(func() { _ = client.DeleteStoragePool(poolName) })

    root := t.TempDir()

    // Backup all config
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"backup", "config", "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("backup config: %v; stderr=%s", err, stderr.String())
        }
    }
    // Locate latest snapshot
    cfgDir := filepath.Join(root, "config")
    entries, err := os.ReadDir(cfgDir)
    if err != nil { t.Fatalf("read config dir: %v", err) }
    var snaps []string
    for _, e := range entries { if e.IsDir() && !strings.HasPrefix(e.Name(), ".") { snaps = append(snaps, e.Name()) } }
    if len(snaps) == 0 { t.Fatalf("no config snapshots under %s", cfgDir) }
    sort.Strings(snaps)
    latest := snaps[len(snaps)-1]
    for _, f := range []string{"networks.json", "storage_pools.json"} {
        if _, err := os.Stat(filepath.Join(cfgDir, latest, f)); err != nil {
            t.Fatalf("missing %s: %v", f, err)
        }
    }

    // Delete resources to create drift
    if err := client.DeleteNetwork(netName); err != nil { t.Fatalf("delete network: %v", err) }
    if err := client.DeleteStoragePool(poolName); err != nil { t.Fatalf("delete storage pool: %v", err) }

    // Preview should show create for both
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"restore", "config", "--target", "dir:" + root})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore config preview: %v; stderr=%s", err, stderr.String())
        }
        s := out.String()
        if !strings.Contains(s, netName) {
            t.Fatalf("expected preview to mention network %s; got:\n%s", netName, s)
        }
        if !strings.Contains(s, poolName) {
            t.Fatalf("expected preview to mention storage pool %s; got:\n%s", poolName, s)
        }
    }

    // Apply changes non-interactively (force to allow any deletions if needed)
    {
        var out, stderr bytes.Buffer
        cmd := cli.NewRootCmd(&out, &stderr)
        cmd.SetArgs([]string{"restore", "config", "--target", "dir:" + root, "--apply", "-y", "--force"})
        if _, err := cmd.ExecuteC(); err != nil {
            t.Fatalf("restore config --apply: %v; stderr=%s", err, stderr.String())
        }
        so := out.String()
        if !strings.Contains(so, "networks: created=") || !strings.Contains(so, "storage_pools: created=") {
            t.Fatalf("expected apply summary for networks and pools; got:\n%s", so)
        }
    }

    // Verify recreated
    nets, err := client.ListNetworks()
    if err != nil { t.Fatalf("list networks: %v", err) }
    pools, err := client.ListStoragePools()
    if err != nil { t.Fatalf("list pools: %v", err) }
    foundNet := false
    for _, n := range nets { if n.Name == netName { foundNet = true; break } }
    if !foundNet { t.Fatalf("expected network %s recreated", netName) }
    foundPool := false
    for _, p := range pools { if p.Name == poolName { foundPool = true; break } }
    if !foundPool { t.Fatalf("expected pool %s recreated", poolName) }
}

