package incusapi

import (
    incuscli "github.com/lxc/incus/client"
    "github.com/lxc/incus/shared/api"
)

// RealClient wraps the official Incus Go client.
type RealClient struct {
    c incuscli.InstanceServer
}

// ConnectLocal connects to the local Incus via the UNIX socket.
func ConnectLocal() (*RealClient, error) {
    c, err := incuscli.ConnectIncusUnix("", nil)
    if err != nil {
        return nil, err
    }
    return &RealClient{c: c}, nil
}

func (r *RealClient) Server() (ServerInfo, error) {
    s, _, err := r.c.GetServer()
    if err != nil {
        return ServerInfo{}, err
    }
    return ServerInfo{ServerVersion: s.Environment.ServerVersion}, nil
}

func (r *RealClient) ListProjects() ([]Project, error) {
    prjs, err := r.c.GetProjects()
    if err != nil {
        return nil, err
    }
    out := make([]Project, 0, len(prjs))
    for _, p := range prjs {
        out = append(out, Project{Name: p.Name, Config: p.Config})
    }
    return out, nil
}

func (r *RealClient) CreateProject(name string, config map[string]string) error {
    req := api.ProjectsPost{
        Name: name,
        ProjectPut: api.ProjectPut{Config: config},
    }
    return r.c.CreateProject(req)
}

func (r *RealClient) DeleteProject(name string) error {
    return r.c.DeleteProject(name)
}

func (r *RealClient) UpdateProject(name string, config map[string]string) error {
    // Need ETag
    _, etag, err := r.c.GetProject(name)
    if err != nil {
        return err
    }
    put := api.ProjectPut{Config: config}
    return r.c.UpdateProject(name, put, etag)
}

func (r *RealClient) ListProfiles() ([]Profile, error) {
    profs, err := r.c.GetProfiles()
    if err != nil {
        return nil, err
    }
    out := make([]Profile, 0, len(profs))
    for _, p := range profs {
        out = append(out, Profile{
            Name:        p.Name,
            Description: p.Description,
            Config:      p.Config,
            Devices:     convertDevices(p.Devices),
        })
    }
    return out, nil
}

func convertDevices(in map[string]map[string]string) map[string]map[string]string {
    if in == nil { return nil }
    out := make(map[string]map[string]string, len(in))
    for k, v := range in {
        inner := make(map[string]string, len(v))
        for k2, v2 := range v { inner[k2] = v2 }
        out[k] = inner
    }
    return out
}
