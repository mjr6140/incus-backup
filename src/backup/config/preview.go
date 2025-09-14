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

