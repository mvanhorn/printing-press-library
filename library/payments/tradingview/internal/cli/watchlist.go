// Copyright 2026 jbriaux and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: local watchlist with quote snapshots (SQLite-backed).

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"tradingview-pp-cli/internal/client"
	"tradingview-pp-cli/internal/store"
)

// ensureWatchlistSchema creates the watchlist and quote_snapshots tables if
// they don't already exist. Idempotent; safe to call before every operation.
func ensureWatchlistSchema(ctx context.Context, db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS watchlist (
			symbol      TEXT PRIMARY KEY,
			description TEXT,
			type        TEXT,
			added_at    TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS quote_snapshots (
			symbol     TEXT NOT NULL,
			price      REAL,
			currency   TEXT,
			price_usd  REAL,
			price_eur  REAL,
			change_pct REAL,
			ts         TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_snap_symbol_ts ON quote_snapshots(symbol, ts)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().ExecContext(ctx, s); err != nil {
			return fmt.Errorf("creating watchlist schema: %w", err)
		}
	}
	return nil
}

func openWatchlistStore(ctx context.Context, flags *rootFlags) (*store.Store, error) {
	dbPath := defaultDBPath("tradingview-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if err := ensureWatchlistSchema(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Short: "Track a local list of symbols and snapshot their quotes for offline use.",
		Long: "Maintain a local SQLite watchlist of symbols. 'sync' fetches current prices\n" +
			"(in USD and EUR) and stores a timestamped snapshot; 'quotes' shows the latest\n" +
			"price for every watched symbol, and 'changes' shows how each moved since the\n" +
			"previous snapshot. All state is local — no account or API key required.",
		Aliases: []string{"wl"},
	}
	cmd.AddCommand(
		newWatchlistAddCmd(flags),
		newWatchlistRemoveCmd(flags),
		newWatchlistListCmd(flags),
		newWatchlistSyncCmd(flags),
		newWatchlistQuotesCmd(flags),
		newWatchlistChangesCmd(flags),
	)
	return cmd
}

type watchlistEntry struct {
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Type        string `json:"type"`
	AddedAt     string `json:"added_at"`
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "add <symbol>...",
		Short:       "Add one or more symbols to the watchlist (validates and resolves each).",
		Example:     "  tradingview-pp-cli watchlist add AAPL BINANCE:BTCUSDT",
		Annotations: map[string]string{"pp:happy-args": "symbol=AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would add symbols to the watchlist")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one symbol is required, e.g. 'watchlist add AAPL'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openWatchlistStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			added := make([]watchlistEntry, 0, len(args))
			now := time.Now().UTC().Format(time.RFC3339)
			for _, a := range args {
				fq, _, rerr := resolveSymbol(ctx, c, a, "")
				if rerr != nil {
					return rerr
				}
				view, found, verr := buildQuoteView(ctx, c, fq)
				if verr != nil {
					return classifyAPIError(verr, flags)
				}
				if !found {
					return fmt.Errorf("no price available for %q; not added (try 'search %s')", fq, a)
				}
				if _, err := db.DB().ExecContext(ctx,
					`INSERT INTO watchlist(symbol, description, type, added_at)
					 VALUES(?, ?, ?, ?)
					 ON CONFLICT(symbol) DO UPDATE SET description=excluded.description, type=excluded.type`,
					fq, view.Description, view.Type, now); err != nil {
					return fmt.Errorf("adding %q: %w", fq, err)
				}
				added = append(added, watchlistEntry{Symbol: fq, Description: view.Description, Type: view.Type, AddedAt: now})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), added, flags)
			}
			for _, e := range added {
				fmt.Fprintf(cmd.OutOrStdout(), "added %s  %s\n", e.Symbol, e.Description)
			}
			return nil
		},
	}
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "remove <symbol>...",
		Aliases:     []string{"rm"},
		Short:       "Remove one or more symbols from the watchlist.",
		Example:     "  tradingview-pp-cli watchlist remove NASDAQ:AAPL",
		Annotations: map[string]string{"pp:happy-args": "symbol=NASDAQ:AAPL"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would remove symbols from the watchlist")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("at least one symbol is required, e.g. 'watchlist remove NASDAQ:AAPL'"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openWatchlistStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()

			removed := 0
			for _, a := range args {
				up := strings.ToUpper(strings.TrimSpace(a))
				var res sql.Result
				var err error
				if strings.Contains(up, ":") {
					// Fully-qualified EXCHANGE:TICKER — exact match.
					res, err = db.DB().ExecContext(ctx, `DELETE FROM watchlist WHERE symbol=?`, up)
				} else {
					// Bare ticker — try to resolve for an exact match, but also
					// match any stored EXCHANGE:<ticker> so removal still works
					// when the symbol-search endpoint is unreachable.
					fq := ""
					if r, _, rerr := resolveSymbol(ctx, c, a, ""); rerr == nil {
						fq = r
					}
					res, err = db.DB().ExecContext(ctx,
						`DELETE FROM watchlist WHERE symbol=? OR symbol LIKE ?`, fq, "%:"+up)
				}
				if err != nil {
					return fmt.Errorf("removing %q: %w", a, err)
				}
				if n, _ := res.RowsAffected(); n > 0 {
					removed += int(n)
					fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", up)
				}
			}
			if removed == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no matching symbols in the watchlist")
			}
			return nil
		},
	}
}

func readWatchlist(ctx context.Context, db *store.Store) ([]watchlistEntry, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT symbol, description, type, added_at FROM watchlist ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("reading watchlist: %w", err)
	}
	defer rows.Close()
	entries := make([]watchlistEntry, 0)
	for rows.Next() {
		var symbol string
		var desc, typ, added sql.NullString
		if err := rows.Scan(&symbol, &desc, &typ, &added); err != nil {
			return nil, fmt.Errorf("scanning watchlist row: %w", err)
		}
		entries = append(entries, watchlistEntry{
			Symbol:      symbol,
			Description: desc.String,
			Type:        typ.String,
			AddedAt:     added.String,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating watchlist: %w", err)
	}
	return entries, nil
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Aliases:     []string{"ls"},
		Short:       "List the symbols in the watchlist.",
		Example:     "  tradingview-pp-cli watchlist list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWatchlistStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := readWatchlist(ctx, db)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
			}
			if len(entries) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "watchlist is empty — add symbols with 'watchlist add <symbol>'")
				return nil
			}
			for _, e := range entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s %-6s %s\n", e.Symbol, e.Type, e.Description)
			}
			return nil
		},
	}
}

type snapshotView struct {
	Symbol    string  `json:"symbol"`
	Price     float64 `json:"price"`
	Currency  string  `json:"currency"`
	PriceUSD  float64 `json:"price_usd"`
	PriceEUR  float64 `json:"price_eur"`
	ChangePct float64 `json:"change_pct"`
	Timestamp string  `json:"ts"`
}

func newWatchlistSyncCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "sync",
		Short:       "Fetch current quotes for every watched symbol and store a timestamped snapshot.",
		Example:     "  tradingview-pp-cli watchlist sync",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would snapshot quotes for every watched symbol")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openWatchlistStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := readWatchlist(ctx, db)
			if err != nil {
				return err
			}
			now := time.Now().UTC().Format(time.RFC3339)

			// Fetch every quote first (network) so the transaction below only
			// wraps fast local writes — never a slow API call.
			type pendingSnap struct {
				entry watchlistEntry
				view  quoteView
			}
			pending := make([]pendingSnap, 0, len(entries))
			for _, e := range entries {
				view, found, verr := buildQuoteView(ctx, c, e.Symbol)
				if verr != nil {
					return classifyAPIError(verr, flags)
				}
				if !found {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: no price for %s; skipped\n", e.Symbol)
					continue
				}
				pending = append(pending, pendingSnap{entry: e, view: view})
			}

			// Write all snapshots atomically: an interrupted sync leaves the
			// store either fully updated or unchanged, never half-written.
			snaps := make([]snapshotView, 0, len(pending))
			if len(pending) > 0 {
				tx, err := db.DB().BeginTx(ctx, nil)
				if err != nil {
					return fmt.Errorf("starting snapshot transaction: %w", err)
				}
				committed := false
				defer func() {
					if !committed {
						_ = tx.Rollback()
					}
				}()
				const keepPerSymbol = 200
				for _, p := range pending {
					if _, err := tx.ExecContext(ctx,
						`INSERT INTO quote_snapshots(symbol, price, currency, price_usd, price_eur, change_pct, ts)
						 VALUES(?, ?, ?, ?, ?, ?, ?)`,
						p.entry.Symbol, p.view.Price, p.view.Currency, p.view.PriceUSD, p.view.PriceEUR, p.view.ChangePct, now); err != nil {
						return fmt.Errorf("storing snapshot for %s: %w", p.entry.Symbol, err)
					}
					// Bound history: keep only the most recent snapshots per
					// symbol so quote_snapshots does not grow without limit.
					if _, err := tx.ExecContext(ctx,
						`DELETE FROM quote_snapshots WHERE symbol=? AND rowid NOT IN (
							SELECT rowid FROM quote_snapshots WHERE symbol=? ORDER BY rowid DESC LIMIT ?
						)`, p.entry.Symbol, p.entry.Symbol, keepPerSymbol); err != nil {
						return fmt.Errorf("pruning snapshots for %s: %w", p.entry.Symbol, err)
					}
					snaps = append(snaps, snapshotView{
						Symbol: p.entry.Symbol, Price: p.view.Price, Currency: p.view.Currency,
						PriceUSD: p.view.PriceUSD, PriceEUR: p.view.PriceEUR, ChangePct: p.view.ChangePct, Timestamp: now,
					})
				}
				if err := tx.Commit(); err != nil {
					return fmt.Errorf("committing snapshots: %w", err)
				}
				committed = true
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), snaps, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "synced %d of %d watched symbols at %s\n", len(snaps), len(entries), now)
			return nil
		},
	}
}

// latestSnapshot returns the most recent snapshot for a symbol, if any.
func latestSnapshot(ctx context.Context, db *store.Store, symbol string) (snapshotView, bool, error) {
	row := db.DB().QueryRowContext(ctx,
		`SELECT price, currency, price_usd, price_eur, change_pct, ts
		 FROM quote_snapshots WHERE symbol=? ORDER BY ts DESC LIMIT 1`, symbol)
	var price, usd, eur, chg sql.NullFloat64
	var cur, ts sql.NullString
	if err := row.Scan(&price, &cur, &usd, &eur, &chg, &ts); err != nil {
		if err == sql.ErrNoRows {
			return snapshotView{}, false, nil
		}
		return snapshotView{}, false, err
	}
	return snapshotView{
		Symbol: symbol, Price: price.Float64, Currency: cur.String,
		PriceUSD: usd.Float64, PriceEUR: eur.Float64, ChangePct: chg.Float64, Timestamp: ts.String,
	}, true, nil
}

func newWatchlistQuotesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "quotes",
		Short:       "Show the latest price (USD + EUR) for every watched symbol.",
		Long: "Show the latest price for each watched symbol. In 'auto' (default) and 'live'\n" +
			"data-source modes this fetches fresh quotes; in 'local' mode it reads the most\n" +
			"recent stored snapshot from 'watchlist sync'.",
		Example:     "  tradingview-pp-cli watchlist quotes --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openWatchlistStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := readWatchlist(ctx, db)
			if err != nil {
				return err
			}

			local := flags.dataSource == "local"
			var c *client.Client
			if !local {
				c, err = flags.newClient()
				if err != nil {
					return err
				}
			}

			out := make([]snapshotView, 0, len(entries))
			for _, e := range entries {
				if !local && c != nil {
					if view, found, verr := buildQuoteView(ctx, c, e.Symbol); verr == nil && found {
						out = append(out, snapshotView{
							Symbol: e.Symbol, Price: view.Price, Currency: view.Currency,
							PriceUSD: view.PriceUSD, PriceEUR: view.PriceEUR, ChangePct: view.ChangePct,
							Timestamp: time.Now().UTC().Format(time.RFC3339),
						})
						continue
					}
					// fall through to local snapshot on live failure (auto mode)
				}
				if snap, ok, serr := latestSnapshot(ctx, db, e.Symbol); serr != nil {
					return serr
				} else if ok {
					out = append(out, snap)
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no quotes — add symbols with 'watchlist add' then 'watchlist sync'")
				return nil
			}
			for _, s := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s $%-12s EUR %-12s (%+.2f%%)\n",
					s.Symbol, fmtNum(s.PriceUSD), fmtNum(s.PriceEUR), s.ChangePct)
			}
			return nil
		},
	}
}

type changeView struct {
	Symbol       string  `json:"symbol"`
	PreviousUSD  float64 `json:"previous_usd"`
	LatestUSD    float64 `json:"latest_usd"`
	DeltaUSD     float64 `json:"delta_usd"`
	DeltaPct     float64 `json:"delta_pct"`
	FromTS       string  `json:"from_ts"`
	ToTS         string  `json:"to_ts"`
	Note         string  `json:"note,omitempty"`
}

func newWatchlistChangesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "changes",
		Short: "Show how each watched symbol moved (USD) between its two most recent snapshots.",
		Long: "Compare the two most recent stored snapshots for each watched symbol and report\n" +
			"the USD price delta since the previous sync. Run 'watchlist sync' at least twice\n" +
			"to populate history. This is local-only insight no single API call provides.",
		Example:     "  tradingview-pp-cli watchlist changes --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := defaultDBPath("tradingview-pp-cli")
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local snapshots yet — run 'tradingview-pp-cli watchlist sync' twice first\n")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			db, err := openWatchlistStore(ctx, flags)
			if err != nil {
				return err
			}
			defer db.Close()
			entries, err := readWatchlist(ctx, db)
			if err != nil {
				return err
			}

			out := make([]changeView, 0, len(entries))
			for _, e := range entries {
				// Read the two most recent snapshots (drain fully before next query).
				rows, qerr := db.DB().QueryContext(ctx,
					`SELECT price_usd, ts FROM quote_snapshots WHERE symbol=? ORDER BY ts DESC LIMIT 2`, e.Symbol)
				if qerr != nil {
					return fmt.Errorf("reading snapshots for %s: %w", e.Symbol, qerr)
				}
				type snap struct {
					usd float64
					ts  string
				}
				var snaps []snap
				for rows.Next() {
					var usd sql.NullFloat64
					var ts sql.NullString
					if serr := rows.Scan(&usd, &ts); serr != nil {
						_ = rows.Close()
						return fmt.Errorf("scanning snapshot for %s: %w", e.Symbol, serr)
					}
					snaps = append(snaps, snap{usd: usd.Float64, ts: ts.String})
				}
				if rerr := rows.Err(); rerr != nil {
					_ = rows.Close()
					return rerr
				}
				_ = rows.Close()

				cv := changeView{Symbol: e.Symbol}
				switch {
				case len(snaps) >= 2:
					cv.LatestUSD = snaps[0].usd
					cv.PreviousUSD = snaps[1].usd
					cv.ToTS = snaps[0].ts
					cv.FromTS = snaps[1].ts
					cv.DeltaUSD = snaps[0].usd - snaps[1].usd
					if snaps[1].usd != 0 {
						cv.DeltaPct = (cv.DeltaUSD / snaps[1].usd) * 100
					}
				case len(snaps) == 1:
					cv.LatestUSD = snaps[0].usd
					cv.ToTS = snaps[0].ts
					cv.Note = "only one snapshot; run 'watchlist sync' again to compute a change"
				default:
					cv.Note = "no snapshots; run 'watchlist sync' first"
				}
				out = append(out, cv)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "watchlist is empty")
				return nil
			}
			for _, c := range out {
				if c.Note != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "%-24s %s\n", c.Symbol, c.Note)
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-24s $%s -> $%s  (%+.4g USD, %+.2f%%)\n",
					c.Symbol, fmtNum(c.PreviousUSD), fmtNum(c.LatestUSD), c.DeltaUSD, c.DeltaPct)
			}
			return nil
		},
	}
}
