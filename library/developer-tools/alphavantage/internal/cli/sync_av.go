// Copyright 2026 lokisbo. Licensed under Apache-2.0. See LICENSE.
//
// Alpha Vantage sync extensions: sync news, sync movers, sync earnings-calendar.
// These mirror the existing novel commands (news sweep, movers brief,
// briefing earnings) but live under `sync` for users who think in sync terms.
//
// Wired into the existing sync command in root.go.

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/alphavantage/internal/store"
	"github.com/spf13/cobra"
)

// addAVSyncSubcommands attaches the three AV-specific sync subcommands to the
// sync parent. Called from root.go right after newSyncCmd().
func addAVSyncSubcommands(syncCmd *cobra.Command, flags *rootFlags) {
	syncCmd.AddCommand(newSyncNewsCmd(flags))
	syncCmd.AddCommand(newSyncMoversCmd(flags))
	syncCmd.AddCommand(newSyncEarningsCalendarCmd(flags))
}

func newSyncNewsCmd(flags *rootFlags) *cobra.Command {
	var flagTickers string
	var flagWatchlist string
	var flagSince string

	cmd := &cobra.Command{
		Use:   "news",
		Short: "Hydrate local store with NEWS_SENTIMENT for a watchlist or tickers",
		Long: `Equivalent to 'news sweep' but lives under sync. One AV call per ticker.
Articles and ticker_sentiment rows land in the local store; FTS5 picks them up
automatically. Use --since last to compute "tickers I haven't sweeped recently"
based on av_quota_log timestamps.`,
		Example: strings.Trim(`
  alphavantage-pp-cli sync news --watchlist us-core --json
  alphavantage-pp-cli sync news --tickers NVDA,AAPL --since last --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagTickers == "" && flagWatchlist == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			tickers, err := resolveTickerSet(cmd, db, flagTickers, flagWatchlist)
			if err != nil {
				return err
			}

			// --since last: skip tickers that have a NEWS_SENTIMENT row in
			// av_quota_log within the lookback window (default 24h).
			if flagSince == "last" {
				tickers = filterRecentlySweeped(cmd, db, tickers, 24)
			}

			if len(tickers) == 0 {
				return printJSONFiltered(cmd.OutOrStdout(), newsSweepResult{}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := newsSweepResult{}
			for _, t := range tickers {
				data, err := c.Get("/query", map[string]string{
					"function": "NEWS_SENTIMENT",
					"tickers":  t,
				})
				if err != nil {
					logQuotaCall(cmd, db, "NEWS_SENTIMENT", t, false, err.Error())
					result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", t, trimErrorMessage(err)))
					continue
				}
				logQuotaCall(cmd, db, "NEWS_SENTIMENT", t, true, "")
				result.TickersProcessed++
				added, skipped, ts, perr := persistNewsResponse(db, data)
				if perr != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("%s: persist: %v", t, perr))
					continue
				}
				result.ArticlesAdded += added
				result.ArticlesSkippedDuplicate += skipped
				result.TickerSentimentRowsAdded += ts
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&flagTickers, "tickers", "", "Comma-separated tickers")
	cmd.Flags().StringVar(&flagWatchlist, "watchlist", "", "Saved watchlist name")
	cmd.Flags().StringVar(&flagSince, "since", "", "When 'last', skip tickers already sweeped in the last 24h (reads av_quota_log)")
	return cmd
}

// filterRecentlySweeped removes tickers whose latest NEWS_SENTIMENT call in
// av_quota_log is younger than lookbackHours.
func filterRecentlySweeped(cmd *cobra.Command, db *store.Store, tickers []string, lookbackHours int) []string {
	out := []string{}
	for _, t := range tickers {
		var n int
		err := db.DB().QueryRowContext(cmd.Context(),
			`SELECT COUNT(*) FROM av_quota_log
			 WHERE function = 'NEWS_SENTIMENT' AND symbol = ? AND ok = 1
			   AND called_at >= datetime('now', ?)`,
			t, fmt.Sprintf("-%d hours", lookbackHours),
		).Scan(&n)
		if err != nil || n == 0 {
			out = append(out, t)
		}
	}
	return out
}

type syncMoversResult struct {
	SnapshotDate  string `json:"snapshot_date"`
	GainersStored int    `json:"gainers_stored"`
	LosersStored  int    `json:"losers_stored"`
	ActiveStored  int    `json:"active_stored"`
}

func newSyncMoversCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "movers",
		Short: "Persist today's TOP_GAINERS_LOSERS snapshot for future diff queries",
		Long: `Run TOP_GAINERS_LOSERS once and persist the gainers/losers/active arrays
to av_movers_snapshots, keyed by today's UTC date. Subsequent reads (pulse us,
movers brief --enrich sentiment) reuse the snapshot at 0 AV cost.

Has a 'daily' subcommand that is an alias for the same operation.`,
		Example: strings.Trim(`
  alphavantage-pp-cli sync movers --json
  alphavantage-pp-cli sync movers daily --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncMovers(cmd, flags)
		},
	}
	// `daily` subcommand: identical to the parent, kept because the spec
	// mentions `sync movers daily`.
	cmd.AddCommand(&cobra.Command{
		Use:   "daily",
		Short: "Alias for 'sync movers' — persist today's snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSyncMovers(cmd, flags)
		},
	})
	return cmd
}

func runSyncMovers(cmd *cobra.Command, flags *rootFlags) error {
	if dryRunOK(flags) {
		return nil
	}
	db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local database: %w", err)
	}
	defer db.Close()
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	data, err := c.Get("/query", map[string]string{"function": "TOP_GAINERS_LOSERS"})
	if err != nil {
		logQuotaCall(cmd, db, "TOP_GAINERS_LOSERS", "", false, err.Error())
		return classifyAPIError(err, flags)
	}
	logQuotaCall(cmd, db, "TOP_GAINERS_LOSERS", "", true, "")

	today := todayYMD()
	result := syncMoversResult{SnapshotDate: today}
	if g, err := extractMoversSide(data, "gainers"); err == nil {
		persistMoversSnapshot(cmd, db, today, "gainers", g)
		result.GainersStored = len(g)
	}
	if l, err := extractMoversSide(data, "losers"); err == nil {
		persistMoversSnapshot(cmd, db, today, "losers", l)
		result.LosersStored = len(l)
	}
	if a, err := extractMoversSide(data, "active"); err == nil {
		persistMoversSnapshot(cmd, db, today, "active", a)
		result.ActiveStored = len(a)
	}
	return printJSONFiltered(cmd.OutOrStdout(), result, flags)
}

type syncEarningsCalendarResult struct {
	Horizon string `json:"horizon"`
	Stored  int    `json:"stored"`
}

func newSyncEarningsCalendarCmd(flags *rootFlags) *cobra.Command {
	var flagHorizon string
	cmd := &cobra.Command{
		Use:   "earnings-calendar",
		Short: "Persist EARNINGS_CALENDAR (CSV) into av_earnings_calendar",
		Long: `Pull the AV earnings calendar for a horizon (3month / 6month / 12month)
and upsert rows into av_earnings_calendar. Downstream commands (screen,
briefing earnings) read from this table.

Cost: 1 AV call.`,
		Example: strings.Trim(`
  alphavantage-pp-cli sync earnings-calendar --horizon 3month --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("alphavantage-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			horizon := flagHorizon
			if horizon == "" {
				horizon = "3month"
			}
			data, err := c.Get("/query", map[string]string{
				"function": "EARNINGS_CALENDAR",
				"horizon":  horizon,
			})
			if err != nil {
				logQuotaCall(cmd, db, "EARNINGS_CALENDAR", "", false, err.Error())
				return classifyAPIError(err, flags)
			}
			logQuotaCall(cmd, db, "EARNINGS_CALENDAR", "", true, "")
			stored := persistEarningsCalendar(cmd, db, data)
			return printJSONFiltered(cmd.OutOrStdout(), syncEarningsCalendarResult{
				Horizon: horizon,
				Stored:  stored,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagHorizon, "horizon", "3month", "Horizon: 3month | 6month | 12month")
	return cmd
}

// persistEarningsCalendar parses the CSV body and upserts rows. Returns the
// number of rows actually stored.
func persistEarningsCalendar(cmd *cobra.Command, db *store.Store, data json.RawMessage) int {
	body := string(data)
	if len(body) > 0 && body[0] == '"' {
		// JSON-quoted body — unquote.
		var unquoted string
		if err := json.Unmarshal(data, &unquoted); err == nil {
			body = unquoted
		}
	}
	reader := csv.NewReader(strings.NewReader(body))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil || len(rows) < 2 {
		return 0
	}
	header := rows[0]
	idx := func(name string) int {
		for i, h := range header {
			if strings.EqualFold(strings.TrimSpace(h), name) {
				return i
			}
		}
		return -1
	}
	symbolIdx := idx("symbol")
	nameIdx := idx("name")
	dateIdx := idx("reportDate")
	fiscalIdx := idx("fiscalDateEnding")
	estimateIdx := idx("estimate")
	currencyIdx := idx("currency")

	stored := 0
	tx, err := db.DB().Begin()
	if err != nil {
		return 0
	}
	defer tx.Rollback()
	for _, r := range rows[1:] {
		if symbolIdx < 0 || dateIdx < 0 || symbolIdx >= len(r) || dateIdx >= len(r) {
			continue
		}
		_, err := tx.Exec(
			`INSERT OR REPLACE INTO av_earnings_calendar
			 (symbol, name, report_date, fiscal_date_ending, estimate, currency)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			r[symbolIdx],
			safeField(r, nameIdx),
			r[dateIdx],
			safeField(r, fiscalIdx),
			safeField(r, estimateIdx),
			safeField(r, currencyIdx),
		)
		if err == nil {
			stored++
		}
	}
	_ = tx.Commit()
	return stored
}

func safeField(row []string, idx int) string {
	if idx < 0 || idx >= len(row) {
		return ""
	}
	return row[idx]
}
