// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: watchlist correlate. Pairwise Pearson correlation across
// the symbols in a named watchlist over a given range.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/yahoo-finance/internal/portstats"
	"github.com/spf13/cobra"
)

type correlationTriple struct {
	A           string  `json:"a"`
	B           string  `json:"b"`
	Correlation float64 `json:"correlation"`
}

type correlationPayload struct {
	Watchlist string              `json:"watchlist"`
	Range     string              `json:"range"`
	Triples   []correlationTriple `json:"triples"`
	Note      string              `json:"note,omitempty"`
}

func newWatchlistCorrelateCmd(flags *rootFlags) *cobra.Command {
	var rng string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "correlate <name>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "Pairwise Pearson correlation across symbols in a watchlist",
		Long: strings.Trim(`
Pulls cached daily closes from the local resources table (resource_type='history')
over the given range and computes pairwise Pearson correlation across
each pair of symbols on the named watchlist.

Returns a flat array of {a, b, correlation} triples (upper triangular).
Identical-series and unequal-length series degrade gracefully (correlation=0).
`, "\n"),
		Example: strings.Trim(`
  yahoo-finance-pp-cli watchlist correlate tech --range 6m
  yahoo-finance-pp-cli watchlist correlate tech --json
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			window, err := cliutil.ParseDurationLoose(rng)
			if err != nil {
				return fmt.Errorf("invalid --range %q: %w", rng, err)
			}
			since := time.Now().Add(-window)

			db, err := openDividendsDB(flags, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()

			symbols, err := watchlistSymbols(db, args[0])
			if err != nil {
				return err
			}
			payload := correlationPayload{
				Watchlist: args[0],
				Range:     rng,
				Triples:   []correlationTriple{},
			}
			if len(symbols) < 2 {
				payload.Note = "watchlist needs at least 2 symbols to correlate"
				if flags.asJSON {
					return flags.printJSON(cmd, payload)
				}
				fmt.Fprintln(cmd.OutOrStdout(), payload.Note)
				return nil
			}

			closesBySymbol := map[string][]float64{}
			for _, sym := range symbols {
				closesBySymbol[sym] = readHistoryCloses(db, sym, since)
			}
			// Pairwise upper-triangular.
			ordered := append([]string(nil), symbols...)
			sort.Strings(ordered)
			for i := 0; i < len(ordered); i++ {
				for j := i + 1; j < len(ordered); j++ {
					a, b := ordered[i], ordered[j]
					ra, rb := portstats.PairedReturns(closesBySymbol[a], closesBySymbol[b])
					if len(ra) < 2 {
						payload.Triples = append(payload.Triples, correlationTriple{A: a, B: b, Correlation: 0})
						continue
					}
					r, err := portstats.Pearson(ra, rb)
					if err != nil {
						payload.Triples = append(payload.Triples, correlationTriple{A: a, B: b, Correlation: 0})
						continue
					}
					payload.Triples = append(payload.Triples, correlationTriple{A: a, B: b, Correlation: r})
				}
			}
			if flags.asJSON {
				return flags.printJSON(cmd, payload)
			}
			headers := []string{"A", "B", "CORRELATION"}
			table := make([][]string, 0, len(payload.Triples))
			for _, t := range payload.Triples {
				table = append(table, []string{t.A, t.B, fmt.Sprintf("%.4f", t.Correlation)})
			}
			return flags.printTable(cmd, headers, table)
		},
	}
	cmd.Flags().StringVar(&rng, "range", "180d", "Lookback window (e.g. 30d, 90d, 180d, 52w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local SQLite path")
	return cmd
}

// readHistoryCloses pulls daily closes for a symbol from the local
// resources table where resource_type='history', filtered to rows whose
// date column is >= since. Each row's data is either a single OHLCV
// object or an array of them.
func readHistoryCloses(db *sql.DB, symbol string, since time.Time) []float64 {
	rows, err := db.Query(`SELECT data FROM resources
		WHERE resource_type='history' AND (id LIKE ? || ':%' OR id = ?)
		ORDER BY id`,
		strings.ToUpper(symbol), strings.ToUpper(symbol))
	if err != nil {
		return nil
	}
	defer rows.Close()
	type ohlcv struct {
		Date  string  `json:"date"`
		Close float64 `json:"close"`
	}
	type dated struct {
		t     time.Time
		close float64
	}
	var all []dated
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
			var arr []ohlcv
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				for _, e := range arr {
					if t := parseInsiderDate(e.Date); !t.IsZero() && !t.Before(since) {
						all = append(all, dated{t: t, close: e.Close})
					}
				}
			}
		} else {
			var e ohlcv
			if err := json.Unmarshal([]byte(raw), &e); err == nil {
				if t := parseInsiderDate(e.Date); !t.IsZero() && !t.Before(since) {
					all = append(all, dated{t: t, close: e.Close})
				}
			}
		}
	}
	// Sort by date ascending so the returns series is chronological.
	sort.Slice(all, func(i, j int) bool { return all[i].t.Before(all[j].t) })
	out := make([]float64, len(all))
	for i, d := range all {
		out[i] = d.close
	}
	return out
}
