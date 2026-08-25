// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: earnings surprise tracker. Computes EPS/revenue beat-miss and
// surprise % from local earnings rows, joins conference-call availability, and
// ranks by surprise magnitude.
//
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type earningsRow struct {
	Ticker         string  `json:"ticker"`
	Name           string  `json:"name"`
	Date           string  `json:"date"`
	Period         string  `json:"period"`
	EPS            string  `json:"eps"`
	EPSEst         string  `json:"eps_est"`
	EPSSurprisePct float64 `json:"eps_surprise_pct"`
	RevSurprisePct float64 `json:"revenue_surprise_pct"`
	Beat           bool    `json:"beat"`
	HasCall        bool    `json:"has_conference_call"`
	absSurprise    float64
}

func newNovelEarningsSeasonCmd(flags *rootFlags) *cobra.Command {
	var (
		flagFrom    string
		flagTo      string
		flagTickers string
		flagLimit   int
		dbPath      string
	)

	cmd := &cobra.Command{
		Use:   "earnings-season",
		Short: "Compute EPS/revenue beat-miss and surprise % from the earnings calendar, ranked, with call links",
		Long: strings.Trim(`
For an earnings window, computes EPS beat/miss and surprise % from reported rows
(those with actual EPS), joins each name to conference-call availability, and
ranks by surprise magnitude.

Use this for retrospective beat/miss and surprise ranking. Do NOT use it for the
forward earnings schedule — use 'catalysts' for upcoming dates.

Reads the local SQLite mirror. Run 'sync' first:
  benzinga-pp-cli sync --resources calendar-earnings,calendar-conference-calls --since 30d
`, "\n"),
		Example:     "  benzinga-pp-cli earnings-season --from 7d --agent --select ticker,eps_surprise_pct,beat",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			from, err := novelDur(flagFrom, 7*24*time.Hour)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --from: %w", err))
			}
			to, err := novelDur(flagTo, 7*24*time.Hour)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --to: %w", err))
			}
			now := time.Now()
			lo := now.Add(-from)
			hi := now.Add(to)
			tickers := novelTickerSet(args, flagTickers)
			inSet := func(t string) bool { return len(tickers) == 0 || tickers[t] }

			db, ok, err := novelStore(cmd, flags, dbPath,
				"benzinga-pp-cli sync --resources calendar-earnings,calendar-conference-calls --since 30d")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "calendar-earnings", flags.maxAge)

			callRows, err := novelRows(cmd.Context(), db, "calendar-conference-calls")
			if err != nil {
				return fmt.Errorf("querying conference calls: %w", err)
			}
			hasCall := map[string]bool{}
			for _, m := range callRows {
				if t := normTicker(novelStr(m, "ticker")); t != "" {
					hasCall[t] = true
				}
			}

			earnRows, err := novelRows(cmd.Context(), db, "calendar-earnings")
			if err != nil {
				return fmt.Errorf("querying earnings: %w", err)
			}

			out := make([]earningsRow, 0)
			for _, m := range earnRows {
				dateStr := novelStr(m, "date")
				// Skip rows with a missing or unparseable date rather than
				// letting them bypass the window filter.
				d, perr := time.Parse("2006-01-02", dateStr)
				if perr != nil || d.Before(lo) || d.After(hi) {
					continue
				}
				t := normTicker(novelStr(m, "ticker"))
				if t == "" || !inSet(t) {
					continue
				}
				eps, hasEps := novelFloat(m, "eps")
				epsEst, hasEst := novelFloat(m, "eps_est")
				if !hasEps {
					// Only reported rows count as "season" results.
					continue
				}
				surprise, hasSurprise := novelFloat(m, "eps_surprise_percent")
				if !hasSurprise && hasEst && epsEst != 0 {
					surprise = (eps - epsEst) / math.Abs(epsEst) * 100
				}
				revSurprise, _ := novelFloat(m, "revenue_surprise_percent")
				row := earningsRow{
					Ticker:         t,
					Name:           novelStr(m, "name"),
					Date:           dateStr,
					Period:         strings.TrimSpace(novelStr(m, "period") + " " + novelStr(m, "period_year")),
					EPS:            novelStr(m, "eps"),
					EPSEst:         novelStr(m, "eps_est"),
					EPSSurprisePct: round2(surprise),
					RevSurprisePct: round2(revSurprise),
					Beat:           hasEst && eps > epsEst,
					HasCall:        hasCall[t],
					absSurprise:    math.Abs(surprise),
				}
				out = append(out, row)
			}

			sort.SliceStable(out, func(i, j int) bool { return out[i].absSurprise > out[j].absSurprise })
			if flagLimit > 0 && len(out) > flagLimit {
				out = out[:flagLimit]
			}

			return novelEmit(cmd, flags, out, func() {
				w := cmd.OutOrStdout()
				if len(out) == 0 {
					fmt.Fprintln(w, "No reported earnings in window. Widen --from/--to or run sync --resources calendar-earnings.")
					return
				}
				fmt.Fprintln(w, "TICKER\tDATE\tPERIOD\tEPS\tEST\tSURPRISE%\tBEAT\tCALL")
				for _, r := range out {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%.2f\t%v\t%v\n",
						r.Ticker, r.Date, r.Period, r.EPS, r.EPSEst, r.EPSSurprisePct, r.Beat, r.HasCall)
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagFrom, "from", "7d", "Look-back window for reported earnings (e.g. 7d, 30d)")
	cmd.Flags().StringVar(&flagTo, "to", "7d", "Look-ahead window (e.g. 7d) — reported rows only are scored")
	cmd.Flags().StringVar(&flagTickers, "tickers", "", "Comma-separated tickers to restrict to")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum rows to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}
