// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
	"github.com/spf13/cobra"
)

// ratesReturnColumns are the period columns of the daily "returns" tab, in the
// order MUFAP renders them. --column is checked against this list because a
// header label that does not exist in the payload parses as "no value" on every
// row, which would report an empty series rather than a bad column name.
var ratesReturnColumns = []string{
	"YTD", "MTD", "1 Day", "15 Days", "30 Days", "90 Days",
	"180 Days", "270 Days", "365 Days", "2 Years", "3 Years",
}

// ratesResource is the mirrored resource the series is derived from; the
// coverage ledger is keyed by the same name.
const ratesResource = "daily-returns"

// ratesMinFunds is the narrowest cross-section a p10/p90 band can be read
// from. Below it the percentiles interpolate between two or three numbers and
// describe those particular funds, not the market -- MEDIAN = P10 = P90 on a
// single-fund date reads as a zero-dispersion policy rate when it is one
// manager's quote. Matches dispersionMinFunds: the statistic is the same one.
const ratesMinFunds = 3

// ratesPoint is one emitted date: the derived rate plus the mirror bookkeeping
// that says which kind of empty an empty date was.
//
// fund_count 0 alone is ambiguous across three genuinely different facts, and
// a machine consumer reading stdout cannot tell them apart without these:
//   - stored_rows 0                          -> MUFAP was asked and published
//     nothing (a weekend or holiday recovered from the coverage ledger);
//   - stored_rows > 0, malformed_rows 0      -> the date has rows, none of them
//     an annualized money-market fund;
//   - malformed_rows == stored_rows          -> a mirror-format mismatch, i.e.
//     a failed read, which must never be indistinguishable from a real zero.
type ratesPoint struct {
	mufap.RatePoint
	StoredRows    int `json:"stored_rows"`
	MalformedRows int `json:"malformed_rows"`
}

// ratesSeries reduces the mirror to one point per date, in date order.
//
// The coverage ledger is read alongside the observations because a date MUFAP
// published nothing on is stored with ZERO observation rows -- backfill saves
// fetched-and-empty deliberately, as that is what separates a holiday from a
// date nobody ever tried. Such a date exists only in mufap_coverage, so a
// series built from mufap_obs alone drops it silently, which is the one hole
// the fund_count contract exists to make visible.
func ratesSeries(column string, obs []store.MUFAPObs, ledger []store.MUFAPCoverage) []ratesPoint {
	decoded := make(map[string][]map[string]string, 32)
	stored := make(map[string]int, 32)
	malformed := make(map[string]int, 8)
	seen := make(map[string]bool, 32)
	dates := make([]string, 0, 32)

	for _, o := range obs {
		if o.Date == "" {
			continue
		}
		if !seen[o.Date] {
			seen[o.Date] = true
			dates = append(dates, o.Date)
		}
		stored[o.Date]++
		// A payload that no longer unmarshals is a mirror-format mismatch, not
		// a yield of zero. Drop the row but keep the date, and carry the count
		// into the point so the loss is legible on stdout too.
		var row map[string]string
		if err := json.Unmarshal([]byte(o.Payload), &row); err != nil {
			malformed[o.Date]++
			continue
		}
		decoded[o.Date] = append(decoded[o.Date], row)
	}
	for _, c := range ledger {
		if c.Date == "" || seen[c.Date] {
			continue
		}
		seen[c.Date] = true
		dates = append(dates, c.Date)
	}
	sort.Strings(dates)

	points := make([]ratesPoint, 0, len(dates))
	for _, d := range dates {
		points = append(points, ratesPoint{
			RatePoint:     mufap.DeriveRate(d, column, decoded[d]),
			StoredRows:    stored[d],
			MalformedRows: malformed[d],
		})
	}
	return points
}

func newNovelRatesCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagColumn string
	var flagDB string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "rates",
		Short: "Derive a daily short-term interest rate series from the cross-section of money-market fund yields.",
		Long: `Derive a market-implied PKR short rate from the local mirror.

For each stored date, the annualized money-market funds' return in --column are
reduced to a median with a p10/p90 band. Money-market funds hold short government
paper and bank placements, so their cross-sectional median tracks the policy rate
without waiting on an SBP release.

fund_count travels with every point: a median over three funds and a median over
thirty are not the same observation, and fewer than three is flagged because the
band is then the gap between two quotes.

Every date the mirror has a record of stays in the JSON, including the dates
MUFAP published nothing on: those carry fund_count 0, and stored_rows /
malformed_rows say whether the date was fetched-and-empty, held no money-market
fund, or failed to decode. The human table omits them and reports the count.

Reads the local mirror only. Populate it first with 'backfill daily'.`,
		Example: `  mufap-pp-cli rates --from 2026-09-01 --to 2026-09-04 --agent
  mufap-pp-cli rates --from 2026-01-01 --column "365 Days" --json
  mufap-pp-cli rates --from 2024-01-01 --limit 30`,
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
				return writeDryRun(cmd.OutOrStdout(), flags, "rates")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Match case-insensitively but resolve back to the canonical label:
			// the stored payload's keys are the rendered header text verbatim,
			// so "30 days" must become "30 Days" before it reaches DeriveRate.
			column := ""
			for _, candidate := range ratesReturnColumns {
				if strings.EqualFold(candidate, strings.TrimSpace(flagColumn)) {
					column = candidate
					break
				}
			}
			if column == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--column %q is not a returns column; want one of: %s",
					flagColumn, strings.Join(ratesReturnColumns, ", ")))
			}

			for _, bound := range []struct{ name, value string }{{"--from", flagFrom}, {"--to", flagTo}} {
				if bound.value == "" {
					continue
				}
				if _, err := time.Parse("2006-01-02", bound.value); err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("%s %q is not a YYYY-MM-DD date", bound.name, bound.value))
				}
			}
			if flagFrom != "" && flagTo != "" && flagFrom > flagTo {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s", flagFrom, flagTo))
			}
			if flagLimit < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--limit must be 0 (no cap) or a positive count"))
			}

			// One warning channel for both audiences: the machine form is
			// marshalled rather than assembled with %q so a column label or
			// path containing a quote cannot break the line.
			ratesWarn := func(human string, event map[string]any) {
				w := cmd.ErrOrStderr()
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					if raw, err := json.Marshal(event); err == nil {
						fmt.Fprintf(w, "%s\n", raw)
						return
					}
				}
				fmt.Fprintln(w, human)
			}

			dbPath := strings.TrimSpace(flagDB)
			if dbPath == "" {
				dbPath = defaultDBPath("mufap-pp-cli")
			}
			ratesEmptyMirror := func(reason string) error {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", reason)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]ratesPoint, 0), flags)
				}
				return nil
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return ratesEmptyMirror(fmt.Sprintf("no local mirror at %s", dbPath))
			}

			s, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("rates: open local mirror %s: %w", dbPath, err)
			}
			defer func() { _ = s.Close() }()

			obs, err := store.LoadMUFAPObs(ctx, s, ratesResource, flagFrom, flagTo)
			if err != nil {
				// The mirror file is shared with the platform's own learn/teach
				// tables, so data.db routinely exists with no MUFAP schema in
				// it at all. A read-only handle cannot create that schema, and
				// an unpopulated cache is a state, not a failure: surfacing the
				// raw "no such table: mufap_obs" would hide the one command
				// that fixes it.
				if strings.Contains(err.Error(), "no such table") {
					return ratesEmptyMirror(fmt.Sprintf("no MUFAP observations in %s", dbPath))
				}
				return fmt.Errorf("rates: %w", err)
			}

			ledger, err := store.LoadMUFAPCoverage(ctx, s, ratesResource, flagFrom, flagTo)
			if err != nil {
				if !strings.Contains(err.Error(), "no such table") {
					return fmt.Errorf("rates: %w", err)
				}
				// No ledger means no record of the fetched-and-empty dates; the
				// observation dates are still a true, if narrower, series.
				ledger = nil
			}

			points := ratesSeries(column, obs, ledger)
			// A rate series is read from its recent end, so --limit keeps the
			// tail rather than truncating away the newest observations.
			//
			// The budget is counted in dates that CARRY A RATE, not in emitted
			// rows. The series deliberately includes fetched-and-empty dates so
			// a hole stays visible, but spending the limit on them would let a
			// weekend pair or an Eid cluster consume the whole window and print
			// "no yields in the local mirror" against a well-populated one.
			// Holes inside the kept span still travel with it; only holes older
			// than the Nth rate-bearing date are dropped.
			if flagLimit > 0 {
				kept := 0
				cut := -1
				for i := len(points) - 1; i >= 0; i-- {
					if points[i].FundCount > 0 {
						kept++
						if kept == flagLimit {
							cut = i
							break
						}
					}
				}
				if cut > 0 {
					points = points[cut:]
				}
			}

			malformed := 0
			malformedDates := 0
			thin := 0
			holes := 0
			for _, p := range points {
				if p.MalformedRows > 0 {
					malformed += p.MalformedRows
					malformedDates++
				}
				switch {
				case p.FundCount == 0:
					holes++
				case p.FundCount < ratesMinFunds:
					thin++
				}
			}
			if malformed > 0 {
				ratesWarn(
					fmt.Sprintf("warning: skipped %d mirrored row(s) across %d date(s) whose payload did not decode; malformed_rows names them per date", malformed, malformedDates),
					map[string]any{
						"event":           "malformed_payloads",
						"rows_skipped":    malformed,
						"dates_affected":  malformedDates,
						"dates_emitted":   len(points),
						"message":         "a payload that did not decode is a mirror-format mismatch, not a yield of zero; per-date counts are in malformed_rows",
						"suggested_fixup": "mufap-pp-cli backfill daily --from <date> --to <date> --force",
					})
			}
			if thin > 0 {
				ratesWarn(
					fmt.Sprintf("warning: %d of %d date(s) have fewer than %d money-market funds reporting; on those dates the p10/p90 band is the distance between a couple of funds, not a market-wide rate", thin, len(points), ratesMinFunds),
					map[string]any{
						"event":               "thin_cross_section",
						"dates_below_minimum": thin,
						"dates_emitted":       len(points),
						"minimum_fund_count":  ratesMinFunds,
						"column":              column,
						"message":             "the p10/p90 band on these dates is the distance between a couple of funds, not a market-wide rate",
					})
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), points, flags)
			}

			window := "the whole mirror"
			switch {
			case flagFrom != "" && flagTo != "":
				window = flagFrom + ".." + flagTo
			case flagFrom != "":
				window = "on or after " + flagFrom
			case flagTo != "":
				window = "on or before " + flagTo
			}

			out := cmd.OutOrStdout()
			rows := make([][]string, 0, len(points))
			for _, p := range points {
				// MUFAP publishes nothing on many weekend and holiday dates,
				// and a stored date can still hold no annualized money-market
				// fund. Such a date has no rate to print, but it stays in the
				// JSON with fund_count 0 so the hole remains visible there.
				if p.FundCount == 0 {
					continue
				}
				funds := strconv.Itoa(p.FundCount)
				if p.FundCount < ratesMinFunds {
					funds += " *"
				}
				rows = append(rows, []string{
					p.Date,
					strconv.FormatFloat(p.MedianYield, 'f', 2, 64),
					strconv.FormatFloat(p.P10, 'f', 2, 64),
					strconv.FormatFloat(p.P90, 'f', 2, 64),
					funds,
				})
			}
			if len(rows) == 0 {
				fmt.Fprintf(out, "no annualized money-market yields in the local mirror for %q over %s\n", column, window)
				if holes > 0 {
					// The mirror IS populated for these dates -- they are
					// fetched-and-empty or hold nothing money-market -- so
					// re-running backfill would re-fetch the same emptiness.
					// Sending the reader there would waste the request.
					fmt.Fprintf(out, "%d date(s) are mirrored with no annualized money-market fund; --json lists them with fund_count 0 and stored_rows saying whether MUFAP published anything at all\n", holes)
					return nil
				}
				fmt.Fprintf(out, "run: mufap-pp-cli backfill daily --from <date> --to <date>\n")
				return nil
			}
			if err := flags.printTable(cmd, []string{"DATE", "MEDIAN", "P10", "P90", "FUNDS"}, rows); err != nil {
				return err
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d date(s), median %s return of annualized money-market funds\n", len(rows), column)
			if thin > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "* fewer than %d funds reporting; that date's band is not a market-wide fact\n", ratesMinFunds)
			}
			if holes > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d date(s) had no reporting money-market fund and are omitted here; --json lists them with fund_count 0\n", holes)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "Earliest validity date to include (YYYY-MM-DD; open-ended when unset)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Latest validity date to include (YYYY-MM-DD; open-ended when unset)")
	cmd.Flags().StringVar(&flagColumn, "column", "30 Days", "Return column to derive the rate from: "+strings.Join(ratesReturnColumns, ", "))
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Keep only the most recent N dates that carry a rate; holes inside that span travel with it (0 = no cap)")
	return cmd
}
