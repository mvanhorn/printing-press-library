// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// dispersionReturnColumns are the return-bearing header labels of the daily
// "returns" tab, in the order MUFAP renders them.
//
// A stored row's payload is keyed by the header label verbatim, spaces and
// all ("30 Days", never "30d"), so a --column value that is not one of these
// reads an absent map key and reports fund_count 0 on every date -- which is
// indistinguishable from a category that genuinely never reported. Validating
// against this list turns that silent empty series into a usage error.
//
// NAV is excluded deliberately: it is a price level, not a return, and its
// cross-sectional spread measures unit denomination rather than performance.
//
// This list is byte-identical to ratesReturnColumns in rates.go, and the two
// belong in internal/mufap next to the payload-key knowledge so a MUFAP header
// rename lands once instead of twice. That consolidation edits two other files
// and is left to a single-owner change.
var dispersionReturnColumns = []string{
	"YTD", "MTD", "1 Day", "15 Days", "30 Days",
	"90 Days", "180 Days", "270 Days", "365 Days",
	"2 Years", "3 Years",
}

// dispersionMinFunds is the narrowest cross-section a p90-p10 spread can be
// read from. Below it the percentiles interpolate between two or three
// numbers and describe those particular funds, not the category.
const dispersionMinFunds = 3

// dispersionSeriesResult is the derived series plus the counts a reader needs
// to tell a genuinely thin category from a lossy mirror.
//
// MalformedRows and DecodeLostDates exist because a payload that no longer
// decodes is a mirror-format mismatch, not a fund that returned zero. Without
// them a partial decode failure silently narrows fund_count, and a date whose
// rows all failed to decode disappears from the series -- where the command's
// own help text has already trained the reader that a missing date means MUFAP
// published nothing that day.
type dispersionSeriesResult struct {
	Points          []mufap.DispersionPoint
	Pooled          []string
	StoredRows      int
	MalformedRows   int
	DecodeLostDates int
	ThinDates       int
}

// dispersionSeries reduces mirrored observation rows to one point per date.
//
// Dates with nothing to say are dropped from Points but not from the counts:
// a date lost entirely to decode failures is tallied in DecodeLostDates so it
// is never reported as a market holiday.
func dispersionSeries(category, column string, obs []store.MUFAPObs) dispersionSeriesResult {
	res := dispersionSeriesResult{StoredRows: len(obs)}

	rowsByDate := make(map[string][]map[string]string, 32)
	dates := make([]string, 0, 32)
	for _, o := range obs {
		// Register the date BEFORE attempting the decode. Registering after it
		// meant a date whose every row failed to decode never entered the date
		// list at all and vanished without a trace; keeping it lets the loop
		// below separate a mirror defect from a day MUFAP did not publish.
		if _, seen := rowsByDate[o.Date]; !seen {
			dates = append(dates, o.Date)
			rowsByDate[o.Date] = make([]map[string]string, 0, 32)
		}
		var row map[string]string
		if err := json.Unmarshal([]byte(o.Payload), &row); err != nil {
			res.MalformedRows++
			continue
		}
		rowsByDate[o.Date] = append(rowsByDate[o.Date], row)
	}
	sort.Strings(dates)

	pooled := make(map[string]struct{}, 8)
	res.Points = make([]mufap.DispersionPoint, 0, len(dates))
	for _, d := range dates {
		rows := rowsByDate[d]
		pt := mufap.DeriveDispersion(d, category, column, rows)
		// A date on which nothing in the category reported has no spread, but
		// it is still an OBSERVED date and must stay in the series with
		// fund_count 0 -- `rates` already keeps its zero-fund dates for the
		// same reason. Dropping it made a real measurement look like missing
		// data: the command emitted [] and told the reader to run `backfill`,
		// which would have re-fetched the very rows it had just discarded.
		//
		// The zeroed percentiles are not a claim about the market: fund_count 0
		// travels with them and is what a consumer must gate on.
		if pt.FundCount == 0 {
			if len(rows) == 0 {
				res.DecodeLostDates++
			}
			res.Points = append(res.Points, pt)
			continue
		}
		// Mirrors DeriveDispersion's own filter so the caller can name the
		// MUFAP labels that actually contributed values. DispersionPoint.
		// Category echoes the user's raw substring, so without this nothing in
		// the output reveals what was pooled.
		for _, r := range rows {
			label := r["Category"]
			if label == "" || !strings.Contains(strings.ToLower(label), strings.ToLower(category)) {
				continue
			}
			if _, ok := mufap.ParseNumber(r[column]); !ok {
				continue
			}
			pooled[label] = struct{}{}
		}
		if pt.FundCount < dispersionMinFunds {
			res.ThinDates++
		}
		res.Points = append(res.Points, pt)
	}

	res.Pooled = make([]string, 0, len(pooled))
	for label := range pooled {
		res.Pooled = append(res.Pooled, label)
	}
	sort.Strings(res.Pooled)
	return res
}

func newNovelDispersionCmd(flags *rootFlags) *cobra.Command {
	var flagCategory string
	var flagColumn string
	var flagFrom string
	var flagTo string
	var flagDB string

	cmd := &cobra.Command{
		Use:   "dispersion",
		Short: "Cross-sectional spread of fund returns within a category on each date.",
		Long: `Measures how far apart the funds in one MUFAP category are on each date.

For every stored validity date in the range, the funds whose category matches
--category are collected and the requested return column is reduced to a
median, a 10th and a 90th percentile; spread_p90_p10 is the gap between the
tails. A widening spread means manager outcomes are diverging inside a
category that is nominally one asset class.

Reads only the local mirror written by 'backfill daily' -- it never calls
MUFAP. Dates on which no fund in the category reported are omitted rather
than emitted as a zero spread: MUFAP publishes nothing on weekends and
holidays, and a zero there is an absence, not a flat market. A date missing
because its mirrored rows failed to decode is counted and reported separately
on stderr, so a mirror defect is never read as a market holiday.

fund_count travels on every row. Percentiles over fewer than three funds are
the distance between two funds; the command says so on stderr rather than
silently letting a two-fund gap be read as a market fact.`,
		Example: `  mufap-pp-cli dispersion --from 2026-09-01 --to 2026-09-04 --category Equity
  mufap-pp-cli dispersion --from 2026-01-01 --to 2026-09-04 --category "Money Market" --column "30 Days" --agent`,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "--from=2026-09-01;--to=2026-09-04;--category=Equity",
		},
		// A stray positional used to be discarded silently, so
		// `dispersion 2026-09-01` answered over the whole mirror instead of
		// that date and never printed help either. NoArgs turns it into
		// cobra's "unknown command" error, which root.go classifies as a usage
		// error and exits 2. NoArgs is safe for the --dry-run probe: it
		// rejects positionals only, and the probe passes none.
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dispersion")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// Accept any casing but resolve to MUFAP's own spelling, because
			// the payload lookup below is an exact map-key hit.
			column := ""
			for _, c := range dispersionReturnColumns {
				if strings.EqualFold(c, strings.TrimSpace(flagColumn)) {
					column = c
					break
				}
			}
			if column == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--column %q is not a return column: want one of %s",
					flagColumn, strings.Join(dispersionReturnColumns, ", ")))
			}

			// DeriveDispersion filters only when the category is non-empty, so
			// --category "" (or whitespace) matches every row and pools
			// annualized money-market yields in the 10-20 band with absolute
			// equity returns. The resulting p90-p10 gap compares a yield with a
			// price change, which is not a spread at all.
			category := strings.TrimSpace(flagCategory)
			if category == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf(`--category must name a MUFAP category (e.g. Equity, "Money Market", Income): an empty value matches every row and pools annualized yields with absolute returns, which makes the p90-p10 spread meaningless`))
			}

			// Stored dates are ISO. A "Sep 01, 2026" bound would compare as a
			// string against ISO dates and quietly select nothing.
			dispersionCheckBound := func(name, v string) error {
				if v == "" {
					return nil
				}
				if _, err := time.Parse("2006-01-02", v); err != nil {
					return usageErr(fmt.Errorf("--%s %q is not a YYYY-MM-DD date", name, v))
				}
				return nil
			}
			if err := dispersionCheckBound("from", flagFrom); err != nil {
				_ = cmd.Usage()
				return err
			}
			if err := dispersionCheckBound("to", flagTo); err != nil {
				_ = cmd.Usage()
				return err
			}
			if flagFrom != "" && flagTo != "" && flagFrom > flagTo {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s", flagFrom, flagTo))
			}

			dbPath := strings.TrimSpace(flagDB)
			if dbPath == "" {
				dbPath = defaultDBPath("mufap-pp-cli")
			}
			dispersionEmptyMirror := func() error {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]mufap.DispersionPoint, 0), flags)
				}
				return nil
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				return dispersionEmptyMirror()
			}

			s, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("dispersion: %w", err)
			}
			defer s.Close()

			obs, err := store.LoadMUFAPObs(ctx, s, "daily-returns", flagFrom, flagTo)
			if err != nil {
				// The database can exist for the learn loop alone, with the
				// observation tables never created. A read-only handle cannot
				// create them, and an unpopulated cache is a state, not a
				// failure, so it reports as an empty mirror.
				if strings.Contains(err.Error(), "no such table") {
					return dispersionEmptyMirror()
				}
				return fmt.Errorf("dispersion: %w", err)
			}

			series := dispersionSeries(category, column, obs)
			points := series.Points

			// Every warning goes to stderr as prose in every output mode:
			// stderr never contaminates machine stdout, and no sibling command
			// emits a JSON event line. Branching these on the humanFriendly
			// global was wrong twice over -- it disagreed with the
			// wantsHumanTable routing that selects stdout's format, so a plain
			// terminal run dumped a raw JSON line onto stderr, and the
			// hand-built payload used %q, which Go-quotes a user-supplied
			// category instead of JSON-quoting it.
			errOut := cmd.ErrOrStderr()
			// An extreme value is KEPT in the spread (deleting it would reshape
			// the percentiles with nothing in the payload to say so), so the
			// terminal reader has to be told it is there. Measured: one
			// "(405.56)" launch artefact turns a real VPS-Debt spread of well
			// under 1pp into 85pp.
			outlierVals, outlierDates := 0, 0
			for _, p := range series.Points {
				if p.OutlierRows > 0 {
					outlierVals += p.OutlierRows
					outlierDates++
				}
			}
			if outlierDates > 0 {
				fmt.Fprintf(errOut, "warning: %d value(s) across %d date(s) lie beyond the Tukey fences and are KEPT in the spread; read min/max and outlier_rows before taking a wide p90-p10 gap as manager dispersion\n",
					outlierVals, outlierDates)
			}
			if series.MalformedRows > 0 {
				fmt.Fprintf(errOut, "warning: skipped %d of %d mirrored row(s) whose payload did not decode; the cross-section below is narrower than the mirror\n",
					series.MalformedRows, series.StoredRows)
			}
			if series.DecodeLostDates > 0 {
				fmt.Fprintf(errOut, "warning: %d date(s) are missing from the series because every mirrored row for them failed to decode, not because MUFAP published nothing\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n",
					series.DecodeLostDates)
			}
			// Both arms disclose the same fact -- that --category is a
			// substring and matched more than one MUFAP label -- so they are
			// mutually exclusive: the convention arm is the more specific
			// diagnosis and already names the labels.
			//
			// The second arm exists because the first one only fires on a MIX
			// of return conventions. MEASURED 2026-09-07: --category Equity
			// pools 6 MUFAP categories that are all "(Absolute Return )" --
			// 29 Equity, 28 Shariah Compliant Equity, 17 VPS-Shariah
			// Compliant Equity, 11 VPS-Equity, 5 Shariah Compliant Dedicated
			// Equity, 1 Dedicated Equity -- so 28 of 91 rows are VPS pension
			// sub-funds, a different product class, and the convention gate
			// stays silent. DispersionPoint.Category echoes the user's raw
			// substring, so without this nothing on any path reveals the
			// pooling.
			switch annualized, absolute := dispersionConventions(series.Pooled); {
			case annualized > 0 && absolute > 0:
				fmt.Fprintf(errOut, "warning: --category %q pooled %d annualized and %d absolute-return MUFAP categories (%s); the p90-p10 gap then compares a yield with a price change -- narrow --category to one return convention\n",
					category, annualized, absolute, strings.Join(series.Pooled, ", "))
			case len(series.Pooled) > 1:
				fmt.Fprintf(errOut, "warning: --category %q matched %d MUFAP categories and pooled them into one cross-section (%s); the spread below is across all of them, not within one category -- narrow --category to separate them\n",
					category, len(series.Pooled), strings.Join(series.Pooled, ", "))
			}
			if series.ThinDates > 0 {
				fmt.Fprintf(errOut, "warning: %d of %d emitted date(s) have fewer than %d funds reporting in category %q; on those dates the p90-p10 spread is the gap between a couple of funds, not a category-wide fact\n",
					series.ThinDates, len(points), dispersionMinFunds, category)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), points, flags)
			}

			out := cmd.OutOrStdout()
			if len(points) == 0 {
				window := "the stored range"
				if flagFrom != "" || flagTo != "" {
					window = strings.TrimSpace(flagFrom + " to " + flagTo)
				}
				fmt.Fprintf(out, "no stored daily-returns rows reported %q for category %q over %s\n", column, category, window)
				fmt.Fprintf(out, "run: mufap-pp-cli backfill daily --from <date> --to <date>\n")
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintf(tw, "DATE\tFUNDS\tMEDIAN\tP10\tP90\tSPREAD\n")
			for _, p := range points {
				funds := fmt.Sprintf("%d", p.FundCount)
				if p.FundCount < dispersionMinFunds {
					funds += " *"
				}
				fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\t%.2f\n", p.Date, funds, p.Median, p.P10, p.P90, p.Spread)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintf(out, "\n%s return, category %q, %d date(s)\n", column, category, len(points))
			if series.ThinDates > 0 {
				fmt.Fprintf(out, "* fewer than %d funds reporting; the spread on that date is not a category-wide fact\n", dispersionMinFunds)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "earliest validity date to include, YYYY-MM-DD (default: earliest stored)")
	cmd.Flags().StringVar(&flagTo, "to", "", "latest validity date to include, YYYY-MM-DD (default: latest stored)")
	cmd.Flags().StringVar(&flagCategory, "category", "Equity", `MUFAP category to measure, matched as a case-insensitive substring (e.g. Equity, "Money Market", Income); may not be empty`)
	cmd.Flags().StringVar(&flagColumn, "column", "YTD", "return column to measure: "+strings.Join(dispersionReturnColumns, ", "))
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	return cmd
}

// dispersionConventions splits pooled MUFAP category labels by return
// convention. MUFAP suffixes the convention onto the label itself, e.g.
// "Money Market (Annualized Return )", so a substring --category can straddle
// both and pool a yield with a price change.
func dispersionConventions(labels []string) (annualized, absolute int) {
	for _, label := range labels {
		if mufap.IsAnnualizedCategory(label) {
			annualized++
			continue
		}
		absolute++
	}
	return annualized, absolute
}
