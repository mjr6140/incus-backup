package volumes

import (
	"context"
	"fmt"
	"io"

	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	pg "incus-backup/src/util/progress"
)

// RestoreVolumeRestic streams a volume tarball from restic into Incus.
func RestoreVolumeRestic(ctx context.Context, bin restic.BinaryInfo, repo string, snapshot restic.Snapshot, client incusapi.Client, project, pool, name string, progressOut io.Writer) error {
	exportPath := volumeExportFilename(project, pool, name, snapshot)
	pr, pw := io.Pipe()
	errCh := make(chan error, 1)

	go func() {
		err := restic.Dump(ctx, bin, repo, snapshot.ID, exportPath, pw, progressOut)
		_ = pw.CloseWithError(err)
		errCh <- err
	}()

	var reader io.Reader = pr
	if progressOut != nil {
		reader = pg.NewReader(reader, 0, "import", progressOut)
	}

	importErr := client.ImportVolume(project, pool, name, reader, progressOut)
	dumpErr := <-errCh
	if importErr != nil {
		return importErr
	}
	if dumpErr != nil {
		return fmt.Errorf("restic dump: %w", dumpErr)
	}
	return nil
}

func volumeExportFilename(project, pool, name string, snapshot restic.Snapshot) string {
	tags := snapshot.TagMap()
	if t := tags["project"]; t != "" {
		project = t
	}
	if t := tags["pool"]; t != "" {
		pool = t
	}
	if t := tags["name"]; t != "" {
		name = t
	}
	ts := tags["timestamp"]
	if ts == "" {
		ts = snapshot.Time.UTC().Format("20060102T150405Z")
	}
	return fmt.Sprintf("volumes/%s/%s/%s/%s/volume.tar.xz", project, pool, name, ts)
}
