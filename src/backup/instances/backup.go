package instances

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "time"

    "incus-backup/src/incusapi"
)

// BackupInstance exports a single instance to the directory backend layout.
// It creates instances/<project>/<name>/<timestamp>/export.tar.xz and writes a manifest and checksums.
func BackupInstance(client incusapi.Client, root, project, name string, optimized bool, snapshot bool, now time.Time) (string, error) {
    ts := now.UTC().Format("20060102T150405Z")
    snapDir := filepath.Join(root, "instances", project, name, ts)
    if err := os.MkdirAll(snapDir, 0o755); err != nil { return "", err }

    snapName := ""
    if snapshot {
        // Snapshot mechanics are handled by client.ExportInstance when snapshot name is provided
        snapName = "tmp-incus-backup"
    }
    r, err := client.ExportInstance(project, name, optimized, snapName)
    if err != nil { return "", err }
    defer r.Close()

    exportPath := filepath.Join(snapDir, "export.tar.xz")
    f, err := os.Create(exportPath)
    if err != nil { return "", err }
    if _, err := io.Copy(f, r); err != nil { f.Close(); return "", err }
    if err := f.Close(); err != nil { return "", err }

    mf := Manifest{
        Type:      "instance",
        Project:   project,
        Name:      name,
        CreatedAt: now.UTC(),
        Options: map[string]string{
            "snapshot":  fmt.Sprintf("%t", snapshot),
            "optimized": fmt.Sprintf("%t", optimized),
        },
    }
    if err := writeJSON(filepath.Join(snapDir, "manifest.json"), mf); err != nil { return "", err }
    if err := writeChecksums(snapDir, []string{"export.tar.xz", "manifest.json"}); err != nil { return "", err }
    return snapDir, nil
}

func writeJSON(path string, v any) error {
    f, err := os.Create(path)
    if err != nil { return err }
    defer f.Close()
    enc := json.NewEncoder(f)
    enc.SetIndent("", "  ")
    return enc.Encode(v)
}

func writeChecksums(dir string, files []string) error {
    out, err := os.Create(filepath.Join(dir, "checksums.txt"))
    if err != nil { return err }
    defer out.Close()
    for _, name := range files {
        p := filepath.Join(dir, name)
        sum, err := sha256File(p)
        if err != nil { return err }
        if _, err := fmt.Fprintf(out, "%s  %s\n", sum, name); err != nil { return err }
    }
    return nil
}

func sha256File(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil { return "", err }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil { return "", err }
    return hex.EncodeToString(h.Sum(nil)), nil
}

