// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
//
// "live" is exact even with --db: this command ALWAYS resolves the AMC list
// and every AMC's fund list over the network, and --db only spares the
// per-fund allocation POSTs. So it can never answer offline. The provenance it
// reports is "live", or "mixed" when the mirror supplied some fund-months --
// never "local".

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
	"github.com/spf13/cobra"
)

// exposureAllocResource is the store resource key allocation rows mirror under.
// It is written only by `backfill allocation`; `backfill daily` mirrors the
// fund panel and never produces a row this command can read.
const exposureAllocResource = "allocation"

// exposureStocksField is the AMOUNT column, in PKR millions.
//
// The sibling StocksOREquitiesPercent column is deliberately unused: MUFAP
// reports every *Percent field as 0.0 for months before roughly 2024 while the
// amount fields stay correct, so a percent-driven series would read as a
// decade of zero equity exposure and then jump.
const exposureStocksField = "StocksOREquities"

// exposureTotalField is net assets (AUM) in PKR millions. It is the
// denominator for equity_share; the row's own TotalPercentage field is the
// literal string "100%" and checks nothing.
const exposureTotalField = "Total"

// exposureMonthRow is one month of industry-wide equity exposure.
//
// FundCount, AbsentFunds and FailedFunds are all reported because the three
// exclusions have different meanings: a fund that reported, a fund-month MUFAP
// never published, and a fetch that broke. Only the first is in the sums.
//
// AMCsResolved/AMCsFailed exist because an AMC whose fund list fails removes
// its entire book from every month at once, which no per-fund counter can
// express: failed_funds stays 0 while the sums are short an AMC.
type exposureMonthRow struct {
	Month               string  `json:"month"`
	MonthKey            string  `json:"month_key"`
	StocksPKRMillion    float64 `json:"stocks_pkr_million"`
	NetAssetsPKRMillion float64 `json:"net_assets_pkr_million"`
	EquityShare         float64 `json:"equity_share"`
	FundCount           int     `json:"fund_count"`
	AbsentFunds         int     `json:"absent_funds"`
	FailedFunds         int     `json:"failed_funds"`
	FundsAttempted      int     `json:"funds_attempted"`
	CachedFunds         int     `json:"cached_funds"`
	StocksUnreported    int     `json:"stocks_unreported_funds"`
	AMCCount            int     `json:"amc_count"`
	AMCsResolved        int     `json:"amcs_resolved"`
	AMCsFailed          int     `json:"amcs_failed"`
}

// exposureFailure is one fetch that errored. Failures are surfaced rather than
// folded into the totals: a fund silently contributing 0 rupees is
// indistinguishable from a fund that genuinely holds no equities.
//
// A Stage "funds" failure carries an empty Month: the fund list is fetched once
// for the whole range, so its loss is range-wide and is reported per month via
// exposureMonthRow.AMCsFailed rather than duplicated across every month.
type exposureFailure struct {
	Month    string `json:"month"`
	Stage    string `json:"stage"`
	AMC      string `json:"amc"`
	Fund     string `json:"fund"`
	FundCode int    `json:"fund_code"`
	Error    string `json:"error"`
}

// exposureView is the command's whole result.
type exposureView struct {
	Months        []exposureMonthRow `json:"months"`
	FetchFailures []exposureFailure  `json:"fetch_failures"`
	AMCsListed    int                `json:"amcs_listed"`
	AMCsUsed      int                `json:"amcs_used"`
	FundsListed   int                `json:"funds_listed"`
	Concurrency   int                `json:"concurrency"`
	// Source is the provenance of THIS invocation -- "live" when every
	// fund-month was fetched from MUFAP, "mixed" when --db served some of
	// them -- and CachedFundMonths is the width that claim rests on. The same
	// string is handed to the agent envelope's meta.source, so the two can
	// never give different answers to one question.
	Source           string `json:"source"`
	CachedFundMonths int    `json:"cached_fund_months"`
	Curtailed        string `json:"curtailed,omitempty"`
}

// exposureFundRef pairs a fund with its AMC so a failed allocation fetch can be
// reported against a name a person recognises rather than a bare integer.
type exposureFundRef struct {
	AMC  string
	Fund MUFAPFund
}

// exposureJob is one (fund, month) allocation fetch. Index is the job's
// position in the dispatch slice, which is how the aggregator tells a job that
// produced a result from one the run budget swallowed.
type exposureJob struct {
	Month    string
	MonthKey string
	Index    int
	Ref      exposureFundRef
}

// exposureResult is one job's outcome. Exactly one of Err, Absent or a value
// pair is meaningful, and the aggregator keys off that distinction. Stage
// overrides the failure stage recorded for Err (empty means "allocation").
type exposureResult struct {
	Job           exposureJob
	Stocks        float64
	Total         float64
	Absent        bool
	Cached        bool
	StocksMissing bool
	Stage         string
	Err           error
}

// exposureParseMonth accepts YYYY-MM and tolerates a full YYYY-MM-DD so a date
// copied out of the daily commands does not become a usage error.
func exposureParseMonth(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	for _, layout := range []string{"2006-01", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC), nil
		}
	}
	return time.Time{}, fmt.Errorf("month %q: want YYYY-MM", s)
}

// exposureListUniverse resolves every AMC's fund list through a bounded pool.
//
// The fund list is month-independent, so it is fetched once for the whole range
// rather than per month. An AMC whose list fails is returned as a failure and
// its funds are absent from every month's denominator instead of quietly
// shrinking the industry.
func exposureListUniverse(ctx context.Context, c *client.Client, amcs []MUFAPAMc, concurrency int) ([]exposureFundRef, []exposureFailure) {
	refs := make([]exposureFundRef, 0)
	fails := make([]exposureFailure, 0)

	type amcResult struct {
		amc   MUFAPAMc
		funds []MUFAPFund
		err   error
	}
	jobs := make(chan MUFAPAMc)
	out := make(chan amcResult)

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for a := range jobs {
				funds, err := mufapFetchFunds(ctx, c, a.AMCId)
				select {
				case out <- amcResult{amc: a, funds: funds, err: err}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for _, a := range amcs {
			select {
			case jobs <- a:
			case <-ctx.Done():
				return
			}
		}
	}()
	go func() {
		wg.Wait()
		close(out)
	}()

	answered := make(map[string]bool, len(amcs))
	for r := range out {
		answered[r.amc.AMCId] = true
		if r.err != nil {
			fails = append(fails, exposureFailure{
				Stage: "funds",
				AMC:   r.amc.Name(),
				Error: r.err.Error(),
			})
			continue
		}
		for _, f := range r.funds {
			refs = append(refs, exposureFundRef{AMC: r.amc.Name(), Fund: f})
		}
	}
	// An expired budget abandons the feeder mid-queue and lets a worker drop an
	// in-flight answer, so an AMC can leave this function neither listed nor
	// failed. Recording it keeps amcs_failed honest: a missing AMC must never
	// look like an AMC with no funds.
	if ctx.Err() != nil {
		for _, a := range amcs {
			if answered[a.AMCId] {
				continue
			}
			fails = append(fails, exposureFailure{
				Stage: "funds",
				AMC:   a.Name(),
				Error: fmt.Sprintf("run budget expired before this AMC's fund list was retrieved: %v", ctx.Err()),
			})
		}
	}
	// Deterministic order keeps the job queue, and therefore any partial run
	// cut short by --timeout, reproducible between invocations.
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].AMC != refs[j].AMC {
			return refs[i].AMC < refs[j].AMC
		}
		return refs[i].Fund.Fund < refs[j].Fund.Fund
	})
	sort.Slice(fails, func(i, j int) bool { return fails[i].AMC < fails[j].AMC })
	return refs, fails
}

// exposureLoadCache reads already-mirrored allocation rows, keyed month -> fund
// code. It is a pure read: this command never writes the shared mirror, because
// a --max-amcs run holds only a slice of the industry and SaveMUFAPDate
// replaces a date's snapshot wholesale.
//
// The upper bound is widened to "<month>-99" so rows mirrored under a
// day-precision date ("2026-07-31") match a month-precision request, which
// plain string comparison would otherwise exclude.
func exposureLoadCache(ctx context.Context, dbPath, fromMonth, toMonth string) (map[string]map[int]map[string]any, error) {
	s, err := store.OpenReadOnlyContext(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = s.Close() }()

	obs, err := store.LoadMUFAPObs(ctx, s, exposureAllocResource, fromMonth, toMonth+"-99")
	if err != nil {
		return nil, err
	}
	byMonth := make(map[string]map[int]map[string]any)
	for _, o := range obs {
		month := o.Date
		if len(month) >= 7 {
			month = month[:7]
		}
		var row map[string]any
		if err := json.Unmarshal([]byte(o.Payload), &row); err != nil {
			continue
		}
		// A payload without a Total is not an allocation row (another
		// resource, or an older mirror shape). Skipping it costs one live
		// fetch; trusting it would silently book the fund at zero.
		if _, ok := mufapAllocNumber(row, exposureTotalField); !ok {
			continue
		}
		code, err := strconv.Atoi(strings.TrimSpace(o.Key))
		if err != nil {
			if code, err = strconv.Atoi(mufapAllocString(row, "FundID")); err != nil {
				continue
			}
		}
		if byMonth[month] == nil {
			byMonth[month] = make(map[int]map[string]any)
		}
		byMonth[month][code] = row
	}
	return byMonth, nil
}

func newNovelExposureCmd(flags *rootFlags) *cobra.Command {
	var flagFrom string
	var flagTo string
	var flagDB string
	var flagMaxAMCs int
	var flagConcurrency int

	cmd := &cobra.Command{
		Use:   "exposure",
		Short: "Total PKR the mutual fund industry holds in listed equities, by month, from per-fund asset allocation.",
		Long: "Total PKR the mutual fund industry holds in listed equities, by month.\n\n" +
			"MUFAP publishes asset allocation one fund and one month at a time, so this\n" +
			"walks every AMC's fund list and sums the StocksOREquities AMOUNT column\n" +
			"across the industry. The percent columns are ignored on purpose: they are\n" +
			"0.0 for every month before roughly 2024 while the amounts stay correct.\n\n" +
			"Failed fetches are reported in fetch_failures and excluded from the sums;\n" +
			"fund_count is the denominator the numbers actually rest on. A wide range is\n" +
			"thousands of POSTs, so raise --timeout: fetches the run budget cuts short\n" +
			"are reported as failures, never as a month that holds no equity.",
		Example: "  mufap-pp-cli exposure --from 2026-07 --to 2026-07 --max-amcs 1 --agent\n" +
			"  mufap-pp-cli exposure --from 2024-01 --to 2026-07 --concurrency 6 --timeout 90m --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--from=2026-07;--to=2026-07;--max-amcs=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "exposure")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(flagFrom) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from is required (YYYY-MM)"))
			}
			if strings.TrimSpace(flagTo) == "" {
				flagTo = flagFrom
			}
			start, err := exposureParseMonth(flagFrom)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from: %w", err))
			}
			end, err := exposureParseMonth(flagTo)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--to: %w", err))
			}
			if end.Before(start) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--from %s is after --to %s", flagFrom, flagTo))
			}
			if flagMaxAMCs < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--max-amcs must be 0 (all) or a positive count"))
			}
			if flagConcurrency < 1 || flagConcurrency > 32 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--concurrency must be between 1 and 32"))
			}

			months := make([]string, 0)
			monthKeys := make(map[string]string)
			for cur := start; !cur.After(end); cur = cur.AddDate(0, 1, 0) {
				m := cur.Format("2006-01")
				// mufap.MonthKey rejects years outside 1990-2200 while
				// exposureParseMonth accepts anything time.Parse takes, so the
				// conversion happens here -- with every other input check, and
				// before the first request -- rather than after the fan-out has
				// already spent a few hundred POSTs on an unusable range.
				key, keyErr := mufap.MonthKey(m)
				if keyErr != nil {
					_ = cmd.Usage()
					return usageErr(keyErr)
				}
				months = append(months, m)
				monthKeys[m] = key
			}

			curtailed := ""
			maxAMCs := flagMaxAMCs
			// The live dogfood matrix runs one flat 30s budget per command, and
			// a full month is thousands of POSTs. Curtail the work, never the
			// network: the point of the matrix is a real fetch.
			if cliutil.IsDogfoodEnv() {
				if len(months) > 1 {
					months = months[:1]
				}
				if maxAMCs == 0 || maxAMCs > 1 {
					maxAMCs = 1
				}
				curtailed = "dogfood: capped to 1 month and 1 AMC"
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			amcs, err := mufapFetchAMCs(ctx, c)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			amcsListed := len(amcs)
			sort.Slice(amcs, func(i, j int) bool { return amcs[i].Name() < amcs[j].Name() })
			if maxAMCs > 0 && len(amcs) > maxAMCs {
				amcs = amcs[:maxAMCs]
			}

			refs, failures := exposureListUniverse(ctx, c, amcs, flagConcurrency)
			// An AMC whose fund list failed is missing from every month at
			// once, and no per-fund counter can say so: failed_funds stays 0
			// while the sums are short that AMC's whole book. Count the loss
			// here and stamp it on every month row below.
			amcsFailed := 0
			for _, f := range failures {
				if f.Stage == "funds" {
					amcsFailed++
				}
			}
			amcsResolved := len(amcs) - amcsFailed
			if amcsResolved < 0 {
				amcsResolved = 0
			}

			// The cache is read-only and optional: a run with no --db, or one
			// pointed at a mirror that has not been backfilled yet, simply
			// fetches everything live.
			cache := map[string]map[int]map[string]any{}
			if flagDB != "" {
				if _, statErr := os.Stat(flagDB); os.IsNotExist(statErr) {
					// `backfill daily` mirrors the fund panel, not allocation:
					// following that advice would leave the cache missing on
					// every fund. Allocation rows come only from the command
					// that writes the "allocation" resource.
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: mufap-pp-cli backfill allocation --from <YYYY-MM> --to <YYYY-MM>\nfetching every fund-month live instead\n", flagDB)
				} else {
					loaded, cacheErr := exposureLoadCache(ctx, flagDB, months[0], months[len(months)-1])
					if cacheErr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: local cache unusable (%v); fetching live\n", cacheErr)
					} else {
						cache = loaded
					}
				}
			}

			jobs := make([]exposureJob, 0)
			results := make([]exposureResult, 0)
			for _, m := range months {
				key := monthKeys[m]
				for _, ref := range refs {
					// The allocation endpoint needs the INTEGER fund code; the
					// GUID returns HTTP 500. A fund listed without one cannot
					// be fetched, and saying so beats booking it at zero.
					if ref.Fund.Fund <= 0 {
						failures = append(failures, exposureFailure{
							Month: m,
							Stage: "fund-code",
							AMC:   ref.AMC,
							Fund:  ref.Fund.Name(),
							Error: "fund has no integer fund code; allocation cannot be requested",
						})
						continue
					}
					if row, ok := cache[m][ref.Fund.Fund]; ok {
						stocks, stocksOK := mufapAllocNumber(row, exposureStocksField)
						total, totalOK := mufapAllocNumber(row, exposureTotalField)
						// exposureLoadCache already refuses rows without a
						// Total, so this only fires on a mirror shape it did
						// not anticipate. Fall through to the live fetch: one
						// extra request beats a fund booked at zero assets.
						if totalOK {
							results = append(results, exposureResult{
								Job:           exposureJob{Month: m, MonthKey: key, Index: -1, Ref: ref},
								Stocks:        stocks,
								Total:         total,
								Cached:        true,
								StocksMissing: !stocksOK,
							})
							continue
						}
					}
					job := exposureJob{Month: m, MonthKey: key, Ref: ref}
					job.Index = len(jobs)
					jobs = append(jobs, job)
				}
			}

			// Bounded worker pool: one goroutine per fund would open thousands
			// of concurrent POSTs against a site that serves one HTML page at a
			// time.
			queue := make(chan exposureJob)
			out := make(chan exposureResult)
			var wg sync.WaitGroup
			for i := 0; i < flagConcurrency; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := range queue {
						res := exposureResult{Job: j}
						row, fetchErr := mufapFetchAllocation(ctx, c, j.Ref.Fund.Fund, j.MonthKey)
						switch {
						case fetchErr != nil:
							res.Err = fetchErr
						case row == nil:
							// nil,nil is a fund-month MUFAP never published,
							// not a broken fetch.
							res.Absent = true
						default:
							stocks, stocksOK := mufapAllocNumber(row, exposureStocksField)
							total, totalOK := mufapAllocNumber(row, exposureTotalField)
							if !totalOK {
								// The same rule exposureLoadCache applies to
								// mirrored rows: no parseable Total means this
								// is not an allocation row, and counting it
								// would book the fund at zero net assets and
								// zero equity inside fund_count.
								res.Stage = "unparseable"
								res.Err = fmt.Errorf("allocation row has no parseable %s field", exposureTotalField)
								break
							}
							res.Stocks = stocks
							res.Total = total
							// A null StocksOREquities on a row whose Total does
							// parse is ambiguous -- MUFAP omits the column
							// rather than writing 0 for funds holding no listed
							// equity -- so the row keeps its real net assets in
							// the denominator and the ambiguity is counted in
							// stocks_unreported_funds instead of hidden.
							res.StocksMissing = !stocksOK
						}
						select {
						case out <- res:
						case <-ctx.Done():
							return
						}
					}
				}()
			}
			go func() {
				defer close(queue)
				for _, j := range jobs {
					select {
					case queue <- j:
					case <-ctx.Done():
						return
					}
				}
			}()
			go func() {
				wg.Wait()
				close(out)
			}()
			seen := make([]bool, len(jobs))
			for r := range out {
				if r.Job.Index >= 0 && r.Job.Index < len(seen) {
					seen[r.Job.Index] = true
				}
				results = append(results, r)
			}
			// An expired context (root --timeout defaults to 60s, against a
			// range that is thousands of POSTs) makes the feeder abandon the
			// rest of the queue and lets a worker drop an in-flight result. Any
			// such fund-month would otherwise appear nowhere -- not attempted,
			// not failed, not absent -- and its month would print as a clean
			// {stocks:0, fund_count:0} row that reads exactly like an industry
			// holding no equity. Book every unrecorded job as a failure.
			budgetErr := fmt.Errorf("run budget expired before this fund-month was recorded (never dispatched, or its result was dropped in flight)")
			if ctx.Err() != nil {
				budgetErr = fmt.Errorf("run budget expired before this fund-month was recorded (never dispatched, or its result was dropped in flight): %w", ctx.Err())
			}
			unfinished := 0
			for i, j := range jobs {
				if seen[i] {
					continue
				}
				unfinished++
				results = append(results, exposureResult{Job: j, Stage: "budget", Err: budgetErr})
			}
			if unfinished > 0 {
				note := fmt.Sprintf("incomplete: %d of %d live fund-month fetch(es) never completed", unfinished, len(jobs))
				if curtailed == "" {
					curtailed = note
				} else {
					curtailed += "; " + note
				}
			}

			byMonth := make(map[string]*exposureMonthRow, len(months))
			order := make([]string, 0, len(months))
			for _, m := range months {
				byMonth[m] = &exposureMonthRow{
					Month:        m,
					MonthKey:     monthKeys[m],
					AMCCount:     len(amcs),
					AMCsResolved: amcsResolved,
					AMCsFailed:   amcsFailed,
				}
				order = append(order, m)
			}
			for _, r := range results {
				row := byMonth[r.Job.Month]
				if row == nil {
					continue
				}
				row.FundsAttempted++
				switch {
				case r.Err != nil:
					row.FailedFunds++
					stage := r.Stage
					if stage == "" {
						stage = "allocation"
					}
					failures = append(failures, exposureFailure{
						Month:    r.Job.Month,
						Stage:    stage,
						AMC:      r.Job.Ref.AMC,
						Fund:     r.Job.Ref.Fund.Name(),
						FundCode: r.Job.Ref.Fund.Fund,
						Error:    r.Err.Error(),
					})
				case r.Absent:
					row.AbsentFunds++
				default:
					row.FundCount++
					row.StocksPKRMillion += r.Stocks
					row.NetAssetsPKRMillion += r.Total
					if r.Cached {
						row.CachedFunds++
					}
					if r.StocksMissing {
						row.StocksUnreported++
					}
				}
			}
			// Fund-code failures are booked against the month before any fetch
			// runs, so fold them into the per-month counters too.
			for _, f := range failures {
				if f.Stage != "fund-code" {
					continue
				}
				if row := byMonth[f.Month]; row != nil {
					row.FailedFunds++
					row.FundsAttempted++
				}
			}

			view := exposureView{
				Months:        make([]exposureMonthRow, 0, len(order)),
				FetchFailures: failures,
				AMCsListed:    amcsListed,
				AMCsUsed:      len(amcs),
				FundsListed:   len(refs),
				Concurrency:   flagConcurrency,
				Curtailed:     curtailed,
			}
			totalFunds := 0
			totalCached := 0
			for _, m := range order {
				row := byMonth[m]
				if row.NetAssetsPKRMillion > 0 {
					row.EquityShare = row.StocksPKRMillion / row.NetAssetsPKRMillion
				}
				totalFunds += row.FundCount
				totalCached += row.CachedFunds
				view.Months = append(view.Months, *row)
			}
			// Provenance is measured from what this run did, not declared from
			// which flags were passed: --db can be set and still serve nothing
			// (a mirror with no "allocation" rows), and the AMC and fund lists
			// go over the network either way.
			view.CachedFundMonths = totalCached
			view.Source = "live"
			if totalCached > 0 {
				view.Source = "mixed"
			}
			sort.Slice(view.FetchFailures, func(i, j int) bool {
				a, b := view.FetchFailures[i], view.FetchFailures[j]
				if a.Month != b.Month {
					return a.Month < b.Month
				}
				if a.Stage != b.Stage {
					return a.Stage < b.Stage
				}
				if a.AMC != b.AMC {
					return a.AMC < b.AMC
				}
				return a.Fund < b.Fund
			})

			if amcsFailed > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d of %d AMC fund list(s) failed; every fund of those AMCs is missing from EVERY month's sums -- amcs_resolved, not amc_count, is the real per-month denominator\n",
					amcsFailed, len(amcs))
			}
			if unfinished > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: the run budget expired with %d of %d live fund-month fetch(es) outstanding; they are counted as failures, not as zero equity -- raise --timeout or narrow --from/--to\n",
					unfinished, len(jobs))
			}
			if n := len(view.FetchFailures); n > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: %d fetch(es) failed and are excluded from every sum; the totals below rest on %d fund-month observation(s) across %d month(s) -- see fetch_failures\n",
					n, totalFunds, len(view.Months))
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				// printJSONFiltered pins the agent envelope's meta.source to
				// "local", which is false on a command that just walked the
				// industry over the network -- and contradicted the body. Emit
				// the source this invocation actually used instead.
				raw, marshalErr := json.Marshal(view)
				if marshalErr != nil {
					return marshalErr
				}
				return printOutputWithFlagsMeta(cmd.OutOrStdout(), json.RawMessage(raw), flags, map[string]any{"source": view.Source})
			}

			if view.Source == "mixed" {
				fmt.Fprintf(cmd.OutOrStdout(), "source mixed: %d of %d fund-month observation(s) came from the local mirror; the AMC and fund lists were fetched live\n", totalCached, totalFunds)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "source live: every AMC list, fund list and allocation row in this table was fetched from MUFAP")
			}
			if totalFunds == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no fund reported allocation for %s..%s across %d AMC(s)\n", flagFrom, flagTo, len(amcs))
				fmt.Fprintln(cmd.OutOrStdout(), "MUFAP publishes allocation per fund-month; older months are often absent entirely.")
				if unfinished > 0 || amcsFailed > 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "This run did not complete, so an empty result here is not evidence of an empty industry.")
				}
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			// Bold each header cell rather than the whole line: tabwriter
			// measures the escape bytes as column width otherwise.
			headers := []string{"MONTH", "STOCKS (PKR MN)", "NET ASSETS (PKR MN)", "EQUITY SHARE", "FUNDS", "ABSENT", "FAILED", "AMCS OK"}
			for i, h := range headers {
				headers[i] = bold(h)
			}
			fmt.Fprintln(tw, strings.Join(headers, "\t"))
			for _, row := range view.Months {
				fmt.Fprintf(tw, "%s\t%.1f\t%.1f\t%.2f%%\t%d\t%d\t%d\t%d/%d\n",
					row.Month, row.StocksPKRMillion, row.NetAssetsPKRMillion,
					row.EquityShare*100, row.FundCount, row.AbsentFunds, row.FailedFunds,
					row.AMCsResolved, row.AMCCount)
			}
			if err := tw.Flush(); err != nil {
				return err
			}
			if n := len(view.FetchFailures); n > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d excluded fetch(es):\n", n)
				for i, f := range view.FetchFailures {
					if i == 10 {
						fmt.Fprintf(cmd.OutOrStdout(), "  ... and %d more (use --agent for the full list)\n", n-10)
						break
					}
					// A funds-stage failure has no month and no fund: it is one
					// AMC list that never resolved, and it costs every month.
					scope := f.Month
					if scope == "" {
						scope = "all months"
					}
					target := cliutil.ScrubTerminal(f.AMC)
					if f.Fund != "" {
						target += " / " + cliutil.ScrubTerminal(f.Fund)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s %s: %s\n",
						f.Stage, scope, target, cliutil.ScrubTerminal(f.Error))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagFrom, "from", "", "First month to report, YYYY-MM (required)")
	cmd.Flags().StringVar(&flagTo, "to", "", "Last month to report, YYYY-MM (default: same as --from)")
	cmd.Flags().StringVar(&flagDB, "db", "", "Optional local mirror to read already-fetched allocation rows from instead of refetching them")
	cmd.Flags().IntVar(&flagMaxAMCs, "max-amcs", 0, "Cap how many AMCs to walk, 0 for all (use a small value to sample a month cheaply)")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "Parallel allocation fetches, 1-32")
	return cmd
}
