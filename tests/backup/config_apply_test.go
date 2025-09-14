package backup_test

import (
    "testing"

    cfg "incus-backup/src/backup/config"
    "incus-backup/src/incusapi"
)

func TestApplyProjectsPlan(t *testing.T) {
    fake := incusapi.NewFake()
    // Current: one project beta
    _ = fake.CreateProject("beta", map[string]string{"b": "2"})

    // Desired: alpha (new), beta updated, gamma (new), and current has an extra 'delta' to delete
    _ = fake.CreateProject("delta", nil)

    current, _ := fake.ListProjects()
    desired := []incusapi.Project{
        {Name: "alpha", Config: map[string]string{"a": "1"}},
        {Name: "beta", Config: map[string]string{"b": "999"}},
        {Name: "gamma", Config: nil},
    }
    plan := cfg.BuildProjectsPlan(current, desired)
    summary, err := cfg.ApplyProjectsPlan(fake, plan)
    if err != nil { t.Fatalf("apply plan: %v", err) }
    if summary == "" { t.Fatalf("empty summary") }

    ps, _ := fake.ListProjects()
    names := map[string]bool{}
    conf := map[string]map[string]string{}
    for _, p := range ps { names[p.Name] = true; conf[p.Name] = p.Config }

    if !names["alpha"] || !names["beta"] || !names["gamma"] {
        t.Fatalf("expected alpha, beta, gamma present; got %+v", names)
    }
    if names["delta"] {
        t.Fatalf("expected delta deleted")
    }
    if conf["beta"]["b"] != "999" {
        t.Fatalf("expected beta updated config; got %+v", conf["beta"]) }
}

