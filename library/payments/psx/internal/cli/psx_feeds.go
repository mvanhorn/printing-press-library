// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

// psx_feeds.go implements the portal's form-POST surfaces (historical OHLCV,
// announcements, payouts) plus the per-symbol quote lookup, and registers every
// hand-authored command onto the root tree.

package cli

import (
	"context"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/cliutil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/psx/internal/psx"
)

// ---- quote -----------------------------------------------------------------

// newPsxQuoteCmd resolves one instrument from the whole-market snapshot. PSX
// exposes no single-symbol quote endpoint, so this filters the market-watch
// table rather than pretending a per-symbol call exists.
func newPsxQuoteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "quote <symbol>",
		Short: "Current price row for one instrument",
		Long: "Use this command for a one-off price on any listed instrument.\n" +
			"Do NOT use it to price a saved set of names; use 'watchlist prices' instead.\n" +
			"PSX has no single-symbol quote endpoint, so this selects the row from the market snapshot.",
		Example:     "  psx-pp-cli quote OGDC --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=OGDC", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "quote")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. OGDC"))
			}
			want := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			t, err := fetchTable(ctx, psxClient(flags), "/market-watch", "", "symbol", "current")
			if err != nil {
				return err
			}
			match := psx.Table{ID: t.ID, Headers: t.Headers, Rows: make([]map[string]string, 0, 1)}
			for _, row := range t.Rows {
				if strings.EqualFold(strings.TrimSpace(row["symbol"]), want) {
					match.Rows = append(match.Rows, row)
					break
				}
			}
			if len(match.Rows) == 0 {
				// An unknown code is a real not-found, not an empty success: the
				// market snapshot lists every tradable instrument.
				return notFoundErr(fmt.Errorf("no instrument %q in the current market snapshot; check the code with 'psx-pp-cli symbols list'", want))
			}
			return emitTable(cmd, flags, "/market-watch#"+want, match, 0, "")
		},
	}
	return cmd
}

// ---- history ---------------------------------------------------------------

// newPsxHistoryCmd fetches full OHLCV history. PSX ignores date parameters
// server-side and always returns the complete series, so any date narrowing
// happens locally and is reported honestly in the envelope.
func newPsxHistoryCmd(flags *rootFlags) *cobra.Command {
	var from, to string
	var limit int
	cmd := &cobra.Command{
		Use:   "history <symbol>",
		Short: "Full OHLCV history for one instrument",
		Long: "Use this command for daily open/high/low/close/volume bars.\n" +
			"Do NOT use it for intraday ticks; use 'timeseries intraday' instead.\n" +
			"PSX ignores date parameters server-side and always returns the complete series, " +
			"so --from and --to filter locally after the fetch.",
		Example:     "  psx-pp-cli history OGDC --from 2026-01-01 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=OGDC;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "history")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. OGDC"))
			}
			sym := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			tables, err := psxClient(flags).PostTables(ctx, "/historical", url.Values{"symbol": {sym}})
			if err != nil {
				return err
			}
			t, ok := psx.FindTable(tables, "historicalTable", "date", "close")
			if !ok {
				if t, ok = psx.FindTable(tables, "", "date", "close"); !ok {
					return fmt.Errorf("no historical table for %s; check the symbol with 'psx-pp-cli symbols list'", sym)
				}
			}
			filtered := psx.Table{ID: t.ID, Headers: t.Headers, Rows: make([]map[string]string, 0, len(t.Rows))}
			for _, row := range t.Rows {
				d := parsePSXDate(row["date"])
				if from != "" && (d == "" || d < from) {
					continue
				}
				if to != "" && (d == "" || d > to) {
					continue
				}
				row["date_iso"] = d
				filtered.Rows = append(filtered.Rows, row)
			}
			if len(filtered.Headers) > 0 {
				filtered.Headers = append(append([]string{}, t.Headers...), "date_iso")
			}
			note := ""
			if from != "" || to != "" {
				note = fmt.Sprintf("PSX returned %d bars and ignores server-side date filters; %d kept after local filtering", len(t.Rows), len(filtered.Rows))
				// If nothing parsed at all, the date format changed upstream —
				// that is a parser failure, not an empty range.
				if len(t.Rows) > 0 && len(filtered.Rows) == 0 {
					parsedAny := false
					for _, row := range t.Rows {
						if parsePSXDate(row["date"]) != "" {
							parsedAny = true
							break
						}
					}
					if !parsedAny {
						return apiErr(fmt.Errorf("none of the %d returned bars had a parseable date; the portal's date format may have changed (run without --from/--to to see raw rows)", len(t.Rows)))
					}
				}
			}
			return emitTable(cmd, flags, "/historical#"+sym, filtered, limit, note)
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "keep bars on or after this date (YYYY-MM-DD, filtered locally)")
	cmd.Flags().StringVar(&to, "to", "", "keep bars on or before this date (YYYY-MM-DD, filtered locally)")
	cmd.Flags().IntVar(&limit, "limit", 0, "maximum bars to return (0 = all)")
	return cmd
}

// parsePSXDate converts the portal's "Aug 19, 2026" rendering into an ISO date
// so local range filtering and joins are lexicographic. Unparseable input
// returns "" and is treated as out-of-range rather than silently kept.
func parsePSXDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Try the common machine formats before the prose path. PSX renders dates
	// differently per surface (ISO in JSON, "Aug 19, 2026" in tables,
	// dd/mm/yyyy in payout cells), and a single-format parser silently drops
	// every row the moment a surface changes.
	for _, layout := range []string{"2006-01-02", "02/01/2006", "02-01-2006", "Jan 2, 2006", "January 2, 2006"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t.Format("2006-01-02")
		}
	}
	months := map[string]string{
		"jan": "01", "feb": "02", "mar": "03", "apr": "04", "may": "05", "jun": "06",
		"jul": "07", "aug": "08", "sep": "09", "oct": "10", "nov": "11", "dec": "12",
	}
	cleaned := strings.NewReplacer(",", " ", "-", " ").Replace(s)
	parts := strings.Fields(cleaned)
	if len(parts) < 3 {
		return ""
	}
	mon, ok := months[strings.ToLower(parts[0])[:min3(len(parts[0]), 3)]]
	if !ok {
		return ""
	}
	day, err := strconv.Atoi(parts[1])
	if err != nil {
		return ""
	}
	year, err := strconv.Atoi(parts[2])
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%04d-%s-%02d", year, mon, day)
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---- announcements ---------------------------------------------------------

// announcementStreams maps the public --stream vocabulary to the portal's
// single-letter type codes, verified against the portal's own select options.
var announcementStreams = map[string]struct {
	code  string
	label string
}{
	"companies": {"C", "Companies Announcements"},
	"psx":       {"E", "PSX Notices"},
	"secp":      {"B", "SECP Notices"},
	"nccpl":     {"D", "NCCPL Notices"},
	"cdc":       {"A", "CDC Notices"},
}

func newPsxAnnouncementsCmd(flags *rootFlags) *cobra.Command {
	var stream, symbol, query, from, to string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "announcements",
		Short: "Search corporate and regulatory announcement streams",
		Long: "Use this command to search the full announcement corpus by stream, symbol, keyword or date.\n" +
			"Do NOT use it for a joined digest across your saved symbols; use 'actions' instead.\n" +
			"Streams: companies, psx, secp, nccpl, cdc.",
		Example:     "  psx-pp-cli announcements --stream companies --symbol OGDC --limit 10 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "announcements")
			}
			key := strings.ToLower(strings.TrimSpace(stream))
			s, ok := announcementStreams[key]
			if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--stream must be one of companies, psx, secp, nccpl, cdc (got %q)", stream))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			t, total, err := fetchAnnouncements(ctx, psxClient(flags), s.code, symbol, query, from, to, limit, offset)
			if err != nil {
				return err
			}
			note := fmt.Sprintf("%s: %d matching entries upstream (showing %d from offset %d)", s.label, total, len(t.Rows), offset)
			return emitTable(cmd, flags, "/announcements#"+s.code, t, 0, note)
		},
	}
	cmd.Flags().StringVar(&stream, "stream", "companies", "announcement stream: companies, psx, secp, nccpl or cdc")
	cmd.Flags().StringVar(&symbol, "symbol", "", "filter to one instrument code, e.g. OGDC")
	cmd.Flags().StringVar(&query, "query", "", "free-text search across announcement titles")
	cmd.Flags().StringVar(&from, "from", "", "earliest date, YYYY-MM-DD")
	cmd.Flags().StringVar(&to, "to", "", "latest date, YYYY-MM-DD")
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum announcements to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "row offset for pagination")
	return cmd
}

// fetchAnnouncements posts the portal's announcement form and also returns the
// upstream total, which the portal reports in a "Showing X to Y of N entries"
// header. Surfacing the total lets callers page without guessing.
func fetchAnnouncements(ctx context.Context, c *psx.Client, code, symbol, query, from, to string, limit, offset int) (psx.Table, int, error) {
	if limit <= 0 {
		limit = 25
	}
	form := url.Values{
		"type":      {code},
		"symbol":    {strings.ToUpper(strings.TrimSpace(symbol))},
		"query":     {query},
		"count":     {strconv.Itoa(limit)},
		"offset":    {strconv.Itoa(offset)},
		"date_from": {from},
		"date_to":   {to},
		"page":      {"annc"},
	}
	body, err := c.PostForm(ctx, "/announcements", form)
	if err != nil {
		return psx.Table{}, 0, err
	}
	total := parseEntriesTotal(string(body))
	tables, err := psx.ParseTables(string(body))
	if err != nil {
		return psx.Table{}, total, err
	}
	t, ok := psx.FindTable(tables, "announcementsTable", "date", "title")
	if !ok {
		if t, ok = psx.FindTable(tables, "", "date"); !ok {
			return psx.Table{Rows: make([]map[string]string, 0)}, total, nil
		}
	}
	return t, total, nil
}

// parseEntriesTotal reads the "of N entries" counter the portal renders above
// paginated tables. Returns 0 when absent rather than guessing.
func parseEntriesTotal(body string) int {
	idx := strings.Index(body, " entries")
	if idx < 0 {
		return 0
	}
	head := body[:idx]
	ofIdx := strings.LastIndex(head, " of ")
	if ofIdx < 0 {
		return 0
	}
	digits := strings.TrimSpace(head[ofIdx+4:])
	digits = strings.ReplaceAll(digits, ",", "")
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}

// ---- payouts ---------------------------------------------------------------

func newPsxPayoutsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "payouts",
		Short:       "Dividend, bonus and right-issue payout history",
		Example:     "  psx-pp-cli payouts company OGDC --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newPsxPayoutsListCmd(flags), newPsxPayoutsCompanyCmd(flags), newPsxPayoutsDeadlineCmd(flags))
	return cmd
}

func newPsxPayoutsListCmd(flags *rootFlags) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Recent payouts across all instruments",
		Long: "Use this command to page through market-wide payout history.\n" +
			"Do NOT use it for one instrument's full history; use 'payouts company' instead.",
		Example:     "  psx-pp-cli payouts list --limit 25 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payouts list")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			t, err := fetchPayouts(ctx, psxClient(flags), "", limit, offset)
			if err != nil {
				return err
			}
			return emitTable(cmd, flags, "/payouts", t, 0, "")
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "maximum payout rows to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "row offset for pagination")
	return cmd
}

func newPsxPayoutsCompanyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "company <symbol>",
		Short: "Full payout history for one instrument",
		Long: "Use this command for one instrument's complete payout record.\n" +
			"Do NOT use it to scan the whole market; use 'payouts list' instead.",
		Example:     "  psx-pp-cli payouts company OGDC --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=OGDC", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payouts company")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. OGDC"))
			}
			sym := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			t, err := fetchPayouts(ctx, psxClient(flags), sym, 0, 0)
			if err != nil {
				return err
			}
			if len(t.Rows) == 0 {
				known, kerr := symbolIsListed(ctx, psxClient(flags), sym)
				if kerr == nil && !known {
					return notFoundErr(fmt.Errorf("no listed instrument %q; check the code with 'psx-pp-cli symbols'", sym))
				}
			}
			return emitTable(cmd, flags, "/company/payouts#"+sym, t, 0, "")
		},
	}
	return cmd
}

// newPsxPayoutsDeadlineCmd computes the last day to buy and still qualify for a
// payout. PSX publishes book-closure dates, not ex-dates, so the deadline is
// derived as the trading day before book closure begins and is labelled as
// derived rather than presented as an exchange-published figure.
func newPsxPayoutsDeadlineCmd(flags *rootFlags) *cobra.Command {
	var upcoming bool
	cmd := &cobra.Command{
		Use:   "deadline <symbol>",
		Short: "Last day to buy and still qualify for an announced payout",
		Long: "Use this command to find the buy deadline for an announced payout.\n" +
			"Do NOT use it to list payout history; use 'payouts company' instead.\n" +
			"PSX publishes book-closure dates rather than ex-dates, so the deadline is derived as the " +
			"calendar day before book closure opens. Market holidays and weekends are not modelled, " +
			"so confirm against the exchange calendar before acting on it.",
		Example:     "  psx-pp-cli payouts deadline OGDC --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "symbol=OGDC", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payouts deadline")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a symbol is required, e.g. OGDC"))
			}
			sym := strings.ToUpper(strings.TrimSpace(args[0]))
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			t, err := fetchPayouts(ctx, psxClient(flags), sym, 0, 0)
			if err != nil {
				return err
			}
			type deadline struct {
				Symbol      string `json:"symbol"`
				BookClosure string `json:"book_closure_from,omitempty"`
				BuyBy       string `json:"buy_by,omitempty"`
				Payout      string `json:"payout,omitempty"`
				Derived     bool   `json:"derived"`
				Upcoming    bool   `json:"upcoming"`
			}
			today := nowUTC().Format("2006-01-02")
			out := make([]deadline, 0)
			for _, row := range t.Rows {
				// PSX renders book closure as a range ("12/05/2026 - 13/05/2026");
				// the deadline is keyed off the day it OPENS, so take the start.
				bc := parsePSXDate(rangeStart(firstNonEmpty(row, "book_closure_from", "bc_from", "book_closure", "from")))
				if bc == "" {
					continue
				}
				d := deadline{
					Symbol: sym, BookClosure: bc, BuyBy: previousDay(bc),
					Payout:  expandPayoutKind(firstNonEmpty(row, "details", "payout", "dividend", "rate", "bonus")),
					Derived: true, Upcoming: bc >= today,
				}
				if upcoming && !d.Upcoming {
					continue
				}
				out = append(out, d)
			}
			view := struct {
				Symbol    string     `json:"symbol"`
				Count     int        `json:"count"`
				Deadlines []deadline `json:"deadlines"`
				Note      string     `json:"note"`
			}{Symbol: sym, Count: len(out), Deadlines: out,
				Note: "buy_by is the calendar day before book closure opens; PSX publishes no ex-date field, and weekends/market holidays are not modelled"}
			if len(out) == 0 {
				known, kerr := symbolIsListed(ctx, psxClient(flags), sym)
				if kerr == nil && !known {
					return notFoundErr(fmt.Errorf("no listed instrument %q", sym))
				}
				view.Note = "no payout with a book-closure date found for " + sym
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-14s %-12s %s\n", "SYMBOL", "BOOK CLOSURE", "BUY BY", "PAYOUT")
			for _, d := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%-12s %-14s %-12s %s\n", cliutil.ScrubTerminal(d.Symbol), cliutil.ScrubTerminal(d.BookClosure), cliutil.ScrubTerminal(d.BuyBy), cliutil.ScrubTerminal(d.Payout))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", view.Note)
			return nil
		},
	}
	cmd.Flags().BoolVar(&upcoming, "upcoming", false, "only deadlines that have not yet passed")
	return cmd
}

// rangeStart returns the first date of a "A - B" range, or the input unchanged
// when it is a single value.
func rangeStart(s string) string {
	s = strings.TrimSpace(s)
	for _, sep := range []string{" - ", " to ", " – ", "-to-"} {
		if i := strings.Index(s, sep); i > 0 {
			return strings.TrimSpace(s[:i])
		}
	}
	return s
}

// previousDay returns the ISO calendar day before d, or d when unparseable.
// Deliberately calendar-based: PSX publishes no trading calendar here, so
// inferring a trading day would be a guess presented as a fact.
func previousDay(d string) string {
	t, err := time.Parse("2006-01-02", d)
	if err != nil {
		return d
	}
	return t.AddDate(0, 0, -1).Format("2006-01-02")
}

// fetchPayouts routes to the per-company endpoint when a symbol is supplied,
// because the portal serves a fuller history there than the filtered list.
func fetchPayouts(ctx context.Context, c *psx.Client, symbol string, limit, offset int) (psx.Table, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	var tables []psx.Table
	var err error
	if sym != "" {
		tables, err = c.PostTables(ctx, "/company/payouts", url.Values{"symbol": {sym}})
	} else {
		if limit <= 0 {
			limit = 25
		}
		tables, err = c.PostTables(ctx, "/payouts", url.Values{
			"symbol": {""},
			"count":  {strconv.Itoa(limit)},
			"offset": {strconv.Itoa(offset)},
		})
	}
	if err != nil {
		return psx.Table{}, err
	}
	for _, t := range tables {
		if len(t.Rows) > 0 {
			return t, nil
		}
	}
	return psx.Table{Rows: make([]map[string]string, 0)}, nil
}

// ---- registration ----------------------------------------------------------

// replaceRootCommand swaps a generated command for a hand-authored one of the
// same name. The generated endpoint commands for PSX's HTML table surfaces
// return page/link metadata rather than rows, because the generator's
// html_extract supports only page|links|embedded-json and has no table mode.
// Keeping the spec declarations (so paths, typed MCP tools and validation stay
// intact) while swapping the implementations gives correct data on both surfaces.
func replaceRootCommand(root *cobra.Command, cmd *cobra.Command) {
	for _, existing := range root.Commands() {
		if existing.Name() == cmd.Name() {
			root.RemoveCommand(existing)
			break
		}
	}
	root.AddCommand(cmd)
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		// HTML-table surfaces: replace the generated leaves with parsers that
		// key rows by header name.
		replaceRootCommand(root, newPsxMarketCmd(flags))
		replaceRootCommand(root, newPsxHistoryCmd(flags))
		replaceRootCommand(root, newPsxScreenerCmd(flags))
		replaceRootCommand(root, newPsxDebtCmd(flags))
		replaceRootCommand(root, newPsxEligibleScripsCmd(flags))
		replaceRootCommand(root, newPsxIndicesCmd(flags))
		replaceRootCommand(root, newPsxCircuitBreakersCmd(flags))
		replaceRootCommand(root, newPsxListingsCmd(flags))
		replaceRootCommand(root, newPsxBoardCmd(flags))
		replaceRootCommand(root, newPsxAnnouncementsCmd(flags))
		replaceRootCommand(root, newPsxPayoutsCmd(flags))
		replaceRootCommand(root, newPsxCompanyCmd(flags))

		// Quote has no upstream endpoint of its own; it filters the snapshot.
		addNovelCommandIfAbsent(root, newPsxQuoteCmd(flags))

		if sectorsCmd, _, err := root.Find([]string{"sectors"}); err == nil && sectorsCmd != nil {
			for _, sub := range newPsxSectorsExtraCmds(flags) {
				for _, existing := range sectorsCmd.Commands() {
					if existing.Name() == sub.Name() {
						sectorsCmd.RemoveCommand(existing)
						break
					}
				}
				sectorsCmd.AddCommand(sub)
			}
		}
	})
}
