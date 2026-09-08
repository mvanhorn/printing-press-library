// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto
//
// "auto" because the source is chosen per invocation: live by default, and
// local -- no network at all -- when --db is passed. Whichever it was is
// reported in summary.source AND in the agent envelope's meta.source, both set
// from the same variable so they cannot give different answers.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/mufap"
	"github.com/mvanhorn/printing-press-library/library/payments/mufap/internal/store"
	"github.com/spf13/cobra"
)

// verifyAllocationRow is one fund-month's netting outcome.
type verifyAllocationRow struct {
	AMC                string  `json:"amc,omitempty"`
	Fund               string  `json:"fund"`
	FundCode           int     `json:"fund_code,omitempty"`
	Month              string  `json:"month"`
	AssetsPercent      float64 `json:"assets_percent"`
	LiabilitiesPercent float64 `json:"liabilities_percent"`
	NetPercent         float64 `json:"net_percent"`
	Deviation          float64 `json:"deviation_from_100"`
	PercentsPopulated  bool    `json:"percents_populated"`
	Pass               bool    `json:"pass"`
}

// verifyAllocationSummary is the run-level verdict.
//
// Unpopulated is counted apart from Failed on purpose: MUFAP leaves every
// percent column at 0.0 for months before roughly 2024 while the amount
// columns stay correct, so folding those into Failed would condemn a decade of
// usable data on the strength of a column nobody filled in.
//
// Funds is always reported against FundsDiscovered, and Truncated/DecodeErrors
// name the two ways they diverge, so a run that checked half the industry can
// never read as an industry that has no failures.
type verifyAllocationSummary struct {
	Month           string   `json:"month"`
	Tolerance       float64  `json:"tolerance"`
	Source          string   `json:"source"`
	AMCs            int      `json:"amcs"`
	Funds           int      `json:"funds"`
	FundsDiscovered int      `json:"funds_discovered"`
	Checked         int      `json:"checked"`
	Passed          int      `json:"passed"`
	Failed          int      `json:"failed"`
	Unpopulated     int      `json:"unpopulated"`
	Absent          int      `json:"absent"`
	FetchErrors     int      `json:"fetch_errors"`
	DecodeErrors    int      `json:"decode_errors"`
	Truncated       bool     `json:"truncated"`
	WorstDeviation  float64  `json:"worst_deviation"`
	WorstFund       string   `json:"worst_fund,omitempty"`
	FailingFunds    []string `json:"failing_funds"`
}

type verifyAllocationReport struct {
	Summary verifyAllocationSummary `json:"summary"`
	Funds   []verifyAllocationRow   `json:"funds"`
}

// verifyAllocationEmitJSON writes the report with an agent-envelope
// meta.source taken from the report itself.
//
// printJSONFiltered would be the shorter call, but it pins meta.source to
// "local" unconditionally, so every networked run emitted meta.source "local"
// beside summary.source "live" -- two answers to one question in one document,
// on the command that actually spends MUFAP requests. Sourcing both from
// Summary.Source keeps them identical on every path, empty ones included.
func verifyAllocationEmitJSON(w io.Writer, report verifyAllocationReport, flags *rootFlags) error {
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(w, json.RawMessage(raw), flags, map[string]any{"source": report.Summary.Source})
}

// verifyAllocationFundRef is one unit of allocation work.
type verifyAllocationFundRef struct {
	AMC  string
	Fund string
	Code int
}

// verifyAllocationLive is one live run's outcome.
//
// Rows may be partial. Checking the whole industry is ~500 POSTs at
// --concurrency 4 against the default 60s --timeout, so the deadline lands
// mid-fan-out on the command's own headline invocation. Discovered is the
// denominator the caller reports Rows against, and Truncated says the gap is a
// deadline rather than a MUFAP silence.
type verifyAllocationLive struct {
	Rows        []verifyAllocationRow
	AMCs        int
	Discovered  int
	Absent      int
	FetchErrors int
	Truncated   bool
}

// verifyMonthKey normalises either accepted spelling of --month to MUFAP's
// M-YYYY endpoint form plus an ISO YYYY-MM prefix for local lookups.
//
// mufap.MonthKey only reads ISO input; M-YYYY (the form the allocation
// endpoint itself requires, and therefore the form users copy from the other
// commands' examples) parses as year 7 and is rejected, so it is handled here.
func verifyMonthKey(s string) (monthKey, isoMonth string, err error) {
	s = strings.TrimSpace(s)
	if key, keyErr := mufap.MonthKey(s); keyErr == nil {
		parts := strings.Split(key, "-")
		m, _ := strconv.Atoi(parts[0])
		y, _ := strconv.Atoi(parts[1])
		return key, fmt.Sprintf("%04d-%02d", y, m), nil
	}
	parts := strings.Split(s, "-")
	if len(parts) == 2 {
		m, mErr := strconv.Atoi(parts[0])
		y, yErr := strconv.Atoi(parts[1])
		if mErr == nil && yErr == nil && m >= 1 && m <= 12 && y >= 1990 && y <= 2200 {
			return fmt.Sprintf("%d-%d", m, y), fmt.Sprintf("%04d-%02d", y, m), nil
		}
	}
	return "", "", fmt.Errorf("month %q: want YYYY-MM (2026-07) or M-YYYY (7-2026)", s)
}

// verifyAllocationPercentRow reduces a raw allocation payload to the percent
// columns the netting invariant reads.
func verifyAllocationPercentRow(raw map[string]any) map[string]float64 {
	out := make(map[string]float64, len(mufap.AllocationAssetFields)+1)
	for _, f := range mufap.AllocationAssetFields {
		if v, ok := mufapAllocNumber(raw, f); ok {
			out[f] = v
		}
	}
	// The liabilities key carries MUFAP's own misspelling; the correctly
	// spelled variant never appears in any vintage of the payload.
	if v, ok := mufapAllocNumber(raw, "LaibilitiesPercent"); ok {
		out["LaibilitiesPercent"] = v
	}
	return out
}

// verifyAllocationLoadLocal reads mirrored allocation payloads for one month.
// The second return is the count of payloads that would not decode.
//
// Both date spellings are probed because the mirror stores the allocation
// resource under whichever key the fetch used: MUFAP's M-YYYY endpoint form,
// or an ISO month/day. The ISO window covers "2026-07" and every "2026-07-DD"
// in a single range.
func verifyAllocationLoadLocal(ctx context.Context, s *store.Store, monthKey, isoMonth string) ([]map[string]any, int, error) {
	obs, err := store.LoadMUFAPObs(ctx, s, "allocation", monthKey, monthKey)
	if err != nil {
		return nil, 0, err
	}
	if len(obs) == 0 {
		obs, err = store.LoadMUFAPObs(ctx, s, "allocation", isoMonth, isoMonth+"-31")
		if err != nil {
			return nil, 0, err
		}
	}
	rows := make([]map[string]any, 0, len(obs))
	decodeErrs := 0
	for _, o := range obs {
		var row map[string]any
		if err := json.Unmarshal([]byte(o.Payload), &row); err != nil {
			// An undecodable payload is a mirror defect, not a netting
			// failure; counting it as one would invent a data-quality problem.
			// It is still counted and reported, because dropping it silently
			// shrinks the universe width the summary exists to make visible.
			decodeErrs++
			continue
		}
		rows = append(rows, row)
	}
	return rows, decodeErrs, nil
}

// verifyAllocationLiveRows fans out AMCs -> funds -> allocation.
//
// Fund listing and allocation fetching run as two bounded phases rather than
// one nested pool so a single slow AMC cannot monopolise the workers.
//
// A --timeout deadline is NOT an error here: it returns whatever completed
// with Truncated set, because discarding several hundred finished checks to
// print nothing is the worse answer. Only a failure to list the AMCs at all
// (nothing to report against) comes back as an error.
func verifyAllocationLiveRows(ctx context.Context, c *client.Client, monthKey string, tolerance float64, maxAMCs, concurrency int) (verifyAllocationLive, error) {
	res := verifyAllocationLive{Rows: make([]verifyAllocationRow, 0)}
	amcs, err := mufapFetchAMCs(ctx, c)
	if err != nil {
		return res, err
	}
	sort.Slice(amcs, func(i, j int) bool { return amcs[i].Name() < amcs[j].Name() })
	if maxAMCs > 0 && len(amcs) > maxAMCs {
		amcs = amcs[:maxAMCs]
	}
	res.AMCs = len(amcs)

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)
	refs := make([]verifyAllocationFundRef, 0, len(amcs)*8)
	absent, fetchErrs := 0, 0

	for _, amc := range amcs {
		wg.Add(1)
		go func(a MUFAPAMc) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			funds, fErr := mufapFetchFunds(ctx, c, a.AMCId)
			mu.Lock()
			defer mu.Unlock()
			if fErr != nil {
				fetchErrs++
				return
			}
			for _, f := range funds {
				// The allocation endpoint returns HTTP 500 for the GUID, so a
				// fund without an integer code cannot be checked at all.
				if f.Fund == 0 {
					continue
				}
				refs = append(refs, verifyAllocationFundRef{AMC: a.Name(), Fund: f.Name(), Code: f.Fund})
			}
		}(amc)
	}
	wg.Wait()

	// Deterministic queue order: a run the deadline cuts short must cover the
	// same funds on a re-run, or the partial report is not comparable.
	sort.Slice(refs, func(i, j int) bool {
		if refs[i].AMC != refs[j].AMC {
			return refs[i].AMC < refs[j].AMC
		}
		return refs[i].Code < refs[j].Code
	})
	res.Discovered = len(refs)
	res.FetchErrors = fetchErrs
	if ctx.Err() != nil {
		// Deadline during fund listing: no allocation was fetched, but the
		// AMC and discovery counts still describe how far the run got.
		res.Truncated = true
		return res, nil
	}

	rows := make([]verifyAllocationRow, 0, len(refs))
	for _, ref := range refs {
		wg.Add(1)
		go func(r verifyAllocationFundRef) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if ctx.Err() != nil {
				return
			}
			raw, aErr := mufapFetchAllocation(ctx, c, r.Code, monthKey)
			mu.Lock()
			defer mu.Unlock()
			if aErr != nil {
				fetchErrs++
				return
			}
			if raw == nil {
				// MUFAP simply does not report this fund-month; it is neither
				// a pass nor a failure of the invariant.
				absent++
				return
			}
			chk := mufap.CheckAllocation(r.Fund, monthKey, verifyAllocationPercentRow(raw), tolerance)
			rows = append(rows, verifyAllocationRow{
				AMC:                r.AMC,
				Fund:               r.Fund,
				FundCode:           r.Code,
				Month:              monthKey,
				AssetsPercent:      chk.AssetsPercent,
				LiabilitiesPercent: chk.LiabilitiesPct,
				NetPercent:         chk.NetPercent,
				Deviation:          chk.Deviation,
				PercentsPopulated:  chk.PercentsPopulated,
				Pass:               chk.Pass,
			})
		}(ref)
	}
	wg.Wait()

	res.Rows = rows
	res.Absent = absent
	res.FetchErrors = fetchErrs
	res.Truncated = ctx.Err() != nil
	return res, nil
}

func newNovelVerifyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Check that fund asset-class percentages net to 100 and flag months whose percent columns are unpopulated.",
		Long: `verify applies arithmetic invariants to MUFAP data that MUFAP itself does not check.

Subcommands:
  allocation  sum(asset percentages) - liabilities percentage == 100, per fund

Exit codes:
  0  no genuine invariant failure (unpopulated months are not failures)
  3  at least one populated fund-month is outside --tolerance`,
		Example:     "  mufap-pp-cli verify allocation --month 7-2026 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify")
			}
			return parentNoSubcommandRunE(flags)(cmd, args)
		},
	}
	cmd.AddCommand(newNovelVerifyAllocationCmd(flags))
	return cmd
}

func newNovelVerifyAllocationCmd(flags *rootFlags) *cobra.Command {
	var flagMonth string
	var flagMaxAMCs int
	var flagTolerance float64
	var flagConcurrency int
	var flagDB string

	cmd := &cobra.Command{
		Use:   "allocation",
		Short: "Net every fund's asset percentages against 100 for one month",
		Long: `allocation checks the netting invariant for every fund in a month:

    sum(asset class percentages) - liabilities percentage == 100

MUFAP's own TotalPercentage field is the literal string "100%" on every row, so
the site displays a passing check it never performs. This command computes it.

Funds whose percent columns are all 0.0 are reported as UNPOPULATED, not as
failures: MUFAP left those columns unfilled for months before roughly 2024
while the amount columns stayed correct, and failing them would discard a
decade of good data. Derive percentages from amount/Total for those months.

By default every fund is fetched live. That is one request per AMC plus one per
fund -- roughly 500 for the whole industry -- so expect it to be slow and to
need a raised --timeout. A run the deadline cuts short still reports what it
checked, with summary.truncated set and summary.funds counted against
summary.funds_discovered; narrow it with --max-amcs to finish sooner.

Pass --db to check an already-mirrored month instead, with no network calls.

Exit codes:
  0  no populated fund-month that was checked is outside --tolerance
     (a run cut short by --timeout also exits 0: read summary.truncated)
  3  at least one populated fund-month is outside --tolerance`,
		Example: `  mufap-pp-cli verify allocation --month 7-2026 --max-amcs 1
  mufap-pp-cli verify allocation --month 7-2026 --timeout 15m --agent
  mufap-pp-cli verify allocation --month 2026-07 --tolerance 0.5 --agent
  mufap-pp-cli verify allocation --month 2019-03 --db ~/.local/share/mufap-pp-cli/mufap-pp-cli.db`,
		SilenceUsage: true,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:typed-exit-codes": "0,3",
			"pp:happy-args":       "--month=2026-07;--max-amcs=1",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify allocation")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if strings.TrimSpace(flagMonth) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--month is required"))
			}
			monthKey, isoMonth, err := verifyMonthKey(flagMonth)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if flagTolerance < 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--tolerance must not be negative"))
			}
			if flagConcurrency < 1 {
				flagConcurrency = 1
			}
			// MUFAP rounds every percent column to two decimals, so summing
			// sixteen of them lands a hair either side of the boundary in
			// binary floating point: one 7-2026 month produced 0.010000000000005
			// and 0.009999999999991 for two funds whose published columns both
			// net to exactly 100.01. The epsilon stops that representation
			// noise, rather than the data, from deciding pass or fail.
			effectiveTolerance := flagTolerance + 1e-9
			maxAMCs := flagMaxAMCs
			if cliutil.IsDogfoodEnv() && (maxAMCs == 0 || maxAMCs > 1) {
				// One AMC still exercises the whole fan-out; the full industry
				// is ~500 POSTs and cannot finish inside the dogfood timeout.
				maxAMCs = 1
			}
			rows := make([]verifyAllocationRow, 0)
			source := "live"
			amcCount, discovered, absent, fetchErrs, decodeErrs := 0, 0, 0, 0, 0
			truncated := false

			if cmd.Flags().Changed("db") {
				source = "local"
				dbPath := flagDB
				if strings.TrimSpace(dbPath) == "" {
					dbPath = defaultDBPath("mufap-pp-cli")
				}
				// Every exit from this branch emits the same report object the
				// success path emits, zeroed. A bare [] here would hand
				// `jq '.summary.failed'` a type error on exactly the paths a
				// caller is least able to anticipate.
				verifyAllocationEmpty := func(msg string) error {
					fmt.Fprint(cmd.ErrOrStderr(), msg)
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return verifyAllocationEmitJSON(cmd.OutOrStdout(), verifyAllocationReport{
							Summary: verifyAllocationSummary{
								Month:        monthKey,
								Tolerance:    flagTolerance,
								Source:       source,
								FailingFunds: make([]string, 0),
							},
							Funds: make([]verifyAllocationRow, 0),
						}, flags)
					}
					return nil
				}
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					return verifyAllocationEmpty(fmt.Sprintf("no local mirror at %s\nrun: mufap-pp-cli backfill allocation --from %s --to %s\n", dbPath, isoMonth, isoMonth))
				}
				st, openErr := store.OpenReadOnlyContext(ctx, dbPath)
				if openErr != nil {
					return openErr
				}
				defer func() { _ = st.Close() }()
				raws, decoded, loadErr := verifyAllocationLoadLocal(ctx, st, monthKey, isoMonth)
				if loadErr != nil {
					// The mirror file is shared with the platform's own tables,
					// so it can exist with no MUFAP schema at all (a `teach`
					// run creates it). A read-only handle cannot create the
					// schema, and an unpopulated cache is a state, not a
					// failure.
					if strings.Contains(loadErr.Error(), "no such table") {
						return verifyAllocationEmpty(fmt.Sprintf("no MUFAP observations in %s\nrun: mufap-pp-cli backfill allocation --from %s --to %s\n", dbPath, isoMonth, isoMonth))
					}
					return loadErr
				}
				decodeErrs = decoded

				// --max-amcs promises "the first N AMCs alphabetically" and the
				// live path sorts before truncating. Mirror rows arrive in
				// row_key order, which backfill sets to the integer fund code
				// rendered as text, so capping in arrival order would select a
				// different AMC here than the same flag selects live.
				amcNames := make([]string, 0, 32)
				seenAMC := map[string]bool{}
				for _, raw := range raws {
					name := mufapAllocString(raw, "AMCName")
					if !seenAMC[name] {
						seenAMC[name] = true
						amcNames = append(amcNames, name)
					}
				}
				sort.Strings(amcNames)
				if maxAMCs > 0 && len(amcNames) > maxAMCs {
					amcNames = amcNames[:maxAMCs]
				}
				keepAMC := make(map[string]bool, len(amcNames))
				for _, name := range amcNames {
					keepAMC[name] = true
				}

				for _, raw := range raws {
					amc := mufapAllocString(raw, "AMCName")
					if !keepAMC[amc] {
						continue
					}
					fund := mufapAllocString(raw, "FundName")
					code := 0
					if v, ok := mufapAllocNumber(raw, "FundID"); ok {
						code = int(v)
					}
					chk := mufap.CheckAllocation(fund, monthKey, verifyAllocationPercentRow(raw), effectiveTolerance)
					rows = append(rows, verifyAllocationRow{
						AMC:                amc,
						Fund:               fund,
						FundCode:           code,
						Month:              monthKey,
						AssetsPercent:      chk.AssetsPercent,
						LiabilitiesPercent: chk.LiabilitiesPct,
						NetPercent:         chk.NetPercent,
						Deviation:          chk.Deviation,
						PercentsPopulated:  chk.PercentsPopulated,
						Pass:               chk.Pass,
					})
				}
				amcCount = len(keepAMC)
				// Undecodable rows were discovered and then lost, so they
				// belong in the denominator, not silently outside it.
				discovered = len(rows) + decodeErrs
			} else {
				c, clientErr := flags.newClient()
				if clientErr != nil {
					return clientErr
				}
				live, liveErr := verifyAllocationLiveRows(ctx, c, monthKey, effectiveTolerance, maxAMCs, flagConcurrency)
				if liveErr != nil {
					if errors.Is(liveErr, context.DeadlineExceeded) {
						liveErr = fmt.Errorf("%w\nhint: the AMC list did not finish inside --timeout; raise it (--timeout 15m) or narrow the run with --max-amcs", liveErr)
					}
					return classifyAPIError(cmd.OutOrStdout(), liveErr, flags)
				}
				rows = live.Rows
				amcCount = live.AMCs
				discovered = live.Discovered
				absent = live.Absent
				fetchErrs = live.FetchErrors
				truncated = live.Truncated
			}
			if rows == nil {
				rows = make([]verifyAllocationRow, 0)
			}

			// The fan-out completes out of order; sort so two runs of the same
			// month produce byte-identical output.
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].AMC != rows[j].AMC {
					return rows[i].AMC < rows[j].AMC
				}
				return rows[i].Fund < rows[j].Fund
			})

			summary := verifyAllocationSummary{
				Month:           monthKey,
				Tolerance:       flagTolerance,
				Source:          source,
				AMCs:            amcCount,
				Funds:           len(rows),
				FundsDiscovered: discovered,
				Absent:          absent,
				FetchErrors:     fetchErrs,
				DecodeErrors:    decodeErrs,
				Truncated:       truncated,
				FailingFunds:    make([]string, 0),
			}
			worstSeen := false
			for _, r := range rows {
				if !r.PercentsPopulated {
					summary.Unpopulated++
					continue
				}
				summary.Checked++
				if r.Pass {
					summary.Passed++
				} else {
					summary.Failed++
					summary.FailingFunds = append(summary.FailingFunds, r.Fund)
				}
				// Seeded from the first populated row rather than compared
				// against the zero value: a month where every fund nets to
				// exactly 100.0 (one asset class at 100.00, the rest 0.00,
				// which sums exactly in float64) would otherwise name no fund
				// at all and print an empty "()".
				if !worstSeen || math.Abs(r.Deviation) > math.Abs(summary.WorstDeviation) {
					worstSeen = true
					summary.WorstDeviation = r.Deviation
					summary.WorstFund = r.Fund
				}
			}

			if summary.Truncated {
				// Stderr in both modes: a machine caller's stdout must stay a
				// single parseable document, and summary.truncated carries the
				// same fact into the payload.
				fmt.Fprintf(cmd.ErrOrStderr(),
					"warning: --timeout expired mid-run; %d of %d discovered fund-month(s) were checked. Raise --timeout (the whole industry is ~500 requests) or narrow the run with --max-amcs.\n",
					summary.Funds, summary.FundsDiscovered)
			}

			worstFundNote := ""
			if summary.WorstFund != "" {
				worstFundNote = fmt.Sprintf(" (%s)", summary.WorstFund)
			}

			report := verifyAllocationReport{Summary: summary, Funds: rows}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if printErr := verifyAllocationEmitJSON(cmd.OutOrStdout(), report, flags); printErr != nil {
					return printErr
				}
			} else {
				w := cmd.OutOrStdout()
				fmt.Fprintf(w, "verify allocation  month %s  tolerance %g pp  source %s\n", summary.Month, summary.Tolerance, summary.Source)
				fmt.Fprintf(w, "amcs %d  funds %d of %d discovered  checked %d  passed %d  failed %d  unpopulated %d  absent %d\n",
					summary.AMCs, summary.Funds, summary.FundsDiscovered, summary.Checked, summary.Passed, summary.Failed, summary.Unpopulated, summary.Absent)
				if summary.FetchErrors > 0 {
					fmt.Fprintf(w, "%d fetch error(s); those fund-months were not checked\n", summary.FetchErrors)
				}
				if summary.DecodeErrors > 0 {
					fmt.Fprintf(w, "%d mirrored row(s) would not decode and are excluded; re-run backfill allocation to repair them\n", summary.DecodeErrors)
				}
				if summary.Truncated {
					fmt.Fprintf(w, "PARTIAL RUN: --timeout expired before every discovered fund was checked\n")
				}
				switch {
				case len(rows) == 0:
					fmt.Fprintf(w, "\nno fund reported an allocation for %s. MUFAP publishes nothing for many\nfund-months, and coverage thins out fast before 2010.\n", summary.Month)
				case summary.Failed > 0:
					fmt.Fprintf(w, "worst deviation %+.4f pp%s\n\n", summary.WorstDeviation, worstFundNote)
					tw := tabwriter.NewWriter(w, 2, 4, 2, ' ', 0)
					fmt.Fprintln(tw, "FUND\tNET %\tDEVIATION\tPOPULATED\tPASS")
					for _, r := range rows {
						if r.PercentsPopulated && !r.Pass {
							fmt.Fprintf(tw, "%s\t%.4f\t%+.4f\tyes\tFAIL\n", r.Fund, r.NetPercent, r.Deviation)
						}
					}
					if flushErr := tw.Flush(); flushErr != nil {
						return flushErr
					}
				case summary.Checked > 0:
					fmt.Fprintf(w, "worst deviation %+.4f pp%s\nevery populated fund-month nets to 100 within tolerance.\n", summary.WorstDeviation, worstFundNote)
				}
				if summary.Unpopulated > 0 {
					fmt.Fprintf(w, "\n%d fund-month(s) report every percent column as 0.0. That is MUFAP's own gap\n(percent columns went unfilled before roughly 2024), not an arithmetic failure,\nso they are counted as unpopulated and do not affect the exit code. Their amount\ncolumns are still good: derive percentages as amount/Total for those months.\n", summary.Unpopulated)
				}
			}

			if summary.Failed > 0 {
				return &cliError{code: 3, err: fmt.Errorf("%d of %d populated fund-month(s) do not net to 100 within %g pp for %s", summary.Failed, summary.Checked, summary.Tolerance, summary.Month)}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagMonth, "month", "", "Month to check, as YYYY-MM (2026-07) or M-YYYY (7-2026)")
	cmd.Flags().IntVar(&flagMaxAMCs, "max-amcs", 0, "Check only the first N AMCs alphabetically (0 = every AMC)")
	cmd.Flags().Float64Var(&flagTolerance, "tolerance", 0.01, "Allowed deviation from 100, in percentage points")
	cmd.Flags().IntVar(&flagConcurrency, "concurrency", 4, "Parallel MUFAP requests during the AMC and fund fan-out")
	cmd.Flags().StringVar(&flagDB, "db", defaultDBPath("mufap-pp-cli"), "Check an already-mirrored month from this local store instead of fetching live")
	return cmd
}
