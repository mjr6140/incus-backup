package safety

import (
    "bufio"
    "fmt"
    "io"
    "strings"
)

// Confirm prompts the user to confirm a potentially destructive action.
// - If opts.Yes is true, it returns true without prompting.
// - If opts.DryRun is true, it returns false but no error (no action should be taken).
// The caller decides what to do with the result.
func Confirm(opts Options, in io.Reader, out io.Writer, question string) (bool, error) {
    if opts.DryRun {
        // No changes in dry-run mode; treat as declined.
        return false, nil
    }
    if opts.Yes {
        return true, nil
    }
    if out != nil {
        fmt.Fprintf(out, "%s [y/N]: ", strings.TrimSpace(question))
    }
    reader := bufio.NewReader(in)
    line, err := reader.ReadString('\n')
    if err != nil && err != io.EOF {
        return false, err
    }
    ans := strings.TrimSpace(strings.ToLower(line))
    return ans == "y" || ans == "yes", nil
}

