// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: overnight watchlist change scan. Joins local ratings, news, and
// signal tables filtered to a ticker set and a since-cursor — a multi-entity diff
// the REST API cannot express in one call.
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

type watchEvent struct {
	Ticker   string `json:"ticker"`
	Kind     string `json:"kind"`
	When     string `json:"when"`
	Headline string `json:"headline"`
	URL      string `json:"url,omitempty"`
	when     time.Time
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince   string
		flagTickers string
		flagLimit   int
		dbPath      string
	)

	cmd := &cobra.Command{
		Use:   "watch [tickers]",
		Short: "See everything that changed on your tickers since you last looked — ratings, news, and signals in one diff",
		Long: strings.Trim(`
Cross-entity change scan over the local store. For a ticker set, collects new
analyst rating changes, breaking news, unusual options activity, and trading
halts since a cutoff and returns them as one time-ordered diff.

Use this for a multi-ticker "what changed on my names since I last looked" view.
Do NOT use it to deep-dive one ticker's intraday move — use 'why'; for upcoming
dated events use 'catalysts'.

Reads the local SQLite mirror. Run 'sync' first:
  benzinga-pp-cli sync --resources calendar-ratings,news,signal-option-activity,signal-halt-resume --since 7d
`, "\n"),
		Example: "  benzinga-pp-cli watch AAPL,NVDA,TSLA --since 24h --agent",
		// An unknown ticker is a valid empty result, not a usage error, so the
		// dogfood error-path probe (garbage positional) must not expect a non-zero exit.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			since, err := novelDur(flagSince, 24*time.Hour)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --since: %w", err))
			}
			cutoff := time.Now().Add(-since)
			tickers := novelTickerSet(args, flagTickers)

			db, ok, err := novelStore(cmd, flags, dbPath,
				"benzinga-pp-cli sync --resources calendar-ratings,news,signal-option-activity,signal-halt-resume --since 7d")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "", flags.maxAge)

			rows, err := novelRows(cmd.Context(), db,
				"calendar-ratings", "news", "signal-option-activity", "signal-halt-resume")
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}

			events := make([]watchEvent, 0)
			inSet := func(t string) bool { return len(tickers) == 0 || tickers[t] }

			for _, m := range rows {
				when := novelEventTime(m)
				// A --since window is always active (default 24h). Skip events
				// outside it — and events whose timestamp cannot be parsed
				// (zero time): a "what changed since X" digest must not surface
				// rows it can't place inside the window as if they were recent.
				if when.IsZero() || when.Before(cutoff) {
					continue
				}
				switch {
				case m["rating_current"] != nil || m["action_company"] != nil:
					t := normTicker(novelStr(m, "ticker"))
					if t == "" || !inSet(t) {
						continue
					}
					events = append(events, watchEvent{
						Ticker: t,
						Kind:   "rating",
						When:   fmtWhen(when),
						Headline: strings.TrimSpace(fmt.Sprintf("%s %s — %s (PT %s→%s)",
							novelStr(m, "analyst"), novelStr(m, "action_company"),
							novelStr(m, "rating_current"), novelStr(m, "pt_prior"), novelStr(m, "pt_current"))),
						URL:  novelStr(m, "url"),
						when: when,
					})
				case m["stocks"] != nil || m["title"] != nil:
					title := novelStr(m, "title")
					for _, t := range novelNewsTickers(m) {
						if !inSet(t) {
							continue
						}
						events = append(events, watchEvent{
							Ticker: t, Kind: "news", When: fmtWhen(when),
							Headline: title, URL: novelStr(m, "url"), when: when,
						})
					}
				case m["put_call"] != nil:
					t := normTicker(novelStr(m, "ticker"))
					if t == "" || !inSet(t) {
						continue
					}
					events = append(events, watchEvent{
						Ticker: t, Kind: "option", When: fmtWhen(when),
						Headline: strings.TrimSpace(fmt.Sprintf("%s %s $%s — %s",
							novelStr(m, "sentiment"), novelStr(m, "put_call"),
							novelStr(m, "strike_price"), novelStr(m, "execution_estimate"))),
						when: when,
					})
				case m["halt_type"] != nil:
					t := normTicker(novelStr(m, "ticker"))
					if t == "" || !inSet(t) {
						continue
					}
					events = append(events, watchEvent{
						Ticker: t, Kind: "halt", When: fmtWhen(when),
						Headline: strings.TrimSpace(novelStr(m, "halt_type") + " — " + novelStr(m, "description")),
						when:     when,
					})
				}
			}

			sort.SliceStable(events, func(i, j int) bool { return events[i].when.After(events[j].when) })
			if flagLimit > 0 && len(events) > flagLimit {
				events = events[:flagLimit]
			}

			return novelEmit(cmd, flags, events, func() {
				out := cmd.OutOrStdout()
				if len(events) == 0 {
					fmt.Fprintln(out, "No changes found. Try a wider --since, more tickers, or run sync.")
					return
				}
				fmt.Fprintln(out, "TICKER\tKIND\tWHEN\tHEADLINE")
				for _, e := range events {
					fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", e.Ticker, e.Kind, e.When, e.Headline)
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagSince, "since", "24h", "Only changes newer than this window (e.g. 24h, 7d, 1w)")
	cmd.Flags().StringVar(&flagTickers, "tickers", "", "Comma-separated tickers (alternative to positional arg)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum change events to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

func fmtWhen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
