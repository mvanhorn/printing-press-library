// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T7 — Play-history by listening context.
//
//	play history --by context [--since <window>]
//
// Aggregates play_history.context_uri, ranks by play count. Spotify's API
// returns the context_uri on every recently-played row but no surface
// aggregates it.

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newPlayCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "play",
		Short: "Local play-history aggregations (run 'sync' first to populate play_history)",
	}
	cmd.AddCommand(newPlayHistoryCmd(flags))
	return cmd
}

func newPlayHistoryCmd(flags *rootFlags) *cobra.Command {
	var byArg string
	var sinceArg string
	cmd := &cobra.Command{
		Use:         "history --by context [--since <window>]",
		Short:       "Rank play history by listening context (playlist/album/artist)",
		Example:     "  spotify-pp-cli play history --by context --since 30d",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if byArg != "context" {
				return usageErr(fmt.Errorf("--by must be 'context' (only mode supported)"))
			}
			var sinceCutoff time.Time
			if sinceArg != "" {
				dur, err := parseDurationWindow(sinceArg)
				if err != nil {
					return usageErr(err)
				}
				sinceCutoff = time.Now().Add(-dur)
			}

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"dry_run": true, "by": byArg}, flags)
			}

			result, err := computePlayHistoryByContext(db.DB(), sinceCutoff)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&byArg, "by", "context", "Aggregation key (currently only 'context')")
	cmd.Flags().StringVar(&sinceArg, "since", "", "Window like '7d', '24h', or '30d' (default: all time)")
	return cmd
}

func parseDurationWindow(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration window %q (use 7d, 24h, etc.)", s)
	}
	if s[len(s)-1] == 'd' {
		days := 0
		_, err := fmt.Sscanf(s, "%dd", &days)
		if err != nil {
			return 0, fmt.Errorf("invalid duration window %q: %w", s, err)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("invalid duration window %q: %w", s, err)
	}
	return d, nil
}

// playHistoryContextRow is the per-context aggregation row. Named (not
// anonymous) so transcendence_test.go can type-assert it from the returned
// map value.
type playHistoryContextRow struct {
	ContextURI  string `json:"context_uri"`
	ContextType string `json:"context_type"`
	PlayCount   int    `json:"play_count"`
}

func computePlayHistoryByContext(db storeQueryer, sinceCutoff time.Time) (map[string]any, error) {
	query := `SELECT COALESCE(context_uri, '__no_context__'), COALESCE(context_type, ''), COUNT(*) AS plays
		FROM play_history`
	args := []any{}
	if !sinceCutoff.IsZero() {
		query += ` WHERE played_at >= ?`
		args = append(args, sinceCutoff.UTC().Format(time.RFC3339))
	}
	query += ` GROUP BY context_uri, context_type`

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var contexts []playHistoryContextRow
	total := 0
	for rows.Next() {
		var r playHistoryContextRow
		if err := rows.Scan(&r.ContextURI, &r.ContextType, &r.PlayCount); err != nil {
			return nil, err
		}
		if r.ContextURI == "__no_context__" {
			r.ContextURI = ""
		}
		contexts = append(contexts, r)
		total += r.PlayCount
	}
	sort.SliceStable(contexts, func(i, j int) bool { return contexts[i].PlayCount > contexts[j].PlayCount })

	return map[string]any{
		"by":           "context",
		"window_since": sinceCutoff,
		"total_plays":  total,
		"contexts":     contexts,
	}, rows.Err()
}
