package restic

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// RequiredVersion defines the minimum restic release we support.
const RequiredVersion = "0.18.0"

// BinaryInfo describes a detected restic CLI binary.
type BinaryInfo struct {
	Path    string
	Version string
}

var versionRegexp = regexp.MustCompile(`restic\s+([0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.]+)?)`)

// Detect locates the restic binary on PATH, queries its version, and returns
// the gathered metadata. The context is used to bound the version subprocess.
func Detect(ctx context.Context) (BinaryInfo, error) {
	exe, err := exec.LookPath("restic")
	if err != nil {
		return BinaryInfo{}, fmt.Errorf("restic binary not found on PATH: %w", err)
	}
	ver, err := queryVersion(ctx, exe)
	if err != nil {
		return BinaryInfo{}, err
	}
	return BinaryInfo{Path: exe, Version: ver}, nil
}

// IsCompatible reports whether the provided version satisfies the minimum
// supported restic release.
func IsCompatible(version string) bool {
	left, ok := parseSemVersion(version)
	if !ok {
		return false
	}
	right, ok := parseSemVersion(RequiredVersion)
	if !ok {
		return false
	}
	return compareSemVersion(left, right) >= 0
}

// queryVersion executes `restic version` and parses the semantic version from
// its output.
func queryVersion(ctx context.Context, exe string) (string, error) {
	// Guard against commands that hang by applying a short timeout.
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, exe, "version")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("restic: capture stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("restic: capture stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("restic: start version command: %w", err)
	}

	version, parseErr := parseVersion(stdout)
	if version == "" && parseErr == nil {
		// Fall back to scanning stderr when stdout did not contain the version.
		version, parseErr = parseVersion(stderr)
	}
	waitErr := cmd.Wait()
	if parseErr != nil {
		return "", parseErr
	}
	if version == "" {
		return "", errors.New("restic: could not parse version output")
	}
	if waitErr != nil {
		return "", fmt.Errorf("restic: version command failed: %w", waitErr)
	}
	return version, nil
}

func parseVersion(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if matches := versionRegexp.FindStringSubmatch(line); len(matches) == 2 {
			return matches[1], nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("restic: read version output: %w", err)
	}
	return "", nil
}

// ExtractVersion is a helper that derives the restic version string from the
// supplied command output. It is primarily exposed for testing.
func ExtractVersion(output string) (string, error) {
	return parseVersion(strings.NewReader(output))
}

type semVersion struct {
	major int
	minor int
	patch int
	pre   string
}

func parseSemVersion(s string) (semVersion, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return semVersion{}, false
	}
	parts := strings.SplitN(s, "-", 2)
	core := parts[0]
	nums := strings.Split(core, ".")
	if len(nums) != 3 {
		return semVersion{}, false
	}
	major, err := strconv.Atoi(nums[0])
	if err != nil {
		return semVersion{}, false
	}
	minor, err := strconv.Atoi(nums[1])
	if err != nil {
		return semVersion{}, false
	}
	patch, err := strconv.Atoi(nums[2])
	if err != nil {
		return semVersion{}, false
	}
	var pre string
	if len(parts) == 2 {
		pre = parts[1]
	}
	return semVersion{major: major, minor: minor, patch: patch, pre: pre}, true
}

func compareSemVersion(a, b semVersion) int {
	switch {
	case a.major != b.major:
		if a.major > b.major {
			return 1
		}
		return -1
	case a.minor != b.minor:
		if a.minor > b.minor {
			return 1
		}
		return -1
	case a.patch != b.patch:
		if a.patch > b.patch {
			return 1
		}
		return -1
	}
	if a.pre == b.pre {
		return 0
	}
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	return strings.Compare(a.pre, b.pre)
}
