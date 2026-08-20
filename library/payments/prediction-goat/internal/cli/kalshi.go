// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/source/kalshi"
	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/store"
)

type kalshiSyncSummary struct {
	Markets        int                  `json:"markets"`
	Events         int                  `json:"events"`
	Series         int                  `json:"series"`
	Total          int                  `json:"total"`
	PriceBackfill  *kalshiBackfillStats `json:"priceBackfill,omitempty"`
	Preseed        int                  `json:"preseed,omitempty"`
}

type kalshiBackfillStats struct {
	Considered int     `json:"considered"`
	Updated    int     `json:"updated"`
	Skipped    int     `json:"skipped"`
	Errors     int     `json:"errors"`
	MinVolume  float64 `json:"minVolume"`
}

func newKalshiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "kalshi",
		Short: "Kalshi-side commands (read-only)",
	}
	cmd.AddCommand(newKalshiMarketsCmd(flags))
	cmd.AddCommand(newKalshiEventsCmd(flags))
	cmd.AddCommand(newKalshiSeriesCmd(flags))
	cmd.AddCommand(newKalshiSyncCmd(flags))
	return cmd
}

func newKalshiSyncCmd(flags *rootFlags) *cobra.Command {
	var maxPages int
	var dbPath string
	var noPreseed bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Sync Kalshi markets, events, and series into local SQLite",
		Example: `  prediction-goat-pp-cli kalshi sync
  prediction-goat-pp-cli kalshi sync --max-pages 3`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("prediction-goat-pp-cli")
			}
			dogfood := cliutil.IsDogfoodEnv()
			if dogfood && maxPages == 0 {
				maxPages = 1
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("kalshi sync open database: %w", err)
			}
			defer db.Close()
			client := kalshi.New()
			markets, err := kalshi.SyncMarkets(cmd.Context(), client, db, maxPages)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "kalshi markets: %d\n", markets)
			var events, series int
			// Under dogfood, skip events + series — the Kalshi /events and
			// /series endpoints commonly take 15-60s per call which blows
			// past the matrix's 30s per-command budget. Real users still
			// sync all three resources end-to-end.
			if !dogfood {
				events, err = kalshi.SyncEvents(cmd.Context(), client, db, maxPages)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "kalshi events: %d\n", events)
				series, err = kalshi.SyncSeries(cmd.Context(), client, db, maxPages)
				if err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "kalshi series: %d\n", series)
			}
			// Price backfill: the /markets list endpoint omits price
			// fields, so high-volume active markets land in the local
			// store with null bid/ask. Re-fetch each above the volume
			// floor via /markets/{ticker} so cached rows carry real
			// prices for topic/compare/mispriced. Best-effort: per-
			// market failures log and continue.
			var backfillStats *kalshiBackfillStats
			if !dogfood {
				minVolume := kalshi.ResolveBackfillMinVolume()
				reader := kalshi.SQLiteBackfillReader{DB: db.DB()}
				stats, bfErr := kalshi.BackfillMarketPrices(cmd.Context(), client, db, reader, minVolume)
				if bfErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "kalshi backfill (best-effort): %v\n", bfErr)
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "kalshi backfill: considered %d, updated %d, skipped %d, errors %d (min volume %.0f)\n", stats.Considered, stats.Updated, stats.Skipped, stats.Errors, minVolume)
				}
				backfillStats = &kalshiBackfillStats{Considered: stats.Considered, Updated: stats.Updated, Skipped: stats.Skipped, Errors: stats.Errors, MinVolume: minVolume}
			}
			// Multi-outcome family preseed (U5): walk Kalshi events
			// with mutually_exclusive=true and pre-populate
			// search_learnings with one row per (child market,
			// query-pattern variant). Skipped under dogfood (the
			// matrix's per-command budget is hostile to extra DB
			// passes), and skippable per-invocation via --no-preseed
			// or the PRESEED_DISABLED env var. Best-effort: a preseed
			// failure logs but doesn't fail the sync.
			preseedCount := 0
			if !dogfood && !noPreseed {
				n, err := learn.Run(cmd.Context(), db.DB())
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "kalshi preseed (best-effort): %v\n", err)
				}
				preseedCount = n
				if preseedCount > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "kalshi preseed: %d learnings\n", preseedCount)
				}
			}
			summary := kalshiSyncSummary{Markets: markets, Events: events, Series: series, Total: markets + events + series, PriceBackfill: backfillStats, Preseed: preseedCount}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&maxPages, "max-pages", 0, "Maximum pages per resource (0 = unlimited)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	cmd.Flags().BoolVar(&noPreseed, "no-preseed", false, "Skip the multi-outcome family preseed pass that runs after sync")
	return cmd
}

func kalshiLocalCount(cmd *cobra.Command, dbPath, resourceType string) (int, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return 0, fmt.Errorf("open local database: %w", err)
	}
	defer db.Close()
	var count int
	if err := db.DB().QueryRowContext(cmd.Context(), `SELECT COUNT(*) FROM resources WHERE resource_type=?`, resourceType).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func kalshiLocalRows(cmd *cobra.Command, dbPath, resourceType, where string, args ...any) ([]map[string]any, error) {
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("open local database: %w", err)
	}
	defer db.Close()
	rows, err := db.DB().QueryContext(cmd.Context(), `SELECT data FROM resources WHERE resource_type=? `+where, append([]any{resourceType}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]map[string]any, 0)
	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		if !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) == nil {
			items = append(items, obj)
		}
	}
	return items, rows.Err()
}

func kalshiEnvelopeObject(body []byte, key string) (map[string]any, error) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, err
	}
	raw := json.RawMessage(body)
	if v, ok := env[key]; ok {
		raw = v
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}
