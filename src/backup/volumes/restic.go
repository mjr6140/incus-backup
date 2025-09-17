package volumes

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	pg "incus-backup/src/util/progress"
)

const (
	resticVolumeDataFilename      = "volume.tar.xz"
	resticVolumeManifestFilename  = "manifest.json"
	resticVolumeChecksumsFilename = "checksums.txt"
)

// BackupVolumeRestic streams a volume export into a restic repository.
func BackupVolumeRestic(ctx context.Context, bin restic.BinaryInfo, repo string, client incusapi.Client, project, pool, name string, optimized, snapshot bool, now time.Time, progressOut io.Writer) (string, error) {
	if err := restic.EnsureRepository(ctx, bin, repo); err != nil {
		return "", err
	}

	ts := now.UTC().Format("20060102T150405Z")
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

	export, err := client.ExportVolume(project, pool, name, optimized, snapName, progressOut)
	if err != nil {
		return "", err
	}
	defer export.Close()

	var reader io.Reader = export
	if statter, ok := export.(interface{ Stat() (os.FileInfo, error) }); ok && progressOut != nil {
		if fi, err := statter.Stat(); err == nil {
			reader = pg.NewReader(export, fi.Size(), "restic backup", progressOut)
		}
	}

	hash := sha256.New()
	reader = io.TeeReader(reader, hash)

	filename := resticVolumeDataFilename
	if err := restic.BackupStream(ctx, bin, repo, filename, resticTagsForVolume(project, pool, name, ts, "data", optimized, snapshot), reader, progressOut); err != nil {
		return "", err
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	manifest := Manifest{
		Type:      "volume",
		Project:   project,
		Pool:      pool,
		Name:      name,
		CreatedAt: now.UTC(),
		Options: map[string]string{
			"snapshot":  fmt.Sprintf("%t", snapshot),
			"optimized": fmt.Sprintf("%t", optimized),
		},
	}
	manifestBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	manifestPath := resticVolumeManifestFilename
	if err := restic.BackupBytes(ctx, bin, repo, manifestPath, resticTagsForVolume(project, pool, name, ts, "manifest", optimized, snapshot), manifestBytes, progressOut); err != nil {
		return "", err
	}

	checksums := fmt.Sprintf("%s  %s\n", sum, resticVolumeDataFilename)
	checksumPath := resticVolumeChecksumsFilename
	if err := restic.BackupBytes(ctx, bin, repo, checksumPath, resticTagsForVolume(project, pool, name, ts, "checksums", optimized, snapshot), []byte(checksums), progressOut); err != nil {
		return "", err
	}

	return ts, nil
}

func resticTagsForVolume(project, pool, name, ts, part string, optimized, snapshot bool) []string {
	tags := []string{
		"type=volume",
		"schema=v1",
		fmt.Sprintf("project=%s", project),
		fmt.Sprintf("pool=%s", pool),
		fmt.Sprintf("name=%s", name),
		fmt.Sprintf("timestamp=%s", ts),
		fmt.Sprintf("part=%s", part),
	}
	if optimized {
		tags = append(tags, "optimized=true")
	}
	if snapshot {
		tags = append(tags, "snapshot=true")
	}
	return tags
}
