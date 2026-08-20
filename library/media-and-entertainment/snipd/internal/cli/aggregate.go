// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command — NOT generator output.
// pp:data-source local
//
// aggregate rolls up counts and distributions across the local mirror: snips per
// show, top tags, favorites by show, snips per episode. One GROUP BY over the
// resources table; read-only.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type aggRow struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
}

var aggDimensions = map[string]string{
	// dimension -> GROUP BY SQL over the snips partition.
	"by-show":    "SELECT json_extract(data,'$.show') AS k, COUNT(*) AS n FROM resources WHERE resource_type='snips' GROUP BY k ORDER BY n DESC, k",
	"by-tag":     "SELECT je.value AS k, COUNT(*) AS n FROM resources, json_each(data,'$.tags') je WHERE resource_type='snips' GROUP BY k ORDER BY n DESC, k",
	"by-episode": "SELECT json_extract(data,'$.episode_title') AS k, COUNT(*) AS n FROM resources WHERE resource_type='snips' GROUP BY json_extract(data,'$.episode_id') ORDER BY n DESC, k",
	"favorites":  "SELECT json_extract(data,'$.show') AS k, COUNT(*) AS n FROM resources WHERE resource_type='snips' AND json_extract(data,'$.favorite')=1 GROUP BY k ORDER BY n DESC, k",
}

func newNovelAggregateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	dims := make([]string, 0, len(aggDimensions))
	for d := range aggDimensions {
		dims = append(dims, d)
	}
	sort.Strings(dims)

	cmd := &cobra.Command{
		Use:   "aggregate [dimension]",
		Short: "Roll up counts and distributions across your corpus — snips per show, top tags, favorites, snips per episode.",
		Long: "Group the corpus and count. Dimensions: " + strings.Join(dims, ", ") + ".\n" +
			"Defaults to by-show. Read-only over the local mirror.",
		Example:     "  snipd-pp-cli aggregate by-show\n  snipd-pp-cli aggregate by-tag --limit 15 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			dim := "by-show"
			if len(args) > 0 {
				dim = strings.ToLower(strings.TrimSpace(args[0]))
			}
			sqlText, ok := aggDimensions[dim]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("unknown dimension %q; choose one of: %s", dim, strings.Join(dims, ", ")))
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

			rows, err := st.DB().QueryContext(ctx, sqlText)
			if err != nil {
				return fmt.Errorf("aggregate query: %w", err)
			}
			defer rows.Close()
			out := make([]aggRow, 0)
			for rows.Next() {
				var k *string
				var n int
				if err := rows.Scan(&k, &n); err != nil {
					return fmt.Errorf("scan: %w", err)
				}
				key := ""
				if k != nil {
					key = *k
				}
				out = append(out, aggRow{Key: key, Count: n})
				if limit > 0 && len(out) >= limit {
					break
				}
			}
			if err := rows.Err(); err != nil {
				return err
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "no data — run: snipd-pp-cli pull")
				return nil
			}
			for _, r := range out {
				key := r.Key
				if key == "" {
					key = "(untagged)"
				}
				fmt.Fprintf(w, "%5d  %s\n", r.Count, key)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite mirror path (default: per-user data dir)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum groups to return (0 = all)")
	return cmd
}
