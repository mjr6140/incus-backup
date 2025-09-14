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

