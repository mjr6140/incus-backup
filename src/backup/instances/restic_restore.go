package instances

import (
	"context"
	"fmt"
	"io"

	"incus-backup/src/incusapi"
	"incus-backup/src/restic"
	pg "incus-backup/src/util/progress"
)

// RestoreInstanceRestic streams an instance export from a restic snapshot into Incus.
func RestoreInstanceRestic(ctx context.Context, bin restic.BinaryInfo, repo string, snapshot restic.Snapshot, client incusapi.Client, project, sourceName, targetName string, progressOut io.Writer) error {
	exportPath := exportFilename(project, sourceName, snapshot)
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

	importErr := client.ImportInstance(project, targetName, reader, progressOut)
	dumpErr := <-errCh
	if importErr != nil {
		return importErr
	}
	if dumpErr != nil {
		return fmt.Errorf("restic dump: %w", dumpErr)
	}
	return nil
}

func exportFilename(project, name string, snapshot restic.Snapshot) string {
	tags := snapshot.TagMap()
	ts := tags["timestamp"]
	if ts == "" {
		ts = snapshot.Time.UTC().Format("20060102T150405Z")
	}
	if tagsName := tags["name"]; tagsName != "" {
		name = tagsName
	}
	if tagsProject := tags["project"]; tagsProject != "" {
		project = tagsProject
	}
	return fmt.Sprintf("instances/%s/%s/%s/export.tar.xz", project, name, ts)
}
