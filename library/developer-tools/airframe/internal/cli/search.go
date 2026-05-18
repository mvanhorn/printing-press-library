// Copyright 2026 ChrisDrit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/airframe/internal/store"

	"github.com/spf13/cobra"
)

func newSearchCmd() *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Full-text search across narratives and owner names (requires --with-fts sync).",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSearch(cmd.Context(), strings.Join(args, " "), limit)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum hits per stream (aircraft + narratives)")
	return cmd
}

type SearchResults struct {
	Query      string                  `json:"query"`
	Aircraft   []store.FTSAircraftHit  `json:"aircraft"`
	Narratives []store.FTSNarrativeHit `json:"narratives"`
}

func runSearch(ctx context.Context, query string, limit int) error {
	dbPath, st, err := openReadStore(ctx)
	if err != nil {
		return err
	}
	defer st.Close()

	ok, err := st.HasFTS(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("FTS5 indexes not built — run `airframe-pp-cli sync --with-fts` (or `--everything`) first")
	}

	if limit <= 0 {
		limit = 25
	}

	air, err := st.SearchAircraftFTS(ctx, query, limit)
	if err != nil {
		return err
	}
	narr, err := st.SearchNarrativesFTS(ctx, query, limit)
	if err != nil {
		return err
	}
	results := SearchResults{Query: query, Aircraft: air, Narratives: narr}

	env := Envelope{
		Meta: Meta{
			Source: "local", DBPath: dbPath, SyncedAt: latestSyncedAt(ctx, st),
			Query: map[string]any{"query": query, "limit": limit},
		},
		Results: results,
	}

	if flagJSON || flagSelect != "" {
		return emitEnvelope(env)
	}
	return renderSearchText(results)
}

func renderSearchText(r SearchResults) error {
	fmt.Printf("Search %q\n\n", r.Query)
	fmt.Printf("Aircraft hits (%d)\n", len(r.Aircraft))
	for _, h := range r.Aircraft {
		fmt.Printf("  %s  %s  %s %s\n      %s\n", h.Registration, h.OwnerName, h.Manufacturer, h.Model, h.Snippet)
	}
	fmt.Printf("\nNarrative hits (%d)\n", len(r.Narratives))
	for _, h := range r.Narratives {
		fmt.Printf("  %s  %s\n      %s\n", h.EventDate, h.EventID, h.Snippet)
	}
	return nil
}
