// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: insiders net-buying. Surfaces symbols where insiders are
// net buyers in a recent window, drawing from locally synced insider rows.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/cliutil"
	"github.com/spf13/cobra"
)

// newInsidersExtCmd is a parent command holding the `net-buying` subcommand.
// Named with "Ext" suffix to avoid collision with any generator-emitted
// `newInsidersCmd` that may appear in future templates.
func newInsidersExtCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "insiders-net-buying",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Surface symbols where insiders are net buyers in a recent window",
	}
	// The leaf is exposed at the top of help text. Keep the parent
	// invocable directly so `insiders-net-buying` works as a single command.
	leaf := newInsidersNetBuyingLeafCmd(flags)
	cmd.RunE = leaf.RunE
	cmd.Flags().AddFlagSet(leaf.Flags())
	cmd.Long = leaf.Long
	cmd.Example = leaf.Example
	return cmd
}

func newInsidersNetBuyingLeafCmd(flags *rootFlags) *cobra.Command {
	var recent string
	var watchlist string
	var all bool
	var limit int
	var dbPath string
	cmd := &cobra.Command{
		Use:         "net-buying",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Companies with positive insider net-shares (requires --all, --watchlist, or seeded watchlist_members)",
		Long: strings.Trim(`
For each symbol in the selected universe, compute net_shares = sum(buys) -
sum(sells) over the lookback window and rank by net positive (sells-outweigh-buys
symbols are omitted so the list focuses on buying activity).

Symbol-source selection (in priority order):

  --watchlist <name>   Restrict to symbols in that named watchlist.
  --all                Scan every symbol with locally synced insider data.
  (neither flag set)   Fall back to every symbol in the watchlist_members
                       table across all watchlists. On a fresh install with no
                       watchlists yet this set is empty and the command will
                       emit "hint: no symbols to scan" — pass --all or
                       --watchlist to escape that case.

Insider rows must already be synced locally via the spec endpoints; this
command reads from the resources table and does not call the live API.
`, "\n"),
		Example: strings.Trim(`
  yahoo-finance-pp-cli insiders-net-buying --recent 30d --all
  yahoo-finance-pp-cli insiders-net-buying --watchlist tech --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			window, err := cliutil.ParseDurationLoose(recent)
			if err != nil {
				return fmt.Errorf("invalid --recent %q: %w", recent, err)
			}
			since := time.Now().Add(-window)

			db, err := openInsidersDB(flags, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			symbols, err := chooseInsiderSymbols(db, watchlist, all)
			if err != nil {
				return err
			}
			if len(symbols) == 0 && cmd != nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "hint: no symbols to scan (pass --all or --watchlist <name>)")
				if flags.asJSON {
					return flags.printJSON(cmd, []insiderNetRow{})
				}
				return nil
			}

			windowDays := int(window.Hours() / 24)
			if windowDays < 1 {
				windowDays = 1
			}
			var out []insiderNetRow
			for _, sym := range symbols {
				net, buys, sells := scanInsiderActivity(db, sym, since)
				if net <= 0 {
					continue
				}
				out = append(out, insiderNetRow{
					Symbol:     sym,
					NetShares:  net,
					BuyCount:   buys,
					SellCount:  sells,
					WindowDays: windowDays,
				})
			}
			// Sort net descending
			sortByNet(out)
			if limit > 0 && len(out) > limit {
				out = out[:limit]
			}
			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			headers := []string{"SYMBOL", "NET_SHARES", "BUYS", "SELLS", "WINDOW_D"}
			table := make([][]string, 0, len(out))
			for _, r := range out {
				table = append(table, []string{
					r.Symbol,
					fmt.Sprintf("%d", r.NetShares),
					fmt.Sprintf("%d", r.BuyCount),
					fmt.Sprintf("%d", r.SellCount),
					fmt.Sprintf("%d", r.WindowDays),
				})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&recent, "recent", "30d", "Lookback window (e.g. 30d, 90d, 6mo)")
	cmd.Flags().StringVar(&watchlist, "watchlist", "", "Restrict to a watchlist (default: union of watchlist_members across all watchlists; pass --all to scan every synced symbol)")
	cmd.Flags().BoolVar(&all, "all", false, "Scan every symbol with locally synced insider rows (overrides --watchlist)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

type insiderNetRow struct {
	Symbol     string `json:"symbol"`
	NetShares  int64  `json:"net_shares"`
	BuyCount   int    `json:"buy_count"`
	SellCount  int    `json:"sell_count"`
	WindowDays int    `json:"window_days"`
}

// openInsidersDB returns a *sql.DB. Always ensures both the transcendence
// schema and a minimal resources table exist, so the insider scan never
// errors on a fresh CLI.
func openInsidersDB(flags *rootFlags, dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		return openDB(flags)
	}
	db, err := sql.Open("sqlite", dbPath+"?_foreign_keys=ON")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(transcendenceSchema); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS resources (
		id TEXT NOT NULL,
		resource_type TEXT NOT NULL,
		data TEXT,
		synced_at DATETIME,
		updated_at DATETIME,
		PRIMARY KEY (resource_type, id)
	)`); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// chooseInsiderSymbols returns the symbol list to scan based on user flags.
// Priority: --watchlist wins; --all hits every symbol with an insider row;
// neither defaults to all watched symbols.
func chooseInsiderSymbols(db *sql.DB, watchlist string, all bool) ([]string, error) {
	if all {
		rows, err := db.Query(`SELECT DISTINCT
			CASE WHEN INSTR(id, ':') > 0 THEN SUBSTR(id, 1, INSTR(id, ':') - 1) ELSE id END
			FROM resources WHERE resource_type LIKE 'insider%'`)
		if err != nil {
			if strings.Contains(err.Error(), "no such table") {
				return nil, nil
			}
			return nil, err
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err == nil {
				out = append(out, strings.ToUpper(s))
			}
		}
		return out, nil
	}
	if watchlist != "" {
		return watchlistSymbols(db, watchlist)
	}
	// All watched across all watchlists
	rows, err := db.Query(`SELECT DISTINCT symbol FROM watchlist_members ORDER BY symbol`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err == nil {
			out = append(out, s)
		}
	}
	return out, nil
}

// scanInsiderActivity reads insider rows for a symbol and computes
// net = sum(buys) - sum(sells). Buy classification: transactionType
// contains "buy"/"purchase"/"acquire" (case-insensitive). Sell
// classification: contains "sell"/"dispose".
//
// Each row's data column is JSON. Two shapes supported:
//  1. Single object: {"date":"2026-02-09","transactionType":"Buy","shares":1000}
//  2. Array:        [{"date":"...","transactionType":"Buy","shares":1000}, ...]
func scanInsiderActivity(db *sql.DB, symbol string, since time.Time) (int64, int, int) {
	rows, err := db.Query(`SELECT data FROM resources
		WHERE resource_type IN ('insider_transactions','insider_purchases')
		AND (id LIKE ? || ':%' OR id = ?)`,
		strings.ToUpper(symbol), strings.ToUpper(symbol))
	if err != nil {
		return 0, 0, 0
	}
	defer rows.Close()
	var net int64
	var buys, sells int
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if raw[0] == '[' {
			var arr []struct {
				Date            string `json:"date"`
				TransactionType string `json:"transactionType"`
				Shares          int64  `json:"shares"`
			}
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				for _, e := range arr {
					if t := parseInsiderDate(e.Date); !t.Before(since) {
						kind := classifyInsiderTxn(e.TransactionType)
						switch kind {
						case 1:
							net += e.Shares
							buys++
						case -1:
							net -= e.Shares
							sells++
						}
					}
				}
			}
		} else {
			var single struct {
				Date            string `json:"date"`
				TransactionType string `json:"transactionType"`
				Shares          int64  `json:"shares"`
			}
			if err := json.Unmarshal([]byte(raw), &single); err == nil {
				if t := parseInsiderDate(single.Date); !t.Before(since) {
					kind := classifyInsiderTxn(single.TransactionType)
					switch kind {
					case 1:
						net += single.Shares
						buys++
					case -1:
						net -= single.Shares
						sells++
					}
				}
			}
		}
	}
	return net, buys, sells
}

// classifyInsiderTxn returns +1 for buy-like, -1 for sell-like, 0 for unknown.
func classifyInsiderTxn(s string) int {
	l := strings.ToLower(s)
	switch {
	case strings.Contains(l, "buy"), strings.Contains(l, "purchase"), strings.Contains(l, "acquire"):
		return 1
	case strings.Contains(l, "sell"), strings.Contains(l, "dispose"):
		return -1
	}
	return 0
}

// parseInsiderDate tolerates YYYY-MM-DD and RFC3339. Returns zero time
// when nothing matches so the !Before(since) gate rejects mystery dates.
func parseInsiderDate(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t
	}
	return time.Time{}
}

// sortByNet sorts the slice by NetShares descending (ties: symbol asc).
func sortByNet(out []insiderNetRow) {
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].NetShares != out[j].NetShares {
			return out[i].NetShares > out[j].NetShares
		}
		return out[i].Symbol < out[j].Symbol
	})
}
