// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: unified forward catalyst agenda. Unions local earnings,
// dividends, splits, IPOs, FDA, conference-calls, guidance, and offerings rows
// into one forward-dated agenda keyed by (date, ticker).
//
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type catalystEvent struct {
	Date   string `json:"date"`
	Ticker string `json:"ticker"`
	Type   string `json:"type"`
	Detail string `json:"detail,omitempty"`
	date   time.Time
}

// catalystResources maps a syncable calendar resource_type to its agenda label.
var catalystResources = []struct{ rt, label string }{
	{"calendar-earnings", "earnings"},
	{"calendar-dividends", "dividend"},
	{"calendar-splits", "split"},
	{"calendar-ipos", "ipo"},
	{"calendar-fda", "fda"},
	{"calendar-conference-calls", "conference-call"},
	{"calendar-guidance", "guidance"},
	{"calendar-offerings", "offering"},
}

func newNovelCatalystsCmd(flags *rootFlags) *cobra.Command {
	var (
		flagAhead   string
		flagTickers string
		flagLimit   int
		dbPath      string
	)

	cmd := &cobra.Command{
		Use:   "catalysts [tickers]",
		Short: "One forward-dated agenda per ticker set unioning earnings, dividends, splits, IPOs, FDA, calls, guidance",
		Long: strings.Trim(`
Unions the forward-dated rows of every synced calendar family into one ordered
agenda for a ticker set, so you see every upcoming dated event in one list
instead of querying eight calendar endpoints.

Use this for upcoming dated events across calendar families. Do NOT use it for
past changes (use 'watch') or computed earnings beat/miss (use 'earnings-season').

Reads the local SQLite mirror. Run 'sync' first:
  benzinga-pp-cli sync --resources calendar-earnings,calendar-dividends,calendar-splits,calendar-ipos,calendar-fda,calendar-conference-calls,calendar-guidance --since 0d
`, "\n"),
		Example: "  benzinga-pp-cli catalysts AAPL,LLY,MRNA --ahead 14d --agent",
		// An unknown ticker yields an empty agenda (exit 0), not a usage error,
		// so the dogfood error-path probe must not expect a non-zero exit.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			ahead, err := novelDur(flagAhead, 14*24*time.Hour)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --ahead: %w", err))
			}
			now := time.Now()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			horizon := today.Add(ahead)
			tickers := novelTickerSet(args, flagTickers)
			inSet := func(t string) bool { return len(tickers) == 0 || tickers[t] }

			db, ok, err := novelStore(cmd, flags, dbPath,
				"benzinga-pp-cli sync --resources calendar-earnings,calendar-dividends,calendar-splits,calendar-ipos,calendar-fda,calendar-conference-calls,calendar-guidance --since 0d")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "", flags.maxAge)

			events := make([]catalystEvent, 0)
			for _, cr := range catalystResources {
				rows, err := novelRows(cmd.Context(), db, cr.rt)
				if err != nil {
					return fmt.Errorf("querying %s: %w", cr.rt, err)
				}
				for _, m := range rows {
					dateStr := novelStr(m, "date")
					if dateStr == "" {
						continue
					}
					d, perr := time.Parse("2006-01-02", dateStr)
					if perr != nil {
						continue
					}
					if d.Before(today) || d.After(horizon) {
						continue
					}
					ticker := normTicker(novelStr(m, "ticker"))
					if ticker == "" {
						ticker = normTicker(catalystNestedTicker(m))
					}
					if ticker == "" || !inSet(ticker) {
						continue
					}
					events = append(events, catalystEvent{
						Date:   dateStr,
						Ticker: ticker,
						Type:   cr.label,
						Detail: catalystDetail(cr.label, m),
						date:   d,
					})
				}
			}

			sort.SliceStable(events, func(i, j int) bool {
				if events[i].date.Equal(events[j].date) {
					return events[i].Ticker < events[j].Ticker
				}
				return events[i].date.Before(events[j].date)
			})
			if flagLimit > 0 && len(events) > flagLimit {
				events = events[:flagLimit]
			}

			return novelEmit(cmd, flags, events, func() {
				out := cmd.OutOrStdout()
				if len(events) == 0 {
					fmt.Fprintln(out, "No upcoming catalysts found. Try a wider --ahead, more tickers, or run sync.")
					return
				}
				fmt.Fprintln(out, "DATE\tTICKER\tTYPE\tDETAIL")
				for _, e := range events {
					fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", e.Date, e.Ticker, e.Type, e.Detail)
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagAhead, "ahead", "14d", "How far ahead to look for catalysts (e.g. 7d, 14d, 1w)")
	cmd.Flags().StringVar(&flagTickers, "tickers", "", "Comma-separated tickers (alternative to positional arg)")
	cmd.Flags().IntVar(&flagLimit, "limit", 100, "Maximum catalysts to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// catalystNestedTicker pulls a ticker out of the nested companies[] array used by
// FDA calendar rows.
func catalystNestedTicker(m map[string]any) string {
	if arr, ok := m["companies"].([]any); ok && len(arr) > 0 {
		if mm, ok := arr[0].(map[string]any); ok {
			return novelStr(mm, "ticker")
		}
	}
	return ""
}

func catalystDetail(label string, m map[string]any) string {
	switch label {
	case "earnings":
		return strings.TrimSpace(fmt.Sprintf("%s %d EPS est %s",
			novelStr(m, "period"), int(mustFloat(m, "period_year")), novelStr(m, "eps_est")))
	case "dividend":
		return strings.TrimSpace("amount " + novelStr(m, "dividend") + " ex " + novelStr(m, "ex_dividend_date"))
	case "conference-call":
		return strings.TrimSpace(novelStr(m, "period") + " call " + novelStr(m, "start_time"))
	case "ipo":
		return strings.TrimSpace(novelStr(m, "deal_status") + " " + novelStr(m, "price_min") + "-" + novelStr(m, "price_max"))
	case "fda":
		return strings.TrimSpace(novelStr(m, "event_type") + " " + novelStr(m, "drug"))
	default:
		return strings.TrimSpace(novelStr(m, "name"))
	}
}

func mustFloat(m map[string]any, key string) float64 {
	f, _ := novelFloat(m, key)
	return f
}
