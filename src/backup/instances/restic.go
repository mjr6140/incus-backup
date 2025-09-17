package instances

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	pg "incus-backup/src/util/progress"
)

// BackupInstanceRestic streams an instance export directly into a restic repository.
func BackupInstanceRestic(ctx context.Context, bin restic.BinaryInfo, repo string, client incusapi.Client, project, name string, optimized, snapshot bool, now time.Time, progressOut io.Writer) (string, error) {
	if err := restic.EnsureRepository(ctx, bin, repo); err != nil {
		return "", err
	}

	ts := now.UTC().Format("20060102T150405Z")
	snapName := ""
	if snapshot {
		snapName = "tmp-incus-backup-" + ts
		if progressOut != nil {
			fmt.Fprintf(progressOut, "[snapshot] create %s@%s\n", name, snapName)
		}
		if err := client.CreateInstanceSnapshot(project, name, snapName); err != nil {
			return "", err
		}
		defer func() {
			if progressOut != nil {
				fmt.Fprintf(progressOut, "[snapshot] delete %s@%s\n", name, snapName)
			}
			_ = client.DeleteInstanceSnapshot(project, name, snapName)
		}()
	}

	export, err := client.ExportInstance(project, name, optimized, snapName, progressOut)
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

	filename := fmt.Sprintf("instances/%s/%s/%s/export.tar.xz", project, name, ts)
	tags := resticTagsForInstance(project, name, ts, "data", optimized, snapshot)
	if err := restic.BackupStream(ctx, bin, repo, filename, tags, reader, progressOut); err != nil {
		return "", err
	}

	sum := hex.EncodeToString(hash.Sum(nil))
	manifest := Manifest{
		Type:      "instance",
		Project:   project,
		Name:      name,
		CreatedAt: now.UTC(),
		Options: map[string]string{
			"snapshot":  fmt.Sprintf("%t", snapshot),
			"optimized": fmt.Sprintf("%t", optimized),
		},
	}
	mfBytes, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return "", err
	}
	manifestPath := strings.ReplaceAll(filename, "export.tar.xz", "manifest.json")
	if err := restic.BackupBytes(ctx, bin, repo, manifestPath, resticTagsForInstance(project, name, ts, "manifest", optimized, snapshot), mfBytes, progressOut); err != nil {
		return "", err
	}

	checksums := fmt.Sprintf("%s  export.tar.xz\n", sum)
	checksumPath := strings.ReplaceAll(filename, "export.tar.xz", "checksums.txt")
	if err := restic.BackupBytes(ctx, bin, repo, checksumPath, resticTagsForInstance(project, name, ts, "checksums", optimized, snapshot), []byte(checksums), progressOut); err != nil {
		return "", err
	}

	return ts, nil
}

func resticTagsForInstance(project, name, ts, part string, optimized, snapshot bool) []string {
	tags := []string{
		"type=instance",
		"schema=v1",
		fmt.Sprintf("project=%s", project),
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
