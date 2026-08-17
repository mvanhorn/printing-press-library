// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
//
// search: substring search across every mirrored entity.

package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type searchHit struct {
	Container string `json:"container"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Type      string `json:"type"`
}

func newNovelSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath, kind string
	var limit int

	cmd := &cobra.Command{
		Use:         "search <term>",
		Short:       "Search every mirrored entity's name, type, and parameters",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Search the name, type, and parameter JSON of every entity across all mirrored
containers (most recent snapshot per container). Use it to find hardcoded
measurement ids, stray vendor pixels, or leftover URLs. Run 'pull' first.`,
		Example: `  # Where is a GA4 measurement id referenced?
  google-tag-manager-pp-cli search "G-ABCDE12345"

  # Restrict to variables
  google-tag-manager-pp-cli search facebook --kind variable --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the local mirror")
				return nil
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search needs a <term>"))
			}
			term := args[0]
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

			label := map[int64]string{}
			ids := make([]any, 0)
			placeholders := ""
			for _, snap := range latestPerContainer(snaps) {
				label[snap.ID] = displayName(snap)
				if placeholders != "" {
					placeholders += ","
				}
				placeholders += "?"
				ids = append(ids, snap.ID)
			}

			like := "%" + term + "%"
			q := `SELECT snapshot_id, kind, name, type FROM gtm_entity
			      WHERE snapshot_id IN (` + placeholders + `)
			        AND (name LIKE ? OR type LIKE ? OR data LIKE ?)`
			qArgs := append([]any{}, ids...)
			qArgs = append(qArgs, like, like, like)
			if kind != "" {
				q += ` AND kind = ?`
				qArgs = append(qArgs, kind)
			}
			q += ` ORDER BY kind, name LIMIT ?`
			qArgs = append(qArgs, limit)

			rows, err := s.DB().QueryContext(ctx, q, qArgs...)
			if err != nil {
				return err
			}
			defer rows.Close()
			hits := make([]searchHit, 0)
			for rows.Next() {
				var snapID int64
				var k, name, typ string
				if err := rows.Scan(&snapID, &k, &name, &typ); err != nil {
					return err
				}
				hits = append(hits, searchHit{Container: label[snapID], Kind: k, Name: name, Type: typ})
			}
			if err := rows.Err(); err != nil {
				return err
			}
			return gtmEmit(cmd, flags, hits, func(w io.Writer) {
				if len(hits) == 0 {
					fmt.Fprintf(w, "no matches for %q\n", term)
					return
				}
				tw := newTabWriter(w)
				fmt.Fprintln(tw, "CONTAINER\tKIND\tTYPE\tNAME")
				for _, h := range hits {
					fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", h.Container, h.Kind, h.Type, h.Name)
				}
				tw.Flush()
			})
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&kind, "kind", "", "Restrict to one entity kind (tag, trigger, variable, ...)")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum matches to return")
	return cmd
}
