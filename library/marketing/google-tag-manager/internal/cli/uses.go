// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// uses: reverse-dependency / blast-radius for a variable or trigger.

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type usesResult struct {
	Target       string   `json:"target"`
	Kind         string   `json:"kind"`
	ReferencedBy []string `json:"referencedBy"`
}

func newNovelUsesCmd(flags *rootFlags) *cobra.Command {
	var dbPath, container string

	cmd := &cobra.Command{
		Use:         "uses <variable-or-trigger>",
		Short:       "Show everything that references a variable or trigger",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Reverse-dependency / blast-radius check: list every tag, trigger, and variable
that references the named variable (by name) or trigger (by name or id). Use it
before deleting or renaming something to see what breaks. Run 'pull' first.`,
		Example: `  # What references this variable?
  google-tag-manager-pp-cli uses "DLV - userId"

  # What references trigger id 42?
  google-tag-manager-pp-cli uses 42 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would show references to the entity")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("uses needs a <variable-or-trigger> name or id"))
			}
			target := args[0]
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

			snap, err := resolveSnapshot(snaps, container)
			if err != nil {
				return usageErr(err)
			}
			entities, err := snapshotEntities(ctx, s.DB(), snap.ID, "")
			if err != nil {
				return err
			}
			idx := buildRefIndex(entities)

			res := usesResult{Target: target, ReferencedBy: []string{}}
			if _, isVar := idx.varByName[target]; isVar {
				res.Kind = "variable"
				res.ReferencedBy = idx.varUsedBy[target]
			} else if t, isTrig := idx.trigByID[target]; isTrig {
				res.Kind = "trigger"
				res.Target = fmt.Sprintf("%s (id %s)", t.Name, target)
				res.ReferencedBy = idx.trigUsedBy[target]
			} else {
				// Try trigger by name.
				for _, t := range idx.triggers {
					if t.Name == target {
						res.Kind = "trigger"
						res.Target = fmt.Sprintf("%s (id %s)", t.Name, t.EntityID)
						res.ReferencedBy = idx.trigUsedBy[t.EntityID]
						break
					}
				}
			}
			if res.Kind == "" {
				return notFoundErr(fmt.Errorf("no variable or trigger named %q in %s", target, snap.Label))
			}
			if res.ReferencedBy == nil {
				res.ReferencedBy = []string{}
			}
			return gtmEmit(cmd, flags, res, func(w io.Writer) {
				fmt.Fprintf(w, "%s %q is referenced by %d entitie(s):\n", res.Kind, res.Target, len(res.ReferencedBy))
				if len(res.ReferencedBy) == 0 {
					fmt.Fprintln(w, "  (nothing — safe to remove)")
					return
				}
				for _, r := range res.ReferencedBy {
					fmt.Fprintf(w, "  %s\n", r)
				}
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&container, "snapshot", "", "Snapshot ref: live, workspace:<id>, container:<id>, version:<id>, or a snapshot id (default: most recent)")
	return cmd
}
