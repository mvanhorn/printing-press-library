// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Transcendence commands — unique to yahoo-finance-pp-cli. Watchlists, portfolios,
// digests, peer compare, sparklines, options moneyness filter, local SQL access.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/store"

	"github.com/spf13/cobra"
	_ "modernc.org/sqlite"
)

const transcendenceSchema = `
CREATE TABLE IF NOT EXISTS watchlists (
	name TEXT PRIMARY KEY,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE TABLE IF NOT EXISTS watchlist_members (
	watchlist TEXT NOT NULL,
	symbol TEXT NOT NULL,
	added_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (watchlist, symbol),
	FOREIGN KEY (watchlist) REFERENCES watchlists(name) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS portfolio_lots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	symbol TEXT NOT NULL,
	shares REAL NOT NULL,
	cost_basis REAL NOT NULL,
	purchased_on DATE NOT NULL,
	note TEXT,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_portfolio_lots_symbol ON portfolio_lots(symbol);
`

// openDB opens the local sqlite file the generated sync uses. Also ensures our
// transcendence tables exist. Safe to call when DB doesn't exist yet — returns
// a fresh one.
func openDB(flags *rootFlags) (*sql.DB, error) {
	path := defaultDBPath("yahoo-finance-pp-cli")
	if _, ferr := os.Stat(path); ferr != nil {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
		s, serr := store.Open(path)
		if serr != nil {
			return nil, serr
		}
		s.Close()
	}
	db, err := sql.Open("sqlite", path+"?_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(transcendenceSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrating transcendence tables: %w", err)
	}
	return db, nil
}

// ---------------------------------------------------------------------------
// watchlist
// ---------------------------------------------------------------------------

func newWatchlistCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "watchlist",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Create, manage, and query local watchlists of ticker symbols",
		Long:  "Watchlists live in your local SQLite database. They power multi-symbol commands like `digest`, `compare`, and `watchlist show`.",
		Example: `  # Create a watchlist and add symbols
  yahoo-finance-pp-cli watchlist create tech
  yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG

  # Show current quotes for everything on the list
  yahoo-finance-pp-cli watchlist show tech

  # List every watchlist you've created
  yahoo-finance-pp-cli watchlist list`,
	}
	cmd.AddCommand(newWatchlistCreateCmd(flags))
	cmd.AddCommand(newWatchlistAddCmd(flags))
	cmd.AddCommand(newWatchlistRemoveCmd(flags))
	cmd.AddCommand(newWatchlistListCmd(flags))
	cmd.AddCommand(newWatchlistShowCmd(flags))
	cmd.AddCommand(newWatchlistDeleteCmd(flags))
	return cmd
}

func newWatchlistCreateCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "create <name>",
		Short:   "Create a new watchlist",
		Args:    cobra.ExactArgs(1),
		Example: "  yahoo-finance-pp-cli watchlist create tech",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.Exec("INSERT OR IGNORE INTO watchlists(name) VALUES(?)", args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watchlist %q ready\n", args[0])
			return nil
		},
	}
}

func newWatchlistAddCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "add <name> <symbol> [symbol...]",
		Short:   "Add one or more symbols to a watchlist",
		Args:    cobra.MinimumNArgs(2),
		Example: "  yahoo-finance-pp-cli watchlist add tech AAPL MSFT NVDA GOOG",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.Exec("INSERT OR IGNORE INTO watchlists(name) VALUES(?)", args[0]); err != nil {
				return err
			}
			added := 0
			for _, sym := range args[1:] {
				sym = strings.ToUpper(strings.TrimSpace(sym))
				if sym == "" {
					continue
				}
				res, err := db.Exec("INSERT OR IGNORE INTO watchlist_members(watchlist, symbol) VALUES(?,?)", args[0], sym)
				if err != nil {
					return err
				}
				n, _ := res.RowsAffected()
				added += int(n)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %d symbols to %q\n", added, args[0])
			return nil
		},
	}
}

func newWatchlistRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name> <symbol> [symbol...]",
		Short: "Remove symbols from a watchlist",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			for _, sym := range args[1:] {
				if _, err := db.Exec("DELETE FROM watchlist_members WHERE watchlist=? AND symbol=?", args[0], strings.ToUpper(sym)); err != nil {
					return err
				}
			}
			return nil
		},
	}
}

func newWatchlistListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List all watchlists with member counts",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(`SELECT w.name, COUNT(m.symbol)
				FROM watchlists w LEFT JOIN watchlist_members m ON m.watchlist = w.name
				GROUP BY w.name ORDER BY w.name`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type entry struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Name, &e.Count); err != nil {
					return err
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"WATCHLIST", "SYMBOLS"}
			rowsOut := make([][]string, 0, len(out))
			for _, e := range out {
				rowsOut = append(rowsOut, []string{e.Name, strconv.Itoa(e.Count)})
			}
			return flags.printTable(cmd, headers, rowsOut)
		},
	}
}

func watchlistSymbols(db *sql.DB, name string) ([]string, error) {
	rows, err := db.Query("SELECT symbol FROM watchlist_members WHERE watchlist=? ORDER BY symbol", name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func newWatchlistShowCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "show <name>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show symbols in a watchlist with live quotes",
		Args:    cobra.ExactArgs(1),
		Example: "  yahoo-finance-pp-cli watchlist show tech",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			symbols, err := watchlistSymbols(db, args[0])
			if err != nil {
				return err
			}
			if len(symbols) == 0 {
				return fmt.Errorf("watchlist %q is empty or does not exist", args[0])
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			quotes, err := fetchQuotes(c, symbols)
			if err != nil {
				// Still show symbols even if the live fetch fails
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"watchlist": args[0], "symbols": symbols, "quote_error": err.Error()})
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: live quote fetch failed: %v\n", err)
				return flags.printTable(cmd, []string{"SYMBOL"}, symbolRows(symbols))
			}
			return renderQuotes(cmd, flags, args[0], quotes)
		},
	}
}

func newWatchlistDeleteCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a watchlist (does not affect other data)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.Exec("DELETE FROM watchlists WHERE name=?", args[0]); err != nil {
				return err
			}
			return nil
		},
	}
}

func symbolRows(symbols []string) [][]string {
	out := make([][]string, len(symbols))
	for i, s := range symbols {
		out[i] = []string{s}
	}
	return out
}

// ---------------------------------------------------------------------------
// quote fetcher + rendering (shared by watchlist, digest, compare, etc.)
// ---------------------------------------------------------------------------

type quoteRow struct {
	Symbol           string  `json:"symbol"`
	ShortName        string  `json:"short_name"`
	RegularPrice     float64 `json:"regular_price"`
	RegularChange    float64 `json:"regular_change"`
	RegularChangePct float64 `json:"regular_change_pct"`
	MarketCap        float64 `json:"market_cap"`
	FiftyTwoWeekHigh float64 `json:"fifty_two_week_high"`
	FiftyTwoWeekLow  float64 `json:"fifty_two_week_low"`
	Currency         string  `json:"currency"`
	MarketState      string  `json:"market_state"`
	PrePostPrice     float64 `json:"pre_post_price,omitempty"`
	PrePostChangePct float64 `json:"pre_post_change_pct,omitempty"`
}

// fetchQuotes calls /v7/finance/quote with chunking to stay under URL length limits.
func fetchQuotes(c *client.Client, symbols []string) ([]quoteRow, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	const chunkSize = 50
	var all []quoteRow
	for i := 0; i < len(symbols); i += chunkSize {
		end := i + chunkSize
		if end > len(symbols) {
			end = len(symbols)
		}
		data, err := c.Get("/v7/finance/quote", map[string]string{"symbols": strings.Join(symbols[i:end], ",")})
		if err != nil {
			return all, err
		}
		var env struct {
			QuoteResponse struct {
				Result []struct {
					Symbol                 string  `json:"symbol"`
					ShortName              string  `json:"shortName"`
					LongName               string  `json:"longName"`
					RegularMarketPrice     float64 `json:"regularMarketPrice"`
					RegularMarketChange    float64 `json:"regularMarketChange"`
					RegularMarketChangePct float64 `json:"regularMarketChangePercent"`
					MarketCap              float64 `json:"marketCap"`
					FiftyTwoWeekHigh       float64 `json:"fiftyTwoWeekHigh"`
					FiftyTwoWeekLow        float64 `json:"fiftyTwoWeekLow"`
					Currency               string  `json:"currency"`
					MarketState            string  `json:"marketState"`
					PostMarketPrice        float64 `json:"postMarketPrice"`
					PostMarketChangePct    float64 `json:"postMarketChangePercent"`
					PreMarketPrice         float64 `json:"preMarketPrice"`
					PreMarketChangePct     float64 `json:"preMarketChangePercent"`
				} `json:"result"`
				Error any `json:"error"`
			} `json:"quoteResponse"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return all, fmt.Errorf("parsing quote response: %w", err)
		}
		for _, r := range env.QuoteResponse.Result {
			name := r.ShortName
			if name == "" {
				name = r.LongName
			}
			q := quoteRow{
				Symbol:           r.Symbol,
				ShortName:        name,
				RegularPrice:     r.RegularMarketPrice,
				RegularChange:    r.RegularMarketChange,
				RegularChangePct: r.RegularMarketChangePct,
				MarketCap:        r.MarketCap,
				FiftyTwoWeekHigh: r.FiftyTwoWeekHigh,
				FiftyTwoWeekLow:  r.FiftyTwoWeekLow,
				Currency:         r.Currency,
				MarketState:      r.MarketState,
			}
			if r.PostMarketPrice > 0 {
				q.PrePostPrice = r.PostMarketPrice
				q.PrePostChangePct = r.PostMarketChangePct
			} else if r.PreMarketPrice > 0 {
				q.PrePostPrice = r.PreMarketPrice
				q.PrePostChangePct = r.PreMarketChangePct
			}
			all = append(all, q)
		}
	}
	return all, nil
}

func renderQuotes(cmd *cobra.Command, flags *rootFlags, label string, quotes []quoteRow) error {
	if flags.asJSON {
		return flags.printJSON(cmd, map[string]any{"label": label, "quotes": quotes})
	}
	if len(quotes) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "(no quotes returned)")
		return nil
	}
	headers := []string{"SYMBOL", "PRICE", "CHG", "CHG%", "NAME"}
	rows := make([][]string, 0, len(quotes))
	for _, q := range quotes {
		rows = append(rows, []string{
			q.Symbol,
			fmt.Sprintf("%.2f", q.RegularPrice),
			fmt.Sprintf("%+.2f", q.RegularChange),
			fmt.Sprintf("%+.2f%%", q.RegularChangePct),
			q.ShortName,
		})
	}
	return flags.printTable(cmd, headers, rows)
}

// ---------------------------------------------------------------------------
// portfolio
// ---------------------------------------------------------------------------

func newPortfolioCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "portfolio",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Track holdings, cost basis, returns, and dividend income locally",
		Long:  "Portfolios are a local SQLite concept. Each 'lot' records shares, cost basis, and purchase date. Commands compute P&L, YTD return, and dividend income against live quotes.",
	}
	cmd.AddCommand(newPortfolioAddCmd(flags))
	cmd.AddCommand(newPortfolioListCmd(flags))
	cmd.AddCommand(newPortfolioRemoveCmd(flags))
	cmd.AddCommand(newPortfolioPerfCmd(flags))
	cmd.AddCommand(newPortfolioGainsCmd(flags))
	return cmd
}

func newPortfolioAddCmd(flags *rootFlags) *cobra.Command {
	var purchased string
	var note string
	cmd := &cobra.Command{
		Use:     "add <symbol> <shares> <cost-per-share>",
		Short:   "Record a purchase lot",
		Args:    cobra.ExactArgs(3),
		Example: "  yahoo-finance-pp-cli portfolio add AAPL 50 185.50 --purchased 2024-06-15",
		RunE: func(cmd *cobra.Command, args []string) error {
			shares, err := strconv.ParseFloat(args[1], 64)
			if err != nil {
				return fmt.Errorf("shares must be a number: %w", err)
			}
			cost, err := strconv.ParseFloat(args[2], 64)
			if err != nil {
				return fmt.Errorf("cost-per-share must be a number: %w", err)
			}
			if purchased == "" {
				purchased = time.Now().Format("2006-01-02")
			}
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			if _, err := db.Exec(
				"INSERT INTO portfolio_lots(symbol, shares, cost_basis, purchased_on, note) VALUES(?,?,?,?,?)",
				strings.ToUpper(args[0]), shares, cost, purchased, note,
			); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added %s: %.2f shares @ %.2f on %s\n", strings.ToUpper(args[0]), shares, cost, purchased)
			return nil
		},
	}
	cmd.Flags().StringVar(&purchased, "purchased", "", "Purchase date (YYYY-MM-DD, default today)")
	cmd.Flags().StringVar(&note, "note", "", "Optional note for this lot")
	return cmd
}

func newPortfolioListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "List all portfolio lots",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query("SELECT id, symbol, shares, cost_basis, purchased_on, COALESCE(note,'') FROM portfolio_lots ORDER BY symbol, purchased_on")
			if err != nil {
				return err
			}
			defer rows.Close()
			type lot struct {
				ID          int64   `json:"id"`
				Symbol      string  `json:"symbol"`
				Shares      float64 `json:"shares"`
				CostBasis   float64 `json:"cost_basis"`
				PurchasedOn string  `json:"purchased_on"`
				Note        string  `json:"note"`
			}
			var out []lot
			for rows.Next() {
				var l lot
				if err := rows.Scan(&l.ID, &l.Symbol, &l.Shares, &l.CostBasis, &l.PurchasedOn, &l.Note); err != nil {
					return err
				}
				out = append(out, l)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"ID", "SYMBOL", "SHARES", "COST", "PURCHASED", "NOTE"}
			table := make([][]string, 0, len(out))
			for _, l := range out {
				table = append(table, []string{
					strconv.FormatInt(l.ID, 10),
					l.Symbol,
					fmt.Sprintf("%.2f", l.Shares),
					fmt.Sprintf("%.2f", l.CostBasis),
					l.PurchasedOn,
					l.Note,
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
}

func newPortfolioRemoveCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <lot-id>",
		Short: "Remove a portfolio lot by id (see `portfolio list`)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return err
			}
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			_, err = db.Exec("DELETE FROM portfolio_lots WHERE id=?", id)
			return err
		},
	}
}

func newPortfolioPerfCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "perf",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Show current market value, cost basis, and unrealized P&L across all lots",
		Example: "  yahoo-finance-pp-cli portfolio perf",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(`SELECT symbol, SUM(shares), SUM(shares*cost_basis)
				FROM portfolio_lots GROUP BY symbol ORDER BY symbol`)
			if err != nil {
				return err
			}
			defer rows.Close()
			type agg struct {
				Symbol    string
				Shares    float64
				CostTotal float64
			}
			var positions []agg
			for rows.Next() {
				var a agg
				if err := rows.Scan(&a.Symbol, &a.Shares, &a.CostTotal); err != nil {
					return err
				}
				positions = append(positions, a)
			}
			if len(positions) == 0 {
				return fmt.Errorf("no portfolio lots — add one with `portfolio add SYMBOL SHARES COST`")
			}
			syms := make([]string, len(positions))
			for i, p := range positions {
				syms[i] = p.Symbol
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			quotes, err := fetchQuotes(c, syms)
			if err != nil {
				return err
			}
			priceBySymbol := map[string]float64{}
			for _, q := range quotes {
				priceBySymbol[q.Symbol] = q.RegularPrice
			}
			type perfRow struct {
				Symbol    string  `json:"symbol"`
				Shares    float64 `json:"shares"`
				CostAvg   float64 `json:"cost_avg"`
				Price     float64 `json:"price"`
				Value     float64 `json:"value"`
				CostTotal float64 `json:"cost_total"`
				GainDol   float64 `json:"gain_dollars"`
				GainPct   float64 `json:"gain_pct"`
			}
			var out []perfRow
			var totalValue, totalCost float64
			for _, p := range positions {
				price := priceBySymbol[p.Symbol]
				value := price * p.Shares
				costAvg := 0.0
				if p.Shares > 0 {
					costAvg = p.CostTotal / p.Shares
				}
				gain := value - p.CostTotal
				pct := 0.0
				if p.CostTotal > 0 {
					pct = gain / p.CostTotal * 100
				}
				out = append(out, perfRow{p.Symbol, p.Shares, costAvg, price, value, p.CostTotal, gain, pct})
				totalValue += value
				totalCost += p.CostTotal
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Value > out[j].Value })
			totalGain := totalValue - totalCost
			totalPct := 0.0
			if totalCost > 0 {
				totalPct = totalGain / totalCost * 100
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"positions":   out,
					"total_value": totalValue,
					"total_cost":  totalCost,
					"total_gain":  totalGain,
					"total_pct":   totalPct,
				})
			}
			headers := []string{"SYMBOL", "SHARES", "AVG COST", "PRICE", "VALUE", "GAIN $", "GAIN %"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					r.Symbol,
					fmt.Sprintf("%.2f", r.Shares),
					fmt.Sprintf("%.2f", r.CostAvg),
					fmt.Sprintf("%.2f", r.Price),
					fmt.Sprintf("%.2f", r.Value),
					fmt.Sprintf("%+.2f", r.GainDol),
					fmt.Sprintf("%+.2f%%", r.GainPct),
				})
			}
			if err := flags.printTable(cmd, headers, table); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nTotal: value=%.2f cost=%.2f gain=%+.2f (%+.2f%%)\n", totalValue, totalCost, totalGain, totalPct)
			return nil
		},
	}
}

func newPortfolioGainsCmd(flags *rootFlags) *cobra.Command {
	var unrealized bool
	cmd := &cobra.Command{
		Use:   "gains",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Per-lot unrealized gain/loss sorted by magnitude",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = unrealized
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query("SELECT id, symbol, shares, cost_basis, purchased_on FROM portfolio_lots")
			if err != nil {
				return err
			}
			defer rows.Close()
			type lot struct {
				ID          int64
				Symbol      string
				Shares      float64
				CostBasis   float64
				PurchasedOn string
			}
			var lots []lot
			symSet := map[string]bool{}
			for rows.Next() {
				var l lot
				if err := rows.Scan(&l.ID, &l.Symbol, &l.Shares, &l.CostBasis, &l.PurchasedOn); err != nil {
					return err
				}
				lots = append(lots, l)
				symSet[l.Symbol] = true
			}
			if len(lots) == 0 {
				return fmt.Errorf("no portfolio lots")
			}
			syms := make([]string, 0, len(symSet))
			for s := range symSet {
				syms = append(syms, s)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			quotes, err := fetchQuotes(c, syms)
			if err != nil {
				return err
			}
			price := map[string]float64{}
			for _, q := range quotes {
				price[q.Symbol] = q.RegularPrice
			}
			type gainRow struct {
				LotID       int64   `json:"lot_id"`
				Symbol      string  `json:"symbol"`
				Shares      float64 `json:"shares"`
				CostBasis   float64 `json:"cost_basis"`
				PurchasedOn string  `json:"purchased_on"`
				Price       float64 `json:"price"`
				Gain        float64 `json:"gain"`
				GainPct     float64 `json:"gain_pct"`
			}
			var out []gainRow
			for _, l := range lots {
				p := price[l.Symbol]
				cost := l.Shares * l.CostBasis
				value := l.Shares * p
				gain := value - cost
				pct := 0.0
				if cost > 0 {
					pct = gain / cost * 100
				}
				out = append(out, gainRow{l.ID, l.Symbol, l.Shares, l.CostBasis, l.PurchasedOn, p, gain, pct})
			}
			sort.Slice(out, func(i, j int) bool { return out[i].Gain > out[j].Gain })
			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"LOT", "SYMBOL", "SHARES", "COST", "PRICE", "PURCHASED", "GAIN", "%"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					strconv.FormatInt(r.LotID, 10),
					r.Symbol,
					fmt.Sprintf("%.2f", r.Shares),
					fmt.Sprintf("%.2f", r.CostBasis),
					fmt.Sprintf("%.2f", r.Price),
					r.PurchasedOn,
					fmt.Sprintf("%+.2f", r.Gain),
					fmt.Sprintf("%+.2f%%", r.GainPct),
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().BoolVar(&unrealized, "unrealized", true, "Show unrealized gains (default)")
	return cmd
}

// ---------------------------------------------------------------------------
// digest — morning briefing across a watchlist
// ---------------------------------------------------------------------------

func newDigestCmd(flags *rootFlags) *cobra.Command {
	var watchlistName string
	var symbolsFlag string
	cmd := &cobra.Command{
		Use:   "digest",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Morning briefing: biggest movers and headline quotes across a watchlist",
		Example: `  yahoo-finance-pp-cli digest --watchlist tech
  yahoo-finance-pp-cli digest --symbols AAPL,MSFT,NVDA`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var syms []string
			if symbolsFlag != "" {
				for _, s := range strings.Split(symbolsFlag, ",") {
					syms = append(syms, strings.ToUpper(strings.TrimSpace(s)))
				}
			} else if !flags.dryRun {
				db, err := openDB(flags)
				if err != nil {
					return err
				}
				defer db.Close()
				name := watchlistName
				if name == "" {
					// fall back to "default"
					name = "default"
				}
				syms, err = watchlistSymbols(db, name)
				if err != nil {
					return err
				}
				if len(syms) == 0 {
					return fmt.Errorf("watchlist %q is empty; add symbols with `watchlist add %s TICKER`", name, name)
				}
			} else {
				// dry-run with no symbols flag: use a placeholder so the request preview is meaningful
				syms = []string{"AAPL"}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if flags.dryRun {
				_, _ = fetchQuotes(c, syms)
				return nil
			}
			quotes, err := fetchQuotes(c, syms)
			if err != nil {
				return err
			}
			sort.Slice(quotes, func(i, j int) bool {
				return quotes[i].RegularChangePct > quotes[j].RegularChangePct
			})
			var gainers, losers []quoteRow
			for _, q := range quotes {
				if q.RegularChangePct >= 0 {
					gainers = append(gainers, q)
				} else {
					losers = append(losers, q)
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"symbols": syms,
					"gainers": gainers,
					"losers":  losers,
					"updated": time.Now().Format(time.RFC3339),
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "== Biggest gainers ==")
			if len(gainers) == 0 {
				fmt.Fprintln(w, "  (none)")
			} else {
				top := gainers
				if len(top) > 5 {
					top = top[:5]
				}
				_ = flags.printTable(cmd, []string{"SYMBOL", "PRICE", "CHG%", "NAME"}, quoteRowsCompact(top))
			}
			fmt.Fprintln(w)
			fmt.Fprintln(w, "== Biggest losers ==")
			if len(losers) == 0 {
				fmt.Fprintln(w, "  (none)")
			} else {
				// losers sorted ascending by change% for display
				sort.Slice(losers, func(i, j int) bool { return losers[i].RegularChangePct < losers[j].RegularChangePct })
				bottom := losers
				if len(bottom) > 5 {
					bottom = bottom[:5]
				}
				_ = flags.printTable(cmd, []string{"SYMBOL", "PRICE", "CHG%", "NAME"}, quoteRowsCompact(bottom))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&watchlistName, "watchlist", "", "Watchlist name (default: 'default')")
	cmd.Flags().StringVar(&symbolsFlag, "symbols", "", "Comma-separated tickers (alternative to --watchlist)")
	return cmd
}

func quoteRowsCompact(qs []quoteRow) [][]string {
	out := make([][]string, len(qs))
	for i, q := range qs {
		out[i] = []string{q.Symbol, fmt.Sprintf("%.2f", q.RegularPrice), fmt.Sprintf("%+.2f%%", q.RegularChangePct), q.ShortName}
	}
	return out
}

// ---------------------------------------------------------------------------
// compare — side-by-side fundamentals across symbols
// ---------------------------------------------------------------------------

func newCompareCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "compare <symbol> <symbol> [symbol...]",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Side-by-side quote comparison for 2+ symbols",
		Args:    cobra.MinimumNArgs(2),
		Example: "  yahoo-finance-pp-cli compare AAPL MSFT GOOG NVDA",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			syms := make([]string, len(args))
			for i, s := range args {
				syms[i] = strings.ToUpper(s)
			}
			quotes, err := fetchQuotes(c, syms)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return flags.printJSON(cmd, quotes)
			}
			headers := []string{"SYMBOL", "PRICE", "CHG%", "52W LOW", "52W HIGH", "MKT CAP", "NAME"}
			rows := make([][]string, 0, len(quotes))
			for _, q := range quotes {
				rows = append(rows, []string{
					q.Symbol,
					fmt.Sprintf("%.2f", q.RegularPrice),
					fmt.Sprintf("%+.2f%%", q.RegularChangePct),
					fmt.Sprintf("%.2f", q.FiftyTwoWeekLow),
					fmt.Sprintf("%.2f", q.FiftyTwoWeekHigh),
					humanMarketCap(q.MarketCap),
					q.ShortName,
				})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
}

func humanMarketCap(v float64) string {
	switch {
	case v >= 1e12:
		return fmt.Sprintf("%.2fT", v/1e12)
	case v >= 1e9:
		return fmt.Sprintf("%.2fB", v/1e9)
	case v >= 1e6:
		return fmt.Sprintf("%.2fM", v/1e6)
	default:
		return fmt.Sprintf("%.0f", v)
	}
}

// ---------------------------------------------------------------------------
// sparkline — terminal-rendered chart from live or cached history
// ---------------------------------------------------------------------------

func newSparklineCmd(flags *rootFlags) *cobra.Command {
	var rng string
	var interval string
	cmd := &cobra.Command{
		Use:     "sparkline <symbol>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Unicode sparkline of a symbol's recent price action",
		Args:    cobra.ExactArgs(1),
		Example: "  yahoo-finance-pp-cli sparkline AAPL --range 3mo",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/v8/finance/chart/"+strings.ToUpper(args[0]), map[string]string{
				"range":    rng,
				"interval": interval,
			})
			if err != nil {
				return err
			}
			if flags.dryRun {
				return nil
			}
			var env struct {
				Chart struct {
					Result []struct {
						Meta struct {
							Symbol             string  `json:"symbol"`
							RegularMarketPrice float64 `json:"regularMarketPrice"`
						} `json:"meta"`
						Indicators struct {
							Quote []struct {
								Close []float64 `json:"close"`
							} `json:"quote"`
						} `json:"indicators"`
					} `json:"result"`
				} `json:"chart"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return err
			}
			if len(env.Chart.Result) == 0 || len(env.Chart.Result[0].Indicators.Quote) == 0 {
				return fmt.Errorf("no chart data returned")
			}
			closes := env.Chart.Result[0].Indicators.Quote[0].Close
			sym := env.Chart.Result[0].Meta.Symbol
			clean := make([]float64, 0, len(closes))
			for _, c := range closes {
				if c > 0 {
					clean = append(clean, c)
				}
			}
			spark := renderSparkline(clean)
			last := 0.0
			if len(clean) > 0 {
				last = clean[len(clean)-1]
			}
			first := 0.0
			if len(clean) > 0 {
				first = clean[0]
			}
			chg := 0.0
			if first > 0 {
				chg = (last - first) / first * 100
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s %s %s  %.2f → %.2f (%+.2f%%)\n", sym, rng, spark, first, last, chg)
			return nil
		},
	}
	cmd.Flags().StringVar(&rng, "range", "1mo", "Range: 5d, 1mo, 3mo, 6mo, 1y, 2y, 5y")
	cmd.Flags().StringVar(&interval, "interval", "1d", "Bar interval")
	return cmd
}

func renderSparkline(data []float64) string {
	if len(data) == 0 {
		return ""
	}
	runes := []rune("▁▂▃▄▅▆▇█")
	min, max := data[0], data[0]
	for _, v := range data {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	span := max - min
	if span <= 0 {
		return strings.Repeat(string(runes[3]), len(data))
	}
	var b strings.Builder
	for _, v := range data {
		idx := int((v - min) / span * float64(len(runes)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(runes) {
			idx = len(runes) - 1
		}
		b.WriteRune(runes[idx])
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// sql — direct SQLite query shell
// ---------------------------------------------------------------------------

func newSQLCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "sql <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Run a raw SQL query against the local database",
		Args:    cobra.ExactArgs(1),
		Example: `  yahoo-finance-pp-cli sql "SELECT symbol, COUNT(*) FROM watchlist_members GROUP BY symbol"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := openDB(flags)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := db.Query(args[0])
			if err != nil {
				return fmt.Errorf("sql: %w", err)
			}
			defer rows.Close()
			cols, err := rows.Columns()
			if err != nil {
				return err
			}
			var out [][]any
			for rows.Next() {
				raw := make([]any, len(cols))
				ptrs := make([]any, len(cols))
				for i := range raw {
					ptrs[i] = &raw[i]
				}
				if err := rows.Scan(ptrs...); err != nil {
					return err
				}
				out = append(out, raw)
			}
			if flags.asJSON {
				result := make([]map[string]any, 0, len(out))
				for _, r := range out {
					m := map[string]any{}
					for i, c := range cols {
						m[c] = r[i]
					}
					result = append(result, m)
				}
				return flags.printJSON(cmd, result)
			}
			rowsOut := make([][]string, 0, len(out))
			for _, r := range out {
				strs := make([]string, len(r))
				for i, v := range r {
					if v == nil {
						strs[i] = ""
					} else {
						strs[i] = fmt.Sprintf("%v", v)
					}
				}
				rowsOut = append(rowsOut, strs)
			}
			return flags.printTable(cmd, cols, rowsOut)
		},
	}
}

// ---------------------------------------------------------------------------
// fx — quick currency conversion via chart data
// ---------------------------------------------------------------------------

func newFXCmd(flags *rootFlags) *cobra.Command {
	var amount float64
	cmd := &cobra.Command{
		Use:     "fx <from> <to>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Quick currency conversion using Yahoo Finance FX pairs",
		Args:    cobra.ExactArgs(2),
		Example: "  yahoo-finance-pp-cli fx USD EUR --amount 100",
		RunE: func(cmd *cobra.Command, args []string) error {
			from := strings.ToUpper(args[0])
			to := strings.ToUpper(args[1])
			pair := from + to + "=X"
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if flags.dryRun {
				_, _ = fetchQuotes(c, []string{pair})
				return nil
			}
			quotes, err := fetchQuotes(c, []string{pair})
			if err != nil {
				return err
			}
			if len(quotes) == 0 {
				return fmt.Errorf("no rate for %s", pair)
			}
			rate := quotes[0].RegularPrice
			converted := amount * rate
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"from": from, "to": to, "rate": rate,
					"amount": amount, "converted": converted,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%.2f %s = %.2f %s  (rate %.6f)\n", amount, from, converted, to, rate)
			return nil
		},
	}
	cmd.Flags().Float64Var(&amount, "amount", 1.0, "Amount to convert (default 1)")
	return cmd
}

// ---------------------------------------------------------------------------
// options moneyness filter — live chain with ATM/OTM/ITM filtering
// ---------------------------------------------------------------------------

func newOptionsChainCmd(flags *rootFlags) *cobra.Command {
	var moneyness string
	var optType string
	var maxDTE int
	var minStrike, maxStrike float64
	cmd := &cobra.Command{
		Use:     "options-chain <symbol>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:   "Options chain with moneyness, DTE, and strike filters",
		Args:    cobra.ExactArgs(1),
		Example: `  yahoo-finance-pp-cli options-chain AAPL --moneyness otm --max-dte 45 --type calls`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/v7/finance/options/"+strings.ToUpper(args[0]), nil)
			if err != nil {
				return err
			}
			if flags.dryRun {
				return nil
			}
			var env struct {
				OptionChain struct {
					Result []struct {
						UnderlyingSymbol string `json:"underlyingSymbol"`
						Quote            struct {
							RegularMarketPrice float64 `json:"regularMarketPrice"`
						} `json:"quote"`
						Options []struct {
							ExpirationDate int64            `json:"expirationDate"`
							Calls          []optionContract `json:"calls"`
							Puts           []optionContract `json:"puts"`
						} `json:"options"`
					} `json:"result"`
				} `json:"optionChain"`
			}
			if err := json.Unmarshal(data, &env); err != nil {
				return err
			}
			if len(env.OptionChain.Result) == 0 {
				return fmt.Errorf("no option data for %s", args[0])
			}
			r := env.OptionChain.Result[0]
			spot := r.Quote.RegularMarketPrice
			now := time.Now()
			type row struct {
				Type         string  `json:"type"`
				Expiration   string  `json:"expiration"`
				DaysToExpiry int     `json:"dte"`
				Strike       float64 `json:"strike"`
				Last         float64 `json:"last"`
				Bid          float64 `json:"bid"`
				Ask          float64 `json:"ask"`
				Volume       int64   `json:"volume"`
				OpenInterest int64   `json:"open_interest"`
				ImpliedVol   float64 `json:"implied_volatility"`
				Moneyness    string  `json:"moneyness"`
			}
			var out []row
			for _, exp := range r.Options {
				expT := time.Unix(exp.ExpirationDate, 0).UTC()
				dte := int(expT.Sub(now).Hours() / 24)
				if maxDTE > 0 && dte > maxDTE {
					continue
				}
				var contracts []struct {
					typeLabel string
					list      []optionContract
				}
				if optType == "" || optType == "calls" || optType == "both" {
					contracts = append(contracts, struct {
						typeLabel string
						list      []optionContract
					}{"call", exp.Calls})
				}
				if optType == "" || optType == "puts" || optType == "both" {
					contracts = append(contracts, struct {
						typeLabel string
						list      []optionContract
					}{"put", exp.Puts})
				}
				for _, bucket := range contracts {
					for _, oc := range bucket.list {
						if minStrike > 0 && oc.Strike < minStrike {
							continue
						}
						if maxStrike > 0 && oc.Strike > maxStrike {
							continue
						}
						m := classifyMoneyness(bucket.typeLabel, oc.Strike, spot)
						if moneyness != "" && moneyness != m && moneyness != "all" {
							continue
						}
						out = append(out, row{
							Type:         bucket.typeLabel,
							Expiration:   expT.Format("2006-01-02"),
							DaysToExpiry: dte,
							Strike:       oc.Strike,
							Last:         oc.LastPrice,
							Bid:          oc.Bid,
							Ask:          oc.Ask,
							Volume:       oc.Volume,
							OpenInterest: oc.OpenInterest,
							ImpliedVol:   oc.ImpliedVolatility,
							Moneyness:    m,
						})
					}
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"symbol":    r.UnderlyingSymbol,
					"spot":      spot,
					"contracts": out,
				})
			}
			headers := []string{"TYPE", "EXPIRES", "DTE", "STRIKE", "LAST", "BID", "ASK", "VOL", "OI", "IV", "M"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					r.Type,
					r.Expiration,
					strconv.Itoa(r.DaysToExpiry),
					fmt.Sprintf("%.2f", r.Strike),
					fmt.Sprintf("%.2f", r.Last),
					fmt.Sprintf("%.2f", r.Bid),
					fmt.Sprintf("%.2f", r.Ask),
					strconv.FormatInt(r.Volume, 10),
					strconv.FormatInt(r.OpenInterest, 10),
					fmt.Sprintf("%.3f", r.ImpliedVol),
					r.Moneyness,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s @ %.2f (%d contracts)\n", r.UnderlyingSymbol, spot, len(out))
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&moneyness, "moneyness", "", "Filter: itm, atm, otm, all")
	cmd.Flags().StringVar(&optType, "type", "", "calls, puts, or both (default both)")
	cmd.Flags().IntVar(&maxDTE, "max-dte", 0, "Max days to expiration (0 = all)")
	cmd.Flags().Float64Var(&minStrike, "min-strike", 0, "Min strike filter")
	cmd.Flags().Float64Var(&maxStrike, "max-strike", 0, "Max strike filter")
	return cmd
}

type optionContract struct {
	Strike            float64 `json:"strike"`
	LastPrice         float64 `json:"lastPrice"`
	Bid               float64 `json:"bid"`
	Ask               float64 `json:"ask"`
	Volume            int64   `json:"volume"`
	OpenInterest      int64   `json:"openInterest"`
	ImpliedVolatility float64 `json:"impliedVolatility"`
}

func classifyMoneyness(optType string, strike, spot float64) string {
	if spot <= 0 {
		return "unknown"
	}
	diff := (strike - spot) / spot
	if diff > -0.02 && diff < 0.02 {
		return "atm"
	}
	if optType == "call" {
		if strike < spot {
			return "itm"
		}
		return "otm"
	}
	// put
	if strike > spot {
		return "itm"
	}
	return "otm"
}

// ---------------------------------------------------------------------------
// auth login-chrome imports a browser session when Yahoo blocks the automatic
// crumb bootstrap from the current IP.
// ---------------------------------------------------------------------------

// chromeLoginEnabled signals whether the Chrome cookie import path is compiled
// in. Left false here; the command still prints instructions to achieve the
// same effect manually (paste session.json from a browser extension).

func newAuthChromeCmd(flags *rootFlags) *cobra.Command {
	var cookiesFile string
	var crumb string
	cmd := &cobra.Command{
		Use:   "login-chrome",
		Short: "Import a Chrome session (cookies + crumb) when the server-side crumb handshake is rate-limited",
		Long: `When Yahoo rate-limits this machine's IP (curl returns HTTP 429 on every endpoint),
import the session from a browser that can reach finance.yahoo.com normally.

1. Visit finance.yahoo.com in Chrome and accept cookies.
2. Use a browser extension (e.g., "Get cookies.txt LOCALLY") to export cookies for *.yahoo.com.
3. Convert to JSON: [{"name":"A1","value":"...","domain":".yahoo.com","path":"/"},...]
4. Run: yahoo-finance-pp-cli auth login-chrome --cookies session.json --crumb <crumb>

Get the crumb from the browser DevTools: open finance.yahoo.com, run in console:
  fetch('/v1/test/getcrumb').then(r => r.text()).then(console.log)`,
		Example: `  yahoo-finance-pp-cli auth login-chrome --cookies ~/Downloads/yahoo-cookies.json --crumb abc123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cookiesFile == "" {
				return fmt.Errorf("--cookies is required (see --help for how to capture)")
			}
			data, err := os.ReadFile(cookiesFile)
			if err != nil {
				return fmt.Errorf("reading cookies file: %w", err)
			}
			var raw []struct {
				Name   string `json:"name"`
				Value  string `json:"value"`
				Path   string `json:"path"`
				Domain string `json:"domain"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				return fmt.Errorf("parsing cookies: %w", err)
			}
			var cookies []*http.Cookie
			for _, r := range raw {
				cookies = append(cookies, &http.Cookie{Name: r.Name, Value: r.Value, Path: r.Path})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := c.SetChromeCookies(cookies, crumb); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "imported %d cookies; crumb set\n", len(cookies))
			return nil
		},
	}
	cmd.Flags().StringVar(&cookiesFile, "cookies", "", "Path to JSON file with exported Chrome cookies for *.yahoo.com (required)")
	cmd.Flags().StringVar(&crumb, "crumb", "", "Crumb string from fetch('/v1/test/getcrumb') in Yahoo Finance DevTools")
	return cmd
}
