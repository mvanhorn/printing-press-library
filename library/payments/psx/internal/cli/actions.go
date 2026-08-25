// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
)

// actionEvent is one normalized corporate action, whatever feed it came from.
type actionEvent struct {
	Symbol   string `json:"symbol"`
	Kind     string `json:"kind"`
	Date     string `json:"date"`
	Headline string `json:"headline"`
	Source   string `json:"source"`
}

// newNovelActionsCmd unions four separate portal feeds against the local
// watchlist. No PSX endpoint returns them together, and no surveyed tool wraps
// any of announcements, payouts, calendar or circuit breakers.
func newNovelActionsCmd(flags *rootFlags) *cobra.Command {
	var useWatchlist bool
	var since, dbPath, symbols string
	var limit int

	cmd := &cobra.Command{
		Use:   "actions",
		Short: "See every announcement, payout, AGM date and circuit-breaker event for the symbols you track, in one place.",
		Long: "Use this command for a joined digest of announcements, payouts, AGM dates and breaker events across the symbols on your watchlist.\n" +
			"Do NOT use it to search the full announcement corpus by keyword or date; use 'announcements' instead.\n" +
			"Do NOT use it to list raw payout history; use 'payouts' instead.",
		Example:     "  psx-pp-cli actions --watchlist --since 7d --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "actions")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Resolve the symbol set: explicit --symbols wins, else the watchlist.
			want := map[string]bool{}
			var ordered []string
			for _, raw := range strings.Split(symbols, ",") {
				if s := strings.ToUpper(strings.TrimSpace(raw)); s != "" {
					if !want[s] {
						want[s] = true
						ordered = append(ordered, s)
					}
				}
			}
			if len(ordered) == 0 && (useWatchlist || symbols == "") {
				if mirrorMissing(dbPath) {
					return writeMirrorHint(cmd, flags, orDefaultDB(dbPath), "watchlist")
				}
				s, _, err := openLocalStore(ctx, dbPath)
				if err != nil {
					return err
				}
				syms, _, err := watchlistSymbols(ctx, s)
				_ = s.Close()
				if err != nil {
					return err
				}
				for _, sym := range syms {
					want[sym] = true
					ordered = append(ordered, sym)
				}
			}

			cutoffISO := ""
			if strings.TrimSpace(since) != "" {
				d, err := cliutil.ParseDurationLoose(since)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--since %q is not a duration (try 7d, 2w): %w", since, err))
				}
				cutoffISO = nowUTC().Add(-d).Format("2006-01-02")
			}

			view := struct {
				Symbols       []string       `json:"symbols"`
				Since         string         `json:"since,omitempty"`
				Count         int            `json:"count"`
				Events        []actionEvent  `json:"events"`
				FetchFailures []fetchFailure `json:"fetch_failures,omitempty"`
				Note          string         `json:"note,omitempty"`
			}{Symbols: ordered, Since: cutoffISO, Events: make([]actionEvent, 0), FetchFailures: make([]fetchFailure, 0)}

			if len(ordered) == 0 {
				view.Note = "no symbols selected; add names with 'psx-pp-cli watchlist track OGDC' or pass --symbols OGDC,LUCK"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}

			// Feeds run concurrently; each error travels with its own result so a
			// failed feed is reported rather than silently shrinking the digest.
			type feedResult struct {
				name     string
				events   []actionEvent
				failures []fetchFailure
				err      error
			}
			c := psxClient(flags)
			results := make(chan feedResult, 4)
			var wg sync.WaitGroup
			run := func(name string, fn func() ([]actionEvent, []fetchFailure, error)) {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ev, fails, err := fn()
					results <- feedResult{name: name, events: ev, failures: fails, err: err}
				}()
			}
			run("announcements", func() ([]actionEvent, []fetchFailure, error) {
				return feedAnnouncements(ctx, c, ordered, cutoffISO, limit)
			})
			run("payouts", func() ([]actionEvent, []fetchFailure, error) { return feedPayouts(ctx, c, ordered) })
			run("calendar", func() ([]actionEvent, []fetchFailure, error) { return feedCalendar(ctx, c, want, cutoffISO) })
			run("circuit-breakers", func() ([]actionEvent, []fetchFailure, error) { return feedBreakers(ctx, c, want) })
			go func() { wg.Wait(); close(results) }()

			failedFeeds := 0
			for r := range results {
				// Keep whatever the feed did produce. A per-symbol error must
				// not discard its siblings' rows, and a feed-level error must
				// not discard the symbols that already succeeded.
				view.Events = append(view.Events, r.events...)
				view.FetchFailures = append(view.FetchFailures, r.failures...)
				if r.err != nil {
					failedFeeds++
					view.FetchFailures = append(view.FetchFailures, fetchFailure{ID: r.name, Error: r.err.Error()})
				}
			}
			sort.Slice(view.Events, func(i, j int) bool {
				if view.Events[i].Date != view.Events[j].Date {
					return view.Events[i].Date > view.Events[j].Date
				}
				return view.Events[i].Symbol < view.Events[j].Symbol
			})
			if limit > 0 && len(view.Events) > limit {
				view.Events = view.Events[:limit]
			}
			view.Count = len(view.Events)

			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d fetch failure(s) across %d of 4 feeds; digest built from %d event(s) that did succeed\n",
					len(view.FetchFailures), failedFeeds, len(view.Events))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Count == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No corporate actions for the selected symbols in this window.")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-12s %-16s %s\n", "DATE", "SYMBOL", "KIND", "HEADLINE")
			for _, e := range view.Events {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-12s %-16s %s\n", cliutil.ScrubTerminal(e.Date), cliutil.ScrubTerminal(e.Symbol), cliutil.ScrubTerminal(e.Kind), cliutil.ScrubTerminal(truncateStr(e.Headline, 70)))
			}
			if len(view.FetchFailures) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\npartial results: %d feed(s) failed\n", len(view.FetchFailures))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&useWatchlist, "watchlist", false, "use the saved watchlist as the symbol set (default when --symbols is empty)")
	cmd.Flags().StringVar(&symbols, "symbols", "", "comma-separated symbols instead of the watchlist")
	cmd.Flags().StringVar(&since, "since", "30d", "only events on or after this window (7d, 2w)")
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum events to return (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "database path")
	return cmd
}

// fetchFailure records one failed feed so partial results stay visible.
type fetchFailure struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// truncateStr trims by rune, not byte, so multi-byte characters are never
// split into invalid UTF-8 (calendar headlines contain an em dash).
func truncateStr(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// normalizeISODate passes through an already-ISO date and otherwise routes
// through the portal's prose format, so every feed sorts on one representation.
func normalizeISODate(s string) string {
	t := strings.TrimSpace(s)
	if len(t) == 10 && t[4] == '-' && t[7] == '-' {
		return t
	}
	if iso := parsePSXDate(t); iso != "" {
		return iso
	}
	return t
}

func feedAnnouncements(ctx context.Context, c *psx.Client, syms []string, cutoff string, limit int) ([]actionEvent, []fetchFailure, error) {
	out := make([]actionEvent, 0)
	fails := make([]fetchFailure, 0)
	per := limit
	if per <= 0 || per > 25 {
		per = 25
	}
	for _, sym := range syms {
		t, _, err := fetchAnnouncements(ctx, c, "C", sym, "", cutoff, "", per, 0)
		if err != nil {
			fails = append(fails, fetchFailure{ID: "announcements:" + sym, Error: err.Error()})
			continue
		}
		for _, row := range t.Rows {
			d := parsePSXDate(row["date"])
			if cutoff != "" && d != "" && d < cutoff {
				continue
			}
			out = append(out, actionEvent{
				Symbol: sym, Kind: "announcement", Date: d,
				Headline: row["title"], Source: "/announcements",
			})
		}
	}
	return out, fails, nil
}

func feedPayouts(ctx context.Context, c *psx.Client, syms []string) ([]actionEvent, []fetchFailure, error) {
	out := make([]actionEvent, 0)
	fails := make([]fetchFailure, 0)
	for _, sym := range syms {
		t, err := fetchPayouts(ctx, c, sym, 25, 0)
		if err != nil {
			fails = append(fails, fetchFailure{ID: "payouts:" + sym, Error: err.Error()})
			continue
		}
		for _, row := range t.Rows {
			date := firstNonEmpty(row, "book_closure_from", "bc_from", "date", "payout_date", "announcement_date")
			out = append(out, actionEvent{
				Symbol: sym, Kind: "payout", Date: parsePSXDate(date),
				Headline: describePayout(row), Source: "/company/payouts",
			})
		}
	}
	return out, fails, nil
}

func feedCalendar(ctx context.Context, c *psx.Client, want map[string]bool, cutoff string) ([]actionEvent, []fetchFailure, error) {
	out := make([]actionEvent, 0)
	from := cutoff
	if from == "" {
		from = nowUTC().AddDate(0, -1, 0).Format("2006-01-02")
	}
	to := nowUTC().AddDate(0, 3, 0).Format("2006-01-02")
	raw, err := c.PostEnvelope(ctx, "/calendar", url.Values{"from": {from}, "to": {to}})
	if err != nil {
		return out, nil, err
	}
	var events []struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
		Type   string `json:"type"`
		Date   string `json:"date"`
		City   string `json:"city"`
	}
	if err := jsonUnmarshalStrict(raw, &events); err != nil {
		return out, nil, err
	}
	for _, e := range events {
		sym := strings.ToUpper(strings.TrimSpace(e.Symbol))
		if !want[sym] {
			continue
		}
		out = append(out, actionEvent{
			Symbol: sym, Kind: strings.ToLower(e.Type), Date: normalizeISODate(e.Date),
			Headline: strings.TrimSpace(e.Type + " — " + e.Name + " (" + e.City + ")"),
			Source:   "/calendar",
		})
	}
	return out, nil, nil
}

func feedBreakers(ctx context.Context, c *psx.Client, want map[string]bool) ([]actionEvent, []fetchFailure, error) {
	out := make([]actionEvent, 0)
	tables, err := c.GetTables(ctx, "/circuit-breakers")
	if err != nil {
		return out, nil, err
	}
	today := nowUTC().Format("2006-01-02")
	for _, t := range tables {
		for _, row := range t.Rows {
			sym := strings.ToUpper(strings.TrimSpace(row["symbol"]))
			if sym == "" || !want[sym] {
				continue
			}
			out = append(out, actionEvent{
				Symbol: sym, Kind: "circuit-breaker", Date: today,
				Headline: summarizeRow(row), Source: "/circuit-breakers",
			})
		}
	}
	return out, nil, nil
}

func firstNonEmpty(row map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(row[k]); v != "" {
			return v
		}
	}
	return ""
}

// describePayout renders a payout row as readable prose. PSX encodes the
// payout itself in a "details" cell such as "32.50%(iii) (D)", where the
// trailing letter is the instrument type — D dividend, B bonus, R right issue.
// The letter is expanded only when recognised; an unknown code is passed
// through verbatim rather than guessed at.
func describePayout(row map[string]string) string {
	details := strings.TrimSpace(row["details"])
	parts := make([]string, 0, 3)
	if details != "" {
		parts = append(parts, expandPayoutKind(details))
	}
	if bc := strings.TrimSpace(row["book_closure"]); bc != "" {
		parts = append(parts, "book closure "+bc)
	}
	if fr := strings.TrimSpace(row["financial_results"]); fr != "" {
		parts = append(parts, "for period "+fr)
	}
	if len(parts) == 0 {
		// Fall back to the generic dump, and if that is empty too (a row whose
		// only populated column is the symbol), say so rather than emitting a
		// blank headline into the digest.
		if generic := summarizeRow(row); generic != "" {
			return generic
		}
		return "payout announced (no details published)"
	}
	return strings.Join(parts, ", ")
}

// expandPayoutKind turns "32.50%(iii) (D)" into "32.50% dividend". Unrecognised
// trailing codes are left untouched.
func expandPayoutKind(details string) string {
	kinds := map[string]string{"(D)": "dividend", "(B)": "bonus", "(R)": "right issue"}
	for code, word := range kinds {
		if strings.HasSuffix(details, code) {
			return strings.TrimSpace(strings.TrimSuffix(details, code)) + " " + word
		}
	}
	return details
}

// summarizeRow renders a row as compact key=value text for feeds whose column
// set varies. Keys are sorted so output is deterministic.
func summarizeRow(row map[string]string) string {
	keys := make([]string, 0, len(row))
	for k := range row {
		if strings.TrimSpace(row[k]) == "" || k == "symbol" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, k+"="+row[k])
	}
	return strings.Join(parts, " ")
}
