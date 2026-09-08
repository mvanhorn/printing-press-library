// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
)

// coverageKnownResources is the fixed set of store resource keys a mirror can
// hold. It is the default scope for the never-fetched walk. Deriving the scope
// from the resources already present in the ledger would be cheaper but could
// never report a resource that was never attempted at all -- which is half of
// what this command exists to answer.
var coverageKnownResources = []string{
	"daily-returns",
	"daily-nav",
	"daily-pricing",
	"daily-payout",
	"daily-ter",
	"monthly",
	"allocation",
}

// coverageMonthlyResources are mirrored once per month, not once per day:
// backfill builds their keys as the last calendar day of each month
// (time.Date(y, m+1, 0)) and stores one ledger row under it. Walking them one
// day at a time would report ~29 of every 30 calendar dates as never-fetched
// against ~1 real row per month, which both floods --gaps-only and inflates
// summary.dates_never_fetched -- the one count callers act on.
var coverageMonthlyResources = map[string]bool{
	"monthly":    true,
	"allocation": true,
}

// The three ledger verdicts. "empty" and "never-fetched" are deliberately not
// collapsed: MUFAP publishes nothing on weekends and holidays, so a fetched
// date holding zero rows is a complete, correct observation, while an absent
// ledger row means the mirror has a hole.
//
// "never-fetched" is the ledger's own limit, not a claim about intent: the
// coverage table has no outcome column and backfill writes a row only after a
// successful save, so a date that failed on every attempt is indistinguishable
// here from one nobody ever asked for. The help text says so.
const (
	coverageStatusRows  = "rows"
	coverageStatusEmpty = "empty"
	coverageStatusNever = "never-fetched"
)

// coverageMaxWindowDays bounds the calendar walk. The never-fetched pass
// materializes one row per (resource, date), so an unbounded --from/--to pair
// would build millions of synthetic rows before printing anything.
const coverageMaxWindowDays = 1830

const coverageDateLayout = "2006-01-02"

// coverageNeverCaveat is printed on every human path that did NOT run the
// never-fetched walk. Without it, "no gaps" reads as an all-clear for holes
// that were never looked for.
const coverageNeverCaveat = "never-fetched dates need both --from and --to: an absent date can only be named against a calendar range"

type coverageRow struct {
	Resource string `json:"resource"`
	Date     string `json:"date"`
	// LedgerDate carries the mirror's own key when it differs from Date.
	// MUFAP's allocation endpoint is addressed as M-YYYY ("7-2026") and the
	// mirror stores whichever key the fetch used (see verify.go), so such a
	// row is reported under the month-end it describes while staying
	// traceable to the key it is actually stored under.
	LedgerDate string `json:"ledger_date,omitempty"`
	RowCount   int    `json:"row_count"`
	FetchedAt  string `json:"fetched_at"`
	Status     string `json:"status"`
}

// coverageSummary counts the whole requested range for one resource. The
// counts are computed before --gaps-only filtering: a summary that shrank
// with the filter would misreport how complete the mirror actually is.
//
// FirstDate/LastDate span only dates carrying a real ledger row, because
// never-fetched dates are synthetic calendar entries and must not widen the
// mirror's claimed observed extent.
type coverageSummary struct {
	Resource string `json:"resource"`
	// Cadence is the unit the counts below are denominated in: a monthly
	// resource's dates_* fields count month-ends, not days, so 1 of 9 is a
	// full mirror for it while it would be a near-total hole for a daily one.
	Cadence           string `json:"cadence"`
	DatesWithRows     int    `json:"dates_with_rows"`
	DatesEmpty        int    `json:"dates_empty"`
	DatesNeverFetched int    `json:"dates_never_fetched"`
	FirstDate         string `json:"first_date"`
	LastDate          string `json:"last_date"`
}

type coverageView struct {
	From string `json:"from"`
	To   string `json:"to"`
	// NeverFetchedChecked is false when --from/--to were not both given. A
	// zero dates_never_fetched then means "not computed", not "no holes",
	// and a consumer must be able to tell those apart.
	NeverFetchedChecked bool              `json:"never_fetched_checked"`
	GapsOnly            bool              `json:"gaps_only"`
	Rows                []coverageRow     `json:"rows"`
	Summary             []coverageSummary `json:"summary"`
}

// coverageCadence names the publication rhythm of a resource key.
func coverageCadence(resource string) string {
	if coverageMonthlyResources[resource] {
		return "monthly"
	}
	return "daily"
}

// coverageIsISODate reports whether a ledger key is a plain YYYY-MM-DD date.
func coverageIsISODate(s string) bool {
	_, err := time.Parse(coverageDateLayout, strings.TrimSpace(s))
	return err == nil
}

// coverageLedgerMonth folds any key a monthly resource can be mirrored under
// onto the "YYYY-MM" it describes. Three spellings occur in the wild: the ISO
// month-end backfill writes, a bare ISO month, and MUFAP's own M-YYYY endpoint
// form ("7-2026") that verify.go documents the mirror keeping verbatim. The
// store filters dates lexically, so an M-YYYY row is invisible to an ISO
// --from/--to window and its month would otherwise be reported as a hole.
func coverageLedgerMonth(date string) (string, bool) {
	date = strings.TrimSpace(date)
	if t, err := time.Parse(coverageDateLayout, date); err == nil {
		return t.Format("2006-01"), true
	}
	if t, err := time.Parse("2006-01", date); err == nil {
		return t.Format("2006-01"), true
	}
	parts := strings.Split(date, "-")
	if len(parts) == 2 {
		m, mErr := strconv.Atoi(parts[0])
		y, yErr := strconv.Atoi(parts[1])
		if mErr == nil && yErr == nil && m >= 1 && m <= 12 && y >= 1990 && y <= 2200 {
			return fmt.Sprintf("%04d-%02d", y, m), true
		}
	}
	return "", false
}

// coverageMonthEnd returns the last calendar day of a "YYYY-MM" month.
func coverageMonthEnd(month string) (time.Time, bool) {
	t, err := time.Parse("2006-01", month)
	if err != nil {
		return time.Time{}, false
	}
	// Day 0 of the next month is the last calendar day of this one -- the same
	// construction backfill uses to build the keys being matched here.
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, t.Location()), true
}

// coverageMonthEndsIn lists the month-end dates that fall inside [from, to].
// A month whose end lies outside the window is omitted: nothing was published
// for it inside the requested range, so it is not a hole in that range.
func coverageMonthEndsIn(fromT, toT time.Time) []time.Time {
	out := make([]time.Time, 0, 12)
	for cur := time.Date(fromT.Year(), fromT.Month(), 1, 0, 0, 0, 0, fromT.Location()); !cur.After(toT); cur = cur.AddDate(0, 1, 0) {
		end := time.Date(cur.Year(), cur.Month()+1, 0, 0, 0, 0, 0, cur.Location())
		if end.Before(fromT) || end.After(toT) {
			continue
		}
		out = append(out, end)
	}
	return out
}

func newNovelCoverageCmd(flags *rootFlags) *cobra.Command {
	var flagResource string
	var flagFrom string
	var flagTo string
	var flagDB string
	var flagGapsOnly bool

	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Show which dates were fetched, which returned zero rows, and which have no ledger row at all.",
		Long: `Reads the local fetch ledger and reports one status per (resource, date):

  rows           the date was fetched and stored row_count rows
  empty          the date was fetched and legitimately held zero rows
                 (MUFAP publishes nothing on most weekends and holidays)
  never-fetched  no ledger row exists for that date: it was never attempted,
                 or every attempt failed. The ledger records successful saves
                 only and has no outcome column, so the two cannot be told
                 apart from here.

Only the third is a hole in the mirror. Detecting it requires both --from and
--to, since an absent date can only be named against a known calendar range;
without them the never-fetched counts print "n/a", never "0".

Cadence: "monthly" and "allocation" are published once per month and mirrored
under the month-end date, so they are walked month-end to month-end and their
counts are denominated in months (summary column CADENCE). Every other
resource is walked one calendar day at a time.`,
		Example: `  mufap-pp-cli coverage --resource daily-returns --from 2026-09-01 --to 2026-09-04
  mufap-pp-cli coverage --from 2026-08-01 --to 2026-09-04 --gaps-only
  mufap-pp-cli coverage --resource monthly --agent`,
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--resource=daily-returns;--from=2026-09-01;--to=2026-09-04",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "coverage")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if len(args) > 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("coverage takes no positional arguments; got %q", args[0]))
			}
			if flagResource != "" {
				known := false
				for _, r := range coverageKnownResources {
					if r == flagResource {
						known = true
						break
					}
				}
				if !known {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("unknown --resource %q: want one of %s", flagResource, strings.Join(coverageKnownResources, ", ")))
				}
			}
			var fromT, toT time.Time
			if flagFrom != "" {
				t, err := time.Parse(coverageDateLayout, flagFrom)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--from must be YYYY-MM-DD; got %q", flagFrom))
				}
				fromT = t
			}
			if flagTo != "" {
				t, err := time.Parse(coverageDateLayout, flagTo)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--to must be YYYY-MM-DD; got %q", flagTo))
				}
				toT = t
			}
			neverChecked := flagFrom != "" && flagTo != ""
			if neverChecked {
				if toT.Before(fromT) {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--from %s is after --to %s", flagFrom, flagTo))
				}
				if days := int(toT.Sub(fromT).Hours()/24) + 1; days > coverageMaxWindowDays {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("range spans %d days; narrow it to %d or fewer", days, coverageMaxWindowDays))
				}
			}

			dbPath := flagDB
			if dbPath == "" {
				dbPath = defaultDBPath("mufap-pp-cli")
			}

			scope := coverageKnownResources
			if flagResource != "" {
				scope = []string{flagResource}
			}

			// A data.db that does not exist and a data.db carrying no
			// mufap_coverage table are the same user-visible state: nothing has
			// been backfilled yet. Both leave the ledger empty and fall through
			// to the walk below, because "no mirror at all" is precisely the
			// case where every date in --from/--to is a hole -- returning early
			// would answer "nothing to report" to the one question asked. The
			// warning still goes to stderr so stdout stays parseable.
			var ledger []store.MUFAPCoverage
			var st *store.Store
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill daily --from <date> --to <date>\n", dbPath)
			} else {
				opened, err := store.OpenReadOnlyContext(ctx, dbPath)
				if err != nil {
					return fmt.Errorf("coverage: open %s: %w", dbPath, err)
				}
				defer func() { _ = opened.Close() }()
				st = opened

				ledger, err = store.LoadMUFAPCoverage(ctx, st, flagResource, flagFrom, flagTo)
				if err != nil {
					// A data.db created by the learn loop exists before any
					// backfill has run, so it carries no mufap_coverage table.
					// A read-only handle cannot create it.
					if !strings.Contains(err.Error(), "no such table") {
						return fmt.Errorf("coverage: %w", err)
					}
					ledger = nil
				}
			}

			// LoadMUFAPCoverage filters dates lexically, which silently drops a
			// monthly row stored under MUFAP's M-YYYY key ("7-2026") from any
			// ISO window. Re-read the monthly resources unfiltered and fold
			// those keys back onto the month they describe, or a fully
			// mirrored month reports as a hole. Bounded: one row per month.
			if st != nil && neverChecked {
				for _, res := range scope {
					if !coverageMonthlyResources[res] {
						continue
					}
					all, aliasErr := store.LoadMUFAPCoverage(ctx, st, res, "", "")
					if aliasErr != nil {
						if !strings.Contains(aliasErr.Error(), "no such table") {
							return fmt.Errorf("coverage: %w", aliasErr)
						}
						continue
					}
					for _, c := range all {
						if coverageIsISODate(c.Date) {
							continue // already offered to the range filter above
						}
						month, ok := coverageLedgerMonth(c.Date)
						if !ok {
							continue
						}
						end, endOK := coverageMonthEnd(month)
						if !endOK || end.Before(fromT) || end.After(toT) {
							continue
						}
						ledger = append(ledger, c)
					}
				}
			}

			rows := make([]coverageRow, 0, len(ledger))
			seen := make(map[string]bool, len(ledger))
			monthSeen := make(map[string]bool, len(ledger))
			for _, c := range ledger {
				status := coverageStatusEmpty
				if c.RowCount > 0 {
					status = coverageStatusRows
				}
				date, ledgerDate := c.Date, ""
				if coverageMonthlyResources[c.Resource] {
					if month, ok := coverageLedgerMonth(c.Date); ok {
						// Any key inside the month proves the month was
						// fetched, whichever spelling it landed under.
						monthSeen[c.Resource+"\x00"+month] = true
						if !coverageIsISODate(c.Date) {
							if end, endOK := coverageMonthEnd(month); endOK {
								date, ledgerDate = end.Format(coverageDateLayout), c.Date
							}
						}
					}
				}
				key := c.Resource + "\x00" + date
				if seen[key] {
					continue // an ISO row already reports this month-end
				}
				seen[key] = true
				rows = append(rows, coverageRow{
					Resource:   c.Resource,
					Date:       date,
					LedgerDate: ledgerDate,
					RowCount:   c.RowCount,
					FetchedAt:  c.FetchedAt,
					Status:     status,
				})
			}

			if neverChecked {
				for _, res := range scope {
					if coverageMonthlyResources[res] {
						for _, end := range coverageMonthEndsIn(fromT, toT) {
							if monthSeen[res+"\x00"+end.Format("2006-01")] {
								continue
							}
							date := end.Format(coverageDateLayout)
							if seen[res+"\x00"+date] {
								continue
							}
							rows = append(rows, coverageRow{Resource: res, Date: date, Status: coverageStatusNever})
						}
						continue
					}
					for d := fromT; !d.After(toT); d = d.AddDate(0, 0, 1) {
						date := d.Format(coverageDateLayout)
						if seen[res+"\x00"+date] {
							continue
						}
						rows = append(rows, coverageRow{Resource: res, Date: date, Status: coverageStatusNever})
					}
				}
			}

			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Resource != rows[j].Resource {
					return rows[i].Resource < rows[j].Resource
				}
				return rows[i].Date < rows[j].Date
			})

			// Summarize before --gaps-only filtering so the counts describe
			// the range rather than the surviving slice.
			summary := make([]coverageSummary, 0)
			byResource := make(map[string]int, len(coverageKnownResources))
			for _, r := range rows {
				idx, ok := byResource[r.Resource]
				if !ok {
					summary = append(summary, coverageSummary{Resource: r.Resource, Cadence: coverageCadence(r.Resource)})
					idx = len(summary) - 1
					byResource[r.Resource] = idx
				}
				switch r.Status {
				case coverageStatusRows:
					summary[idx].DatesWithRows++
				case coverageStatusEmpty:
					summary[idx].DatesEmpty++
				default:
					summary[idx].DatesNeverFetched++
					continue
				}
				if summary[idx].FirstDate == "" || r.Date < summary[idx].FirstDate {
					summary[idx].FirstDate = r.Date
				}
				if r.Date > summary[idx].LastDate {
					summary[idx].LastDate = r.Date
				}
			}
			sort.SliceStable(summary, func(i, j int) bool { return summary[i].Resource < summary[j].Resource })

			if flagGapsOnly {
				kept := make([]coverageRow, 0, len(rows))
				for _, r := range rows {
					if r.Status != coverageStatusRows {
						kept = append(kept, r)
					}
				}
				rows = kept
			}

			view := coverageView{
				From:                flagFrom,
				To:                  flagTo,
				NeverFetchedChecked: neverChecked,
				GapsOnly:            flagGapsOnly,
				Rows:                rows,
				Summary:             summary,
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}

			out := cmd.OutOrStdout()
			if len(rows) == 0 {
				switch {
				case flagGapsOnly && neverChecked:
					fmt.Fprintf(out, "no gaps: every date checked in %s..%s was fetched and returned rows\n", flagFrom, flagTo)
				case flagGapsOnly:
					// Without both dates the walk never ran, so an empty result
					// proves only that no ledger row is empty -- it says
					// nothing about dates the mirror never recorded. Claiming
					// "no gaps" here would assert the opposite of what ran.
					fmt.Fprintln(out, "no empty dates among the ledger rows read; never-fetched dates were NOT checked")
					fmt.Fprintln(out, coverageNeverCaveat)
				default:
					fmt.Fprintf(out, "no ledger rows for the requested range in %s\n", dbPath)
					fmt.Fprintln(out, "run: mufap-pp-cli backfill daily --from <date> --to <date>")
					if !neverChecked {
						fmt.Fprintln(out, coverageNeverCaveat)
					}
				}
				return nil
			}
			tw := newTabWriter(out)
			fmt.Fprintln(tw, "RESOURCE\tDATE\tROWS\tSTATUS\tFETCHED AT")
			for _, r := range rows {
				count := "-"
				fetched := r.FetchedAt
				if r.Status == coverageStatusNever {
					fetched = "-"
				} else {
					count = fmt.Sprintf("%d", r.RowCount)
				}
				if fetched == "" {
					fetched = "-"
				}
				date := r.Date
				if r.LedgerDate != "" {
					date = fmt.Sprintf("%s (key %s)", r.Date, r.LedgerDate)
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", r.Resource, date, count, r.Status, fetched)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			fmt.Fprintln(out)
			stw := newTabWriter(out)
			fmt.Fprintln(stw, "RESOURCE\tCADENCE\tWITH ROWS\tEMPTY\tNEVER FETCHED\tFIRST\tLAST")
			for _, sm := range summary {
				never := fmt.Sprintf("%d", sm.DatesNeverFetched)
				if !neverChecked {
					never = "n/a"
				}
				first, last := sm.FirstDate, sm.LastDate
				if first == "" {
					first = "-"
				}
				if last == "" {
					last = "-"
				}
				fmt.Fprintf(stw, "%s\t%s\t%d\t%d\t%s\t%s\t%s\n", sm.Resource, sm.Cadence, sm.DatesWithRows, sm.DatesEmpty, never, first, last)
			}
			if err := stw.Flush(); err != nil {
				return err
			}
			if !neverChecked {
				fmt.Fprintln(out, "\n"+coverageNeverCaveat)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagResource, "resource", "", "Store resource key to report ("+strings.Join(coverageKnownResources, ", ")+"); empty reports all")
	cmd.Flags().StringVar(&flagFrom, "from", "", "Start date YYYY-MM-DD (inclusive); required with --to to detect never-fetched dates")
	cmd.Flags().StringVar(&flagTo, "to", "", "End date YYYY-MM-DD (inclusive); required with --from to detect never-fetched dates")
	cmd.Flags().StringVar(&flagDB, "db", "", "SQLite database file path (default: resolved data directory data.db)")
	cmd.Flags().BoolVar(&flagGapsOnly, "gaps-only", false, "Show only dates that are empty or never-fetched")
	return cmd
}
