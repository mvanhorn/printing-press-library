// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.
// Phase 3: time-windowed new articles using local first-seen (synced_at) timestamps.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// nejmSinceCutoffString renders a cutoff instant in the exact shape the store
// writes synced_at (UTC RFC3339). The explicit .UTC() matters: the cutoff
// comes from time.Now() in local time, and formatting a local instant with a
// literal "Z" suffix would silently shift the window for any non-UTC user.
func nejmSinceCutoffString(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func newNovelSinceCmd(flags *rootFlags) *cobra.Command {
	var freeOnly bool
	var limit int

	cmd := &cobra.Command{
		Use:         "since <window>",
		Short:       "Show NEJM articles first seen in your local corpus within a time window.",
		Long:        "Lists articles added to the local corpus within the given time window (e.g. 48h, 7d, 2w). Uses the local synced_at timestamp — not the publication date — so it reliably shows what changed since your last sync.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     "  nejm-pp-cli since 48h\n  nejm-pp-cli since 7d --free --json",
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cutoff, err := nejmParseSinceDuration(args[0])
			if err != nil {
				return usageErr(fmt.Errorf("invalid time window %q: use e.g. 48h, 7d, 2w", args[0]))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := nejmOpenStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()

			clauses := []string{`synced_at >= ?`}
			qargs := []interface{}{nejmSinceCutoffString(cutoff)}
			if freeOnly {
				clauses = append(clauses, `is_free = 1`)
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
				fmt.Fprintf(cmd.ErrOrStderr(), "hint: no articles seen in the last %s. Run 'nejm-pp-cli sync' to fetch new articles.\n", args[0])
				if flags.asJSON {
					return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
				}
				return nil
			}
			return nejmPrintArticles(cmd, items, flags)
		},
	}
	cmd.Flags().BoolVar(&freeOnly, "free", false, "Show only open-access articles")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of results (0 = all)")
	return cmd
}
