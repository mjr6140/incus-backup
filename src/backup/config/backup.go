package config

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "time"

    "incus-backup/src/incusapi"
)

// BackupAll exports supported declarative config pieces into a single snapshot
// directory under config/<timestamp>/ and writes manifest + checksums.
func BackupAll(client incusapi.Client, root string, now time.Time) (string, error) {
    ts := now.UTC().Format("20060102T150405Z")
    snapDir := filepath.Join(root, "config", ts)
    if err := os.MkdirAll(snapDir, 0o755); err != nil {
        return "", err
    }

    includes := make([]string, 0, 4)
    files := make([]string, 0, 8)

    // Projects
    if err := writeProjects(client, snapDir); err != nil {
        return "", err
    }
    includes = append(includes, "projects")
    files = append(files, "projects.json")

    // Profiles
    if err := writeProfiles(client, snapDir); err != nil {
        return "", err
    }
    includes = append(includes, "profiles")
    files = append(files, "profiles.json")

    // Manifest + checksums
    mf := Manifest{Type: "config", CreatedAt: now.UTC(), Includes: includes}
    if err := writeJSON(filepath.Join(snapDir, "manifest.json"), mf); err != nil {
        return "", err
    }
    files = append(files, "manifest.json")
    if err := writeChecksums(snapDir, files); err != nil {
        return "", err
    }
    return snapDir, nil
}

// BackupProjects exports Incus projects into config/<timestamp>/projects.json
// and writes a manifest.json and checksums.txt. Returns the snapshot directory path.
func BackupProjects(client incusapi.Client, root string, now time.Time) (string, error) {
    // Prepare snapshot dir
    ts := now.UTC().Format("20060102T150405Z")
    snapDir := filepath.Join(root, "config", ts)
    if err := os.MkdirAll(snapDir, 0o755); err != nil {
        return "", err
    }

    // Fetch projects and sort
    projects, err := client.ListProjects()
    if err != nil {
        return "", err
    }
    sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })

    // Write projects.json
    pj := filepath.Join(snapDir, "projects.json")
    if err := writeJSON(pj, projects); err != nil {
        return "", err
    }

    // Write manifest.json
    mf := Manifest{Type: "config", CreatedAt: now.UTC(), Includes: []string{"projects"}}
    if err := writeJSON(filepath.Join(snapDir, "manifest.json"), mf); err != nil {
        return "", err
    }

    // Write checksums.txt
    if err := writeChecksums(snapDir, []string{"projects.json", "manifest.json"}); err != nil {
        return "", err
    }

    return snapDir, nil
}

func writeProjects(client incusapi.Client, snapDir string) error {
    projects, err := client.ListProjects()
    if err != nil {
        return err
    }
    sort.Slice(projects, func(i, j int) bool { return projects[i].Name < projects[j].Name })
    return writeJSON(filepath.Join(snapDir, "projects.json"), projects)
}

func writeProfiles(client incusapi.Client, snapDir string) error {
    profiles, err := client.ListProfiles()
    if err != nil {
        return err
    }
    sort.Slice(profiles, func(i, j int) bool { return profiles[i].Name < profiles[j].Name })
    return writeJSON(filepath.Join(snapDir, "profiles.json"), profiles)
}

func writeJSON(path string, v any) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    return enc.Encode(v)
}

func writeChecksums(dir string, files []string) error {
    outPath := filepath.Join(dir, "checksums.txt")
    out, err := os.Create(outPath)
    if err != nil {
        return err
    }
    defer out.Close()
    for _, name := range files {
        p := filepath.Join(dir, name)
        sum, err := sha256File(p)
        if err != nil {
            return err
        }
        if _, err := fmt.Fprintf(out, "%s  %s\n", sum, name); err != nil {
            return err
        }
    }
    return nil
}

func sha256File(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil {
        return "", err
    }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil {
        return "", err
    }
    return hex.EncodeToString(h.Sum(nil)), nil
}
