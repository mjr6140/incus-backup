package config

import (
    "fmt"

    "incus-backup/src/incusapi"
)

// ApplyProjectsPlan applies the given plan using the Incus client.
// It performs creations and updates before deletions. Returns a summary string.
func ApplyProjectsPlan(client incusapi.Client, plan ProjectPlan) (string, error) {
    created := 0
    updated := 0
    deleted := 0

    for _, p := range plan.ToCreate {
        if err := client.CreateProject(p.Name, p.Config); err != nil {
            return "", err
        }
        created++
    }
    for _, u := range plan.ToUpdate {
        if err := client.UpdateProject(u.Name, u.Desired); err != nil {
            return "", err
        }
        updated++
    }
    for _, p := range plan.ToDelete {
        if err := client.DeleteProject(p.Name); err != nil {
            return "", err
        }
        deleted++
    }
    return fmt.Sprintf("projects: created=%d updated=%d deleted=%d", created, updated, deleted), nil
}

// ApplyNetworksPlan applies network creations and updates. Deletions are applied
// only if allowDelete is true.
func ApplyNetworksPlan(client incusapi.Client, plan NetworkPlan, allowDelete bool) (string, error) {
    created, updated, deleted := 0, 0, 0
    for _, n := range plan.ToCreate {
        if err := client.CreateNetwork(n); err != nil { return "", err }
        created++
    }
    for _, u := range plan.ToUpdate {
        if err := client.UpdateNetwork(incusapi.Network{Name: u.Name, Config: u.DesiredConf}); err != nil { return "", err }
        updated++
    }
    if allowDelete {
        for _, n := range plan.ToDelete {
            if err := client.DeleteNetwork(n.Name); err != nil { return "", err }
            deleted++
        }
    }
    return fmt.Sprintf("networks: created=%d updated=%d deleted=%d", created, updated, deleted), nil
}

// ApplyStoragePoolsPlan applies storage pool creations and updates. Deletions
// only if allowDelete is true.
func ApplyStoragePoolsPlan(client incusapi.Client, plan StoragePoolPlan, allowDelete bool) (string, error) {
    created, updated, deleted := 0, 0, 0
    for _, p := range plan.ToCreate {
        if err := client.CreateStoragePool(p); err != nil { return "", err }
        created++
    }
    for _, u := range plan.ToUpdate {
        if err := client.UpdateStoragePool(incusapi.StoragePool{Name: u.Name, Config: u.DesiredConf}); err != nil { return "", err }
        updated++
    }
    if allowDelete {
        for _, p := range plan.ToDelete {
            if err := client.DeleteStoragePool(p.Name); err != nil { return "", err }
            deleted++
        }
    }
    return fmt.Sprintf("storage_pools: created=%d updated=%d deleted=%d", created, updated, deleted), nil
}
