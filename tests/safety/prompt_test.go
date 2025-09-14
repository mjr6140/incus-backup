package safety_test

import (
    "bytes"
    "strings"
    "testing"

    "incus-backup/src/safety"
)

func TestConfirm_AutoYes(t *testing.T) {
    in := strings.NewReader("")
    var out bytes.Buffer
    ok, err := safety.Confirm(safety.Options{Yes: true}, in, &out, "proceed?")
    if err != nil { t.Fatal(err) }
    if !ok { t.Fatalf("expected auto-yes to confirm") }
}

func TestConfirm_DryRun(t *testing.T) {
    in := strings.NewReader("y\n")
    var out bytes.Buffer
    ok, err := safety.Confirm(safety.Options{DryRun: true}, in, &out, "proceed?")
    if err != nil { t.Fatal(err) }
    if ok { t.Fatalf("expected dry-run to decline") }
}

func TestConfirm_UserInput(t *testing.T) {
    cases := []struct{ in string; want bool }{
        {"y\n", true},
        {"yes\n", true},
        {"Y\n", true},
        {"No\n", false},
        {"\n", false},
    }
    for _, c := range cases {
        in := strings.NewReader(c.in)
        var out bytes.Buffer
        got, err := safety.Confirm(safety.Options{}, in, &out, "apply changes?")
        if err != nil { t.Fatal(err) }
        if got != c.want {
            t.Fatalf("input %q: got %v want %v", c.in, got, c.want)
        }
        if !strings.Contains(out.String(), "apply changes?") {
            t.Fatalf("prompt missing question; got %q", out.String())
        }
    }
}

