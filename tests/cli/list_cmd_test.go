package cli_test

import (
    "bytes"
    "os"
    "path/filepath"
    "strings"
    "testing"

    "incus-backup/src/cli"
)

func TestListCmd_Table_All(t *testing.T) {
    root := t.TempDir()
    mk := func(p string) { if err := os.MkdirAll(filepath.Join(root, p), 0o755); err != nil { t.Fatalf("mkdir %s: %v", p, err) } }
    mk("instances/default/web/20250101T010101Z")
    mk("volumes/default/pool/vol1/20250102T020202Z")
    mk("images/abcdef/20250103T030303Z")
    mk("config/20250104T040404Z")

    var out, err bytes.Buffer
    cmd := cli.NewRootCmd(&out, &err)
    cmd.SetArgs([]string{"list", "all", "--target", "dir:" + root})
    if _, e := cmd.ExecuteC(); e != nil {
        t.Fatalf("unexpected error: %v", e)
    }
    s := out.String()
    if !strings.Contains(s, "TYPE") || !strings.Contains(s, "TIMESTAMP") {
        t.Fatalf("missing header in table output: %q", s)
    }
    if !strings.Contains(s, "instance") || !strings.Contains(s, "default") || !strings.Contains(s, "20250101T010101Z") {
        t.Fatalf("missing expected instance row content: %q", s)
    }
}
