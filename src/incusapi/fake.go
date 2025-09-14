package incusapi

import "sort"

// FakeClient is an in-memory implementation for unit tests.
type FakeClient struct {
    ServerVersionStr string
    ProjectsMap      map[string]Project
    ProfilesMap      map[string]Profile
    NetworksMap      map[string]Network
    StoragePoolsMap  map[string]StoragePool
}

func NewFake() *FakeClient {
    return &FakeClient{
        ProjectsMap:     map[string]Project{},
        ProfilesMap:     map[string]Profile{},
        NetworksMap:     map[string]Network{},
        StoragePoolsMap: map[string]StoragePool{},
    }
}

func (f *FakeClient) Server() (ServerInfo, error) {
    return ServerInfo{ServerVersion: f.ServerVersionStr}, nil
}

func (f *FakeClient) ListProjects() ([]Project, error) {
    out := make([]Project, 0, len(f.ProjectsMap))
    for _, p := range f.ProjectsMap {
        out = append(out, p)
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
    return out, nil
}

func (f *FakeClient) CreateProject(name string, config map[string]string) error {
    if _, ok := f.ProjectsMap[name]; ok {
        // mimic Incus conflict
        return &ConflictError{Resource: "project", Name: name}
    }
    if config == nil {
        config = map[string]string{}
    }
    f.ProjectsMap[name] = Project{Name: name, Config: config}
    return nil
}

func (f *FakeClient) DeleteProject(name string) error {
    if _, ok := f.ProjectsMap[name]; !ok {
        return &NotFoundError{Resource: "project", Name: name}
    }
    delete(f.ProjectsMap, name)
    return nil
}

func (f *FakeClient) UpdateProject(name string, config map[string]string) error {
    if _, ok := f.ProjectsMap[name]; !ok {
        return &NotFoundError{Resource: "project", Name: name}
    }
    if config == nil {
        config = map[string]string{}
    }
    p := f.ProjectsMap[name]
    // copy map defensively
    copied := make(map[string]string, len(config))
    for k, v := range config { copied[k] = v }
    p.Config = copied
    f.ProjectsMap[name] = p
    return nil
}

func (f *FakeClient) ListProfiles() ([]Profile, error) {
    out := make([]Profile, 0, len(f.ProfilesMap))
    for _, p := range f.ProfilesMap {
        out = append(out, p)
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
    return out, nil
}

func (f *FakeClient) ListNetworks() ([]Network, error) {
    out := make([]Network, 0, len(f.NetworksMap))
    for _, n := range f.NetworksMap {
        out = append(out, n)
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
    return out, nil
}

func (f *FakeClient) ListStoragePools() ([]StoragePool, error) {
    out := make([]StoragePool, 0, len(f.StoragePoolsMap))
    for _, p := range f.StoragePoolsMap {
        out = append(out, p)
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
    return out, nil
}

type ConflictError struct{ Resource, Name string }
func (e *ConflictError) Error() string { return e.Resource + " conflict: " + e.Name }

type NotFoundError struct{ Resource, Name string }
func (e *NotFoundError) Error() string { return e.Resource + " not found: " + e.Name }
