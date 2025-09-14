package target

import (
    "fmt"
    "path/filepath"
    "runtime"
    "strings"
)

// Target represents a parsed backup target URI.
// Example: dir:/mnt/nas/sysbackup/incus
type Target struct {
    // Raw is the original input string.
    Raw string
    // Scheme is the backend scheme (e.g., "dir").
    Scheme string
    // Value is the scheme-specific value. For dir, this is the absolute path.
    Value string

    // DirPath is set when Scheme == "dir" and contains a cleaned absolute path.
    DirPath string
}

// SupportedSchemes lists the schemes the parser accepts.
var SupportedSchemes = map[string]struct{}{
    "dir": {},
}

// Parse parses a target URI like "dir:/path" into a Target structure.
func Parse(raw string) (Target, error) {
    t := Target{Raw: raw}
    s := strings.TrimSpace(raw)
    if s == "" {
        return t, fmt.Errorf("target must not be empty; expected format 'dir:/path'")
    }
    // Expect <scheme>:<value>
    i := strings.Index(s, ":")
    if i <= 0 || i == len(s)-1 {
        return t, fmt.Errorf("invalid target %q; expected format '<scheme>:<value>' (e.g., 'dir:/path')", raw)
    }
    scheme := strings.ToLower(strings.TrimSpace(s[:i]))
    val := strings.TrimSpace(s[i+1:])
    if _, ok := SupportedSchemes[scheme]; !ok {
        return t, fmt.Errorf("unsupported backend scheme %q", scheme)
    }
    t.Scheme = scheme
    t.Value = val

    switch scheme {
    case "dir":
        // Require absolute paths. On Windows, allow paths like C:\\ but our
        // primary target is Linux hosts; still be defensive.
        if val == "" {
            return t, fmt.Errorf("directory target path must not be empty")
        }
        // Normalize multiple slashes etc.
        clean := filepath.Clean(val)
        // Treat paths starting with \\?\ as absolute on Windows; otherwise use IsAbs
        if !(filepath.IsAbs(clean) || (runtime.GOOS == "windows" && strings.HasPrefix(clean, `\\\\?\\`))) {
            return t, fmt.Errorf("directory target must be an absolute path: %q", val)
        }
        t.DirPath = clean
        // Set canonical Value to cleaned path
        t.Value = clean
    }
    return t, nil
}

// IsSupported returns true if the scheme is recognized.
func IsSupported(scheme string) bool {
    _, ok := SupportedSchemes[strings.ToLower(scheme)]
    return ok
}

// String returns a canonical string form of the target.
func (t Target) String() string {
    if t.Scheme == "dir" && t.DirPath != "" {
        return fmt.Sprintf("%s:%s", t.Scheme, t.DirPath)
    }
    if t.Scheme != "" {
        return fmt.Sprintf("%s:%s", t.Scheme, t.Value)
    }
    return t.Raw
}

