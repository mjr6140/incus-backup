//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	backendpkg "incus-backup/src/backend"
	"incus-backup/src/cli"
	restictest "incus-backup/tests/internal/restictest"
)

type resticSnapshotJSON struct {
	ID   string    `json:"id"`
	Time time.Time `json:"time"`
	Tags []string  `json:"tags"`
}

func TestResticPruneCommand(t *testing.T) {
	if os.Getenv("INCUS_TESTS") != "1" {
		t.Skip("INCUS_TESTS=1 not set")
	}

	restictest.RequireBinary(t)
	repoParent := t.TempDir()
	repo := restictest.InitRepo(t, repoParent)
	t.Setenv("RESTIC_PASSWORD", restictest.TestPassword)

	proj := "itest-restic-prune-" + time.Now().UTC().Format("20060102T150405")
	run(t, "incus", "project", "create", proj)
	t.Cleanup(func() { _ = exec.Command("incus", "project", "delete", proj).Run() })

	inst := "rpinst"
	run(t, "incus", "--project", proj, "launch", "images:alpine/3.18", inst)
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "delete", "--force", inst).Run() })

	pool := "default"
	vol := "rpvol"
	run(t, "incus", "--project", proj, "storage", "volume", "create", pool, vol, "size=16MiB")
	t.Cleanup(func() { _ = exec.Command("incus", "--project", proj, "storage", "volume", "delete", pool, vol).Run() })

	performBackups := func() {
		var out, errBuf bytes.Buffer
		cmd := cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"backup", "config", "--target", "restic:" + repo})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("config backup failed: %v; stderr=%s", err, errBuf.String())
		}

		out.Reset()
		errBuf.Reset()
		cmd = cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"backup", "instances", inst, "--project", proj, "--target", "restic:" + repo})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("instance backup failed: %v; stderr=%s", err, errBuf.String())
		}

		out.Reset()
		errBuf.Reset()
		cmd = cli.NewRootCmd(&out, &errBuf)
		cmd.SetArgs([]string{"backup", "volumes", pool + "/" + vol, "--project", proj, "--target", "restic:" + repo})
		if _, err := cmd.ExecuteC(); err != nil {
			t.Fatalf("volume backup failed: %v; stderr=%s", err, errBuf.String())
		}
	}

	performBackups()
	time.Sleep(2 * time.Second)
	run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo second > /root/payload.txt")
	run(t, "incus", "--project", proj, "storage", "volume", "attach", pool, vol, inst, "/mnt/vol")
	run(t, "incus", "--project", proj, "exec", inst, "--", "sh", "-lc", "echo second > /mnt/vol/payload.txt")
	run(t, "incus", "--project", proj, "storage", "volume", "detach", pool, vol, inst)
	performBackups()

	beforeInst := resticTimestamps(t, repo, "type=instance")
	if len(beforeInst) < 2 {
		t.Fatalf("expected multiple instance timestamps before prune, got %v", beforeInst)
	}
	latestInst := beforeInst[len(beforeInst)-1]

	beforeCfg := resticTimestamps(t, repo, "type=config")
	if len(beforeCfg) < 2 {
		t.Fatalf("expected multiple config timestamps before prune, got %v", beforeCfg)
	}
	latestCfg := beforeCfg[len(beforeCfg)-1]

	beforeVol := resticTimestamps(t, repo, "type=volume")
	if len(beforeVol) < 2 {
		t.Fatalf("expected multiple volume timestamps before prune, got %v", beforeVol)
	}
	latestVol := beforeVol[len(beforeVol)-1]

	var out, errBuf bytes.Buffer
	cmd := cli.NewRootCmd(&out, &errBuf)
	cmd.SetArgs([]string{"prune", "--target", "restic:" + repo, "--keep", "1", "--yes"})
	if _, err := cmd.ExecuteC(); err != nil {
		t.Fatalf("prune failed: %v; stderr=%s", err, errBuf.String())
	}

	afterInst := resticTimestamps(t, repo, "type=instance")
	if len(afterInst) != 1 || afterInst[0] != latestInst {
		t.Fatalf("expected single instance timestamp %s, got %v", latestInst, afterInst)
	}
	afterCfg := resticTimestamps(t, repo, "type=config")
	if len(afterCfg) != 1 || afterCfg[0] != latestCfg {
		t.Fatalf("expected single config timestamp %s, got %v", latestCfg, afterCfg)
	}
	afterVol := resticTimestamps(t, repo, "type=volume")
	if len(afterVol) != 1 || afterVol[0] != latestVol {
		t.Fatalf("expected single volume timestamp %s, got %v", latestVol, afterVol)
	}
}

func resticTimestamps(t *testing.T, repo, tag string) []string {
	t.Helper()
	cmd := restictest.Command(repo, "restic", "snapshots", "--json", "--tag", tag)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("restic snapshots failed: %v\n%s", err, string(out))
	}
	var snaps []resticSnapshotJSON
	if err := json.Unmarshal(out, &snaps); err != nil {
		t.Fatalf("decode restic snapshots: %v", err)
	}
	var timestamps []string
	for _, snap := range snaps {
		ts := extractTimestamp(tagMapFromList(snap.Tags), snap.Time)
		timestamps = append(timestamps, ts)
	}
	sort.Strings(timestamps)
	return timestamps
}

func extractTimestamp(tags map[string]string, fallback time.Time) string {
	if ts, ok := tags["timestamp"]; ok && ts != "" {
		return ts
	}
	return fallback.UTC().Format("20060102T150405Z")
}

func tagMapFromList(tags []string) map[string]string {
	out := make(map[string]string, len(tags))
	for _, tag := range tags {
		parts := strings.SplitN(tag, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		} else {
			out[tag] = ""
		}
	}
	return out
}
