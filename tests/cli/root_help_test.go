package cli_test

import (
    "bytes"
    "strings"
    "testing"

    "incus-backup/src/cli"
)

func TestRootHelp_ShowsUsage(t *testing.T) {
    var out, err bytes.Buffer
    cmd := cli.NewRootCmd(&out, &err)
    cmd.SetArgs([]string{"--help"})

    if _, e := cmd.ExecuteC(); e != nil {
        t.Fatalf("unexpected error: %v", e)
    }
    o := out.String()
    if !strings.Contains(o, "Usage:") || !strings.Contains(o, "incus-backup") {
        t.Fatalf("help output missing expected content; got: %s", o)
    }
}

