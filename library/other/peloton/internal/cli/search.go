// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/store"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across synced workouts and songs",
		Long: `Runs FTS5 across the local store: workouts (title + instructor) and
songs (title + artists + album). Results are interleaved by bm25 score —
the kind field on each hit tells you whether it matched a workout or a
song.

The query string is passed verbatim to FTS5: phrases ("low impact"),
prefixes (cure*), and NEAR( ) all work. Run ` + "`peloton sync`" + ` first to
populate the store; an empty store returns no hits.`,
		Example: `  peloton-pp-cli search 'cure'
  peloton-pp-cli search 'cody rigsby' --limit 10
  peloton-pp-cli search '"low impact"' --json | jq '.[] | select(.kind=="workout")'`,
		Args:        cobra.MinimumNArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			ctx := cmd.Context()
			st, err := store.Open(ctx, "")
			if err != nil {
				return Errf(CodeAPI, "opening store: %w", err)
			}
			defer st.Close()
			hits, err := st.Search(ctx, query, limit)
			if err != nil {
				return Errf(CodeAPI, "search: %w", err)
			}
			return emitSearchHits(cmd, flags, hits)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Max hits to return")
	return cmd
}

func emitSearchHits(cmd *cobra.Command, flags *rootFlags, hits []store.SearchHit) error {
	wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()
	if wantJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		if !flags.compact {
			enc.SetIndent("", "  ")
		}
		return enc.Encode(hits)
	}
	if len(hits) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no hits — has `peloton sync` run yet?")
		return nil
	}
	for _, h := range hits {
		switch h.Kind {
		case "workout":
			fmt.Fprintf(cmd.OutOrStdout(), "[workout] %s  %s — %s\n", h.Date, h.Title, h.Subtitle)
		case "song":
			fmt.Fprintf(cmd.OutOrStdout(), "[song]    %s — %s\n", h.Title, h.Subtitle)
		}
	}
	return nil
}
