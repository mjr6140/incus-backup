//go:build integration

package integration

import (
    "bytes"
    "os"
    "testing"
    "time"

    "incus-backup/src/cli"
    "incus-backup/src/incusapi"
    restictest "incus-backup/tests/internal/restictest"
)

func TestResticConfigBackupAndRestore(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" {
        t.Skip("INCUS_TESTS=1 not set")
    }

    restictest.RequireBinary(t)
    repoParent := t.TempDir()
    repo := restictest.InitRepo(t, repoParent)
    t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

    client, err := incusapi.ConnectLocal()
    if err != nil {
        t.Fatalf("connect incus: %v", err)
    }

    ts := time.Now().UTC().Format("20060102T150405")
    netName := "rcfg-net-" + ts
    poolName := "rcfg-pool-" + ts

    if err := client.CreateNetwork(incusapi.Network{Name: netName, Type: "bridge", Managed: true}); err != nil {
        t.Fatalf("create network: %v", err)
    }
    t.Cleanup(func() { _ = client.DeleteNetwork(netName) })

    if err := client.CreateStoragePool(incusapi.StoragePool{Name: poolName, Driver: "dir"}); err != nil {
        t.Fatalf("create storage pool: %v", err)
    }
    t.Cleanup(func() { _ = client.DeleteStoragePool(poolName) })

    var out, errBuf bytes.Buffer
    cmd := cli.NewRootCmd(&out, &errBuf)
    cmd.SetArgs([]string{"backup", "config", "--target", "restic:" + repo})
    if _, err := cmd.ExecuteC(); err != nil {
        t.Fatalf("restic config backup failed: %v; stderr=%s", err, errBuf.String())
    }

    if err := client.DeleteNetwork(netName); err != nil {
        t.Fatalf("delete network: %v", err)
    }
    if err := client.DeleteStoragePool(poolName); err != nil {
        t.Fatalf("delete storage pool: %v", err)
    }

    out.Reset()
    errBuf.Reset()
    cmd = cli.NewRootCmd(&out, &errBuf)
    cmd.SetArgs([]string{"restore", "config", "--target", "restic:" + repo, "--apply", "-y", "--force"})
    if _, err := cmd.ExecuteC(); err != nil {
        t.Fatalf("restic config restore failed: %v; stderr=%s", err, errBuf.String())
    }

    nets, err := client.ListNetworks()
    if err != nil {
        t.Fatalf("list networks: %v", err)
    }
    if !containsNetwork(nets, netName) {
        t.Fatalf("expected network %s restored", netName)
    }

    pools, err := client.ListStoragePools()
    if err != nil {
        t.Fatalf("list pools: %v", err)
    }
    if !containsPool(pools, poolName) {
        t.Fatalf("expected storage pool %s restored", poolName)
    }
}

func containsNetwork(nets []incusapi.Network, name string) bool {
    for _, n := range nets {
        if n.Name == name {
            return true
        }
    }
    return false
}

func containsPool(pools []incusapi.StoragePool, name string) bool {
    for _, p := range pools {
        if p.Name == name {
            return true
        }
    }
    return false
}

