package incusapi_test

import (
    "testing"

    "incus-backup/src/incusapi"
)

func TestFakeClient_ProjectLifecycle(t *testing.T) {
    c := incusapi.NewFake()

    // Initially empty
    ps, err := c.ListProjects()
    if err != nil { t.Fatal(err) }
    if len(ps) != 0 { t.Fatalf("got %d projects, want 0", len(ps)) }

    // Create
    if err := c.CreateProject("p1", map[string]string{"features.images": "true"}); err != nil {
        t.Fatalf("create: %v", err)
    }
    // Duplicate should conflict
    if err := c.CreateProject("p1", nil); err == nil {
        t.Fatalf("expected conflict on duplicate create")
    }

    // List
    ps, err = c.ListProjects()
    if err != nil { t.Fatal(err) }
    if len(ps) != 1 || ps[0].Name != "p1" { t.Fatalf("unexpected list: %+v", ps) }

    // Delete
    if err := c.DeleteProject("p1"); err != nil { t.Fatalf("delete: %v", err) }
    if err := c.DeleteProject("p1"); err == nil { t.Fatalf("expected not found on delete") }
}

