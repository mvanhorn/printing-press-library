// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/spf13/cobra"
)

// freshnessRow is one fund in MUFAP's latest-available-per-fund view.
//
// LagDays is reference-minus-validity in whole days, where the reference is
// the summary's lag_reference_date (the modal validity date). It is SIGNED:
// a negative value means the row is priced AHEAD of the reference, which is a
// real MUFAP behaviour, not an error -- MEASURED 2026-09-06: 42 of 551 rows
// carried a validity date later than the modal one, 22 of them a full day
// past the calendar date of the fetch. A forward-priced NAV is ahead of the
// market, so it must never be reported as stale.
type freshnessRow struct {
	Fund         string `json:"fund"`
	Category     string `json:"category"`
	ValidityDate string `json:"validity_date"`
	LagDays      int    `json:"lag_days"`
}

// freshnessSummary describes the whole response, not any one fund.
//
// THE ANCHOR. Lag is measured against LagReferenceDate, which is the MODAL
// validity date -- the date carried by more rows than any other. It is not
// the newest date and not wall-clock time, and both exclusions are deliberate:
//
//   - Wall-clock would make the command irreproducible and would move the
//     answer over weekends and holidays.
//   - The NEWEST date is wrong because MUFAP forward-prices part of the
//     industry. MEASURED 2026-09-06: newest = 2026-09-07, modal = 2026-09-04.
//     Anchoring on the newest date reported 529 of 551 rows as lagging (96%),
//     gave the 345 entirely current modal-date funds a lag of 3 days, and gave
//     the 8 funds priced on the day of the fetch a lag of 1 day. A fund
//     published today cannot be one day stale.
//
// THE PARTITION. ForwardDatedFunds, FundsOnReferenceDate and LaggingFunds are
// measured against that one reference and are a genuine partition of
// RowsWithValidityDate -- ahead of it, on it, behind it -- so a consumer can
// add them up. They previously used two different anchors, which double-counted
// the 20 rows between the modal and newest dates and overflowed the universe.
// RowsWithValidityDate + RowsMissingValidityDate == RowsFetched.
//
// MinLagDaysThreshold is the ECHOED --min-lag-days flag, a filter and not a
// measurement; the measured extremes are MaxLagDays and MinLagDaysObserved.
// RowsMatchingFilter is how many rows cleared the threshold and Returned how
// many survived --limit; neither is a count of stale funds.
//
// RowsWithValidityDate counts ROWS, not funds. MEASURED on 2026-09-04: 388
// rows of tab=returns collapse to 339 distinct fund names, because VPS pension
// funds legitimately reuse one name across three sub-funds. DistinctFunds
// therefore counts Sector|Category|Fund Name keys, and the two are reported
// side by side rather than one standing in for the other.
type freshnessSummary struct {
	Tab                     string `json:"tab"`
	View                    string `json:"view"`
	DateColumn              string `json:"date_column"`
	RowsFetched             int    `json:"rows_fetched"`
	RowsWithValidityDate    int    `json:"rows_with_validity_date"`
	DistinctFunds           int    `json:"distinct_funds"`
	UniverseWidthReliable   bool   `json:"universe_width_reliable"`
	DistinctValidityDates   int    `json:"distinct_validity_dates"`
	NewestValidityDate      string `json:"newest_validity_date"`
	OldestValidityDate      string `json:"oldest_validity_date"`
	ModalValidityDate       string `json:"modal_validity_date"`
	LagReferenceDate        string `json:"lag_reference_date"`
	LagReferenceBasis       string `json:"lag_reference_basis"`
	MaxLagDays              int    `json:"max_lag_days"`
	MinLagDaysObserved      int    `json:"min_lag_days_observed"`
	ForwardDatedFunds       int    `json:"forward_dated_funds"`
	FundsOnReferenceDate    int    `json:"funds_on_reference_date"`
	LaggingFunds            int    `json:"lagging_funds"`
	MinLagDaysThreshold     int    `json:"min_lag_days_threshold"`
	RowsMatchingFilter      int    `json:"rows_matching_filter"`
	Returned                int    `json:"returned"`
	RowsMissingValidityDate int    `json:"rows_missing_validity_date"`
	RowsMissingFundName     int    `json:"rows_missing_fund_name"`
}

type freshnessView struct {
	Summary freshnessSummary `json:"summary"`
	Rows    []freshnessRow   `json:"rows"`
}

func newNovelFreshnessCmd(flags *rootFlags) *cobra.Command {
	var flagTab string
	var flagLimit int
	var flagMinLagDays int

	cmd := &cobra.Command{
		Use:   "freshness",
		Short: "Audit MUFAP's undated latest-per-fund view for funds behind the industry's modal validity date.",
		Long: "Audit how stale MUFAP's default daily view is.\n\n" +
			"Requesting a daily tab without a date returns MUFAP's \"latest available\n" +
			"per fund\" fallback: every fund is filled with its most recently published\n" +
			"row, so one response mixes many validity dates. That view is useful for\n" +
			"spotting funds that stopped reporting, and dangerous as a data source --\n" +
			"differencing it as a single day's panel silently compares rows that are\n" +
			"weeks or years apart.\n\n" +
			"The command takes no date. Lag is measured against the MODAL validity date\n" +
			"in the same response -- the date more rows carry than any other, reported as\n" +
			"lag_reference_date -- never against wall-clock time and never against the\n" +
			"newest date. Wall-clock would make the answer irreproducible and would drift\n" +
			"over weekends and holidays. The newest date is worse: MUFAP forward-prices\n" +
			"part of the industry, so on 2026-09-06 the newest validity date was\n" +
			"2026-09-07 and anchoring on it reported 96% of a current universe as stale,\n" +
			"including funds published that very morning.\n\n" +
			"forward_dated_funds, funds_on_reference_date and lagging_funds are measured\n" +
			"against that single reference and partition rows_with_validity_date exactly,\n" +
			"so they add up. Row lag_days is signed: negative means the fund is priced\n" +
			"AHEAD of the reference date. Forward-dated funds are therefore counted in the\n" +
			"summary but never listed as lagging.\n\n" +
			"min_lag_days_threshold echoes --min-lag-days and is a filter, not a finding;\n" +
			"the measured extremes are max_lag_days and min_lag_days_observed.\n\n" +
			"tab=payout carries no Validity Date column; its dates are read from\n" +
			"Payout Date instead, and the column actually used is reported as\n" +
			"date_column. tab=pricing and tab=ter are current reference data rather\n" +
			"than a dated panel, so their row count is not a universe width.",
		Example: "  mufap-pp-cli freshness --limit=5\n" +
			"  mufap-pp-cli freshness --tab nav --min-lag-days 7 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "freshness")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("freshness takes no positional arguments; got %q", args[0]))
			}

			tab := strings.ToLower(strings.TrimSpace(flagTab))
			knownTab := false
			for _, name := range MUFAPDailyTabNames {
				if name == tab {
					knownTab = true
					break
				}
			}
			if !knownTab {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tab %q is not one of %s", flagTab, strings.Join(MUFAPDailyTabNames, ", ")))
			}
			if flagLimit <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be greater than 0"))
			}
			// A negative threshold is meaningful here: lag is signed against
			// the modal date, so a forward-priced fund carries a NEGATIVE lag.
			// Rejecting negatives made those rows unlistable at any flag value
			// -- on --tab payout that hid the 115 most RECENT payouts, which
			// are exactly the rows a reader wants.

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// The empty date is the whole point of this command: it selects
			// MUFAP's latest-available-per-fund fallback, the only view that
			// exposes per-fund staleness. An explicit date would return a
			// clean single-validity-date panel with nothing to audit.
			table, err := mufapFetchDaily(ctx, c, tab, "")
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			// Resolve columns by label rather than by position: MUFAP's five
			// daily tabs share a header prefix but not a full header list, and
			// a positional read would key every row to the wrong cell if a tab
			// gains a column.
			//
			// Both probes are the package-shared ones on purpose. MEASURED: the
			// name column is "Fund Name" only on tab=returns and plain "Fund" on
			// nav, pricing, payout and ter, and tab=payout has no Validity Date
			// column at all -- its date lives in "Payout Date". A resolver that
			// looks for one spelling returns "", every row then reads the
			// zero-value map key, and the command emits blank funds or blames a
			// permanent column difference on an upstream page change.
			dateCol := MUFAPValidityColumn(table.Headers)
			if dateCol == "" && len(table.Rows) > 0 {
				return fmt.Errorf("tab %s carries neither a Validity Date nor a Payout Date column (headers: %s): page shape changed", tab, strings.Join(table.Headers, ", "))
			}
			// tab=pricing and tab=ter are current reference data, not a dated
			// panel, so their row count is not a roster for any single date.
			// The caveat travels with the payload, as in panel.go.
			widthReliable := MUFAPTabIsDateFiltered(tab)

			type freshnessObs struct {
				fund     string
				category string
				date     string
			}
			obs := make([]freshnessObs, 0, len(table.Rows))
			dateCounts := make(map[string]int)
			// Sector|Category|Fund Name, because fund NAME alone is not unique:
			// VPS pension funds reuse one name across three sub-funds.
			fundKeys := make(map[string]struct{})
			missing := 0
			missingName := 0
			for _, r := range table.Rows {
				iso, ok := mufap.NormalizeValidityDate(r[dateCol])
				if !ok {
					// A blank, "-" or "N/A" validity cell is a fund MUFAP
					// lists but has never priced; it has no lag to report.
					missing++
					continue
				}
				name := MUFAPRowName(r)
				if name == "" {
					// Counted, never silently rendered as a blank fund: an
					// unidentifiable row must not read as a real one.
					missingName++
				}
				if key := MUFAPRowKey(r); key != "" {
					fundKeys[key] = struct{}{}
				}
				obs = append(obs, freshnessObs{
					fund:     name,
					category: strings.TrimSpace(r["Category"]),
					date:     iso,
				})
				dateCounts[iso]++
			}

			// ISO dates sort lexically, so newest/oldest/modal need no parsing.
			// Ties on the modal count resolve to the later date.
			newest, oldest, modal := "", "", ""
			modalCount := 0
			for d, n := range dateCounts {
				if newest == "" || d > newest {
					newest = d
				}
				if oldest == "" || d < oldest {
					oldest = d
				}
				if n > modalCount || (n == modalCount && d > modal) {
					modal, modalCount = d, n
				}
			}

			// THE REFERENCE. The modal validity date is the industry's own
			// clock: it is derived from the data (so the command stays
			// reproducible and clock-independent) but, unlike the newest date,
			// it is not dragged into the future by MUFAP's forward-priced
			// funds. MEASURED 2026-09-06: newest 2026-09-07 vs modal
			// 2026-09-04, and anchoring on the newest date called the 345
			// funds sitting on the modal date 3 days stale.
			reference := modal
			referenceBasis := "modal_validity_date"

			// Lag is reference-minus-row, parsed in UTC so no DST shift can
			// move a day boundary, and SIGNED so a forward-priced fund reads
			// as ahead of the reference rather than as fresh-but-stale.
			freshnessLagDays := func(date string) int {
				if date == "" || reference == "" {
					return 0
				}
				a, errA := time.Parse("2006-01-02", date)
				b, errB := time.Parse("2006-01-02", reference)
				if errA != nil || errB != nil {
					return 0
				}
				return int(b.Sub(a).Hours() / 24)
			}

			rows := make([]freshnessRow, 0, len(obs))
			maxLag, minLag := 0, 0
			// The three buckets partition obs: ahead of the reference, on it,
			// behind it. One anchor for all three, so they sum to len(obs).
			ahead, onReference, behind := 0, 0, 0
			for i, o := range obs {
				lag := freshnessLagDays(o.date)
				if i == 0 || lag > maxLag {
					maxLag = lag
				}
				if i == 0 || lag < minLag {
					minLag = lag
				}
				switch {
				case lag < 0:
					ahead++
				case lag == 0:
					onReference++
				default:
					behind++
				}
				// --min-lag-days 0 means "every dated row", forward-priced rows
				// included; a positive threshold filters to staleness only.
				if lag >= flagMinLagDays || flagMinLagDays <= 0 {
					rows = append(rows, freshnessRow{
						Fund:         o.fund,
						Category:     o.category,
						ValidityDate: o.date,
						LagDays:      lag,
					})
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].LagDays != rows[j].LagDays {
					return rows[i].LagDays > rows[j].LagDays
				}
				return rows[i].Fund < rows[j].Fund
			})
			matched := len(rows)
			if len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}

			view := freshnessView{
				Summary: freshnessSummary{
					Tab:                     tab,
					View:                    "latest-available-per-fund",
					DateColumn:              dateCol,
					RowsFetched:             len(table.Rows),
					RowsWithValidityDate:    len(obs),
					DistinctFunds:           len(fundKeys),
					UniverseWidthReliable:   widthReliable,
					DistinctValidityDates:   len(dateCounts),
					NewestValidityDate:      newest,
					OldestValidityDate:      oldest,
					ModalValidityDate:       modal,
					LagReferenceDate:        reference,
					LagReferenceBasis:       referenceBasis,
					MaxLagDays:              maxLag,
					MinLagDaysObserved:      minLag,
					ForwardDatedFunds:       ahead,
					FundsOnReferenceDate:    onReference,
					LaggingFunds:            behind,
					MinLagDaysThreshold:     flagMinLagDays,
					RowsMatchingFilter:      matched,
					Returned:                len(rows),
					RowsMissingValidityDate: missing,
					RowsMissingFundName:     missingName,
				},
				Rows: rows,
			}

			out := cmd.OutOrStdout()
			if !wantsHumanTable(out, flags) {
				// printJSONFiltered pins meta.source to "local", but this
				// command has no local path at all: every row below was just
				// fetched from MUFAP's undated latest-per-fund view.
				// MEASURED 2026-09-07: 551 live-fetched rows were reported
				// as meta.source "local".
				raw, marshalErr := json.Marshal(view)
				if marshalErr != nil {
					return marshalErr
				}
				return printOutputWithFlagsMeta(out, json.RawMessage(raw), flags, map[string]any{"source": "live"})
			}

			fmt.Fprintf(out, "MUFAP daily %s -- latest available per fund (no date requested)\n", tab)
			fmt.Fprintln(out, "This is a fallback view, NOT one day's panel: MUFAP fills each fund with its")
			fmt.Fprintln(out, "most recently published row, so these rows carry many different validity dates.")
			fmt.Fprintln(out, "Do not difference it as a single daily observation -- request an explicit date")
			fmt.Fprintln(out, "for that. Lag below is measured against the MODAL validity date in this same")
			fmt.Fprintln(out, "response -- the industry's own clock -- not against today and not against the")
			fmt.Fprintln(out, "newest date, which MUFAP's forward-priced funds push into the future.")
			if dateCol != "" && !strings.EqualFold(dateCol, "Validity Date") {
				fmt.Fprintf(out, "This tab has no Validity Date column; dates below are read from %q.\n", dateCol)
			}
			if !widthReliable {
				fmt.Fprintln(out, "This tab is current reference data, not a dated panel, so its row count")
				fmt.Fprintln(out, "is not a universe width for any single date.")
			}
			fmt.Fprintln(out)

			if view.Summary.RowsWithValidityDate == 0 {
				fmt.Fprintln(out, "No fund in this response carried a readable validity date.")
				if missing > 0 {
					fmt.Fprintf(out, "%d row(s) had a blank or non-date validity cell.\n", missing)
				} else {
					// Not a weekend story: this view is the undated
					// latest-available-per-fund fallback, which is served on
					// non-trading days too. Nothing at all means MUFAP served
					// no rows -- an outage or a page change.
					fmt.Fprintln(out, "MUFAP returned no rows at all for this tab. The undated view is served on")
					fmt.Fprintln(out, "non-trading days too, so this is an outage or a page change, not a holiday.")
				}
				return nil
			}

			fmt.Fprintf(out, "rows with a date:        %d of %d fetched\n", view.Summary.RowsWithValidityDate, view.Summary.RowsFetched)
			fmt.Fprintf(out, "distinct funds:          %d\n", view.Summary.DistinctFunds)
			fmt.Fprintf(out, "distinct validity dates: %d\n", view.Summary.DistinctValidityDates)
			fmt.Fprintf(out, "newest / oldest:         %s / %s\n", newest, oldest)
			fmt.Fprintf(out, "lag reference date:      %s (%s, %d rows)\n", reference, referenceBasis, modalCount)
			fmt.Fprintf(out, "lag range observed:      %d to %d day(s) (negative = priced ahead of the reference)\n", minLag, maxLag)
			fmt.Fprintln(out)
			fmt.Fprintln(out, "Split of the dated rows against that one reference (these three add up):")
			fmt.Fprintf(out, "  ahead of it  (forward-dated): %d\n", ahead)
			fmt.Fprintf(out, "  on it        (current):       %d\n", onReference)
			fmt.Fprintf(out, "  behind it    (lagging):       %d\n", behind)
			fmt.Fprintf(out, "  total dated rows:             %d\n", view.Summary.RowsWithValidityDate)
			if missing > 0 {
				fmt.Fprintf(out, "rows without a date:     %d\n", missing)
			}
			if missingName > 0 {
				fmt.Fprintf(out, "rows without a name:     %d\n", missingName)
			}
			fmt.Fprintln(out)

			if matched == 0 {
				fmt.Fprintf(out, "No row lags the reference date %s by %d day(s) or more.\n", reference, flagMinLagDays)
				return nil
			}

			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "FUND\tCATEGORY\tVALIDITY\tLAG (DAYS)")
			for _, r := range rows {
				// A row whose tab carries no recognised name column would
				// otherwise render as an empty cell indistinguishable from a
				// fund genuinely called nothing.
				fund := r.Fund
				if fund == "" {
					fund = "(no fund name column)"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%d\n", fund, r.Category, r.ValidityDate, r.LagDays)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(out, "\nShowing %d of %d row(s) matching --min-lag-days >= %d. Raise --limit to see more.\n",
				len(rows), matched, flagMinLagDays)
			if flagMinLagDays == 0 {
				fmt.Fprintf(out, "--min-lag-days 0 includes the %d row(s) sitting ON the reference date, which are\n", onReference)
				fmt.Fprintln(out, "current, not stale. The stale count is the 'behind it' line above.")
			}
			if ahead > 0 {
				fmt.Fprintf(out, "%d forward-dated row(s) are priced ahead of the reference and are never listed as lagging.\n", ahead)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagTab, "tab", "returns", "daily tab to audit: "+strings.Join(MUFAPDailyTabNames, ", "))
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "maximum stale funds to list, most stale first")
	cmd.Flags().IntVar(&flagMinLagDays, "min-lag-days", 1, "list only funds lagging lag_reference_date (the modal validity date) by at least this many days; a filter, echoed back as min_lag_days_threshold")
	return cmd
}
