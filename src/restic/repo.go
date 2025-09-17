package restic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// EnsureRepository verifies that the given repository path has been
// initialised. If the repository is missing it attempts to run `restic init`.
func EnsureRepository(ctx context.Context, bin BinaryInfo, repo string) error {
	if err := probeSnapshots(ctx, bin, repo, true); err == nil {
		return nil
	} else if !isUnknownLimitFlag(err) {
		if isNotRepository(err.Error()) {
			if _, initStderr, initErr := runCommand(ctx, bin, repo, []string{"init"}, nil); initErr != nil {
				return fmt.Errorf("restic: init repository: %w: %s", initErr, initStderr)
			}
			return nil
		}
		return err
	}
	// Retry without --limit for restic versions that do not support it.
	if err := probeSnapshots(ctx, bin, repo, false); err == nil {
		return nil
	} else if isNotRepository(err.Error()) {
		if _, initStderr, initErr := runCommand(ctx, bin, repo, []string{"init"}, nil); initErr != nil {
			return fmt.Errorf("restic: init repository: %w: %s", initErr, initStderr)
		}
		return nil
	} else {
		return err
	}
}

func probeSnapshots(ctx context.Context, bin BinaryInfo, repo string, useLimit bool) error {
	args := []string{"snapshots", "--json"}
	if useLimit {
		args = append(args, "--limit", "1")
	}
	_, stderr, err := runCommand(ctx, bin, repo, args, nil)
	if err != nil {
		return fmt.Errorf("restic: probe repository: %w: %s", err, stderr)
	}
	return nil
}

func isUnknownLimitFlag(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "unknown flag: --limit") || strings.Contains(err.Error(), "unknown option --limit")
}

// BackupStream runs `restic backup --stdin` with the provided reader.
func BackupStream(ctx context.Context, bin BinaryInfo, repo string, filename string, tags []string, r io.Reader, progress io.Writer) error {
	args := []string{"backup", "--stdin", "--stdin-filename", filename}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	cmd := exec.CommandContext(ctx, bin.Path, args...)
	cmd.Env = appendRepoEnv(cmd.Env, repo)
	if progress != nil {
		cmd.Stdout = progress
		cmd.Stderr = progress
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("restic: acquire stdin: %w", err)
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("restic: start backup: %w", err)
	}
	copyErr := make(chan error, 1)
	go func() {
		_, err := io.Copy(stdin, r)
		stdin.Close()
		copyErr <- err
	}()
	waitErr := cmd.Wait()
	streamErr := <-copyErr
	if streamErr != nil {
		return fmt.Errorf("restic: stream backup data: %w", streamErr)
	}
	if waitErr != nil {
		return fmt.Errorf("restic: backup failed: %w", waitErr)
	}
	return nil
}

// BackupBytes is a convenience wrapper around BackupStream for small payloads
// such as manifests or checksums.
func BackupBytes(ctx context.Context, bin BinaryInfo, repo string, filename string, tags []string, data []byte, progress io.Writer) error {
	return BackupStream(ctx, bin, repo, filename, tags, bytes.NewReader(data), progress)
}

// Dump streams the specified file from a snapshot to the writer via
// `restic dump`.
func Dump(ctx context.Context, bin BinaryInfo, repo string, snapshotID string, path string, w io.Writer, progress io.Writer) error {
	args := []string{"dump", snapshotID, path}
	cmd := exec.CommandContext(ctx, bin.Path, args...)
	cmd.Env = appendRepoEnv(cmd.Env, repo)
	cmd.Stdout = w
	if progress != nil {
		cmd.Stderr = progress
	} else {
		cmd.Stderr = io.Discard
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("restic: dump %s: %w", path, err)
	}
	return nil
}

// ForgetSnapshots removes the specified snapshot IDs from the repository. When
// prune is true the command also performs `restic forget --prune` to reclaim
// storage immediately.
func ForgetSnapshots(ctx context.Context, bin BinaryInfo, repo string, snapshotIDs []string, prune bool) error {
	if len(snapshotIDs) == 0 {
		return nil
	}
	args := []string{"forget"}
	if prune {
		args = append(args, "--prune")
	}
	args = append(args, snapshotIDs...)
	if _, stderr, err := runCommand(ctx, bin, repo, args, nil); err != nil {
		return fmt.Errorf("restic: forget snapshots: %w: %s", err, stderr)
	}
	return nil
}

// Snapshot represents a restic snapshot as returned by `restic snapshots --json`.
type Snapshot struct {
	ID      string    `json:"id"`
	ShortID string    `json:"short_id"`
	Time    time.Time `json:"time"`
	Tags    []string  `json:"tags"`
	Paths   []string  `json:"paths"`
}

// TagMap converts a snapshot's tags (key=value) into a map.
func (s Snapshot) TagMap() map[string]string {
	out := make(map[string]string, len(s.Tags))
	for _, tag := range s.Tags {
		if parts := strings.SplitN(tag, "=", 2); len(parts) == 2 {
			out[parts[0]] = parts[1]
		} else {
			out[tag] = ""
		}
	}
	return out
}

// ListSnapshots returns snapshots matching the provided tags.
func ListSnapshots(ctx context.Context, bin BinaryInfo, repo string, tags []string) ([]Snapshot, error) {
	args := []string{"snapshots", "--json"}
	for _, tag := range tags {
		args = append(args, "--tag", tag)
	}
	stdout, stderr, err := runCommand(ctx, bin, repo, args, nil)
	if err != nil {
		return nil, fmt.Errorf("restic: list snapshots: %w: %s", err, stderr)
	}
	var snaps []Snapshot
	if err := json.Unmarshal([]byte(stdout), &snaps); err != nil {
		return nil, fmt.Errorf("restic: parse snapshots json: %w", err)
	}
	sort.Slice(snaps, func(i, j int) bool { return snaps[i].Time.Before(snaps[j].Time) })
	return snaps, nil
}

func appendRepoEnv(env []string, repo string) []string {
	if env == nil {
		env = os.Environ()
	}
	return append(env, fmt.Sprintf("RESTIC_REPOSITORY=%s", repo))
}

func isNotRepository(stderr string) bool {
	s := strings.ToLower(stderr)
	return strings.Contains(s, "is not a repository") || strings.Contains(s, "does not look like a restic repository")
}

func runCommand(ctx context.Context, bin BinaryInfo, repo string, args []string, stdin io.Reader) (string, string, error) {
	cmd := exec.CommandContext(ctx, bin.Path, args...)
	cmd.Env = appendRepoEnv(nil, repo)
	if stdin != nil {
		cmd.Stdin = stdin
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	return stdoutBuf.String(), stderrBuf.String(), err
}
