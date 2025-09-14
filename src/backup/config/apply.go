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

