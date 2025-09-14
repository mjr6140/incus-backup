//go:build integration

package integration

import (
    "fmt"
    "os"
    "testing"
    "time"

    incus "github.com/lxc/incus/client"
    "github.com/lxc/incus/shared/api"
)

func TestIncusProbe_ProjectLifecycle(t *testing.T) {
    if os.Getenv("INCUS_TESTS") != "1" {
        t.Skip("INCUS_TESTS=1 not set; skipping integration test")
    }

    // Connect to the local Incus UNIX socket.
    c, err := incus.ConnectIncusUnix("", nil)
    if err != nil {
        t.Fatalf("connect incus: %v", err)
    }

    s, _, err := c.GetServer()
    if err != nil {
        t.Fatalf("get server info: %v", err)
    }
    t.Logf("Incus server version: %s", s.Environment.ServerVersion)

    name := fmt.Sprintf("itest-%d", time.Now().UnixNano())
    req := api.ProjectsPost{
        Name: name,
        ProjectPut: api.ProjectPut{
            Config: map[string]string{},
        },
    }

    // Create a temporary project; it should be empty and fast to create/delete.
    if err := c.CreateProject(req); err != nil {
        t.Fatalf("create project %q: %v", name, err)
    }
    t.Cleanup(func() {
        _ = c.DeleteProject(name)
    })

    // Verify it exists.
    p, _, err := c.GetProject(name)
    if err != nil {
        t.Fatalf("get project %q: %v", name, err)
    }
    if p.Name != name {
        t.Fatalf("unexpected project name: got %q want %q", p.Name, name)
    }

    // Delete explicitly to validate delete path.
    if err := c.DeleteProject(name); err != nil {
        t.Fatalf("delete project %q: %v", name, err)
    }
}

