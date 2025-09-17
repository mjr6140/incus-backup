package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"incus-backup/src/restic"
	"incus-backup/src/safety"
)

type resticDetectorFunc func(context.Context) (restic.BinaryInfo, error)

var detectResticFn resticDetectorFunc = restic.Detect

func checkResticBinary(cmd *cobra.Command, interactive bool) (restic.BinaryInfo, error) {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	info, err := detectResticFn(ctx)
	if err != nil {
		return restic.BinaryInfo{}, err
	}
	if restic.IsCompatible(info.Version) {
		return info, nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Warning: restic %s detected; incus-backup requires %s or newer.\n", info.Version, restic.RequiredVersion)
	if !interactive {
		return restic.BinaryInfo{}, fmt.Errorf("restic %s is older than required %s", info.Version, restic.RequiredVersion)
	}

	opts := getSafetyOptions(cmd)
	if opts.Yes || opts.Force {
		return info, nil
	}
	ok, err := safety.Confirm(opts, cmd.InOrStdin(), cmd.OutOrStdout(), "Proceed with unsupported restic version?")
	if err != nil {
		return restic.BinaryInfo{}, err
	}
	if !ok {
		return restic.BinaryInfo{}, errors.New("aborted: restic version is below supported minimum")
	}
	return info, nil
}

func resticNotImplemented(cmd *cobra.Command) error {
	if _, err := checkResticBinary(cmd, true); err != nil {
		return err
	}
	return fmt.Errorf("restic backend is not implemented yet")
}

// SetResticDetectorForTest allows tests to stub the restic detection pipeline.
// The returned function restores the previous detector.
func SetResticDetectorForTest(fn resticDetectorFunc) func() {
	prev := detectResticFn
	detectResticFn = fn
	return func() {
		detectResticFn = prev
	}
}
