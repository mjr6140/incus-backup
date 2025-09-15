package cli

import (
    "errors"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "text/tabwriter"

    "github.com/spf13/cobra"

    "incus-backup/src/safety"
    "incus-backup/src/target"
)

func newPruneCmd(stdout, stderr io.Writer) *cobra.Command {
    var keep int
    cmd := &cobra.Command{
        Use:   "prune [all|instances|volumes|images|config]",
        Short: "Prune old snapshots (keep N per resource)",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            kind := "all"
            if len(args) == 1 { kind = strings.ToLower(args[0]) }
            if keep <= 0 { return errors.New("--keep must be > 0") }
            tgtStr, _ := cmd.Flags().GetString("target")
            if tgtStr == "" { return errors.New("--target is required (e.g., dir:/path)") }
            tgt, err := target.Parse(tgtStr)
            if err != nil { return err }
            if tgt.Scheme != "dir" { return fmt.Errorf("prune: only directory backend is supported") }

            toDelete, err := planPrune(tgt.DirPath, kind, keep)
            if err != nil { return err }

            // Preview
            tw := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
            fmt.Fprintln(tw, "TYPE\tPROJECT\tPOOL\tNAME\tFINGERPRINT\tTIMESTAMP\tACTION")
            for _, p := range toDelete { fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\tdelete\n", p.Type, p.Project, p.Pool, p.Name, p.Fingerprint, p.Timestamp) }
            _ = tw.Flush()

            opts := getSafetyOptions(cmd)
            if opts.DryRun || len(toDelete) == 0 { return nil }
            ok, err := safety.Confirm(opts, os.Stdin, stdout, fmt.Sprintf("Delete %d snapshots?", len(toDelete)))
            if err != nil || !ok { return err }
            // Apply deletions
            for _, p := range toDelete {
                _ = os.RemoveAll(p.Path)
            }
            return nil
        },
    }
    cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
    cmd.Flags().IntVar(&keep, "keep", 3, "Number of recent snapshots to keep per resource")
    return cmd
}

type pruneCandidate struct {
    Type, Project, Pool, Name, Fingerprint, Timestamp, Path string
}

func planPrune(root, kind string, keep int) ([]pruneCandidate, error) {
    var del []pruneCandidate
    // instances
    if kind == "all" || kind == "instances" {
        base := filepath.Join(root, "instances")
        projects, _ := os.ReadDir(base)
        for _, pr := range projects { if !pr.IsDir() || strings.HasPrefix(pr.Name(), ".") { continue }
            names, _ := os.ReadDir(filepath.Join(base, pr.Name()))
            for _, nm := range names { if !nm.IsDir() || strings.HasPrefix(nm.Name(), ".") { continue }
                snaps, _ := os.ReadDir(filepath.Join(base, pr.Name(), nm.Name()))
                var ts []string
                for _, s := range snaps { if s.IsDir() && !strings.HasPrefix(s.Name(), ".") { ts = append(ts, s.Name()) } }
                sort.Strings(ts)
                if len(ts) > keep {
                    for _, old := range ts[:len(ts)-keep] {
                        p := filepath.Join(base, pr.Name(), nm.Name(), old)
                        del = append(del, pruneCandidate{Type: "instance", Project: pr.Name(), Name: nm.Name(), Timestamp: old, Path: p})
                    }
                }
            }
        }
    }
    // volumes
    if kind == "all" || kind == "volumes" {
        base := filepath.Join(root, "volumes")
        projects, _ := os.ReadDir(base)
        for _, pr := range projects { if !pr.IsDir() || strings.HasPrefix(pr.Name(), ".") { continue }
            pools, _ := os.ReadDir(filepath.Join(base, pr.Name()))
            for _, pool := range pools { if !pool.IsDir() || strings.HasPrefix(pool.Name(), ".") { continue }
                names, _ := os.ReadDir(filepath.Join(base, pr.Name(), pool.Name()))
                for _, nm := range names { if !nm.IsDir() || strings.HasPrefix(nm.Name(), ".") { continue }
                    snaps, _ := os.ReadDir(filepath.Join(base, pr.Name(), pool.Name(), nm.Name()))
                    var ts []string
                    for _, s := range snaps { if s.IsDir() && !strings.HasPrefix(s.Name(), ".") { ts = append(ts, s.Name()) } }
                    sort.Strings(ts)
                    if len(ts) > keep {
                        for _, old := range ts[:len(ts)-keep] {
                            p := filepath.Join(base, pr.Name(), pool.Name(), nm.Name(), old)
                            del = append(del, pruneCandidate{Type: "volume", Project: pr.Name(), Pool: pool.Name(), Name: nm.Name(), Timestamp: old, Path: p})
                        }
                    }
                }
            }
        }
    }
    // images
    if kind == "all" || kind == "images" {
        base := filepath.Join(root, "images")
        fps, _ := os.ReadDir(base)
        for _, fp := range fps { if !fp.IsDir() || strings.HasPrefix(fp.Name(), ".") { continue }
            snaps, _ := os.ReadDir(filepath.Join(base, fp.Name()))
            var ts []string
            for _, s := range snaps { if s.IsDir() && !strings.HasPrefix(s.Name(), ".") { ts = append(ts, s.Name()) } }
            sort.Strings(ts)
            if len(ts) > keep {
                for _, old := range ts[:len(ts)-keep] {
                    p := filepath.Join(base, fp.Name(), old)
                    del = append(del, pruneCandidate{Type: "image", Fingerprint: fp.Name(), Timestamp: old, Path: p})
                }
            }
        }
    }
    // config
    if kind == "all" || kind == "config" {
        base := filepath.Join(root, "config")
        snaps, _ := os.ReadDir(base)
        var ts []string
        for _, s := range snaps { if s.IsDir() && !strings.HasPrefix(s.Name(), ".") { ts = append(ts, s.Name()) } }
        sort.Strings(ts)
        if len(ts) > keep {
            for _, old := range ts[:len(ts)-keep] {
                p := filepath.Join(base, old)
                del = append(del, pruneCandidate{Type: "config", Timestamp: old, Path: p})
            }
        }
    }
    return del, nil
}

