package incusapi

// Project models a minimal Incus project for our purposes.
type Project struct {
    Name   string
    Config map[string]string
}

// ServerInfo exposes key server metadata we care about.
type ServerInfo struct {
    ServerVersion string
}

// Client is a narrow interface over the Incus API used by our app.
// Keep it small and focused on what we actually need so it stays mockable.
type Client interface {
    // Server
    Server() (ServerInfo, error)

    // Projects
    ListProjects() ([]Project, error)
    CreateProject(name string, config map[string]string) error
    DeleteProject(name string) error
    UpdateProject(name string, config map[string]string) error
}
