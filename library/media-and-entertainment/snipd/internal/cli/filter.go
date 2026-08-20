// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command — NOT generator output.
// pp:data-source local
//
// filter slices the corpus by structured dimensions (show, favorite, tag, date)
// and returns compact rows or a count. Free-text concepts belong in search /
// synthesize; this is the SQL-shaped path over the local mirror.
package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// normalizeDateFlag validates a --since/--until value and normalizes it to the
// YYYY-MM-DD granularity the stored episode publish_date uses. Two problems it
// closes: an unvalidated typo like "zzzz" would otherwise be a perfectly valid
// lexical SQL predicate that silently returns an empty/misleading slice; and a
// full timestamp like "2026-01-15T00:00:00Z" compared lexically against a plain
// "2026-01-15" would drop the boundary-day episode (the shorter stored string
// sorts first). Comparing on the date part only keeps the range correct.
func normalizeDateFlag(name, val string) (string, error) {
	if val == "" {
		return "", nil
	}
	for _, layout := range []string{"2006-01-02", time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, val); err == nil {
			return t.Format("2006-01-02"), nil
		}
	}
	return "", fmt.Errorf("invalid --%s %q: expected a date (2006-01-02) or an RFC3339 timestamp", name, val)
}

func newNovelFilterCmd(flags *rootFlags) *cobra.Command {
	var dbPath, show, tag, since, until string
	var favorite, count, withTranscript bool
	var limit int

	cmd := &cobra.Command{
		Use:   "filter",
		Short: "Slice the corpus by show, favorite, tag, or date and return compact rows or a count.",
		Long: "Return snips matching structured criteria — a show, favorites only, a tag, or\n" +
			"an episode date range. Combine flags to narrow further, or add --count for just\n" +
			"the number. For free-text concepts use search or synthesize instead.",
		Example:     "  snipd-pp-cli filter --show \"NN/G UX Podcast\" --agent\n  snipd-pp-cli filter --favorite --count\n  snipd-pp-cli filter --tag ai --since 2026-05-01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			nSince, err := normalizeDateFlag("since", since)
			if err != nil {
				return err
			}
			nUntil, err := normalizeDateFlag("until", until)
			if err != nil {
				return err
			}
			since, until = nSince, nUntil
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			st, dbResolved, err := openCorpusForRead(ctx, dbPath)
			if err != nil {
				return err
			}
			if st == nil {
				return emitNoCorpus(cmd, flags, dbResolved)
			}
			defer st.Close()

			var clauses []string
			var qargs []any
			if show != "" {
				clauses = append(clauses, "json_extract(data,'$.show') = ?")
				qargs = append(qargs, show)
			}
			if favorite {
				clauses = append(clauses, "json_extract(data,'$.favorite') = 1")
			}
			if tag != "" {
				clauses = append(clauses, "EXISTS (SELECT 1 FROM json_each(data,'$.tags') WHERE lower(value) = lower(?))")
				qargs = append(qargs, tag)
			}
			// Date range is on the parent episode's publish_date. Both sides are
			// normalized to YYYY-MM-DD (see normalizeDateFlag), so lexical order is
			// also chronological. Match snips whose episode falls in range.
			if since != "" {
				clauses = append(clauses, "json_extract(data,'$.episode_id') IN (SELECT id FROM resources WHERE resource_type='episodes' AND json_extract(data,'$.publish_date') >= ?)")
				qargs = append(qargs, since)
			}
			if until != "" {
				clauses = append(clauses, "json_extract(data,'$.episode_id') IN (SELECT id FROM resources WHERE resource_type='episodes' AND json_extract(data,'$.publish_date') <= ?)")
				qargs = append(qargs, until)
			}
			where := strings.Join(clauses, " AND ")

			if count {
				q := "SELECT COUNT(*) FROM resources WHERE resource_type='snips'"
				if where != "" {
					q += " AND " + where
				}
				row := st.DB().QueryRowContext(ctx, q, qargs...)
				var n int
				if err := row.Scan(&n); err != nil {
					return fmt.Errorf("count: %w", err)
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]int{"count": n}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%d\n", n)
				return nil
			}

			snips, err := querySnips(ctx, st, where, "", qargs, limit)
			if err != nil {
				return err
			}
			rows := make([]compactSnip, 0, len(snips))
			for _, s := range snips {
				rows = append(rows, toCompact(s, withTranscript))
			}
			return printCompact(cmd, flags, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path (default: per-user data dir)")
	cmd.Flags().StringVar(&show, "show", "", "Restrict to a single show")
	cmd.Flags().BoolVar(&favorite, "favorite", false, "Only favorited snips")
	cmd.Flags().StringVar(&tag, "tag", "", "Only snips carrying this tag")
	cmd.Flags().StringVar(&since, "since", "", "Only snips whose episode published on/after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Only snips whose episode published on/before this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum rows to return (0 = all)")
	cmd.Flags().BoolVar(&count, "count", false, "Return just the matching count")
	cmd.Flags().BoolVar(&withTranscript, "transcript", false, "Include full transcripts (larger output)")
	return cmd
}
