package instances

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"

    "incus-backup/src/incusapi"
)

// RestoreInstance imports an instance export from the given snapshot directory.
func RestoreInstance(client incusapi.Client, snapDir, project, targetName string) error {
    // sanity: load manifest to confirm type
    b, err := os.ReadFile(filepath.Join(snapDir, "manifest.json"))
    if err != nil { return err }
    var mf Manifest
    if err := json.Unmarshal(b, &mf); err != nil { return err }
    if mf.Type != "instance" { return fmt.Errorf("not an instance snapshot: %s", snapDir) }
    // open export
    f, err := os.Open(filepath.Join(snapDir, "export.tar.xz"))
    if err != nil { return err }
    defer f.Close()
    return client.ImportInstance(project, targetName, io.NopCloser(f))
}

