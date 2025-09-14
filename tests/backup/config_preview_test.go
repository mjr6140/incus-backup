package backup_test

import (
    "testing"

    cfg "incus-backup/src/backup/config"
    "incus-backup/src/incusapi"
)

func TestBuildProjectsPlan(t *testing.T) {
    current := []incusapi.Project{{Name: "alpha", Config: map[string]string{"a": "1"}}, {Name: "beta", Config: map[string]string{"b": "2"}}}
    desired := []incusapi.Project{{Name: "alpha", Config: map[string]string{"a": "1", "x": "9"}}, {Name: "gamma", Config: map[string]string{"g": "3"}}}

    plan := cfg.BuildProjectsPlan(current, desired)
    if len(plan.ToCreate) != 1 || plan.ToCreate[0].Name != "gamma" {
        t.Fatalf("unexpected create set: %+v", plan.ToCreate)
    }
    if len(plan.ToDelete) != 1 || plan.ToDelete[0].Name != "beta" {
        t.Fatalf("unexpected delete set: %+v", plan.ToDelete)
    }
    if len(plan.ToUpdate) != 1 || plan.ToUpdate[0].Name != "alpha" {
        t.Fatalf("unexpected update set: %+v", plan.ToUpdate)
    }
}

func TestBuildNetworksPlan(t *testing.T) {
    current := []incusapi.Network{{Name: "br0", Type: "bridge", Config: map[string]string{"ipv4.address": "auto"}}}
    desired := []incusapi.Network{{Name: "br0", Type: "bridge", Config: map[string]string{"ipv4.address": "10.0.3.1/24"}}, {Name: "br1", Type: "bridge"}}
    plan := cfg.BuildNetworksPlan(current, desired)
    if len(plan.ToCreate) != 1 || plan.ToCreate[0].Name != "br1" { t.Fatalf("unexpected create: %+v", plan.ToCreate) }
    if len(plan.ToUpdate) != 1 || plan.ToUpdate[0].Name != "br0" { t.Fatalf("unexpected update: %+v", plan.ToUpdate) }
    if len(plan.ToDelete) != 0 { t.Fatalf("unexpected delete: %+v", plan.ToDelete) }
}

func TestBuildStoragePoolsPlan(t *testing.T) {
    current := []incusapi.StoragePool{{Name: "default", Driver: "dir", Config: map[string]string{"source": "/var/lib/incus/storage-pools/default"}}}
    desired := []incusapi.StoragePool{{Name: "default", Driver: "dir", Config: map[string]string{"source": "/data/incus/default"}}, {Name: "fast", Driver: "zfs"}}
    plan := cfg.BuildStoragePoolsPlan(current, desired)
    if len(plan.ToCreate) != 1 || plan.ToCreate[0].Name != "fast" { t.Fatalf("unexpected create: %+v", plan.ToCreate) }
    if len(plan.ToUpdate) != 1 || plan.ToUpdate[0].Name != "default" { t.Fatalf("unexpected update: %+v", plan.ToUpdate) }
}
