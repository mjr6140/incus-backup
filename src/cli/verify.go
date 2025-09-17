package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"incus-backup/src/target"
)

func newVerifyCmd(stdout, stderr io.Writer) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "verify [all|instances|volumes|images|config]",
		Short: "Verify checksums for snapshots in the target",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := "all"
			if len(args) == 1 {
				kind = strings.ToLower(args[0])
			}
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return errors.New("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}
			if tgt.Scheme == "restic" {
				return resticNotImplemented(cmd)
			}
			if tgt.Scheme != "dir" {
				return fmt.Errorf("verify: unsupported backend %s", tgt.Scheme)
			}

			if output == "json" {
				// Collect then emit JSON to produce valid array output.
				results, err := runVerify(tgt.DirPath, kind)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(results)
			}
			// Stream table output incrementally for faster feedback.
			// Use fixed-width columns to avoid misalignment as we flush rows.
			const (
				wType = 8
				wProj = 12
				wPool = 12
				wName = 18
				wFP   = 16
				wTS   = 16
			)
			headerFmt := fmt.Sprintf("%%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%-%ds %%s\n",
				wType, wProj, wPool, wName, wFP, wTS)
			rowFmt := headerFmt
			fmt.Fprintf(stdout, headerFmt, "TYPE", "PROJECT", "POOL", "NAME", "FINGERPRINT", "TIMESTAMP", "STATUS")
			err = runVerifyStreaming(tgt.DirPath, kind, func(r verifyResult) {
				fmt.Fprintf(stdout, rowFmt,
					safePad(r.Type), safePad(r.Project), safePad(r.Pool), safePad(r.Name), safePad(r.Fingerprint), safePad(r.Timestamp), r.Status)
				for _, file := range r.Files {
					fmt.Fprintf(stdout, "    - %s: %s\n", file.Name, renderFileDetail(file))
				}
			})
			return err
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json")
	return cmd
}

type verifyResult struct {
	Type        string             `json:"type"`
	Project     string             `json:"project,omitempty"`
	Pool        string             `json:"pool,omitempty"`
	Name        string             `json:"name,omitempty"`
	Fingerprint string             `json:"fingerprint,omitempty"`
	Timestamp   string             `json:"timestamp"`
	Status      string             `json:"status"`
	Path        string             `json:"path"`
	Files       []verifyFileResult `json:"files,omitempty"`
}

type verifyFileResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Error    string `json:"error,omitempty"`
}

func runVerify(root, kind string) ([]verifyResult, error) {
	var out []verifyResult
	if err := walkSnapshots(root, kind, func(r verifyResult) { out = append(out, r) }); err != nil {
		return nil, err
	}
	return out, nil
}

func safePad(s string) string { return s }

func renderFileDetail(f verifyFileResult) string {
	switch f.Status {
	case "mismatch":
		return fmt.Sprintf("mismatch (expected=%s actual=%s)", f.Expected, f.Actual)
	case "missing":
		if f.Error != "" {
			return fmt.Sprintf("missing (%s)", f.Error)
		}
		if f.Expected != "" {
			return fmt.Sprintf("missing (expected=%s)", f.Expected)
		}
	case "error":
		if f.Error != "" {
			return fmt.Sprintf("error (%s)", f.Error)
		}
	}
	if f.Error != "" {
		return fmt.Sprintf("%s (%s)", f.Status, f.Error)
	}
	return f.Status
}

// runVerifyStreaming walks snapshots and calls cb for each result, allowing
// callers to print progress incrementally.
func runVerifyStreaming(root, kind string, cb func(verifyResult)) error {
	return walkSnapshots(root, kind, cb)
}

func walkSnapshots(root, kind string, cb func(verifyResult)) error {
	if kind == "all" || kind == "instances" {
		instBase := filepath.Join(root, "instances")
		for _, project := range sortedVisibleDirs(instBase) {
			projPath := filepath.Join(instBase, project)
			for _, name := range sortedVisibleDirs(projPath) {
				snapPath := filepath.Join(projPath, name)
				for _, ts := range sortedVisibleDirs(snapPath) {
					dir := filepath.Join(snapPath, ts)
					status, files := verifySnapshotDir(dir)
					cb(verifyResult{Type: "instance", Project: project, Name: name, Timestamp: ts, Status: status, Path: dir, Files: files})
				}
			}
		}
	}
	if kind == "all" || kind == "volumes" {
		volBase := filepath.Join(root, "volumes")
		for _, project := range sortedVisibleDirs(volBase) {
			projPath := filepath.Join(volBase, project)
			for _, pool := range sortedVisibleDirs(projPath) {
				poolPath := filepath.Join(projPath, pool)
				for _, name := range sortedVisibleDirs(poolPath) {
					namePath := filepath.Join(poolPath, name)
					for _, ts := range sortedVisibleDirs(namePath) {
						dir := filepath.Join(namePath, ts)
						status, files := verifySnapshotDir(dir)
						cb(verifyResult{Type: "volume", Project: project, Pool: pool, Name: name, Timestamp: ts, Status: status, Path: dir, Files: files})
					}
				}
			}
		}
	}
	if kind == "all" || kind == "images" {
		imgBase := filepath.Join(root, "images")
		for _, fingerprint := range sortedVisibleDirs(imgBase) {
			imgPath := filepath.Join(imgBase, fingerprint)
			for _, ts := range sortedVisibleDirs(imgPath) {
				dir := filepath.Join(imgPath, ts)
				status, files := verifySnapshotDir(dir)
				cb(verifyResult{Type: "image", Fingerprint: fingerprint, Timestamp: ts, Status: status, Path: dir, Files: files})
			}
		}
	}
	if kind == "all" || kind == "config" {
		cfgBase := filepath.Join(root, "config")
		for _, ts := range sortedVisibleDirs(cfgBase) {
			dir := filepath.Join(cfgBase, ts)
			status, files := verifySnapshotDir(dir)
			cb(verifyResult{Type: "config", Timestamp: ts, Status: status, Path: dir, Files: files})
		}
	}
	return nil
}

func sortedVisibleDirs(path string) []string {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names
}

func verifySnapshotDir(dir string) (string, []verifyFileResult) {
	f, err := os.Open(filepath.Join(dir, "checksums.txt"))
	if err != nil {
		return "error", []verifyFileResult{{Name: "checksums.txt", Status: "missing", Error: err.Error()}}
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var files []verifyFileResult
	hasMismatch := false
	hasError := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) != 2 {
			hasError = true
			files = append(files, verifyFileResult{Name: line, Status: "error", Error: "invalid checksum entry"})
			continue
		}
		want := parts[0]
		name := parts[1]
		actual, ferr := sha256File(filepath.Join(dir, name))
		if ferr != nil {
			hasError = true
			status := "error"
			if errors.Is(ferr, os.ErrNotExist) {
				status = "missing"
			}
			files = append(files, verifyFileResult{Name: name, Status: status, Expected: want, Error: ferr.Error()})
			continue
		}
		if strings.EqualFold(want, actual) {
			files = append(files, verifyFileResult{Name: name, Status: "ok", Expected: want, Actual: actual})
			continue
		}
		hasMismatch = true
		files = append(files, verifyFileResult{Name: name, Status: "mismatch", Expected: want, Actual: actual})
	}
	if err := scanner.Err(); err != nil {
		hasError = true
		files = append(files, verifyFileResult{Name: "checksums.txt", Status: "error", Error: err.Error()})
	}
	switch {
	case hasError:
		return "error", files
	case hasMismatch:
		return "mismatch", files
	default:
		return "ok", files
	}
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
