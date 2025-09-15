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
    "strings"
    "text/tabwriter"

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
            if tgtStr == "" { return errors.New("--target is required (e.g., dir:/path)") }
            tgt, err := target.Parse(tgtStr)
            if err != nil { return err }
            if tgt.Scheme != "dir" { return fmt.Errorf("verify: only directory backend is supported") }

            results, err := runVerify(tgt.DirPath, kind)
            if err != nil { return err }
            switch output {
            case "json":
                enc := json.NewEncoder(stdout)
                enc.SetIndent("", "  ")
                return enc.Encode(results)
            default:
                tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
                fmt.Fprintln(tw, "TYPE\tPROJECT\tPOOL\tNAME\tFINGERPRINT\tTIMESTAMP\tSTATUS")
                for _, r := range results { fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Type, r.Project, r.Pool, r.Name, r.Fingerprint, r.Timestamp, r.Status) }
                return tw.Flush()
            }
        },
    }
    cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
    cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json")
    return cmd
}

type verifyResult struct {
    Type        string `json:"type"`
    Project     string `json:"project,omitempty"`
    Pool        string `json:"pool,omitempty"`
    Name        string `json:"name,omitempty"`
    Fingerprint string `json:"fingerprint,omitempty"`
    Timestamp   string `json:"timestamp"`
    Status      string `json:"status"`
    Path        string `json:"path"`
}

func runVerify(root, kind string) ([]verifyResult, error) {
    var out []verifyResult
    // instances
    if kind == "all" || kind == "instances" {
        instBase := filepath.Join(root, "instances")
        projects, _ := os.ReadDir(instBase)
        for _, pr := range projects { if !pr.IsDir() || strings.HasPrefix(pr.Name(), ".") { continue }
            names, _ := os.ReadDir(filepath.Join(instBase, pr.Name()))
            for _, nm := range names { if !nm.IsDir() || strings.HasPrefix(nm.Name(), ".") { continue }
                snaps, _ := os.ReadDir(filepath.Join(instBase, pr.Name(), nm.Name()))
                for _, ts := range snaps { if !ts.IsDir() || strings.HasPrefix(ts.Name(), ".") { continue }
                    p := filepath.Join(instBase, pr.Name(), nm.Name(), ts.Name())
                    status := verifySnapshotDir(p)
                    out = append(out, verifyResult{Type: "instance", Project: pr.Name(), Name: nm.Name(), Timestamp: ts.Name(), Status: status, Path: p})
                }
            }
        }
    }
    // volumes
    if kind == "all" || kind == "volumes" {
        volBase := filepath.Join(root, "volumes")
        projects, _ := os.ReadDir(volBase)
        for _, pr := range projects { if !pr.IsDir() || strings.HasPrefix(pr.Name(), ".") { continue }
            pools, _ := os.ReadDir(filepath.Join(volBase, pr.Name()))
            for _, pool := range pools { if !pool.IsDir() || strings.HasPrefix(pool.Name(), ".") { continue }
                names, _ := os.ReadDir(filepath.Join(volBase, pr.Name(), pool.Name()))
                for _, nm := range names { if !nm.IsDir() || strings.HasPrefix(nm.Name(), ".") { continue }
                    snaps, _ := os.ReadDir(filepath.Join(volBase, pr.Name(), pool.Name(), nm.Name()))
                    for _, ts := range snaps { if !ts.IsDir() || strings.HasPrefix(ts.Name(), ".") { continue }
                        p := filepath.Join(volBase, pr.Name(), pool.Name(), nm.Name(), ts.Name())
                        status := verifySnapshotDir(p)
                        out = append(out, verifyResult{Type: "volume", Project: pr.Name(), Pool: pool.Name(), Name: nm.Name(), Timestamp: ts.Name(), Status: status, Path: p})
                    }
                }
            }
        }
    }
    // images
    if kind == "all" || kind == "images" {
        imgBase := filepath.Join(root, "images")
        fps, _ := os.ReadDir(imgBase)
        for _, fp := range fps { if !fp.IsDir() || strings.HasPrefix(fp.Name(), ".") { continue }
            snaps, _ := os.ReadDir(filepath.Join(imgBase, fp.Name()))
            for _, ts := range snaps { if !ts.IsDir() || strings.HasPrefix(ts.Name(), ".") { continue }
                p := filepath.Join(imgBase, fp.Name(), ts.Name())
                status := verifySnapshotDir(p)
                out = append(out, verifyResult{Type: "image", Fingerprint: fp.Name(), Timestamp: ts.Name(), Status: status, Path: p})
            }
        }
    }
    // config
    if kind == "all" || kind == "config" {
        cfgBase := filepath.Join(root, "config")
        snaps, _ := os.ReadDir(cfgBase)
        for _, ts := range snaps { if !ts.IsDir() || strings.HasPrefix(ts.Name(), ".") { continue }
            p := filepath.Join(cfgBase, ts.Name())
            status := verifySnapshotDir(p)
            out = append(out, verifyResult{Type: "config", Timestamp: ts.Name(), Status: status, Path: p})
        }
    }
    return out, nil
}

func verifySnapshotDir(dir string) string {
    f, err := os.Open(filepath.Join(dir, "checksums.txt"))
    if err != nil { return fmt.Sprintf("missing checksums.txt: %v", err) }
    defer f.Close()
    scanner := bufio.NewScanner(f)
    ok := true
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" { continue }
        // Expect format: <sha256>  <filename>
        parts := strings.SplitN(line, "  ", 2)
        if len(parts) != 2 { ok = false; continue }
        want := parts[0]
        name := parts[1]
        sum, err := sha256File(filepath.Join(dir, name))
        if err != nil || !strings.EqualFold(want, sum) { ok = false }
    }
    if ok { return "ok" }
    return "mismatch"
}

func sha256File(path string) (string, error) {
    f, err := os.Open(path)
    if err != nil { return "", err }
    defer f.Close()
    h := sha256.New()
    if _, err := io.Copy(h, f); err != nil { return "", err }
    return hex.EncodeToString(h.Sum(nil)), nil
}

