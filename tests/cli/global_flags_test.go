package cli_test

import (
    "testing"

    "incus-backup/src/cli"
)

func TestGlobalFlags_Present(t *testing.T) {
    cmd := cli.NewRootCmd(nil, nil)
    for _, name := range []string{"dry-run", "yes", "force"} {
        if f := cmd.PersistentFlags().Lookup(name); f == nil {
            t.Fatalf("missing global flag --%s", name)
        }
    }
}

