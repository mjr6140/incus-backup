package instances

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"

    "incus-backup/src/incusapi"
    pg "incus-backup/src/util/progress"
)

// RestoreInstance imports an instance export from the given snapshot directory.
func RestoreInstance(client incusapi.Client, snapDir, project, targetName string, progressOut io.Writer) error {
    // sanity: load manifest to confirm type
    b, err := os.ReadFile(filepath.Join(snapDir, "manifest.json"))
    if err != nil { return err }
    var mf Manifest
    if err := json.Unmarshal(b, &mf); err != nil { return err }
    if mf.Type != "instance" { return fmt.Errorf("not an instance snapshot: %s", snapDir) }
    // open export
    exportPath := filepath.Join(snapDir, "export.tar.xz")
    f, err := os.Open(exportPath)
    if err != nil { return err }
    defer f.Close()
    var reader io.Reader = f
    if st, err := os.Stat(exportPath); err == nil && progressOut != nil {
        reader = pg.NewReader(f, st.Size(), "import", progressOut)
    }
    return client.ImportInstance(project, targetName, io.NopCloser(reader))
}
