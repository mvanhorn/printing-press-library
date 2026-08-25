// Copyright 2026 waveriderai and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: single-ticker move explainer. Merges local option activity,
// halts, rating changes, and news for one symbol into one chronological timeline
// — no single Benzinga endpoint returns a cross-source story.
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

type whyEvent struct {
	When     string `json:"when"`
	Kind     string `json:"kind"`
	Headline string `json:"headline"`
	URL      string `json:"url,omitempty"`
	when     time.Time
}

func newNovelWhyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagWindow string
		dbPath     string
	)

	cmd := &cobra.Command{
		Use:   "why <TICKER>",
		Short: "Build one time-ordered catalyst timeline for a ticker by merging options, halts, ratings, and news",
		Long: strings.Trim(`
Assembles one chronological catalyst timeline for a single ticker by merging the
local unusual-options-activity, trading-halt, analyst-rating, and news rows.

Use this to answer "why is X moving" with options + halts + ratings + headlines
stitched in order. Do NOT use it for a watchlist sweep (use 'watch') or for
upcoming dated events (use 'catalysts').

Reads the local SQLite mirror. Run 'sync' first:
  benzinga-pp-cli sync --resources signal-option-activity,signal-halt-resume,calendar-ratings,news --since 7d
`, "\n"),
		Example: "  benzinga-pp-cli why NVDA --window 1d --agent",
		// An unknown ticker yields an empty timeline (exit 0), not a usage error,
		// so the dogfood error-path probe must not expect a non-zero exit.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a ticker argument is required, e.g. 'why NVDA'"))
			}
			ticker := normTicker(args[0])

			window, err := novelDur(flagWindow, 7*24*time.Hour)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --window: %w", err))
			}
			cutoff := time.Now().Add(-window)

			db, ok, err := novelStore(cmd, flags, dbPath,
				"benzinga-pp-cli sync --resources signal-option-activity,signal-halt-resume,calendar-ratings,news --since 7d")
			if err != nil {
				return err
			}
			if !ok {
				return nil
			}
			defer db.Close()
			maybeEmitSyncHints(cmd, db, "", flags.maxAge)

			rows, err := novelRows(cmd.Context(), db,
				"signal-option-activity", "signal-halt-resume", "calendar-ratings", "news")
			if err != nil {
				return fmt.Errorf("querying local store: %w", err)
			}

			events := make([]whyEvent, 0)
			for _, m := range rows {
				when := novelEventTime(m)
				// The --window cutoff is always active (default 7d). Skip events
				// outside it — and events whose timestamp cannot be parsed (zero
				// time): a windowed catalyst timeline must not surface rows it
				// can't place inside the window.
				if when.IsZero() || when.Before(cutoff) {
					continue
				}
				matches := false
				var ev whyEvent
				switch {
				case m["put_call"] != nil:
					if normTicker(novelStr(m, "ticker")) == ticker {
						matches = true
						ev = whyEvent{Kind: "option", Headline: strings.TrimSpace(fmt.Sprintf(
							"%s %s $%s exp %s — %s", novelStr(m, "sentiment"), novelStr(m, "put_call"),
							novelStr(m, "strike_price"), novelStr(m, "date_expiration"), novelStr(m, "execution_estimate")))}
					}
				case m["halt_type"] != nil:
					if normTicker(novelStr(m, "ticker")) == ticker {
						matches = true
						ev = whyEvent{Kind: "halt", Headline: strings.TrimSpace(
							novelStr(m, "halt_type") + " — " + novelStr(m, "description"))}
					}
				case m["rating_current"] != nil || m["action_company"] != nil:
					if normTicker(novelStr(m, "ticker")) == ticker {
						matches = true
						ev = whyEvent{Kind: "rating", Headline: strings.TrimSpace(fmt.Sprintf(
							"%s %s — %s (PT %s→%s)", novelStr(m, "analyst"), novelStr(m, "action_company"),
							novelStr(m, "rating_current"), novelStr(m, "pt_prior"), novelStr(m, "pt_current"))),
							URL: novelStr(m, "url")}
					}
				case m["stocks"] != nil || m["title"] != nil:
					for _, t := range novelNewsTickers(m) {
						if t == ticker {
							matches = true
							ev = whyEvent{Kind: "news", Headline: novelStr(m, "title"), URL: novelStr(m, "url")}
							break
						}
					}
				}
				if matches {
					ev.When = fmtWhen(when)
					ev.when = when
					events = append(events, ev)
				}
			}

			// Chronological ascending — a timeline reads forward in time.
			sort.SliceStable(events, func(i, j int) bool { return events[i].when.Before(events[j].when) })

			result := map[string]any{
				"ticker":   ticker,
				"window":   window.String(),
				"count":    len(events),
				"timeline": events,
			}
			return novelEmit(cmd, flags, result, func() {
				out := cmd.OutOrStdout()
				if len(events) == 0 {
					fmt.Fprintf(out, "No catalysts found for %s in the last %s. Try a wider --window or run sync.\n", ticker, window)
					return
				}
				fmt.Fprintf(out, "Catalyst timeline for %s (last %s):\n\n", ticker, window)
				for _, e := range events {
					fmt.Fprintf(out, "  %s  [%s]  %s\n", e.When, e.Kind, e.Headline)
				}
			})
		},
	}

	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Look-back window for catalysts (e.g. 1d, 24h, 7d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}
