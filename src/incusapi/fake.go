package incusapi

import "sort"

// FakeClient is an in-memory implementation for unit tests.
type FakeClient struct {
    ServerVersionStr string
    ProjectsMap      map[string]Project
}

func NewFake() *FakeClient {
    return &FakeClient{ProjectsMap: map[string]Project{}}
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

type ConflictError struct{ Resource, Name string }
func (e *ConflictError) Error() string { return e.Resource + " conflict: " + e.Name }

type NotFoundError struct{ Resource, Name string }
func (e *NotFoundError) Error() string { return e.Resource + " not found: " + e.Name }

