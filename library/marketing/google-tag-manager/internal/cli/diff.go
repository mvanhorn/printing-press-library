// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// diff: field-level diff between two mirrored snapshots.

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type diffResult struct {
	From    string       `json:"from"`
	To      string       `json:"to"`
	Added   int          `json:"added"`
	Removed int          `json:"removed"`
	Changed int          `json:"changed"`
	Changes []diffChange `json:"changes"`
}

func newNovelDiffCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "diff <from> <to>",
		Short:       "Field-level diff between two mirrored snapshots",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Compare two snapshots from the local mirror and report added, removed, and
changed entities with the changed field names. Refs may be 'live',
'workspace:<id>', 'version:<id>', 'container:<id>', or a numeric snapshot id.
Run 'pull' for each side first.`,
		Example: `  # What does the live container change vs a workspace?
  google-tag-manager-pp-cli diff workspace:7 live

  # Compare two containers
  google-tag-manager-pp-cli diff container:9876543 container:1234567 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff two mirrored snapshots")
				return nil
			}
			if len(args) < 2 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("diff needs two snapshot refs: <from> <to>"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, snaps, ok, err := gtmReadSnapshots(ctx, cmd, flags, gtmDBPath(dbPath))
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer s.Close()

			from, err := resolveSnapshot(snaps, args[0])
			if err != nil {
				return usageErr(err)
			}
			// Scope the second ref to the first ref's container so a bare
			// "diff live workspace:7" compares within one container instead of
			// silently crossing containers. Explicit "container:<id>" refs
			// still switch containers on purpose.
			to, err := resolveSnapshotScoped(snaps, args[1], from.ContainerID)
			if err != nil {
				return usageErr(err)
			}
			fromEnts, err := snapshotEntities(ctx, s.DB(), from.ID, "")
			if err != nil {
				return err
			}
			toEnts, err := snapshotEntities(ctx, s.DB(), to.ID, "")
			if err != nil {
				return err
			}
			changes := diffSnapshots(fromEnts, toEnts)
			res := diffResult{From: from.Label, To: to.Label, Changes: changes}
			for _, c := range changes {
				switch c.Op {
				case "added":
					res.Added++
				case "removed":
					res.Removed++
				case "changed":
					res.Changed++
				}
			}
			return gtmEmit(cmd, flags, res, func(w io.Writer) {
				fmt.Fprintf(w, "%s -> %s: +%d added, -%d removed, ~%d changed\n",
					from.Label, to.Label, res.Added, res.Removed, res.Changed)
				tw := newTabWriter(w)
				for _, c := range changes {
					sym := map[string]string{"added": "+", "removed": "-", "changed": "~"}[c.Op]
					detail := ""
					if len(c.ChangedFields) > 0 {
						detail = "(" + strings.Join(c.ChangedFields, ", ") + ")"
					}
					fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\n", sym, c.Kind, c.Name, detail)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
