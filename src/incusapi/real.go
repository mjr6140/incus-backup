package incusapi

import (
    "errors"
    "io"
    "os"
    "strings"
    "time"
    incuscli "github.com/lxc/incus/client"
    "github.com/lxc/incus/shared/api"
    "github.com/lxc/incus/shared/ioprogress"
    "fmt"
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

func (r *RealClient) ListNetworks() ([]Network, error) {
    nets, err := r.c.GetNetworks()
    if err != nil {
        return nil, err
    }
    out := make([]Network, 0, len(nets))
    for _, n := range nets {
        out = append(out, Network{
            Name:        n.Name,
            Description: n.Description,
            Managed:     n.Managed,
            Type:        n.Type,
            Config:      n.Config,
        })
    }
    return out, nil
}

func (r *RealClient) ListStoragePools() ([]StoragePool, error) {
    pools, err := r.c.GetStoragePools()
    if err != nil {
        return nil, err
    }
    out := make([]StoragePool, 0, len(pools))
    for _, p := range pools {
        out = append(out, StoragePool{
            Name:        p.Name,
            Driver:      p.Driver,
            Description: p.Description,
            Config:      p.Config,
        })
    }
    return out, nil
}

func (r *RealClient) CreateNetwork(n Network) error {
    req := api.NetworksPost{
        Name: n.Name,
        Type: n.Type,
        NetworkPut: api.NetworkPut{
            Description: n.Description,
            Config:      n.Config,
        },
    }
    return r.c.CreateNetwork(req)
}

func (r *RealClient) UpdateNetwork(n Network) error {
    _, etag, err := r.c.GetNetwork(n.Name)
    if err != nil { return err }
    put := api.NetworkPut{Description: n.Description, Config: n.Config}
    return r.c.UpdateNetwork(n.Name, put, etag)
}

func (r *RealClient) DeleteNetwork(name string) error {
    return r.c.DeleteNetwork(name)
}

func (r *RealClient) CreateStoragePool(p StoragePool) error {
    req := api.StoragePoolsPost{
        Name:   p.Name,
        Driver: p.Driver,
        StoragePoolPut: api.StoragePoolPut{
            Description: p.Description,
            Config:      p.Config,
        },
    }
    return r.c.CreateStoragePool(req)
}

func (r *RealClient) UpdateStoragePool(p StoragePool) error {
    _, etag, err := r.c.GetStoragePool(p.Name)
    if err != nil { return err }
    put := api.StoragePoolPut{Description: p.Description, Config: p.Config}
    return r.c.UpdateStoragePool(p.Name, put, etag)
}

func (r *RealClient) DeleteStoragePool(name string) error {
    return r.c.DeleteStoragePool(name)
}

func (r *RealClient) ListInstances(project string) ([]Instance, error) {
    srv := r.c
    if project != "" && project != "default" {
        srv = srv.UseProject(project)
    }
    insts, err := srv.GetInstances(api.InstanceTypeAny)
    if err != nil { return nil, err }
    out := make([]Instance, 0, len(insts))
    for _, in := range insts {
        out = append(out, Instance{Project: project, Name: in.Name, Type: string(in.Type)})
    }
    return out, nil
}

func (r *RealClient) ExportInstance(project, name string, optimized bool, snapshot string, progressOut io.Writer) (io.ReadCloser, error) {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    req := api.InstanceBackupsPost{
        Name:                 "",
        InstanceOnly:         false,
        OptimizedStorage:     optimized,
        CompressionAlgorithm: "",
    }
    op, err := srv.CreateInstanceBackup(name, req)
    if err != nil { return nil, err }
    // Show server-side backup creation status
    if progressOut != nil {
        var last string
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        for {
            _ = op.Refresh()
            st := op.Get().Status
            if st != "" && st != last {
                fmt.Fprintf(progressOut, "\r[server] %s", st)
                last = st
            }
            if err := op.Wait(); err == nil {
                // Ensure final status printed
                _ = op.Refresh()
                st2 := op.Get().Status
                if st2 != "" && st2 != last {
                    fmt.Fprintf(progressOut, "\r[server] %s", st2)
                }
                fmt.Fprint(progressOut, "\n")
                break
            }
            <-ticker.C
        }
    } else {
        if err := op.Wait(); err != nil { return nil, err }
    }
    // Determine backup name from operation resources
    resources := op.Get().Resources
    if len(resources["backups"]) == 0 {
        return nil, errors.New("no backup resource returned")
    }
    backupURL := resources["backups"][0]
    // Extract the last path segment as name
    seg := backupURL[strings.LastIndex(backupURL, "/")+1:]
    // Prepare a temp file and download with progress
    f, err := os.CreateTemp("", "incus-export-*.tar")
    if err != nil { return nil, err }
    // Ensure cleanup of server-side backup once done
    defer func() { go func() { bop, _ := srv.DeleteInstanceBackup(name, seg); if bop != nil { _ = bop.Wait() } }() }()
    // Download into the temp file
    reqFile := incuscli.BackupFileRequest{BackupFile: f}
    if progressOut != nil {
        reqFile.ProgressHandler = func(pd ioprogress.ProgressData) {
            fmt.Fprintf(progressOut, "\r[download] %s", pd.Text)
        }
    }
    if _, err := srv.GetInstanceBackupFile(name, seg, &reqFile); err != nil {
        _ = f.Close(); _ = os.Remove(f.Name())
        return nil, err
    }
    if progressOut != nil { fmt.Fprint(progressOut, "\n") }
    if _, err := f.Seek(0, io.SeekStart); err != nil { _ = f.Close(); _ = os.Remove(f.Name()); return nil, err }
    // Return a ReadCloser that deletes the temp file on close
    return &tempFileReadCloser{File: f}, nil
}

func (r *RealClient) ImportInstance(project, targetName string, rstream io.Reader, progressOut io.Writer) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    args := incuscli.InstanceBackupArgs{BackupFile: rstream, Name: targetName}
    op, err := srv.CreateInstanceFromBackup(args)
    if err != nil { return err }
    var lastStatus string
    var done chan struct{}
    if progressOut != nil {
        // Poll the operation status periodically to surface "Running" etc.
        done = make(chan struct{})
        go func() {
            for {
                select {
                case <-done:
                    return
                case <-time.After(1 * time.Second):
                    _ = op.Refresh()
                    st := op.Get().Status
                    if st != lastStatus && st != "" {
                        fmt.Fprintf(progressOut, "\r[server] %s", st)
                        lastStatus = st
                    }
                }
            }
        }()
    }
    err = op.Wait()
    if progressOut != nil {
        // Ensure we print the final status if we missed it between polls.
        _ = op.Refresh()
        st := op.Get().Status
        if st != "" && st != lastStatus {
            fmt.Fprintf(progressOut, "\r[server] %s", st)
        }
        if done != nil { close(done) }
        fmt.Fprint(progressOut, "\n")
    }
    return err
}

type tempFileReadCloser struct{ *os.File }
func (t *tempFileReadCloser) Close() error { name := t.Name(); err := t.File.Close(); _ = os.Remove(name); return err }

func (r *RealClient) InstanceExists(project, name string) (bool, error) {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    _, _, err := srv.GetInstance(name)
    if err != nil {
        if strings.Contains(err.Error(), "not found") {
            return false, nil
        }
        return false, err
    }
    return true, nil
}

func (r *RealClient) StopInstance(project, name string, force bool) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    put := api.InstanceStatePut{Action: "stop", Force: force, Timeout: 60}
    op, err := srv.UpdateInstanceState(name, put, "")
    if err != nil { return err }
    return op.Wait()
}

func (r *RealClient) DeleteInstance(project, name string) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    op, err := srv.DeleteInstance(name)
    if err != nil { return err }
    return op.Wait()
}

func (r *RealClient) CreateInstanceSnapshot(project, name, snapshot string) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    req := api.InstanceSnapshotsPost{Name: snapshot}
    op, err := srv.CreateInstanceSnapshot(name, req)
    if err != nil { return err }
    return op.Wait()
}

func (r *RealClient) DeleteInstanceSnapshot(project, name, snapshot string) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    op, err := srv.DeleteInstanceSnapshot(name, snapshot)
    if err != nil { return err }
    return op.Wait()
}

// Volumes
func (r *RealClient) ListCustomVolumes(project string) ([]Volume, error) {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    pools, err := srv.GetStoragePools()
    if err != nil { return nil, err }
    var out []Volume
    for _, p := range pools {
        vols, err := srv.GetStoragePoolVolumes(p.Name)
        if err != nil { return nil, err }
        for _, v := range vols {
            if v.Type != "custom" { continue }
            // Skip snapshots (Incus returns custom snapshot volumes as "vol/snap")
            if strings.Contains(v.Name, "/") { continue }
            out = append(out, Volume{Project: project, Pool: p.Name, Name: v.Name, ContentType: v.ContentType})
        }
    }
    return out, nil
}

func (r *RealClient) VolumeExists(project, pool, name string) (bool, error) {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    _, _, err := srv.GetStoragePoolVolume(pool, "custom", name)
    if err != nil {
        if strings.Contains(err.Error(), "not found") { return false, nil }
        return false, err
    }
    return true, nil
}

func (r *RealClient) CreateVolumeSnapshot(project, pool, name, snapshot string) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    req := api.StorageVolumeSnapshotsPost{Name: snapshot}
    op, err := srv.CreateStoragePoolVolumeSnapshot(pool, "custom", name, req)
    if err != nil { return err }
    return op.Wait()
}

func (r *RealClient) DeleteVolumeSnapshot(project, pool, name, snapshot string) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    op, err := srv.DeleteStoragePoolVolumeSnapshot(pool, "custom", name, snapshot)
    if err != nil { return err }
    return op.Wait()
}

func (r *RealClient) ExportVolume(project, pool, name string, optimized bool, snapshot string, progressOut io.Writer) (io.ReadCloser, error) {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    req := api.StoragePoolVolumeBackupsPost{OptimizedStorage: optimized, VolumeOnly: false}
    // Create backup
    op, err := srv.CreateStoragePoolVolumeBackup(pool, name, req)
    if err != nil { return nil, err }
    if progressOut != nil {
        var last string
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        for {
            _ = op.Refresh()
            st := op.Get().Status
            if st != "" && st != last {
                fmt.Fprintf(progressOut, "\r[server] %s", st)
                last = st
            }
            if err := op.Wait(); err == nil {
                _ = op.Refresh()
                st2 := op.Get().Status
                if st2 != "" && st2 != last { fmt.Fprintf(progressOut, "\r[server] %s", st2) }
                fmt.Fprint(progressOut, "\n")
                break
            }
            <-ticker.C
        }
    } else {
        if err := op.Wait(); err != nil { return nil, err }
    }
    // Determine backup name from operation resources
    resources := op.Get().Resources
    if len(resources["backups"]) == 0 {
        return nil, errors.New("no volume backup resource returned")
    }
    backupURL := resources["backups"][0]
    seg := backupURL[strings.LastIndex(backupURL, "/")+1:]
    // temp file
    f, err := os.CreateTemp("", "incus-vol-export-*.tar")
    if err != nil { return nil, err }
    // Download with progress
    reqFile := incuscli.BackupFileRequest{BackupFile: f}
    if progressOut != nil {
        reqFile.ProgressHandler = func(pd ioprogress.ProgressData) { fmt.Fprintf(progressOut, "\r[download] %s", pd.Text) }
    }
    if _, err := srv.GetStoragePoolVolumeBackupFile(pool, name, seg, &reqFile); err != nil {
        _ = f.Close(); _ = os.Remove(f.Name())
        return nil, err
    }
    if progressOut != nil { fmt.Fprint(progressOut, "\n") }
    if _, err := f.Seek(0, io.SeekStart); err != nil { _ = f.Close(); _ = os.Remove(f.Name()); return nil, err }
    // Delete server-side backup (best-effort)
    go func() { _op, _ := srv.DeleteStoragePoolVolumeBackup(pool, name, seg); if _op != nil { _ = _op.Wait() } }()
    return &tempFileReadCloser{File: f}, nil
}

func (r *RealClient) ImportVolume(project, poolTarget, nameTarget string, reader io.Reader, progressOut io.Writer) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    args := incuscli.StoragePoolVolumeBackupArgs{BackupFile: reader, Name: nameTarget}
    op, err := srv.CreateStoragePoolVolumeFromBackup(poolTarget, args)
    if err != nil { return err }
    if progressOut != nil {
        last := ""
        ticker := time.NewTicker(1 * time.Second)
        defer ticker.Stop()
        for {
            _ = op.Refresh()
            st := op.Get().Status
            if st != "" && st != last { fmt.Fprintf(progressOut, "\r[server] %s", st); last = st }
            if err := op.Wait(); err == nil { break }
            <-ticker.C
        }
        fmt.Fprint(progressOut, "\n")
    } else {
        if err := op.Wait(); err != nil { return err }
    }
    return nil
}

func (r *RealClient) DeleteVolume(project, pool, name string) error {
    srv := r.c
    if project != "" && project != "default" { srv = srv.UseProject(project) }
    return srv.DeleteStoragePoolVolume(pool, "custom", name)
}
