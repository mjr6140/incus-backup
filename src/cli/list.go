package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"incus-backup/src/backend"
	dir "incus-backup/src/backend/directory"
	backendrestic "incus-backup/src/backend/restic"
	"incus-backup/src/restic"
	"incus-backup/src/target"
)

func newListCmd(stdout, stderr io.Writer) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "list [all|instances|volumes|images|config]",
		Short: "List backups in the target backend",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind := backend.KindAll
			if len(args) == 1 {
				kind = strings.ToLower(args[0])
			}
			tgtStr, _ := cmd.Flags().GetString("target")
			if tgtStr == "" {
				return errors.New("--target is required (e.g., dir:/path)")
			}
			tgt, err := target.Parse(tgtStr)
			if err != nil {
				return err
			}
			var be backend.StorageBackend
			switch tgt.Scheme {
			case "dir":
				b, err := dir.New(tgt.DirPath)
				if err != nil {
					return err
				}
				be = b
			case "restic":
				info, err := checkResticBinary(cmd, true)
				if err != nil {
					return err
				}
				ctx := cmd.Context()
				if ctx == nil {
					ctx = context.Background()
				}
				if err := restic.EnsureRepository(ctx, info, tgt.Value); err != nil {
					return err
				}
				b, err := backendrestic.New(ctx, info, tgt.Value)
				if err != nil {
					return err
				}
				be = b
			default:
				return fmt.Errorf("unsupported backend: %s", tgt.Scheme)
			}
			entries, err := be.List(kind)
			if err != nil {
				return err
			}
			switch output {
			case "json":
				enc := json.NewEncoder(stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			case "table", "":
				return renderTable(stdout, entries)
			default:
				return fmt.Errorf("unsupported --output: %s", output)
			}
		},
	}
	cmd.Flags().String("target", "", "Backend target URI (e.g., dir:/path)")
	cmd.Flags().StringVarP(&output, "output", "o", "table", "Output format: table|json")
	return cmd
}

func renderTable(w io.Writer, entries []backend.Entry) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "TYPE\tPROJECT\tPOOL\tNAME\tFINGERPRINT\tTIMESTAMP")
	for _, e := range entries {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.Type, e.Project, e.Pool, e.Name, e.Fingerprint, e.Timestamp)
	}
	return tw.Flush()
}
