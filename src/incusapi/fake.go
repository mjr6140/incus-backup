package incusapi

import (
    "bytes"
    "io"
    "sort"
)

// FakeClient is an in-memory implementation for unit tests.
type FakeClient struct {
    ServerVersionStr string
    ProjectsMap      map[string]Project
    ProfilesMap      map[string]Profile
    NetworksMap      map[string]Network
    StoragePoolsMap  map[string]StoragePool
    Instances        map[string]map[string][]byte // project -> name -> export bytes
    Snapshots        map[string]map[string]struct{} // key: project/name@snap -> exists
}

func NewFake() *FakeClient {
    return &FakeClient{
        ProjectsMap:     map[string]Project{},
        ProfilesMap:     map[string]Profile{},
        NetworksMap:     map[string]Network{},
        StoragePoolsMap: map[string]StoragePool{},
        Instances:       map[string]map[string][]byte{},
        Snapshots:       map[string]map[string]struct{}{},
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

func (f *FakeClient) CreateNetwork(n Network) error {
    if _, ok := f.NetworksMap[n.Name]; ok {
        return &ConflictError{Resource: "network", Name: n.Name}
    }
    f.NetworksMap[n.Name] = n
    return nil
}

func (f *FakeClient) UpdateNetwork(n Network) error {
    if _, ok := f.NetworksMap[n.Name]; !ok {
        return &NotFoundError{Resource: "network", Name: n.Name}
    }
    f.NetworksMap[n.Name] = n
    return nil
}

func (f *FakeClient) DeleteNetwork(name string) error {
    if _, ok := f.NetworksMap[name]; !ok {
        return &NotFoundError{Resource: "network", Name: name}
    }
    delete(f.NetworksMap, name)
    return nil
}

func (f *FakeClient) CreateStoragePool(p StoragePool) error {
    if _, ok := f.StoragePoolsMap[p.Name]; ok {
        return &ConflictError{Resource: "storage_pool", Name: p.Name}
    }
    f.StoragePoolsMap[p.Name] = p
    return nil
}

func (f *FakeClient) UpdateStoragePool(p StoragePool) error {
    if _, ok := f.StoragePoolsMap[p.Name]; !ok {
        return &NotFoundError{Resource: "storage_pool", Name: p.Name}
    }
    f.StoragePoolsMap[p.Name] = p
    return nil
}

func (f *FakeClient) DeleteStoragePool(name string) error {
    if _, ok := f.StoragePoolsMap[name]; !ok {
        return &NotFoundError{Resource: "storage_pool", Name: name}
    }
    delete(f.StoragePoolsMap, name)
    return nil
}

func (f *FakeClient) ListInstances(project string) ([]Instance, error) {
    var out []Instance
    if m, ok := f.Instances[project]; ok {
        for name := range m {
            out = append(out, Instance{Project: project, Name: name})
        }
    }
    sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
    return out, nil
}

func (f *FakeClient) ExportInstance(project, name string, optimized bool, snapshot string, _ io.Writer) (io.ReadCloser, error) {
    if f.Instances[project] == nil {
        return io.NopCloser(bytes.NewReader([]byte(""))), nil
    }
    data := f.Instances[project][name]
    if data == nil {
        // Use default content
        data = []byte("FAKE-EXPORT")
    }
    return io.NopCloser(bytes.NewReader(data)), nil
}

func (f *FakeClient) ImportInstance(project, targetName string, r io.Reader, _ io.Writer) error {
    if f.Instances[project] == nil { f.Instances[project] = map[string][]byte{} }
    b, err := io.ReadAll(r)
    if err != nil { return err }
    name := targetName
    if name == "" { name = "restored" }
    f.Instances[project][name] = b
    return nil
}

func (f *FakeClient) InstanceExists(project, name string) (bool, error) {
    if f.Instances[project] == nil { return false, nil }
    _, ok := f.Instances[project][name]
    return ok, nil
}

func (f *FakeClient) StopInstance(project, name string, force bool) error {
    // No-op in fake
    return nil
}

func (f *FakeClient) DeleteInstance(project, name string) error {
    if f.Instances[project] == nil { return &NotFoundError{Resource: "instance", Name: name} }
    if _, ok := f.Instances[project][name]; !ok { return &NotFoundError{Resource: "instance", Name: name} }
    delete(f.Instances[project], name)
    return nil
}

func (f *FakeClient) CreateInstanceSnapshot(project, name, snapshot string) error {
    key := project + "/" + name
    if f.Snapshots[key] == nil { f.Snapshots[key] = map[string]struct{}{} }
    f.Snapshots[key][snapshot] = struct{}{}
    return nil
}

func (f *FakeClient) DeleteInstanceSnapshot(project, name, snapshot string) error {
    key := project + "/" + name
    if f.Snapshots[key] != nil { delete(f.Snapshots[key], snapshot) }
    return nil
}

type ConflictError struct{ Resource, Name string }
func (e *ConflictError) Error() string { return e.Resource + " conflict: " + e.Name }

type NotFoundError struct{ Resource, Name string }
func (e *NotFoundError) Error() string { return e.Resource + " not found: " + e.Name }
