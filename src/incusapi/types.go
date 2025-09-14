package incusapi

import "io"
// Project models a minimal Incus project for our purposes.
type Project struct {
    Name   string
    Config map[string]string
}

// Profile mirrors the important fields of an Incus profile.
type Profile struct {
    Name        string
    Description string
    Config      map[string]string
    Devices     map[string]map[string]string
}

// Network captures minimal managed network info.
type Network struct {
    Name        string
    Description string
    Managed     bool
    Type        string
    Config      map[string]string
}

// StoragePool captures minimal storage pool info.
type StoragePool struct {
    Name        string
    Driver      string
    Description string
    Config      map[string]string
}

// Instance captures minimal instance info.
type Instance struct {
    Project string
    Name    string
    Type    string // container|virtual-machine
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

    // Profiles
    ListProfiles() ([]Profile, error)

    // Networks
    ListNetworks() ([]Network, error)
    CreateNetwork(n Network) error
    UpdateNetwork(n Network) error
    DeleteNetwork(name string) error

    // Storage pools
    ListStoragePools() ([]StoragePool, error)
    CreateStoragePool(p StoragePool) error
    UpdateStoragePool(p StoragePool) error
    DeleteStoragePool(name string) error

    // Instances
    ListInstances(project string) ([]Instance, error)
    // ExportInstance returns a tar stream of the instance export. If snapshot is non-empty,
    // it should export from that snapshot. If optimized is true, use backend-optimized export.
    ExportInstance(project, name string, optimized bool, snapshot string) (io.ReadCloser, error)
    // ImportInstance creates/restores an instance from the given tar stream with optional target name.
    ImportInstance(project, targetName string, r io.Reader) error

    // Instance lifecycle helpers
    InstanceExists(project, name string) (bool, error)
    StopInstance(project, name string, force bool) error
    DeleteInstance(project, name string) error
}
