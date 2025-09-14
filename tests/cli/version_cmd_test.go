package cli_test

import (
    "bytes"
    "strings"
    "testing"

    "incus-backup/src/cli"
    "incus-backup/src/version"
)

func TestVersionCommand_PrintsVersion(t *testing.T) {
    var out, err bytes.Buffer
    cmd := cli.NewRootCmd(&out, &err)
    cmd.SetArgs([]string{"version"})

    if _, e := cmd.ExecuteC(); e != nil {
        t.Fatalf("unexpected error: %v", e)
    }
    if !strings.Contains(out.String(), version.Version) {
        t.Fatalf("expected version %q in output; got: %s", version.Version, out.String())
    }
}

