// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Phase 3: open-access finder — surface free full-text NEJM articles newest first.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newNovelOpenAccessCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var since string

	cmd := &cobra.Command{
		Use:         "open-access",
		Short:       "Surface free full-text NEJM articles in the corpus, newest first.",
		Long:        "Lists NEJM articles marked is_free=true in the local corpus, newest first. Requires synced and enriched data (run 'nejm-pp-cli sync' to fetch articles, then 'nejm-pp-cli article <doi> --enrich' to enrich individual articles).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli open-access\n  nejm-pp-cli open-access --limit 20 --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "article", flags.maxAge)

			var clauses []string
			var qargs []interface{}
			clauses = append(clauses, `is_free = 1`)

			if since != "" {
				cutoff, err := nejmParseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				clauses = append(clauses, `date >= ?`)
				qargs = append(qargs, cutoff.Format("2006-01-02"))
			}

			where := clauses[0]
			for _, c := range clauses[1:] {
				where += " AND " + c
			}

			items, err := nejmQueryArticles(db, where, qargs, limit)
			if err != nil {
				return fmt.Errorf("querying articles: %w", err)
			}
			if len(items) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no open-access articles found. Run 'nejm-pp-cli sync' to fetch articles, then 'nejm-pp-cli article <doi> --enrich' to enrich individual articles with free/paywalled flags.")
				if flags.asJSON {
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
				}
				return nil
			}
			return nejmPrintArticles(cmd, items, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results (0 = all)")
	cmd.Flags().StringVar(&since, "since", "", "Only articles published since (e.g. 30d, 6w)")
	return cmd
}
