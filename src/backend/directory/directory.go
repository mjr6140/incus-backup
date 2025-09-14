package directory

import (
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"

    "incus-backup/src/backend"
)

// Backend implements backend.StorageBackend for the filesystem layout.
type Backend struct {
    Root string // absolute directory path
}

func New(root string) (*Backend, error) {
    if root == "" {
        return nil, errors.New("directory backend root must not be empty")
    }
    info, err := os.Stat(root)
    if err != nil {
        return nil, fmt.Errorf("stat root: %w", err)
    }
    if !info.IsDir() {
        return nil, fmt.Errorf("root is not a directory: %s", root)
    }
    return &Backend{Root: root}, nil
}

func (b *Backend) List(kind string) ([]backend.Entry, error) {
    kinds := []string{backend.KindInstance, backend.KindVolume, backend.KindImage, backend.KindConfig}
    if kind != "" && kind != backend.KindAll {
        kinds = []string{kind}
    }
    var entries []backend.Entry
    for _, k := range kinds {
        switch k {
        case backend.KindInstance:
            e, err := b.listInstances()
            if err != nil {
                return nil, err
            }
            entries = append(entries, e...)
        case backend.KindVolume:
            e, err := b.listVolumes()
            if err != nil {
                return nil, err
            }
            entries = append(entries, e...)
        case backend.KindImage:
            e, err := b.listImages()
            if err != nil {
                return nil, err
            }
            entries = append(entries, e...)
        case backend.KindConfig:
            e, err := b.listConfig()
            if err != nil {
                return nil, err
            }
            entries = append(entries, e...)
        }
    }
    sort.Slice(entries, func(i, j int) bool {
        a, c := entries[i], entries[j]
        if a.Type != c.Type {
            return a.Type < c.Type
        }
        if a.Project != c.Project {
            return a.Project < c.Project
        }
        if a.Pool != c.Pool {
            return a.Pool < c.Pool
        }
        if a.Name != c.Name {
            return a.Name < c.Name
        }
        if a.Fingerprint != c.Fingerprint {
            return a.Fingerprint < c.Fingerprint
        }
        return a.Timestamp < c.Timestamp
    })
    return entries, nil
}

func (b *Backend) listInstances() ([]backend.Entry, error) {
    base := filepath.Join(b.Root, "instances")
    return walkThreeLevel(base, func(project, name, ts string, full string) backend.Entry {
        return backend.Entry{Type: "instance", Project: project, Name: name, Timestamp: ts, Path: full}
    })
}

func (b *Backend) listVolumes() ([]backend.Entry, error) {
    base := filepath.Join(b.Root, "volumes")
    // volumes/<project>/<pool>/<name>/<timestamp>
    var entries []backend.Entry
    // project level
    projDirs, err := readDirNames(base)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    for _, project := range projDirs {
        poolsPath := filepath.Join(base, project)
        poolDirs, err := readDirNames(poolsPath)
        if err != nil {
            return nil, err
        }
        for _, pool := range poolDirs {
            namesPath := filepath.Join(poolsPath, pool)
            nameDirs, err := readDirNames(namesPath)
            if err != nil {
                return nil, err
            }
            for _, name := range nameDirs {
                tsPath := filepath.Join(namesPath, name)
                timestamps, err := readDirNames(tsPath)
                if err != nil {
                    return nil, err
                }
                for _, ts := range timestamps {
                    full := filepath.Join(tsPath, ts)
                    entries = append(entries, backend.Entry{Type: "volume", Project: project, Pool: pool, Name: name, Timestamp: ts, Path: full})
                }
            }
        }
    }
    return entries, nil
}

func (b *Backend) listImages() ([]backend.Entry, error) {
    base := filepath.Join(b.Root, "images")
    // images/<fingerprint>/<timestamp>
    var entries []backend.Entry
    fps, err := readDirNames(base)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    for _, fp := range fps {
        tsPath := filepath.Join(base, fp)
        timestamps, err := readDirNames(tsPath)
        if err != nil {
            return nil, err
        }
        for _, ts := range timestamps {
            full := filepath.Join(tsPath, ts)
            entries = append(entries, backend.Entry{Type: "image", Fingerprint: fp, Timestamp: ts, Path: full})
        }
    }
    return entries, nil
}

func (b *Backend) listConfig() ([]backend.Entry, error) {
    base := filepath.Join(b.Root, "config")
    // config/<timestamp>
    timestamps, err := readDirNames(base)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    var entries []backend.Entry
    for _, ts := range timestamps {
        entries = append(entries, backend.Entry{Type: "config", Timestamp: ts, Path: filepath.Join(base, ts)})
    }
    return entries, nil
}

// walkThreeLevel walks base/<level1>/<level2>/<level3>
// and returns an entry built from the callback.
func walkThreeLevel(base string, build func(a, b, c, full string) backend.Entry) ([]backend.Entry, error) {
    var entries []backend.Entry
    aDirs, err := readDirNames(base)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, nil
        }
        return nil, err
    }
    for _, a := range aDirs {
        bPath := filepath.Join(base, a)
        bDirs, err := readDirNames(bPath)
        if err != nil {
            return nil, err
        }
        for _, b := range bDirs {
            cPath := filepath.Join(bPath, b)
            cDirs, err := readDirNames(cPath)
            if err != nil {
                return nil, err
            }
            for _, c := range cDirs {
                full := filepath.Join(cPath, c)
                entries = append(entries, build(a, b, c, full))
            }
        }
    }
    return entries, nil
}

func readDirNames(path string) ([]string, error) {
    entries, err := os.ReadDir(path)
    if err != nil {
        return nil, err
    }
    names := make([]string, 0, len(entries))
    for _, e := range entries {
        if e.IsDir() {
            name := e.Name()
            // skip hidden
            if strings.HasPrefix(name, ".") {
                continue
            }
            names = append(names, name)
        }
    }
    sort.Strings(names)
    return names, nil
}

