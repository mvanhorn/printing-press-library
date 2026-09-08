// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// universeUnknownGroup labels rows whose grouping column is absent or blank.
// Folding them into a real group would understate that group's width, which is
// the one number this command exists to report honestly.
const universeUnknownGroup = "(unknown)"

// universeHumanRowCap bounds the human table. A decade of daily data grouped by
// category is tens of thousands of lines; the summary already carries the
// verdict, so the tail is truncated with a pointer to --json.
const universeHumanRowCap = 250

// universeRow is one date in the mirror, or one (date, group) pair when --by is
// set. FundCount is always the date's TOTAL width even on a grouped row, so a
// grouped row can be read on its own without re-joining the ungrouped table.
type universeRow struct {
	Date       string `json:"date"`
	FundCount  int    `json:"fund_count"`
	Group      string `json:"group,omitempty"`
	GroupCount int    `json:"group_count,omitempty"`
}

// universeSummary is the regime marker. min/max width are the load-bearing
// fields: a universe that silently narrows mid-window makes every rank,
// percentile and spread statistic computed across that window incomparable,
// and nothing in the per-date table shouts about it on its own.
type universeSummary struct {
	Resource      string  `json:"resource"`
	From          string  `json:"from"`
	To            string  `json:"to"`
	By            string  `json:"by"`
	Dates         int     `json:"dates"`
	FirstDate     string  `json:"first_date"`
	LastDate      string  `json:"last_date"`
	MinFundCount  int     `json:"min_fund_count"`
	MinDate       string  `json:"min_fund_count_date"`
	MaxFundCount  int     `json:"max_fund_count"`
	MaxDate       string  `json:"max_fund_count_date"`
	MeanFundCount float64 `json:"mean_fund_count"`
	WidthSpread   int     `json:"width_spread"`
	// DateFiltered is false for the pricing and ter resources, whose rows are
	// current reference data rather than a dated panel, so their counts are not
	// a reporting roster for the date they are stored under.
	DateFiltered bool `json:"date_filtered"`
	// WindowCovered is false when --from/--to were given and the FETCH LEDGER
	// has no entry for some date in that window, i.e. the width verdict below
	// describes less than what was asked for.
	//
	// This must come from the ledger, not from the stored rows. LoadMUFAPObs
	// already clips to [from, to], so the mirror's own first/last stored date
	// can never fall outside the window and comparing them to the endpoints
	// only asks "is there a row on exactly these two dates" -- false on a
	// complete mirror whenever an endpoint lands on a weekend or a holiday,
	// and false for essentially every ISO window on the month-keyed resources.
	WindowCovered bool `json:"window_covered"`
	// DatesNeverFetched counts calendar dates in the window with no ledger
	// entry at all. Zero alongside WindowCovered false means the window is
	// fetched but MUFAP published nothing on the missing dates.
	DatesNeverFetched int `json:"dates_never_fetched"`
}

type universeView struct {
	Summary universeSummary `json:"summary"`
	Rows    []universeRow   `json:"rows"`
}

func newNovelUniverseCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagResource string
	var flagBy string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "universe",
		Short: "Report how many funds reported on each date, by sector and category.",
		Long: `Report how many funds reported on each date in the local mirror.

Universe width is a regime marker. MUFAP's panel is not a fixed roster: funds
launch, mature and stop reporting, and whole categories appear only after a
rule change. Any cross-sectional statistic -- a rank, a percentile, a median
spread -- computed across dates of different width is not like-for-like, so
this command prints the narrowest and widest date in the window before it
prints anything else.

Reads the local mirror only; it never calls MUFAP. Populate the mirror with
` + "`backfill`" + ` first.`,
		Example: `  mufap-pp-cli universe --from 2026-09-01 --to 2026-09-04
  mufap-pp-cli universe --from 2026-01-01 --to 2026-09-04 --by category --agent
  mufap-pp-cli universe --resource monthly --by sector --json`,
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--from=2026-09-01;--to=2026-09-04",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "universe")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Every resource name is also a plausible bare argument, so
			// `universe daily-nav` would otherwise run silently against the
			// default resource and report the wrong universe's width.
			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("universe takes no positional arguments; got %q (did you mean --resource %s?)", args[0], args[0]))
			}

			// MUFAP's own date encodings vary by surface, but the mirror
			// stores one canonical ISO date, so the window is compared as a
			// plain string and must be exactly that shape.
			universeIsISODate := func(s string) bool {
				if len(s) != 10 || s[4] != '-' || s[7] != '-' {
					return false
				}
				for i := 0; i < len(s); i++ {
					if i == 4 || i == 7 {
						continue
					}
					if s[i] < '0' || s[i] > '9' {
						return false
					}
				}
				return true
			}
			if flagFrom != "" && !universeIsISODate(flagFrom) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %q is not a YYYY-MM-DD date", flagFrom))
			}
			if flagTo != "" && !universeIsISODate(flagTo) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to %q is not a YYYY-MM-DD date", flagTo))
			}
			if flagFrom != "" && flagTo != "" && flagFrom > flagTo {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s", flagFrom, flagTo))
			}

			// Accept the bare daily tab name as an alias: the store key is
			// "daily-returns" but the tab everyone names is "returns".
			resource := strings.ToLower(strings.TrimSpace(flagResource))
			knownResources := make([]string, 0, len(MUFAPDailyTabNames)+2)
			for _, tab := range MUFAPDailyTabNames {
				knownResources = append(knownResources, mufapResourceForTab(tab))
				if resource == tab {
					resource = mufapResourceForTab(tab)
				}
			}
			knownResources = append(knownResources, "monthly", "allocation")
			validResource := false
			for _, k := range knownResources {
				if resource == k {
					validResource = true
					break
				}
			}
			if !validResource {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--resource %q is unknown: want one of %s", flagResource, strings.Join(knownResources, ", ")))
			}

			// --by names a payload column, not a stored column: the grouping
			// value lives inside each row's JSON payload under MUFAP's own
			// header label.
			var groupField string
			switch strings.ToLower(strings.TrimSpace(flagBy)) {
			case "":
				groupField = ""
			case "sector":
				groupField = "Sector"
			case "category":
				groupField = "Category"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--by %q is unknown: want sector or category", flagBy))
			}

			// MEASURED: tab=pricing and tab=ter answer an explicit date with 551
			// rows where tab=returns answers with 388, because they are current
			// reference data rather than a dated panel. Their per-date row count
			// is therefore not a reporting roster and must not be read as a
			// universe width for that date.
			dateFiltered := MUFAPTabIsDateFiltered(strings.TrimPrefix(resource, "daily-"))

			// Shaped like a populated result so a machine caller parses one
			// schema whether or not the mirror exists yet -- same contract as
			// panelEmpty in panel.go. The summary is the whole reason this
			// command exists, so the empty paths carry a zeroed summary rather
			// than degrading to a bare [].
			universeEmpty := func() universeView {
				return universeView{
					Summary: universeSummary{
						Resource:     resource,
						From:         flagFrom,
						To:           flagTo,
						By:           strings.ToLower(strings.TrimSpace(flagBy)),
						DateFiltered: dateFiltered,
					},
					Rows: make([]universeRow, 0),
				}
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("mufap-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), universeEmpty(), flags)
				}
				return nil
			}

			s, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("universe: %w", err)
			}
			defer s.Close()

			obs, err := store.LoadMUFAPObs(ctx, s, resource, flagFrom, flagTo)
			if err != nil {
				// A database that exists but has never held a MUFAP backfill
				// is the same state as a missing file, not a failure. The
				// read-only handle cannot create the schema to prove it, so
				// the driver's missing-table error is the only signal.
				if strings.Contains(err.Error(), "no such table") {
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), universeEmpty(), flags)
					}
					return nil
				}
				return fmt.Errorf("universe: %w", err)
			}

			// Payloads decode through map[string]any rather than the daily
			// tabs' map[string]string because the allocation resource stores
			// JSON numbers alongside its text fields; a string-only decode
			// fails on the whole object and would drop every allocation row.
			universeGroupOf := func(payload string) string {
				var row map[string]any
				if json.Unmarshal([]byte(payload), &row) != nil {
					return universeUnknownGroup
				}
				v, ok := row[groupField]
				if !ok || v == nil {
					return universeUnknownGroup
				}
				var text string
				if str, isStr := v.(string); isStr {
					text = strings.TrimSpace(str)
				} else {
					text = strings.TrimSpace(fmt.Sprintf("%v", v))
				}
				if text == "" {
					return universeUnknownGroup
				}
				return text
			}

			// (resource, date, row_key) is the stored primary key, so keys are
			// already distinct per date; the set is kept anyway so a mirror
			// imported from elsewhere cannot inflate a date's width.
			widths := make(map[string]int)
			seen := make(map[string]map[string]bool)
			groupCounts := make(map[string]map[string]int)
			for _, o := range obs {
				keys, ok := seen[o.Date]
				if !ok {
					keys = make(map[string]bool)
					seen[o.Date] = keys
				}
				if keys[o.Key] {
					continue
				}
				keys[o.Key] = true
				widths[o.Date]++
				if groupField == "" {
					continue
				}
				byGroup, ok := groupCounts[o.Date]
				if !ok {
					byGroup = make(map[string]int)
					groupCounts[o.Date] = byGroup
				}
				byGroup[universeGroupOf(o.Payload)]++
			}

			dates := make([]string, 0, len(widths))
			for d := range widths {
				dates = append(dates, d)
			}
			sort.Strings(dates)

			rows := make([]universeRow, 0, len(dates))
			summary := universeEmpty().Summary
			summary.Dates = len(dates)
			total := 0
			for i, d := range dates {
				n := widths[d]
				total += n
				if i == 0 || n < summary.MinFundCount {
					summary.MinFundCount = n
					summary.MinDate = d
				}
				if i == 0 || n > summary.MaxFundCount {
					summary.MaxFundCount = n
					summary.MaxDate = d
				}
				if groupField == "" {
					rows = append(rows, universeRow{Date: d, FundCount: n})
					continue
				}
				names := make([]string, 0, len(groupCounts[d]))
				for g := range groupCounts[d] {
					names = append(names, g)
				}
				sort.Strings(names)
				for _, g := range names {
					rows = append(rows, universeRow{Date: d, FundCount: n, Group: g, GroupCount: groupCounts[d][g]})
				}
			}
			// Dates with no stored rows deliberately never enter widths -- a
			// never-fetched date must not be counted as a zero-width date --
			// which means the width verdict silently describes only what the
			// mirror happens to hold. Record whether that reaches the ends of
			// the requested window so the verdict can say when it does not.
			//
			// The ledger is the only thing that knows the difference between
			// "MUFAP published nothing that day" and "nobody ever asked", so
			// the verdict is drawn from it rather than from the row set.
			summary.WindowCovered = flagFrom == "" && flagTo == ""
			if flagFrom != "" && flagTo != "" {
				cov, covErr := store.LoadMUFAPCoverage(ctx, s, resource, flagFrom, flagTo)
				if covErr == nil {
					seen := make(map[string]bool, len(cov))
					for _, c := range cov {
						seen[c.Date] = true
					}
					missing := 0
					for _, d := range universeWindowDates(flagFrom, flagTo, resource) {
						if !seen[d] {
							missing++
						}
					}
					summary.DatesNeverFetched = missing
					summary.WindowCovered = missing == 0
				}
			}
			if len(dates) > 0 {
				summary.FirstDate = dates[0]
				summary.LastDate = dates[len(dates)-1]
				// Two decimals: a mean width of 314.6667 implies a precision
				// the underlying integer counts do not have.
				summary.MeanFundCount = math.Round(float64(total)/float64(len(dates))*100) / 100
				summary.WidthSpread = summary.MaxFundCount - summary.MinFundCount
			}

			view := universeView{Summary: summary, Rows: rows}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			out := cmd.OutOrStdout()
			window := "all stored dates"
			if flagFrom != "" || flagTo != "" {
				window = fmt.Sprintf("%s .. %s", universeOrDash(flagFrom), universeOrDash(flagTo))
			}
			if len(dates) == 0 {
				fmt.Fprintf(out, "universe width: %s\n", resource)
				fmt.Fprintf(out, "  no rows stored for %s over %s.\n", resource, window)
				fmt.Fprint(out, "  MUFAP publishes nothing on weekends and holidays, so an empty window can be correct.\n")
				fmt.Fprintf(out, "  check what was actually fetched: mufap-pp-cli coverage --resource %s\n", resource)
				return nil
			}

			fmt.Fprintf(out, "universe width: %s   %s .. %s   (%d dates)\n",
				resource, summary.FirstDate, summary.LastDate, summary.Dates)
			fmt.Fprintf(out, "  MIN WIDTH  %6d funds on %s\n", summary.MinFundCount, summary.MinDate)
			fmt.Fprintf(out, "  MAX WIDTH  %6d funds on %s\n", summary.MaxFundCount, summary.MaxDate)
			fmt.Fprintf(out, "  mean       %6.1f funds\n", summary.MeanFundCount)
			if !summary.DateFiltered {
				fmt.Fprintf(out, "\n  NOTE: %s is current reference data, not a dated panel: every stored date\n", resource)
				fmt.Fprint(out, "  repeats the full reference list, so these counts are not a reporting roster.\n")
			}
			switch {
			case summary.WidthSpread > 0 && summary.MaxFundCount > 0:
				pct := float64(summary.WidthSpread) / float64(summary.MaxFundCount) * 100
				fmt.Fprintf(out, "\n  width varies by %d funds (%.1f%% of the widest date) across this window.\n",
					summary.WidthSpread, pct)
				fmt.Fprint(out, "  Ranks, percentiles and spreads are NOT comparable across dates of different width.\n")
				if !summary.WindowCovered {
					fmt.Fprintf(out, "  Requested %s but the mirror holds only %s .. %s; the true spread may be wider.\n",
						window, summary.FirstDate, summary.LastDate)
				}
			case !summary.WindowCovered:
				// A "constant width" all-clear drawn from 2 of 200 requested
				// dates is the same failure this command exists to catch, just
				// inverted, so the claim is withheld until the window is filled.
				fmt.Fprintf(out, "\n  width is constant across the %d stored dates, but the mirror holds only\n", summary.Dates)
				fmt.Fprintf(out, "  %s .. %s of the requested %s -- not yet a like-for-like window.\n",
					summary.FirstDate, summary.LastDate, window)
				hint := "mufap-pp-cli coverage --resource " + resource
				if flagFrom != "" {
					hint += " --from " + flagFrom
				}
				if flagTo != "" {
					hint += " --to " + flagTo
				}
				fmt.Fprintf(out, "  see what is missing: %s\n", hint)
			default:
				fmt.Fprint(out, "\n  width is constant across this window; cross-sectional stats are like-for-like.\n")
			}
			fmt.Fprintln(out)

			if groupField == "" {
				fmt.Fprintf(out, "%-12s  %6s\n", "DATE", "FUNDS")
			} else {
				fmt.Fprintf(out, "%-12s  %6s  %-44s  %6s\n", "DATE", "FUNDS", strings.ToUpper(groupField), "COUNT")
			}
			shown := rows
			if len(shown) > universeHumanRowCap {
				shown = shown[:universeHumanRowCap]
			}
			for _, r := range shown {
				if groupField == "" {
					fmt.Fprintf(out, "%-12s  %6d\n", r.Date, r.FundCount)
					continue
				}
				fmt.Fprintf(out, "%-12s  %6d  %-44s  %6d\n", r.Date, r.FundCount, r.Group, r.GroupCount)
			}
			if len(rows) > len(shown) {
				fmt.Fprintf(cmd.ErrOrStderr(), "\nShowing %d of %d rows. For the full series: add --json.\n", len(shown), len(rows))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earliest date to report, YYYY-MM-DD (default: earliest stored)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Latest date to report, YYYY-MM-DD (default: latest stored)")
	cmd.Flags().StringVar(&flagResource, "resource", "daily-returns", "Mirrored resource: daily-returns|daily-nav|daily-pricing|daily-payout|daily-ter|monthly|allocation (bare tab names accepted)")
	cmd.Flags().StringVar(&flagBy, "by", "", "Break each date's count down by a payload column: sector or category")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// universeOrDash renders an open-ended window bound.
func universeOrDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// universeWindowDates lists the dates a resource is expected to carry across
// [from, to].
//
// The daily resources are keyed to calendar dates; "monthly" and "allocation"
// are mirrored under exactly one month-end per month, so walking day by day
// over those would report ~29 of every 30 dates as never-fetched on a fully
// mirrored resource and turn a complete mirror into a wall of fabricated holes.
func universeWindowDates(from, to, resource string) []string {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		return nil
	}
	out := make([]string, 0)
	if resource == "monthly" || resource == "allocation" {
		cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, time.UTC)
		for !cur.After(end) {
			monthEnd := time.Date(cur.Year(), cur.Month()+1, 0, 0, 0, 0, 0, time.UTC)
			if !monthEnd.Before(start) && !monthEnd.After(end) {
				out = append(out, monthEnd.Format("2006-01-02"))
			}
			cur = cur.AddDate(0, 1, 0)
		}
		return out
	}
	for cur := start; !cur.After(end); cur = cur.AddDate(0, 0, 1) {
		out = append(out, cur.Format("2006-01-02"))
	}
	return out
}
