package volumes

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
    "path/filepath"

    "incus-backup/src/incusapi"
    pg "incus-backup/src/util/progress"
)

func RestoreVolume(client incusapi.Client, snapDir, project, poolTarget, targetName string, progressOut io.Writer) error {
    b, err := os.ReadFile(filepath.Join(snapDir, "manifest.json"))
    if err != nil { return err }
    var mf Manifest
    if err := json.Unmarshal(b, &mf); err != nil { return err }
    if mf.Type != "volume" { return fmt.Errorf("not a volume snapshot: %s", snapDir) }
    f, err := os.Open(filepath.Join(snapDir, "volume.tar.xz"))
    if err != nil { return err }
    defer f.Close()
    var reader io.Reader = f
    if st, err := f.Stat(); err == nil && progressOut != nil { reader = pg.NewReader(f, st.Size(), "import", progressOut) }
    return client.ImportVolume(project, poolTarget, targetName, reader, progressOut)
}

