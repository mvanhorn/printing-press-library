// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
	"github.com/spf13/cobra"
)

const (
	// One HTML fetch per calendar day: a range this wide is a typo far more
	// often than an intention, and MUFAP rate-limits.
	backfillMaxDates  = 1500
	backfillMaxMonths = 360
	// errors[] is a summary field, not a log. Past this many entries the
	// individual messages stop being readable and only the count matters.
	backfillMaxErrors = 25
)

// backfillOpts are the range and destination flags every backfill subcommand
// shares. Registered once as persistent flags on the parent so the three
// children cannot drift apart.
type backfillOpts struct {
	from  string
	to    string
	db    string
	force bool
}

func (o *backfillOpts) resolveDB() string {
	if p := strings.TrimSpace(o.db); p != "" {
		return p
	}
	return defaultDBPath("mufap-pp-cli")
}

// backfillSummary is the machine-readable outcome of one run.
//
// DatesStored counts (resource, date) snapshots committed. It never exceeds
// DatesAttempted: only a date this run explicitly asked for is committed, so
// a ragged response can no longer seed the ledger for a date nobody requested.
//
// ZeroRowDates and DatesIncomplete are the two halves of a distinction the
// caller cannot afford to lose: the first is MUFAP publishing nothing, the
// second is this run failing to find out. Only the first is committed to the
// coverage ledger. DatesNeedFetch is a third, narrower case: a date this run
// saw only as somebody else's carried rows and therefore did not write at all.
type backfillSummary struct {
	Resource string `json:"resource"`
	From     string `json:"from"`
	To       string `json:"to"`
	DBPath   string `json:"db_path"`
	// False for the pricing and ter tabs, whose rows are current reference
	// data rather than a panel for the requested date, so their per-date row
	// counts are not universe widths. Always true for monthly and allocation.
	DateFiltered   bool     `json:"date_filtered"`
	DatesInRange   int      `json:"dates_in_range"`
	DatesAttempted int      `json:"dates_attempted"`
	DatesStored    int      `json:"dates_stored"`
	RowsStored     int      `json:"rows_stored"`
	DatesSkipped   int      `json:"dates_skipped"`
	ZeroRowDates   []string `json:"zero_row_dates"`
	// Dates that were attempted but deliberately NOT committed because the
	// fetch or the fan-out behind them was incomplete. Re-running retries
	// them; nothing in the ledger claims they were ever mirrored.
	DatesIncomplete []string `json:"dates_incomplete"`
	// Truncated records that the run STOPPED EARLY with dates still to go,
	// because the run budget (--timeout) expired. The remaining dates are not
	// listed in DatesIncomplete -- the loop simply breaks -- so without this
	// flag a truncated run is indistinguishable from a complete one in the
	// machine-readable summary, and the only trace is an advisory string in
	// Errors.
	Truncated bool `json:"truncated"`
	// Dates this run saw ONLY as ragged rows carried inside another date's
	// response, and therefore refused to write. MEASURED 2026-09-06: asked
	// for a date it has no panel for, MUFAP answers with a
	// latest-available-per-fund fallback, so those rows are a fraction of the
	// real panel -- 2026-09-03 arrived as 135 carried rows against a true
	// panel of 524, and 2026-09-04 as 345 against 388. Committing them would
	// write a coverage row, and every later run would then skip the real
	// fetch. These dates still need a fetch of their own.
	DatesNeedFetch []string `json:"dates_need_fetch"`
	// Carried dates later than both the requested window and the calendar day
	// this run happened on. Many funds price "forward", so a response carries
	// NAVs dated ahead of today (2026-09-07 = 22 rows, measured on 2026-09-06);
	// no complete panel can exist for those yet, so they are refused and are
	// NOT listed as needing a fetch.
	DatesForwardDated []string `json:"dates_forward_dated"`
	// Rows carried out of a response that this run committed under no date at
	// all, so the exclusion is counted rather than silent.
	CarriedRowsDropped int      `json:"carried_rows_dropped"`
	Errors             []string `json:"errors"`
}

func backfillNewSummary(resource, from, to, dbPath string) backfillSummary {
	return backfillSummary{
		Resource: resource,
		From:     from,
		To:       to,
		DBPath:   dbPath,
		// Monthly and allocation are genuinely dated; the daily command
		// overrides this from the tab it was asked for.
		DateFiltered:      true,
		ZeroRowDates:      make([]string, 0),
		DatesIncomplete:   make([]string, 0),
		DatesNeedFetch:    make([]string, 0),
		DatesForwardDated: make([]string, 0),
		Errors:            make([]string, 0),
	}
}

// backfillAddErr records a per-date failure without aborting the run: one bad
// date must not cost the caller the dates that did land.
func backfillAddErr(sum *backfillSummary, format string, a ...any) {
	switch {
	case len(sum.Errors) < backfillMaxErrors:
		sum.Errors = append(sum.Errors, fmt.Sprintf(format, a...))
	case len(sum.Errors) == backfillMaxErrors:
		sum.Errors = append(sum.Errors, fmt.Sprintf("further errors suppressed after %d", backfillMaxErrors))
	}
}

// backfillProgress reports a committed snapshot on stderr so a long range
// shows movement without polluting the stdout summary.
func backfillProgress(cmd *cobra.Command, flags *rootFlags, resource, date string, rows, total int) {
	if flags != nil && flags.quiet {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%-14s %s  rows=%-5d total=%d\n", resource, date, rows, total)
}

// backfillClipDates keeps a summary line readable: past a handful of dates the
// individual values stop carrying information and only the count does.
func backfillClipDates(dates []string) ([]string, string) {
	const max = 8
	if len(dates) > max {
		return dates[:max], fmt.Sprintf(" (+%d more)", len(dates)-max)
	}
	return dates, ""
}

// backfillSortedDates renders a date set as a sorted slice. Never nil, so the
// machine summary emits [] and not null for an empty one.
func backfillSortedDates(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for d := range set {
		out = append(out, d)
	}
	sort.Strings(out)
	return out
}

// backfillIncompleteErr converts a run that did not deliver what it was asked
// for into a typed non-zero exit, AFTER the resumable summary has been printed.
//
// Exiting 0 on an incomplete backfill is the failure mode this whole command is
// built to avoid: a script sees success, accepts a partially populated mirror,
// and every later cross-section is computed on a panel with holes in it. This
// CLI's sibling records the same shape -- a backfill that stored 33 of 131
// dates and said nothing.
//
// The condition is deliberately NARROW, and gating on len(Errors) > 0 would be
// wrong. Errors also carries ADVISORY entries that do not mean anything failed:
// "this tab is current reference data, not a dated panel" and "tab has no
// validity-date column" are both recorded there on runs that stored every row
// they were asked for. Only two things count:
//
//   - DatesIncomplete: dates this run asked for, attempted, and did not store.
//     Nothing was written for them, so they stay retryable.
//   - Truncated: the run budget expired with dates still to go.
//
// Deliberately NOT failures: ZeroRowDates (MUFAP publishes nothing on weekends
// and holidays, and the ledger records that on purpose), DatesForwardDated (no
// panel can exist yet, so it is unfetchable rather than missed), and dates
// refused because the tab is not date-filtered.
func backfillIncompleteErr(sum backfillSummary) error {
	if len(sum.DatesIncomplete) == 0 && !sum.Truncated {
		return nil
	}
	switch {
	case len(sum.DatesIncomplete) > 0 && sum.Truncated:
		return apiErr(fmt.Errorf("incomplete: %d requested date(s) were not stored and the run budget expired early; the summary above lists them and the mirror is resumable -- re-run the same command to retry",
			len(sum.DatesIncomplete)))
	case sum.Truncated:
		return apiErr(fmt.Errorf("incomplete: the run budget expired before every requested date was reached; the summary above names where to resume -- re-run the same command"))
	default:
		return apiErr(fmt.Errorf("incomplete: %d requested date(s) were not stored; the summary above lists them and nothing was written for them, so re-running the same command retries exactly those",
			len(sum.DatesIncomplete)))
	}
}

func backfillEmit(cmd *cobra.Command, flags *rootFlags, sum backfillSummary) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		// printJSONFiltered would be the shorter call, but it pins the agent
		// envelope's meta.source to "local" -- false on the one command whose
		// whole job is spending MUFAP requests. MEASURED 2026-09-07: a
		// backfill into an empty store fetched 524 rows over the network and
		// still reported meta.source "local". Report what this run actually
		// did; a run that attempted no date touched no network.
		source := "live"
		if sum.DatesAttempted == 0 {
			source = "local"
		}
		raw, err := json.Marshal(sum)
		if err != nil {
			return err
		}
		if err := printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": source}); err != nil {
			return err
		}
		return backfillIncompleteErr(sum)
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "%s  %s..%s\n", sum.Resource, sum.From, sum.To)
	fmt.Fprintf(out, "  dates in range %d, attempted %d, stored %d, skipped %d\n",
		sum.DatesInRange, sum.DatesAttempted, sum.DatesStored, sum.DatesSkipped)
	fmt.Fprintf(out, "  rows stored    %d\n", sum.RowsStored)
	if len(sum.ZeroRowDates) > 0 {
		shown, suffix := backfillClipDates(sum.ZeroRowDates)
		// Not a failure. MUFAP publishes nothing on weekends and holidays,
		// and the ledger records those as fetched-and-empty on purpose.
		fmt.Fprintf(out, "  published nothing: %s%s\n", strings.Join(shown, " "), suffix)
	}
	if len(sum.DatesIncomplete) > 0 {
		shown, suffix := backfillClipDates(sum.DatesIncomplete)
		// The opposite of the line above, and the reason the two are never
		// merged: these dates were NOT written, so nothing later mistakes a
		// failed run for a date MUFAP had no data for.
		fmt.Fprintf(out, "  not stored (incomplete): %s%s\n", strings.Join(shown, " "), suffix)
	}
	if len(sum.DatesNeedFetch) > 0 {
		shown, suffix := backfillClipDates(sum.DatesNeedFetch)
		// Named here, not left for the operator to discover via --force: a
		// carried date holds a FRACTION of its own panel, so writing it would
		// have made it indistinguishable from a date that was really fetched.
		if sum.DateFiltered {
			fmt.Fprintf(out, "  need their own fetch (only carried rows seen): %s%s\n", strings.Join(shown, " "), suffix)
			fmt.Fprintln(out, "    re-run with --from/--to covering those dates")
		} else {
			// A reference tab ignores the date filter, so there is no
			// per-date panel to go and get. Listed anyway: the rows were
			// excluded, and an exclusion is disclosed rather than dropped.
			fmt.Fprintf(out, "  carried and refused (this tab is not date-filtered, so they are not separately fetchable): %s%s\n", strings.Join(shown, " "), suffix)
		}
	}
	if len(sum.DatesForwardDated) > 0 {
		shown, suffix := backfillClipDates(sum.DatesForwardDated)
		// Not fetchable, so deliberately kept out of the line above.
		fmt.Fprintf(out, "  forward-dated, refused (no panel can exist yet): %s%s\n", strings.Join(shown, " "), suffix)
	}
	if sum.CarriedRowsDropped > 0 {
		fmt.Fprintf(out, "  carried rows dropped: %d (belonged to a date this run did not ask for)\n", sum.CarriedRowsDropped)
	}
	if !sum.DateFiltered {
		fmt.Fprintln(out, "  note: this tab is current reference data, not a dated panel; its row counts are not per-date universe widths")
	}
	for _, e := range sum.Errors {
		fmt.Fprintf(out, "  ! %s\n", e)
	}
	if sum.DatesAttempted == 0 && sum.DatesSkipped > 0 {
		fmt.Fprintln(out, "  every date in range was already mirrored; pass --force to re-fetch")
	}
	fmt.Fprintf(out, "  mirror: %s\n", sum.DBPath)
	return backfillIncompleteErr(sum)
}

func backfillParseDay(s, flag string) (time.Time, error) {
	t, err := time.Parse("2006-01-02", strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, fmt.Errorf("%s %q: want YYYY-MM-DD", flag, s)
	}
	return t, nil
}

// backfillParseMonth accepts YYYY-MM or YYYY-MM-DD and returns the first of
// that month.
func backfillParseMonth(s, flag string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01-02", "2006-01"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("%s %q: want YYYY-MM or YYYY-MM-DD", flag, s)
}

// backfillDates expands an inclusive range into every calendar date in it.
// Weekends and holidays are kept: fetching them is how the ledger learns they
// published nothing, which is not the same fact as never having been tried.
func backfillDates(from, to string) ([]string, error) {
	start, err := backfillParseDay(from, "--from")
	if err != nil {
		return nil, err
	}
	end, err := backfillParseDay(to, "--to")
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, fmt.Errorf("--to %s is before --from %s", to, from)
	}
	out := make([]string, 0)
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if len(out) >= backfillMaxDates {
			return nil, fmt.Errorf("%s..%s exceeds %d dates; backfill in smaller windows", from, to, backfillMaxDates)
		}
		out = append(out, d.Format("2006-01-02"))
	}
	return out, nil
}

// backfillMonthEnds expands a range into one month-end ISO date per month.
func backfillMonthEnds(from, to string) ([]string, error) {
	start, err := backfillParseMonth(from, "--from")
	if err != nil {
		return nil, err
	}
	end, err := backfillParseMonth(to, "--to")
	if err != nil {
		return nil, err
	}
	if end.Before(start) {
		return nil, fmt.Errorf("--to %s is before --from %s", to, from)
	}
	out := make([]string, 0)
	for cur := start; !cur.After(end); cur = cur.AddDate(0, 1, 0) {
		if len(out) >= backfillMaxMonths {
			return nil, fmt.Errorf("%s..%s exceeds %d months; backfill in smaller windows", from, to, backfillMaxMonths)
		}
		// Day 0 of the next month is the last calendar day of this one.
		out = append(out, time.Date(cur.Year(), cur.Month()+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
	}
	return out, nil
}

// backfillClampDogfood bounds a live-matrix run to a single date. The dogfood
// harness enforces a flat per-command budget that a real multi-date fetch
// cannot meet. The fetch still happens, so the matrix exercises the live
// endpoint instead of a stub.
func backfillClampDogfood(cmd *cobra.Command, dates []string) []string {
	if !cliutil.IsDogfoodEnv() || len(dates) <= 1 {
		return dates
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "dogfood: clamping %d dates to %s to fit the harness budget\n", len(dates), dates[0])
	return dates[:1]
}

// backfillUniqueKey keeps (resource, date, row_key) unique. MUFAP occasionally
// lists the same fund twice in one panel; suffixing the duplicate keeps both
// rows instead of losing the whole date to a primary-key collision.
func backfillUniqueKey(seen map[string]int, key string) string {
	seen[key]++
	if n := seen[key]; n > 1 {
		return fmt.Sprintf("%s #%d", key, n)
	}
	return key
}

// backfillRowKey is the stored row_key for a daily or monthly row: the shared
// Sector|Category|Fund Name identity, with an index fallback for a row that
// carries none of those columns.
//
// The fund name alone is NOT an identity. MEASURED on 2026-09-04, tab=returns:
// 388 rows collapse to 339 distinct names (49 collisions, 12.6%) because a VPS
// pension fund legitimately reuses one name across its money-market, debt and
// equity sub-funds. Keying on the name made those sub-funds fight over one key
// and take a positional "#2" suffix, so which NAV a key pointed at moved with
// MUFAP's row order from one date to the next.
func backfillRowKey(row map[string]string, seen map[string]int, index int) string {
	key := MUFAPRowKey(row)
	if key == "" {
		key = fmt.Sprintf("row-%d", index)
	}
	return backfillUniqueKey(seen, key)
}

func newNovelBackfillCmd(flags *rootFlags) *cobra.Command {
	opts := &backfillOpts{}

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Fetch the daily fund panel or monthly allocation across a date range into the local store.",
		Long: `Mirror MUFAP panels into the local SQLite store, one date at a time.

Each date commits on its own, so a run is resumable: re-run the same range and
every date already in the coverage ledger is skipped unless --force is passed.
Dates that legitimately published nothing (weekends, holidays) are recorded as
fetched-and-empty, which is the only thing that distinguishes them from dates
that were never attempted.

The default --timeout bounds the whole run. A long range that runs out of
budget stops cleanly with what it stored intact; re-run to continue, or raise
--timeout.`,
		Example: "  mufap-pp-cli backfill daily --from 2026-09-01 --to 2026-09-04\n" +
			"  mufap-pp-cli backfill daily --tab nav --from 2026-08-01 --to 2026-08-31 --force\n" +
			"  mufap-pp-cli backfill monthly --from 2026-06 --to 2026-08\n" +
			"  mufap-pp-cli backfill allocation --from 2026-07 --to 2026-07 --json",
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"mcp:local-write":     "true",
			"pp:parent-group":     "true",
			"pp:happy-args":       "--from=2026-09-03;--to=2026-09-03",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backfill")
			}
			// A group, not an alias: guessing a resource here would write a
			// mirror the caller never asked for. Cobra lets a non-root parent
			// take arbitrary positionals, so a typo like "backfill dail"
			// arrives here -- printing help and exiting 0 would tell a machine
			// caller the write succeeded when nothing was written.
			return parentNoSubcommandRunE(flags)(cmd, args)
		},
	}
	cmd.PersistentFlags().StringVar(&opts.from, "from", "", "First date to fetch, YYYY-MM-DD (required)")
	cmd.PersistentFlags().StringVar(&opts.to, "to", "", "Last date to fetch, YYYY-MM-DD (default: same as --from)")
	cmd.PersistentFlags().StringVar(&opts.db, "db", "", "SQLite mirror path (default: the standard data directory)")
	cmd.PersistentFlags().BoolVar(&opts.force, "force", false, "Re-fetch dates already recorded in the coverage ledger")

	cmd.AddCommand(newBackfillDailyCmd(flags, opts))
	cmd.AddCommand(newBackfillMonthlyCmd(flags, opts))
	cmd.AddCommand(newBackfillAllocationCmd(flags, opts))
	return cmd
}

func newBackfillDailyCmd(flags *rootFlags, opts *backfillOpts) *cobra.Command {
	tab := MUFAPDailyTabNames[0]

	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Mirror one daily tab (returns, nav, pricing, payout, ter) across a date range.",
		Long: `Fetch one daily tab for every calendar date in [--from, --to] and store it.

Rows are grouped by their OWN "Validity Date" column, never by the requested
date, so a ragged response can never be mislabelled. Only the date this run
asked for is committed: rows that arrive under some OTHER date are counted,
named in dates_need_fetch and discarded, never written.

MEASURED: asked for a date it holds no panel for, MUFAP answers with a ragged
latest-available-per-fund fallback -- one two-date range came back carrying
rows for eleven earlier dates and one forward-dated one. Each is a fraction of
that date's real panel (2026-09-03 arrived as 135 rows against a true 524), so
committing them would enter those dates in the coverage ledger as complete:
every later run would then skip the real fetch and "coverage --gaps-only"
would report no gap. Fetch such a date explicitly instead; --force ("re-fetch
the dates I asked for") does not extend to a date nobody asked for.

tab=pricing and tab=ter are current reference data, not a dated panel: MEASURED
2026-09-04, both answered an explicit date with 551 rows where tab=returns
answered with 388. Mirroring either across a range stores today's snapshot
under every date, so date_filtered is reported false and those row counts must
not be read as per-date universe widths.`,
		Example: "  mufap-pp-cli backfill daily --from 2026-09-03 --to 2026-09-03\n" +
			"  mufap-pp-cli backfill daily --tab payout --from 2026-08-01 --to 2026-08-31",
		Annotations: map[string]string{
			"mcp:read-only":       "false",
			"mcp:local-write":     "true",
			"pp:happy-args":       "--from=2026-09-03;--to=2026-09-03",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backfill daily")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(opts.from) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from is required"))
			}
			tabName := strings.ToLower(strings.TrimSpace(tab))
			valid := false
			for _, name := range MUFAPDailyTabNames {
				if name == tabName {
					valid = true
					break
				}
			}
			if !valid {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tab %q: want one of %s", tab, strings.Join(MUFAPDailyTabNames, ", ")))
			}
			from := strings.TrimSpace(opts.from)
			to := strings.TrimSpace(opts.to)
			if to == "" {
				to = from
			}
			dates, err := backfillDates(from, to)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			dates = backfillClampDogfood(cmd, dates)

			resource := mufapResourceForTab(tabName)
			dbPath := opts.resolveDB()
			sum := backfillNewSummary(resource, dates[0], dates[len(dates)-1], dbPath)
			sum.DatesInRange = len(dates)
			// MEASURED: pricing and ter ignore the date filter, so a range
			// mirrors one current snapshot under many dates. Said once,
			// loudly, and carried in the machine summary, because the
			// per-date row count is then not a universe width.
			sum.DateFiltered = MUFAPTabIsDateFiltered(tabName)
			if !sum.DateFiltered {
				backfillAddErr(&sum, "tab %s is current reference data, not a dated panel: every date in this range stores the same snapshot, and those row counts are not per-date universe widths", tabName)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			if err := store.EnsureMUFAPSchema(ctx, s); err != nil {
				return err
			}
			fetched, err := store.MUFAPFetchedDates(ctx, s, resource)
			if err != nil {
				return err
			}

			// requested is every date this run will visit on its own;
			// pending is the part of it still ahead. A row carried out of
			// another date's response is a partial view of its own date,
			// so it must never stand in for -- or seed the ledger of -- a
			// date whose own pass is still coming or has already run.
			requested := make(map[string]bool, len(dates))
			pending := make(map[string]bool, len(dates))
			for _, d := range dates {
				requested[d] = true
				pending[d] = true
			}
			// Carried dates accumulate across the whole run so a date carried
			// out of several responses is named once.
			carriedNeedFetch := map[string]bool{}
			carriedForward := map[string]bool{}
			// MUFAP publishes on Pakistan Standard Time (UTC+5, no DST), so
			// the run's own calendar day is taken there: a machine running in
			// UTC would otherwise call the current PKT date "forward-dated"
			// for the last five hours of every day.
			runDay := time.Now().UTC().Add(5 * time.Hour).Format("2006-01-02")
			// A carried date is unfetchable only when it is beyond BOTH the
			// requested window and today. A row dated after the window but in
			// the past has a real panel of its own and belongs in
			// dates_need_fetch, not here. ISO dates compare lexically.
			forwardAfter := dates[len(dates)-1]
			if runDay > forwardAfter {
				forwardAfter = runDay
			}

			warnedNoDateColumn := false
			for _, date := range dates {
				if ctx.Err() != nil {
					sum.Truncated = true
					backfillAddErr(&sum, "run budget expired at %s; re-run to resume from here", date)
					break
				}
				delete(pending, date)
				if fetched[date] && !opts.force {
					sum.DatesSkipped++
					continue
				}
				sum.DatesAttempted++
				t, err := mufapFetchDaily(ctx, c, tabName, date)
				if err != nil {
					// Nothing is written: a date whose fetch failed must
					// stay retryable, not enter the ledger as empty.
					sum.DatesIncomplete = append(sum.DatesIncomplete, date)
					backfillAddErr(&sum, "%s: %v", date, err)
					continue
				}
				// tab=payout has no "Validity Date" column at all -- its
				// date column is "Payout Date" -- so the shared probe
				// covers both rather than each call site guessing.
				vcol := MUFAPValidityColumn(t.Headers)
				if vcol == "" && !warnedNoDateColumn {
					// Only reachable on a tab that ships no date column at
					// all; the explicit single-date query is then the only
					// date available, and the caller must know that.
					backfillAddErr(&sum, "tab %s has no validity-date column; rows keyed on the requested date", tabName)
					warnedNoDateColumn = true
				}

				grouped := map[string][]store.MUFAPRow{}
				keys := map[string]map[string]int{}
				order := make([]string, 0, 2)
				undated := 0
				for i, row := range t.Rows {
					iso := date
					if vcol != "" {
						norm, ok := mufap.NormalizeValidityDate(row[vcol])
						if !ok {
							undated++
							continue
						}
						iso = norm
					}
					if _, seen := grouped[iso]; !seen {
						order = append(order, iso)
						keys[iso] = map[string]int{}
						grouped[iso] = make([]store.MUFAPRow, 0, len(t.Rows))
					}
					payload, err := json.Marshal(row)
					if err != nil {
						backfillAddErr(&sum, "%s: encoding row %d: %v", iso, i, err)
						continue
					}
					grouped[iso] = append(grouped[iso], store.MUFAPRow{
						Key:     backfillRowKey(row, keys[iso], i),
						Payload: string(payload),
					})
				}
				if undated > 0 {
					backfillAddErr(&sum, "%s: %d row(s) had an unparseable %s and were not stored", date, undated, vcol)
				}
				if _, ok := grouped[date]; !ok {
					// Fetched-and-empty is a fact worth storing: it is what
					// separates a holiday from a date nobody ever tried.
					order = append(order, date)
					grouped[date] = make([]store.MUFAPRow, 0)
				}

				now := time.Now()
				for _, gd := range order {
					// Rows carried out of THIS date's response are a partial
					// view of gd, never a snapshot of it, so NOTHING carried
					// is committed. There is no middle option: SaveMUFAPDate
					// writes the coverage row in the same transaction as the
					// rows, and the ledger claim is the dangerous half --
					// once a date is in mufap_coverage it is "fetched"
					// forever and every later run skips it.
					//
					// MEASURED 2026-09-06: MUFAP answers a date it has no
					// panel for with a ragged latest-available-per-fund
					// fallback, so --from 2026-09-05 --to 2026-09-06 carried
					// FOURTEEN dates out of two responses -- 2026-09-03 at
					// 135 rows against a true panel of 524, 2026-09-04 at 345
					// against 388, four dates at a single row each, and the
					// forward-dated 2026-09-07 at 22. Committing those made
					// the real panels unreachable without --force while
					// coverage --gaps-only reported a clean all-clear.
					if gd != date {
						sum.CarriedRowsDropped += len(grouped[gd])
						w := cmd.ErrOrStderr()
						switch {
						case pending[gd]:
							// Its own fetch is still ahead in this run, and
							// that pass is the only one allowed to write it.
							fmt.Fprintf(w, "%-14s %s  held %d ragged row(s) for %s until its own pass\n", resource, date, len(grouped[gd]), gd)
						case requested[gd]:
							// Its own pass already ran -- stored, skipped
							// or failed -- and is authoritative either way.
							fmt.Fprintf(w, "%-14s %s  dropped %d ragged row(s) for %s; its own pass in this run is authoritative\n", resource, date, len(grouped[gd]), gd)
						case gd > forwardAfter:
							// Many funds price "forward", so a response
							// carries NAVs dated past the window and past
							// today. There is no panel to go and fetch, so
							// this is deliberately NOT a gap to chase.
							carriedForward[gd] = true
							fmt.Fprintf(w, "%-14s %s  refused %d forward-dated row(s) for %s; no panel exists for it yet\n", resource, date, len(grouped[gd]), gd)
						case fetched[gd]:
							// --force means "re-fetch the dates I asked
							// for", and this date was never asked for.
							// SaveMUFAPDate replaces a snapshot wholesale,
							// so forcing here would delete a complete
							// out-of-range mirror and leave the strays.
							fmt.Fprintf(w, "%-14s %s  ragged rows for already-mirrored %s left alone\n", resource, date, gd)
						default:
							// Never requested, never fetched. This is the
							// case that used to fall through to the save
							// below and enter a 1-row stray in the ledger as
							// a complete snapshot. It is named in
							// dates_need_fetch so the operator is told which
							// dates still owe a real fetch.
							carriedNeedFetch[gd] = true
							fmt.Fprintf(w, "%-14s %s  refused %d carried row(s) for unrequested %s; it needs a fetch of its own\n", resource, date, len(grouped[gd]), gd)
						}
						continue
					}
					if err := store.SaveMUFAPDate(ctx, s, resource, gd, grouped[gd], now); err != nil {
						sum.DatesIncomplete = append(sum.DatesIncomplete, gd)
						backfillAddErr(&sum, "%s: saving: %v", gd, err)
						continue
					}
					fetched[gd] = true
					sum.DatesStored++
					sum.RowsStored += len(grouped[gd])
					if len(grouped[gd]) == 0 {
						sum.ZeroRowDates = append(sum.ZeroRowDates, gd)
					}
					backfillProgress(cmd, flags, resource, gd, len(grouped[gd]), sum.RowsStored)
				}
			}
			// Carried dates are reported, not left for the operator to
			// discover by noticing that a later run skipped everything.
			sum.DatesNeedFetch = backfillSortedDates(carriedNeedFetch)
			sum.DatesForwardDated = backfillSortedDates(carriedForward)
			if len(sum.DatesNeedFetch) > 0 {
				shown, suffix := backfillClipDates(sum.DatesNeedFetch)
				remedy := "fetch them explicitly"
				if !sum.DateFiltered {
					remedy = "this tab is not date-filtered, so there is no per-date panel to fetch for them"
				}
				backfillAddErr(&sum, "%d date(s) appeared only as ragged carried rows and were NOT stored: %s%s; MUFAP served its latest-available-per-fund fallback, so those rows are a fraction of each date's panel -- %s",
					len(sum.DatesNeedFetch), strings.Join(shown, " "), suffix, remedy)
			}
			return backfillEmit(cmd, flags, sum)
		},
	}
	cmd.Flags().StringVar(&tab, "tab", MUFAPDailyTabNames[0], "Daily tab to mirror: "+strings.Join(MUFAPDailyTabNames, ", "))
	return cmd
}

func newBackfillMonthlyCmd(flags *rootFlags, opts *backfillOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "monthly",
		Short: "Mirror the monthly net-assets panel for every month in a date range.",
		Long: `Fetch the monthly net-assets panel for each month in [--from, --to].

--from and --to accept YYYY-MM or YYYY-MM-DD; each month is stored under its
own month-end ISO date. The panel carries no per-row date -- its sixth column
is a month label like "July-2026 ( Rupees in million )" -- so the requested
month-end is the observation date.`,
		Example: "  mufap-pp-cli backfill monthly --from 2026-07 --to 2026-08\n" +
			"  mufap-pp-cli backfill monthly --from 2026-07-31 --to 2026-07-31 --force",
		Annotations: map[string]string{
			"mcp:read-only":   "false",
			"mcp:local-write": "true",
			// A month-shaped range on purpose, not the day-shaped
			// --from=2026-09-03 the daily commands use: --from is parsed as a
			// month here, so 2026-09-03 would resolve to month-end 2026-09-30,
			// a month that has not happened yet. That returns an empty panel
			// and writes a fetched-and-empty coverage row for a future month,
			// which every later run then skips. 2026-07-31 is a complete month.
			"pp:happy-args":       "--from=2026-07-31;--to=2026-07-31",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backfill monthly")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(opts.from) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from is required"))
			}
			from := strings.TrimSpace(opts.from)
			to := strings.TrimSpace(opts.to)
			if to == "" {
				to = from
			}
			months, err := backfillMonthEnds(from, to)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			months = backfillClampDogfood(cmd, months)

			const resource = "monthly"
			dbPath := opts.resolveDB()
			sum := backfillNewSummary(resource, months[0], months[len(months)-1], dbPath)
			sum.DatesInRange = len(months)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			if err := store.EnsureMUFAPSchema(ctx, s); err != nil {
				return err
			}
			fetched, err := store.MUFAPFetchedDates(ctx, s, resource)
			if err != nil {
				return err
			}

			for _, monthEnd := range months {
				if ctx.Err() != nil {
					sum.Truncated = true
					backfillAddErr(&sum, "run budget expired at %s; re-run to resume from here", monthEnd)
					break
				}
				if fetched[monthEnd] && !opts.force {
					sum.DatesSkipped++
					continue
				}
				sum.DatesAttempted++
				t, err := mufapFetchMonthly(ctx, c, monthEnd)
				if err != nil {
					sum.DatesIncomplete = append(sum.DatesIncomplete, monthEnd)
					backfillAddErr(&sum, "%s: %v", monthEnd, err)
					continue
				}
				rows := make([]store.MUFAPRow, 0, len(t.Rows))
				keys := map[string]int{}
				for i, row := range t.Rows {
					payload, err := json.Marshal(row)
					if err != nil {
						backfillAddErr(&sum, "%s: encoding row %d: %v", monthEnd, i, err)
						continue
					}
					rows = append(rows, store.MUFAPRow{
						Key:     backfillRowKey(row, keys, i),
						Payload: string(payload),
					})
				}
				if err := store.SaveMUFAPDate(ctx, s, resource, monthEnd, rows, time.Now()); err != nil {
					sum.DatesIncomplete = append(sum.DatesIncomplete, monthEnd)
					backfillAddErr(&sum, "%s: saving: %v", monthEnd, err)
					continue
				}
				sum.DatesStored++
				sum.RowsStored += len(rows)
				if len(rows) == 0 {
					sum.ZeroRowDates = append(sum.ZeroRowDates, monthEnd)
				}
				backfillProgress(cmd, flags, resource, monthEnd, len(rows), sum.RowsStored)
			}
			return backfillEmit(cmd, flags, sum)
		},
	}
	return cmd
}

func newBackfillAllocationCmd(flags *rootFlags, opts *backfillOpts) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "allocation",
		Short: "Mirror every fund's asset allocation for each month in a date range.",
		Long: `Fetch asset allocation for every fund of every AMC, month by month.

This fans out: one request per AMC to list its funds, then one request per
fund per month. Expect it to be slow and to need a raised --timeout.

A fund-month MUFAP has no record for is skipped rather than treated as an
error -- allocation reporting predates neither every fund nor every month.
Percent fields read 0.0 for months before ~2024 while the amount fields stay
correct, so downstream readers must derive shares from amount/Total.`,
		Example: "  mufap-pp-cli backfill allocation --from 2026-07 --to 2026-07 --timeout 30m\n" +
			"  mufap-pp-cli backfill allocation --from 2026-01 --to 2026-06 --timeout 30m --json",
		Annotations: map[string]string{
			"mcp:read-only":   "false",
			"mcp:local-write": "true",
			// --timeout=30m in pp:happy-args below is LOAD-BEARING, and matches this
			// command's own Example. The fan-out is one request per AMC plus one per
			// fund per month (~550 funds), several minutes; the root --timeout
			// default is 1m. MEASURED without it: the run budget expires mid-fan-out
			// at 60s, dates_stored is 0 of 1, 93 rows are discarded, errors[] is
			// populated -- and the process still EXITS 0, so the live-dogfood harness
			// scored a truncated no-op as a passing happy path. pp:happy-args is
			// appended verbatim by the harness, so the timeout belongs here.
			//
			// A month-shaped range on purpose, not the day-shaped
			// --from=2026-09-03 the daily commands use: --from is parsed as a
			// month here, so 2026-09-03 would resolve to month-end 2026-09-30,
			// a month that has not happened yet. That returns an empty panel
			// and writes a fetched-and-empty coverage row for a future month,
			// which every later run then skips. 2026-07-31 is a complete month.
			"pp:happy-args":       "--from=2026-07-31;--to=2026-07-31;--timeout=30m",
			"pp:typed-exit-codes": "0,2,5",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "backfill allocation")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(opts.from) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from is required"))
			}
			from := strings.TrimSpace(opts.from)
			to := strings.TrimSpace(opts.to)
			if to == "" {
				to = from
			}
			months, err := backfillMonthEnds(from, to)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			months = backfillClampDogfood(cmd, months)

			const resource = "allocation"
			dbPath := opts.resolveDB()
			sum := backfillNewSummary(resource, months[0], months[len(months)-1], dbPath)
			sum.DatesInRange = len(months)

			// The fan-out is one request per fund per month, far past the
			// dogfood budget, so the live matrix samples it instead of
			// walking it. Zero means unbounded.
			maxAMCs, maxFunds := 0, 0
			if cliutil.IsDogfoodEnv() {
				maxAMCs, maxFunds = 1, 3
				fmt.Fprintln(cmd.ErrOrStderr(), "dogfood: sampling 1 AMC and 3 funds to fit the harness budget")
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			s, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return err
			}
			defer func() { _ = s.Close() }()
			if err := store.EnsureMUFAPSchema(ctx, s); err != nil {
				return err
			}
			fetched, err := store.MUFAPFetchedDates(ctx, s, resource)
			if err != nil {
				return err
			}

			todo := make([]string, 0, len(months))
			for _, monthEnd := range months {
				if fetched[monthEnd] && !opts.force {
					sum.DatesSkipped++
					continue
				}
				todo = append(todo, monthEnd)
			}
			if len(todo) == 0 {
				return backfillEmit(cmd, flags, sum)
			}

			// Listed once and reused across months: the AMC and fund rosters
			// are not month-scoped, and re-listing them per month would
			// double the request count for nothing.
			amcs, err := mufapFetchAMCs(ctx, c)
			if err != nil {
				return err
			}
			if maxAMCs > 0 && len(amcs) > maxAMCs {
				amcs = amcs[:maxAMCs]
			}
			fundsByAMC := map[string][]MUFAPFund{}

			for _, monthEnd := range todo {
				if ctx.Err() != nil {
					sum.Truncated = true
					backfillAddErr(&sum, "run budget expired at %s; re-run to resume from here", monthEnd)
					break
				}
				monthKey, err := mufap.MonthKey(monthEnd)
				if err != nil {
					sum.DatesIncomplete = append(sum.DatesIncomplete, monthEnd)
					backfillAddErr(&sum, "%s: %v", monthEnd, err)
					continue
				}
				sum.DatesAttempted++

				rows := make([]store.MUFAPRow, 0)
				keys := map[string]int{}
				// The fan-out's own health, per month. A request that failed
				// and a fund-month MUFAP genuinely has no record for both
				// contribute nothing to rows, so only these counters can tell
				// the two apart afterwards.
				var (
					amcsFailed  int
					fundsTried  int
					fundsFailed int
					fundsEmpty  int
					expired     bool
				)
				for _, amc := range amcs {
					if ctx.Err() != nil {
						expired = true
						break
					}
					funds, cached := fundsByAMC[amc.AMCId]
					if !cached {
						funds, err = mufapFetchFunds(ctx, c, amc.AMCId)
						if err != nil {
							amcsFailed++
							backfillAddErr(&sum, "%s: %s: %v", monthEnd, amc.Name(), err)
							continue
						}
						if maxFunds > 0 && len(funds) > maxFunds {
							funds = funds[:maxFunds]
						}
						fundsByAMC[amc.AMCId] = funds
					}
					before := len(rows)
					for _, f := range funds {
						if ctx.Err() != nil {
							expired = true
							break
						}
						fundsTried++
						row, err := mufapFetchAllocation(ctx, c, f.Fund, monthKey)
						if err != nil {
							fundsFailed++
							backfillAddErr(&sum, "%s: %s: %v", monthEnd, f.Name(), err)
							continue
						}
						if row == nil {
							// An absent fund-month, not a failure -- but
							// counted, because mufapFetchAllocation also
							// returns (nil, nil) for a REJECTED PARAMETER:
							// MUFAP answers a bad fund code or a stale
							// session with HTTP 200 and an empty table, never
							// a parameter error. One absent fund is data; a
							// month in which every fund is absent is an
							// outage wearing the same shape.
							fundsEmpty++
							continue
						}
						payload, err := json.Marshal(row)
						if err != nil {
							fundsFailed++
							backfillAddErr(&sum, "%s: encoding %s: %v", monthEnd, f.Name(), err)
							continue
						}
						rows = append(rows, store.MUFAPRow{
							// The integer fund code, not the FundID GUID:
							// the allocation endpoint keys on the integer.
							Key:     backfillUniqueKey(keys, fmt.Sprint(f.Fund)),
							Payload: string(payload),
						})
					}
					if expired {
						break
					}
					if flags == nil || !flags.quiet {
						fmt.Fprintf(cmd.ErrOrStderr(), "%-14s %s  %-38s funds=%-4d rows=%d\n",
							resource, monthEnd, amc.Name(), len(funds), len(rows)-before)
					}
				}

				// A month is committed only when its whole fan-out landed.
				// SaveMUFAPDate writes a coverage row even for zero rows and
				// every later run skips a date the ledger calls fetched, so
				// committing a fan-out that half failed -- or one the --timeout
				// cut in half -- would freeze a total outage in place as "MUFAP
				// published nothing", silently and permanently. The rows already
				// collected go with it: a partial month is not a month, and one
				// re-run is cheap next to an unfalsifiable zero.
				// Transport failures are not the only way a fan-out fails to
				// land. Three silent paths reach here with zero errors and zero
				// rows, and each would otherwise be frozen into the ledger as
				// "MUFAP published nothing" and skipped forever after:
				//   - the AMC roster came back empty (envelope data was []),
				//   - no AMC yielded a single fund,
				//   - every allocation returned the empty-200 shape, which is
				//     what a rejected parameter or an expired session looks
				//     like on this endpoint.
				// A sampled run is incomplete by construction for the same
				// reason: under the dogfood harness this walks 1 AMC and 3
				// funds, and committing that as a whole month would let the
				// verification matrix itself poison the operator's real mirror.
				sampled := maxAMCs > 0 || maxFunds > 0
				noRoster := len(amcs) == 0 || fundsTried == 0
				allEmpty := fundsTried > 0 && fundsEmpty == fundsTried
				if expired || amcsFailed > 0 || fundsFailed > 0 || sampled || noRoster || allEmpty {
					sum.DatesIncomplete = append(sum.DatesIncomplete, monthEnd)
					reason := fmt.Sprintf("%d/%d AMC roster fetches and %d/%d fund fetches failed",
						amcsFailed, len(amcs), fundsFailed, fundsTried)
					switch {
					case sampled:
						reason = fmt.Sprintf("sampled run (%d AMC(s), %d fund(s) per AMC): a sample is not a month", maxAMCs, maxFunds)
					case noRoster:
						reason = fmt.Sprintf("no fund roster returned (%d AMC(s), %d fund fetch(es) attempted)", len(amcs), fundsTried)
					case allEmpty:
						reason = fmt.Sprintf("all %d fund(s) returned MUFAP's empty-200 shape, which is indistinguishable from a rejected parameter or an expired session", fundsTried)
					}
					if expired {
						reason += "; run budget expired mid-fan-out"
					}
					backfillAddErr(&sum, "%s: not stored (%s); %d row(s) discarded, re-run to retry this month",
						monthEnd, reason, len(rows))
					if expired {
						break
					}
					continue
				}

				if err := store.SaveMUFAPDate(ctx, s, resource, monthEnd, rows, time.Now()); err != nil {
					sum.DatesIncomplete = append(sum.DatesIncomplete, monthEnd)
					backfillAddErr(&sum, "%s: saving: %v", monthEnd, err)
					continue
				}
				sum.DatesStored++
				sum.RowsStored += len(rows)
				if len(rows) == 0 {
					sum.ZeroRowDates = append(sum.ZeroRowDates, monthEnd)
				}
				backfillProgress(cmd, flags, resource, monthEnd, len(rows), sum.RowsStored)
			}
			return backfillEmit(cmd, flags, sum)
		},
	}
	return cmd
}
