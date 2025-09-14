package config

import (
    "sort"

    "incus-backup/src/incusapi"
)

// ProjectPlan describes differences between current and desired projects.
type ProjectPlan struct {
    ToCreate []incusapi.Project
    ToDelete []incusapi.Project
    ToUpdate []ProjectUpdate
}

type ProjectUpdate struct {
    Name    string
    Current map[string]string
    Desired map[string]string
}

// BuildProjectsPlan computes a plan to transform current -> desired projects.
// It treats a missing key and an empty value equivalently when comparing.
func BuildProjectsPlan(current, desired []incusapi.Project) ProjectPlan {
    cur := map[string]incusapi.Project{}
    des := map[string]incusapi.Project{}
    for _, p := range current {
        cur[p.Name] = p
    }
    for _, p := range desired {
        des[p.Name] = p
    }

    var plan ProjectPlan

    // Creations and updates
    for name, want := range des {
        have, exists := cur[name]
        if !exists {
            plan.ToCreate = append(plan.ToCreate, want)
            continue
        }
        if !equalConfig(have.Config, want.Config) {
            plan.ToUpdate = append(plan.ToUpdate, ProjectUpdate{
                Name:    name,
                Current: copyMap(have.Config),
                Desired: copyMap(want.Config),
            })
        }
    }
    // Deletions
    for name, have := range cur {
        if _, exists := des[name]; !exists {
            plan.ToDelete = append(plan.ToDelete, have)
        }
    }

    // Deterministic ordering
    sort.Slice(plan.ToCreate, func(i, j int) bool { return plan.ToCreate[i].Name < plan.ToCreate[j].Name })
    sort.Slice(plan.ToDelete, func(i, j int) bool { return plan.ToDelete[i].Name < plan.ToDelete[j].Name })
    sort.Slice(plan.ToUpdate, func(i, j int) bool { return plan.ToUpdate[i].Name < plan.ToUpdate[j].Name })
    return plan
}

func equalConfig(a, b map[string]string) bool {
    if len(a) != len(b) {
        // Not definitive; account for nil vs empty
        // Continue with key-by-key
    }
    if len(a) == 0 && len(b) == 0 {
        return true
    }
    for k, va := range a {
        if vb, ok := b[k]; !ok || va != vb {
            return false
        }
    }
    for k := range b {
        if _, ok := a[k]; !ok {
            return false
        }
    }
    return true
}

func copyMap(m map[string]string) map[string]string {
    if m == nil {
        return nil
    }
    out := make(map[string]string, len(m))
    for k, v := range m {
        out[k] = v
    }
    return out
}

// NetworkPlan describes differences for networks.
type NetworkPlan struct {
    ToCreate []incusapi.Network
    ToDelete []incusapi.Network
    ToUpdate []NetworkUpdate
}

type NetworkUpdate struct {
    Name        string
    CurrentConf map[string]string
    DesiredConf map[string]string
    // Note: Type changes are not supported; treat as recreate manually.
}

func BuildNetworksPlan(current, desired []incusapi.Network) NetworkPlan {
    cur := map[string]incusapi.Network{}
    des := map[string]incusapi.Network{}
    for _, n := range current { cur[n.Name] = n }
    for _, n := range desired { des[n.Name] = n }
    var plan NetworkPlan
    for name, want := range des {
        have, ok := cur[name]
        if !ok { plan.ToCreate = append(plan.ToCreate, want); continue }
        if have.Type != want.Type { // type change requires manual intervention
            plan.ToUpdate = append(plan.ToUpdate, NetworkUpdate{Name: name, CurrentConf: have.Config, DesiredConf: want.Config})
            continue
        }
        if !equalConfig(have.Config, want.Config) || have.Description != want.Description {
            plan.ToUpdate = append(plan.ToUpdate, NetworkUpdate{Name: name, CurrentConf: have.Config, DesiredConf: want.Config})
        }
    }
    for name, have := range cur {
        if _, ok := des[name]; !ok { plan.ToDelete = append(plan.ToDelete, have) }
    }
    sort.Slice(plan.ToCreate, func(i, j int) bool { return plan.ToCreate[i].Name < plan.ToCreate[j].Name })
    sort.Slice(plan.ToDelete, func(i, j int) bool { return plan.ToDelete[i].Name < plan.ToDelete[j].Name })
    sort.Slice(plan.ToUpdate, func(i, j int) bool { return plan.ToUpdate[i].Name < plan.ToUpdate[j].Name })
    return plan
}

// StoragePoolPlan describes differences for storage pools.
type StoragePoolPlan struct {
    ToCreate []incusapi.StoragePool
    ToDelete []incusapi.StoragePool
    ToUpdate []StoragePoolUpdate
}

type StoragePoolUpdate struct {
    Name        string
    CurrentConf map[string]string
    DesiredConf map[string]string
}

func BuildStoragePoolsPlan(current, desired []incusapi.StoragePool) StoragePoolPlan {
    cur := map[string]incusapi.StoragePool{}
    des := map[string]incusapi.StoragePool{}
    for _, p := range current { cur[p.Name] = p }
    for _, p := range desired { des[p.Name] = p }
    var plan StoragePoolPlan
    for name, want := range des {
        have, ok := cur[name]
        if !ok { plan.ToCreate = append(plan.ToCreate, want); continue }
        if have.Driver != want.Driver { // driver change not supported in-place
            plan.ToUpdate = append(plan.ToUpdate, StoragePoolUpdate{Name: name, CurrentConf: have.Config, DesiredConf: want.Config})
            continue
        }
        if !equalConfig(have.Config, want.Config) || have.Description != want.Description {
            plan.ToUpdate = append(plan.ToUpdate, StoragePoolUpdate{Name: name, CurrentConf: have.Config, DesiredConf: want.Config})
        }
    }
    for name, have := range cur {
        if _, ok := des[name]; !ok { plan.ToDelete = append(plan.ToDelete, have) }
    }
    sort.Slice(plan.ToCreate, func(i, j int) bool { return plan.ToCreate[i].Name < plan.ToCreate[j].Name })
    sort.Slice(plan.ToDelete, func(i, j int) bool { return plan.ToDelete[i].Name < plan.ToDelete[j].Name })
    sort.Slice(plan.ToUpdate, func(i, j int) bool { return plan.ToUpdate[i].Name < plan.ToUpdate[j].Name })
    return plan
}
