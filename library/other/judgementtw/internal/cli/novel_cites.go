// Copyright 2026 wayne-lai. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"github.com/spf13/cobra"

	"judgementtw-pp-cli/internal/judicial"
)

// newCitesCmd builds 'cites statute <code> [--article N]' — the
// statute-citation graph novel feature.
func newCitesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cites",
		Short: "Query the local citation graph (statute / case references)",
		Long:  `The local store indexes every statute reference parsed from synced judgments. Use 'cites statute' to count occurrences across courts and years.`,
	}
	cmd.AddCommand(newCitesStatuteCmd(flags))
	return cmd
}

func newCitesStatuteCmd(flags *rootFlags) *cobra.Command {
	var article int
	cmd := &cobra.Command{
		Use:   "statute [statute-name]",
		Short: "List judgments that cite a statute, broken down by court and year",
		Long: `Query the locally-synced citation graph: 'cites statute 毒品危害防制條例' returns every
synced judgment that references that statute, with per-court and per-year counts.

Use --article N to narrow to a specific article (e.g. §17 of the same statute).`,
		Example: `  # All synced citations of 毒品危害防制條例
  judgementtw-pp-cli cites statute 毒品危害防制條例 --json

  # Only Article 17
  judgementtw-pp-cli cites statute 毒品危害防制條例 --article 17 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			counts, err := judicial.CountByStatute(ctx, db, args[0], article)
			if err != nil {
				return err
			}
			return emitJSON(cmd.OutOrStdout(), counts, flags)
		},
	}
	cmd.Flags().IntVar(&article, "article", 0, "Specific article number (e.g. 17 for §17)")
	return cmd
}

// newCitedByCmd builds 'cited-by <jid>' — the reverse-citation precedent
// lookup novel feature.
func newCitedByCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "cited-by [jid]",
		Short: "List locally-synced judgments that cite the given JID",
		Long: `Reverse-citation lookup: which synced judgments reference the given JID
in their body text? Surfaces appellate review, follow-on rulings, and
precedent fan-out without re-querying the website.`,
		Example: `  judgementtw-pp-cli cited-by TPHM,110,毒抗,1212,20210831,1 --json
  judgementtw-pp-cli cited-by TPHM,110,毒抗,1212,20210831,1 --limit 10 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			db, err := openJudicialDB(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			ids, err := judicial.CitedBy(ctx, db, args[0], limit)
			if err != nil {
				return err
			}
			out := make([]map[string]any, 0, len(ids))
			for _, id := range ids {
				out = append(out, map[string]any{"jid": id})
			}
			return emitJSON(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = all)")
	return cmd
}
