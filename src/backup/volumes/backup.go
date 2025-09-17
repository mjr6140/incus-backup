package volumes

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
	pg "incus-backup/src/util/progress"
)

type Manifest struct {
	Type      string            `json:"type"` // volume
	Project   string            `json:"project"`
	Pool      string            `json:"pool"`
	Name      string            `json:"name"`
	CreatedAt time.Time         `json:"createdAt"`
	Options   map[string]string `json:"options,omitempty"`
}

func BackupVolume(client incusapi.Client, root, project, pool, name string, optimized, snapshot bool, now time.Time, progressOut io.Writer) (string, error) {
	ts := now.UTC().Format("20060102T150405Z")
	snapDir := filepath.Join(root, "volumes", project, pool, name, ts)
	if err := os.MkdirAll(snapDir, 0o755); err != nil {
		return "", err
	}

	snapName := ""
	if snapshot {
		snapName = "tmp-incus-backup-" + ts
		if progressOut != nil {
			fmt.Fprintf(progressOut, "[snapshot] create %s/%s@%s\n", pool, name, snapName)
		}
		if err := client.CreateVolumeSnapshot(project, pool, name, snapName); err != nil {
			return "", err
		}
		defer func() {
			if progressOut != nil {
				fmt.Fprintf(progressOut, "[snapshot] delete %s/%s@%s\n", pool, name, snapName)
			}
			_ = client.DeleteVolumeSnapshot(project, pool, name, snapName)
		}()
	}

	r, err := client.ExportVolume(project, pool, name, optimized, snapName, "", progressOut)
	if err != nil {
		return "", err
	}
	defer r.Close()

	exportPath := filepath.Join(snapDir, "volume.tar.xz")
	f, err := os.Create(exportPath)
	if err != nil {
		return "", err
	}
	reader := io.Reader(r)
	if s, ok := r.(interface{ Stat() (os.FileInfo, error) }); ok && progressOut != nil {
		if fi, err := s.Stat(); err == nil {
			reader = pg.NewReader(r, fi.Size(), "write", progressOut)
		}
	}
	if _, err := io.Copy(f, reader); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}

	mf := Manifest{Type: "volume", Project: project, Pool: pool, Name: name, CreatedAt: now.UTC(), Options: map[string]string{"snapshot": fmt.Sprintf("%t", snapshot), "optimized": fmt.Sprintf("%t", optimized)}}
	if err := writeJSON(filepath.Join(snapDir, "manifest.json"), mf); err != nil {
		return "", err
	}
	if err := writeChecksums(snapDir, []string{"volume.tar.xz", "manifest.json"}); err != nil {
		return "", err
	}
	return snapDir, nil
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
	out, err := os.Create(filepath.Join(dir, "checksums.txt"))
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
