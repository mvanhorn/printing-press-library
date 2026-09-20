// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// pp:data-source computed
// Supported strategies: auto, local, live, or computed. Change this default deliberately.
//
// `computed` describes the DEFAULT path, which is what this annotation is
// about: with no --recompute the command prints a shipped in-code ledger
// seeded from nepraper's own registry and makes NO request (verified with
// HTTP(S)_PROXY pointed at 127.0.0.1:1 — exit 0, 25 entries, meta.source
// "ledger", meta.recomputed false).
//
// NOT `local`: internal/store carries only the Printing Press learn and
// playbook tables, so there is no NEPRA table and no sync command, and
// `--data-source local` can answer nothing here. NOT `live` either: live
// fetching happens only behind --recompute, which re-derives every selected
// entry from the published documents and reports per entry whether it
// reproduced.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// newNovelConflictsCmd builds the conflict and break ledger.
//
// The command's one job is to print NEPRA contradicting itself with both
// figures, both citations and an explicit refusal to say which is right — and
// to flag, per entry, whether this build can RE-DERIVE the entry from the
// published document today or only quote it. A consumer must never be able to
// mistake a curated claim for a computed one, and must never receive a
// reconciled number.
func newNovelConflictsCmd(flags *rootFlags) *cobra.Command {
	var (
		surface    string
		kind       string
		fyCSV      string
		recompute  bool
		breakRatio float64
		strict     bool
		limit      int
	)

	cmd := &cobra.Command{
		Use:   "conflicts",
		Short: "List every place two published NEPRA sources disagree on the same figure",
		Long: "Every place two published NEPRA figures for one key disagree, printed side by side with " +
			"both table-and-page citations, the ratio and an explicit \"which is correct is unknowable\".\n\n" +
			"NOTHING here is reconciled. There is no preferred side, no average and no latest-wins: for the " +
			"FY2024-25 MEPCO figures each side is internally consistent with its own chart and the report " +
			"carries no tie-breaker, so both reach you with their provenance and neither is chosen.\n\n" +
			"Every entry carries a `computable` flag saying whether this build re-derived it from a " +
			"committed capture of the published document (verified) or can only quote it until a document " +
			"is fetched (live_only). Pass --recompute to re-derive from the live documents and report, per " +
			"entry, whether it reproduced.\n\n" +
			"With no selector it prints the whole ledger and makes NO request.",
		Example: "  nepra-pp-cli conflicts\n" +
			"  nepra-pp-cli conflicts --surface per --agent\n" +
			"  nepra-pp-cli conflicts --surface per --kind break --break-ratio 50\n" +
			"  nepra-pp-cli conflicts --surface gen --kind arithmetic --fy 2020-21\n" +
			"  nepra-pp-cli conflicts --surface capacity --recompute --strict",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--surface=per",
			"pp:typed-exit-codes": "true",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. HELP-ONLY BRANCH. No selector: print the whole ledger and the
			//    catalogue banner, and make NO request. The ledger IS the
			//    product here, not a fallback for a failed fetch.
			//    --dry-run is excluded so the dry-run contract below still
			//    fires for a bare invocation.
			if !dryRunOK(flags) && surface == "" && kind == "" && fyCSV == "" && !recompute && !strict &&
				limit == 0 && breakRatio == conflictsDefaultBreakRatio {
				return conflictsRun(cmd, flags, conflictsSelection{
					BreakRatio: conflictsDefaultBreakRatio, Catalogue: true,
				})
			}

			// 2. DRY RUN.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "conflicts")
			}

			// 3. USAGE ERRORS, before any I/O.
			sel, err := conflictsValidate(surface, kind, fyCSV, recompute, strict, breakRatio, limit)
			if err != nil {
				return err
			}
			return conflictsRun(cmd, flags, sel)
		},
	}

	cmd.Flags().StringVar(&surface, "surface", "",
		"Restrict to one source surface: per (Performance Evaluation Report PDFs), gen (plant-month "+
			"generation workbooks), capacity (installed-capacity definitions for one date). Omit for all")
	cmd.Flags().StringVar(&kind, "kind", "",
		"Restrict to one entry kind: conflict (two published figures for one key), break (an "+
			"adjacent-period step inside one published series), arithmetic (a row that fails its own "+
			"arithmetic). Omit for all")
	cmd.Flags().StringVar(&fyCSV, "fy", "",
		"Comma-separated fiscal years, e.g. 2024-25,2020-21. Accepts 2024-25, FY2024-25 and 2024-2025. "+
			"Without --recompute it filters the ledger; with --recompute it selects the documents to read")
	cmd.Flags().BoolVar(&recompute, "recompute", false,
		"Re-derive every selected entry from the published documents and report per entry whether it "+
			"reproduced. Fetches multi-MB PDFs and ~0.5 MB workbooks; the byte cost is printed to stderr "+
			"before the first request")
	cmd.Flags().Float64Var(&breakRatio, "break-ratio", conflictsDefaultBreakRatio,
		"Minimum step (max/min of two adjacent published periods) that counts as a break. Emitted in "+
			"meta.break_ratio, because a break count is not quotable without its threshold")
	cmd.Flags().BoolVar(&strict, "strict", false,
		"With --recompute, exit non-zero when an entry the ledger marks verified fails to reproduce from "+
			"the document")
	cmd.Flags().IntVar(&limit, "limit", 0,
		"Keep at most this many entries after filtering; meta.entries_total keeps the pre-limit count")
	return cmd
}

// conflictsSelection is a validated request.
type conflictsSelection struct {
	Surface    string
	Kind       string
	FYs        []string
	Recompute  bool
	Strict     bool
	BreakRatio float64
	Limit      int
	// Catalogue asks for the human catalogue banner, which the no-selector
	// branch prints instead of a filter summary.
	Catalogue bool
}

// conflictsValidate turns flags into a selection, or into a typed usage error.
// Nothing here touches the network.
func conflictsValidate(surface, kind, fyCSV string, recompute, strict bool,
	breakRatio float64, limit int) (conflictsSelection, error) {
	sel := conflictsSelection{
		Surface:   strings.ToLower(strings.TrimSpace(surface)),
		Kind:      strings.ToLower(strings.TrimSpace(kind)),
		Recompute: recompute, Strict: strict, BreakRatio: breakRatio, Limit: limit,
	}

	if sel.Surface != "" && !conflictsHasValue(conflictSurfaces, sel.Surface) {
		return sel, usageErr(fmt.Errorf("unknown --surface %q; the accepted values are %s",
			surface, strings.Join(conflictSurfaces, ", ")))
	}
	if sel.Kind != "" && !conflictsHasValue(conflictKinds, sel.Kind) {
		return sel, usageErr(fmt.Errorf("unknown --kind %q; the accepted values are %s",
			kind, strings.Join(conflictKinds, ", ")))
	}
	// A VALID kind paired with a surface that cannot produce it is refused
	// with the measured reason. An empty list would let an agent read a
	// refusal as a finding of zero.
	if ok, refusal := conflictSupported(sel.Surface, sel.Kind); !ok {
		return sel, usageErr(fmt.Errorf(
			"--surface %s --kind %s cannot be produced [%s]: %s\ninstead: %s\nthe pairs that do work are %s",
			refusal.Surface, refusal.Kind, refusal.Verdict, refusal.Reason, refusal.Instead,
			strings.Join(conflictSupportedPairs(), ", ")))
	}

	if limit < 0 {
		return sel, usageErr(fmt.Errorf("--limit must be zero or positive, got %d; zero means no limit", limit))
	}
	if breakRatio <= 1 {
		return sel, usageErr(fmt.Errorf(
			"--break-ratio must be greater than 1, got %s: a break ratio of 1 or less would flag every "+
				"ordinary year-on-year change as a break", strconv.FormatFloat(breakRatio, 'f', -1, 64)))
	}
	if breakRatio < conflictsLedgerBreakFloor && !recompute {
		return sel, usageErr(fmt.Errorf(
			"--break-ratio %s is below the shipped ledger's measured floor of %s, which the ledger cannot "+
				"answer without inventing entries it never measured; pass --recompute to re-derive every "+
				"adjacent step from the published document",
			strconv.FormatFloat(breakRatio, 'f', -1, 64),
			strconv.FormatFloat(conflictsLedgerBreakFloor, 'f', -1, 64)))
	}
	if strict && !recompute {
		return sel, usageErr(fmt.Errorf(
			"--strict asserts a recomputation; pass --recompute. On its own there is nothing for it to " +
				"assert, because the shipped ledger cannot fail to reproduce itself"))
	}

	fys, err := conflictsParseFYs(fyCSV, sel.Surface, recompute)
	if err != nil {
		return sel, err
	}
	sel.FYs = fys
	return sel, nil
}

func conflictsHasValue(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

// conflictsParseFYs normalises the --fy list and refuses years that cannot be
// read.
//
// FY2023-24 for --surface per is refused ALWAYS, with or without --recompute:
// NEPRA's own index links that report and it returns 404 with a 9-byte body,
// so the honest answer is the availability reason, not an empty result set
// that would read as "the report says nothing".
func conflictsParseFYs(csv, surface string, recompute bool) ([]string, error) {
	if strings.TrimSpace(csv) == "" {
		return nil, nil
	}
	var out []string
	for _, part := range strings.Split(csv, ",") {
		raw := strings.TrimSpace(part)
		if raw == "" {
			continue
		}
		f, err := nepraparse.ParseFiscalYear(strings.TrimPrefix(strings.ToUpper(raw), "FY"))
		if err != nil {
			return nil, usageErr(fmt.Errorf(
				"--fy %q is not a fiscal year; accepted forms are 2024-25, FY2024-25 and 2024-2025", raw))
		}
		norm := "FY" + f.Label()

		if surface == conflictSurfacePER || surface == "" {
			if src, ok := nepraper.SourceFor(norm); ok && src.Availability != nepraper.AvailabilityPublished {
				if surface == conflictSurfacePER {
					return nil, usageErr(fmt.Errorf(
						"--fy %s --surface per cannot be answered: %s",
						norm, nepraper.AvailabilityFor(norm).Reason))
				}
			}
		}
		if recompute {
			if err := conflictsYearReadable(norm, surface); err != nil {
				return nil, err
			}
		}
		out = append(out, norm)
	}
	return out, nil
}

// conflictsYearReadable refuses a --recompute year whose document this build
// cannot fetch, naming the years it can.
func conflictsYearReadable(norm, surface string) error {
	perOK := false
	if src, ok := nepraper.SourceFor(norm); ok && src.Availability == nepraper.AvailabilityPublished {
		perOK = true
	}
	genOK := conflictsGenYearReachable(norm)
	switch surface {
	case conflictSurfacePER:
		if !perOK {
			return usageErr(fmt.Errorf(
				"--recompute --surface per cannot read %s; the reachable PER years are %s",
				norm, strings.Join(conflictsPERYears(), ", ")))
		}
	case conflictSurfaceGen, conflictSurfaceCapacity:
		if !genOK {
			return usageErr(fmt.Errorf(
				"--recompute --surface %s cannot read %s; the reachable workbook years are %s",
				surface, norm, strings.Join(conflictsGenYears, ", ")))
		}
	default:
		if !perOK && !genOK {
			return usageErr(fmt.Errorf(
				"--recompute cannot read %s on any surface; the reachable PER years are %s and the "+
					"reachable workbook years are %s",
				norm, strings.Join(conflictsPERYears(), ", "), strings.Join(conflictsGenYears, ", ")))
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Output
// ---------------------------------------------------------------------------

// conflictsMeta is the envelope's meta block. Every field here is a fact about
// what this run did, so no count is quotable without its conditions.
type conflictsMeta struct {
	Source     string  `json:"source"`
	LedgerAsOf string  `json:"ledger_as_of"`
	BreakRatio float64 `json:"break_ratio"`
	// BreakRatioLedgerFloor is the smallest adjacent step the shipped ledger
	// enumerates. A break count below it is not answerable from the ledger.
	BreakRatioLedgerFloor float64  `json:"break_ratio_ledger_floor"`
	EntriesTotal          int      `json:"entries_total"`
	EntriesReturned       int      `json:"entries_returned"`
	Truncated             bool     `json:"truncated"`
	Recomputed            bool     `json:"recomputed"`
	SurfacesRequested     []string `json:"surfaces_requested"`
	KindsRequested        []string `json:"kinds_requested"`
	FiscalYearsRequested  []string `json:"fiscal_years_requested,omitempty"`
	// Reconciled is false for the whole document, asserted once at the top so
	// a consumer does not have to read every entry to know it.
	Reconciled bool `json:"reconciled"`
	// BytesFetched is 0 on the offline path and is the DECODED total on the
	// recompute path.
	BytesFetched int `json:"bytes_fetched"`
	// Verified and LiveOnly partition the returned entries by whether this
	// build re-derived them.
	Verified int `json:"entries_verified"`
	LiveOnly int `json:"entries_live_only"`
}

// conflictsResults is the envelope's results block.
type conflictsResults struct {
	Entries []conflictEntry `json:"entries"`
	// Unverified are the figures that must never be read as numbers.
	Unverified []conflictUnverified `json:"unverified"`
	// UnverifiedGapFYs are years believed truncated whose label was never
	// captured. A known gap, never interpolated.
	UnverifiedGapFYs   []string                 `json:"unverified_gap_fys"`
	Refusals           []conflictRefusal        `json:"refusals"`
	DeclaredIncomplete []conflictIncompleteness `json:"declared_incomplete"`
	Recompute          []conflictRecompute      `json:"recompute"`
	Artifacts          []nepraArtifact          `json:"artifacts,omitempty"`
	// Skipped records series a break scan refused to build, so a skip is
	// visible rather than folded into a zero.
	Skipped []perBreakSkip `json:"skipped,omitempty"`
	// ScanCensus is what the zero-load-factor scan actually looked at, per
	// fiscal year read.
	ScanCensus []genScanCensus `json:"scan_census,omitempty"`
}

// conflictsRun does the work for a validated selection.
func conflictsRun(cmd *cobra.Command, flags *rootFlags, sel conflictsSelection) error {
	ledger := conflictLedger()

	results := conflictsResults{
		Unverified:         conflictUnverifiedFigures(),
		UnverifiedGapFYs:   append([]string(nil), nepraper.UnverifiedGapFYs...),
		Refusals:           conflictsRefusalsFor(sel),
		DeclaredIncomplete: conflictLedgerIncompleteness,
		Recompute:          []conflictRecompute{},
	}

	var shortfalls []conflictRecompute
	if sel.Recompute {
		derived, artifacts, skipped, census, read, err := conflictsDerive(cmd, flags, sel, ledger)
		if err != nil {
			return err
		}
		results.Artifacts = artifacts
		results.Skipped = skipped
		results.ScanCensus = census
		var rec []conflictRecompute
		ledger, rec = conflictsReconcileAgainstLedger(ledger, derived, sel, read)
		results.Recompute = rec
		for _, r := range rec {
			// A verified entry that was READ and did not reproduce is a
			// shortfall. One whose document was never fetched is not: absence
			// of a look is not evidence of a failure.
			if r.Verdict == conflictNotRead || r.Verdict == conflictReproduced {
				continue
			}
			if r.LedgerComputable == conflictVerified {
				shortfalls = append(shortfalls, r)
			}
		}
	}

	entries := conflictsFilter(ledger, sel)
	filtered := len(entries)
	truncated := false
	if sel.Limit > 0 && len(entries) > sel.Limit {
		entries = entries[:sel.Limit]
		truncated = true
	}
	results.Entries = entries

	meta := conflictsMeta{
		Source:                conflictsSourceLabel(sel.Recompute),
		LedgerAsOf:            conflictsLedgerAsOf,
		BreakRatio:            sel.BreakRatio,
		BreakRatioLedgerFloor: conflictsLedgerBreakFloor,
		EntriesTotal:          filtered,
		EntriesReturned:       len(entries),
		Truncated:             truncated,
		Recomputed:            sel.Recompute,
		SurfacesRequested:     conflictsRequested(sel.Surface, conflictSurfaces),
		KindsRequested:        conflictsRequested(sel.Kind, conflictKinds),
		FiscalYearsRequested:  sel.FYs,
		Reconciled:            false,
	}
	for _, a := range results.Artifacts {
		meta.BytesFetched += a.Bytes
	}
	for _, e := range entries {
		if e.Computable == conflictVerified {
			meta.Verified++
			continue
		}
		meta.LiveOnly++
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		conflictsWriteSummary(cmd, sel, meta, results)
	}

	// The completeness assertion. An entry the ledger marks verified that does
	// NOT reproduce from the document is reported loudly, and --strict turns it
	// into a non-zero exit.
	for _, s := range shortfalls {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"COMPLETENESS: %s is marked %s in the ledger as of %s but did not reproduce from the document "+
				"(%s: %s). Do not treat either figure as confirmed until this is explained.\n",
			s.ID, s.LedgerComputable, conflictsLedgerAsOf, s.Verdict, s.Detail)
	}
	if len(shortfalls) > 0 && sel.Strict {
		return fmt.Errorf("%d of %d recomputed entries marked verified failed to reproduce from the "+
			"published documents", len(shortfalls), len(results.Recompute))
	}

	envelope := struct {
		Meta    conflictsMeta    `json:"meta"`
		Results conflictsResults `json:"results"`
	}{Meta: meta, Results: results}

	payload, err := json.MarshalIndent(envelope, "", "  ")
	if err != nil {
		// NEVER discard a marshal error here. A conflict list that fails to
		// marshal and is written as an empty document is a real conflict
		// reported as no conflict.
		return fmt.Errorf("marshalling the conflict ledger: %w", err)
	}
	wrapped, err := wrapPlatformStructuredOutput(payload, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

func conflictsSourceLabel(recompute bool) string {
	if recompute {
		return "ledger+live"
	}
	return "ledger"
}

func conflictsRequested(one string, all []string) []string {
	if one == "" {
		return append([]string(nil), all...)
	}
	return []string{one}
}

// conflictsRefusalsFor returns the refusals worth printing for this selection:
// the (surface, kind) matrix filtered to the requested surface, plus the
// standing scope refusals.
func conflictsRefusalsFor(sel conflictsSelection) []conflictRefusal {
	out := make([]conflictRefusal, 0, len(conflictRefusals)+len(conflictScopeRefusals))
	for _, r := range conflictRefusals {
		if sel.Surface != "" && r.Surface != sel.Surface {
			continue
		}
		if sel.Kind != "" && r.Kind != sel.Kind {
			continue
		}
		out = append(out, r)
	}
	out = append(out, conflictScopeRefusals...)
	return out
}

// conflictsSelects is the ONE predicate that decides whether an entry is in
// scope.
//
// The output filter and the recompute reconciler both use it, and they must:
// reconciling a wider set than the run prints produced a false COMPLETENESS
// alarm on the two MEPCO break entries, which sit below the default threshold
// of 100 and so were never in the derivation's scope to begin with.
func conflictsSelects(e conflictEntry, sel conflictsSelection) bool {
	if sel.Surface != "" && e.Surface != sel.Surface {
		return false
	}
	if sel.Kind != "" && e.Kind != sel.Kind {
		return false
	}
	// A break below the requested threshold is not a break at that threshold.
	// Nothing else is threshold-dependent.
	if e.Kind == conflictKindBreak && e.BreakRatioSeen != nil && *e.BreakRatioSeen < sel.BreakRatio {
		return false
	}
	if len(sel.FYs) > 0 && !conflictsMatchesFY(e, sel.FYs) {
		return false
	}
	return true
}

// conflictsFilter applies the selection and sorts deterministically.
func conflictsFilter(in []conflictEntry, sel conflictsSelection) []conflictEntry {
	out := make([]conflictEntry, 0, len(in))
	for _, e := range in {
		if conflictsSelects(e, sel) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Surface != b.Surface {
			return a.Surface < b.Surface
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if a.Key.PeriodFY != b.Key.PeriodFY {
			// Newest period first: a reader wants the current contradiction.
			return a.Key.PeriodFY > b.Key.PeriodFY
		}
		if a.Key.Entity != b.Key.Entity {
			return a.Key.Entity < b.Key.Entity
		}
		if a.Key.Metric != b.Key.Metric {
			return a.Key.Metric < b.Key.Metric
		}
		return a.ID < b.ID
	})
	return out
}

// conflictsMatchesFY keeps an entry whose key period, either side's report
// year, or recompute requirement names one of the requested years. A
// cross-report conflict is about two years and must survive a filter on
// either.
func conflictsMatchesFY(e conflictEntry, want []string) bool {
	have := map[string]bool{
		nepraper.NormalizeFY(e.Key.PeriodFY): true,
		nepraper.NormalizeFY(e.A.ReportFY):   true,
		nepraper.NormalizeFY(e.B.ReportFY):   true,
	}
	for _, n := range e.RecomputeNeeds {
		have[nepraper.NormalizeFY(n)] = true
	}
	for _, w := range want {
		if have[nepraper.NormalizeFY(w)] {
			return true
		}
	}
	return false
}

// conflictsWriteSummary prints the human view to stderr, so stdout stays a
// clean document.
func conflictsWriteSummary(cmd *cobra.Command, sel conflictsSelection, meta conflictsMeta,
	res conflictsResults) {
	w := cmd.ErrOrStderr()
	if sel.Catalogue {
		fmt.Fprintf(w,
			"%d ledger entries across surfaces %s and kinds %s, measured on %s. Nothing here is "+
				"reconciled: both figures reach you with their citations and neither is chosen.\n",
			meta.EntriesTotal, strings.Join(conflictSurfaces, "/"), strings.Join(conflictKinds, "/"),
			conflictsLedgerAsOf)
		fmt.Fprintf(w, "  %d entries this build re-derived from a committed capture, %d it can only quote "+
			"until --recompute fetches the document.\n", meta.Verified, meta.LiveOnly)
		fmt.Fprintf(w, "  (surface, kind) pairs that work: %s\n", strings.Join(conflictSupportedPairs(), ", "))
		fmt.Fprintf(w, "  %d pairs are refused with a measured reason; %d further questions are declared "+
			"out of reach; %d classes of conflict the corpus holds are declared NOT enumerated here.\n",
			len(conflictRefusals), len(conflictScopeRefusals), len(conflictLedgerIncompleteness))
	}
	for _, e := range res.Entries {
		fmt.Fprintf(w, "%-52s %-9s %-10s %-28s %s\n", e.ID, e.Kind, e.Computable,
			e.Key.String(), conflictsSideLine(e))
	}
	fmt.Fprintf(w, "%d entries after filters (break threshold %s, ledger floor %s)",
		meta.EntriesReturned, strconv.FormatFloat(sel.BreakRatio, 'f', -1, 64),
		strconv.FormatFloat(conflictsLedgerBreakFloor, 'f', -1, 64))
	if meta.Truncated {
		fmt.Fprintf(w, ", TRUNCATED from %d by --limit", meta.EntriesTotal)
	}
	if meta.Recomputed {
		fmt.Fprintf(w, ", %d recomputed against %d bytes fetched", len(res.Recompute), meta.BytesFetched)
	}
	fmt.Fprintln(w)
}

// conflictsSideLine renders the two figures WITHOUT choosing between them.
func conflictsSideLine(e conflictEntry) string {
	a, b := conflictsSideText(e.A), conflictsSideText(e.B)
	ratio := "ratio undefined"
	if e.Ratio != nil {
		ratio = fmt.Sprintf("ratio %.7f", *e.Ratio)
	}
	return a + " vs " + b + "  " + ratio
}

func conflictsSideText(s conflictSide) string {
	if s.Value == nil {
		return "<" + s.ValueKind + ">"
	}
	label := strconv.FormatFloat(*s.Value, 'f', -1, 64)
	if s.Table != "" {
		label += " [" + s.Table
		if s.Page > 0 {
			label += fmt.Sprintf(" p%d", s.Page)
		}
		label += "]"
	}
	return label
}

// ---------------------------------------------------------------------------
// Recompute
// ---------------------------------------------------------------------------

// The verdicts a recomputation can reach.
const (
	conflictReproduced    = "reproduced"
	conflictFiguresDiffer = "figures_differ"
	// conflictNotInDocument means the documents WERE read and did not produce
	// this entry. That is a real finding about the ledger.
	conflictNotInDocument = "not_found_in_document"
	// conflictNotRead means the entry needs a document this run never
	// fetched. It is kept strictly apart from the verdict above, because
	// "nobody looked" is not evidence and must never fire the completeness
	// assertion.
	conflictNotRead    = "required_document_not_read"
	conflictNewFinding = "new_finding"
)

// conflictRecompute is one entry's recomputation result.
type conflictRecompute struct {
	ID      string `json:"id"`
	Surface string `json:"surface"`
	Kind    string `json:"kind"`
	// Verdict is reproduced | figures_differ | not_found_in_document |
	// new_finding.
	Verdict          string   `json:"verdict"`
	Reproduced       bool     `json:"reproduced"`
	LedgerComputable string   `json:"ledger_computable"`
	LedgerA          *float64 `json:"ledger_a,omitempty"`
	LedgerB          *float64 `json:"ledger_b,omitempty"`
	ObservedA        *float64 `json:"observed_a,omitempty"`
	ObservedB        *float64 `json:"observed_b,omitempty"`
	ObservedRatio    *float64 `json:"observed_ratio"`
	Detail           string   `json:"detail,omitempty"`
}

// conflictsDerive fetches the documents this selection needs and derives
// entries from them.
func conflictsDerive(cmd *cobra.Command, flags *rootFlags, sel conflictsSelection,
	ledger []conflictEntry) ([]conflictEntry, []nepraArtifact, []perBreakSkip, []genScanCensus,
	map[string]bool, error) {
	perYears, genYears := conflictsYearsToRead(sel, ledger)
	read := map[string]bool{}
	for _, fy := range perYears {
		read[fy] = true
	}
	for _, fy := range genYears {
		read[fy] = true
	}

	// Refuse before fetching anything when the selection reduces to entries
	// nothing can answer.
	if len(perYears) == 0 && len(genYears) == 0 {
		return nil, nil, nil, nil, nil, usageErr(fmt.Errorf(
			"--recompute has nothing to read for this selection: no selected entry names a document this " +
				"build can fetch. Drop --recompute to print the shipped ledger, or widen --surface/--fy"))
	}

	conflictsWriteByteCost(cmd, perYears, genYears)

	c, err := flags.newClient()
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()

	var derived []conflictEntry
	var artifacts []nepraArtifact
	var skips []perBreakSkip
	var census []genScanCensus

	var reports []*nepraper.Report
	for _, fy := range perYears {
		raw, url, contentType, ferr := conflictsFetchPER(ctx, c, fy)
		if ferr != nil {
			return nil, nil, nil, nil, nil, conflictsFetchError(cmd, flags, ferr)
		}
		r, perr := conflictsPERReport(raw, fy)
		if perr != nil {
			return nil, nil, nil, nil, nil, perr
		}
		content, cerr := perContentHash(r)
		if cerr != nil {
			return nil, nil, nil, nil, nil, cerr
		}
		artifacts = append(artifacts, newNepraArtifact(url, raw, content, contentType, len(r.Observations), 0))
		reports = append(reports, r)
		if r.DetectedFY != "" && nepraper.NormalizeFY(r.DetectedFY) != nepraper.NormalizeFY(fy) {
			// Both are recorded and neither is overridden.
			fmt.Fprintf(cmd.ErrOrStderr(),
				"NOTE: %s was requested but the document's running header says %s; both recorded, neither "+
					"overridden.\n", nepraper.NormalizeFY(fy), r.DetectedFY)
		}
		breaks, bskips := perBreaks(r, sel.BreakRatio)
		derived = append(derived, breaks...)
		skips = append(skips, bskips...)
	}
	if len(reports) == 1 {
		derived = append(derived, perConflictEntries(reports[0])...)
	} else if len(reports) > 1 {
		// More than one report read: check across them as well as within
		// each, which is how a figure republished in a later comparison table
		// is checked against the year it was first published in.
		derived = append(derived, perConflictEntries(reports[0], reports[1:]...)...)
	}

	for _, fy := range genYears {
		raw, url, contentType, ferr := conflictsFetchWorkbook(ctx, c, fy)
		if ferr != nil {
			return nil, nil, nil, nil, nil, conflictsFetchError(cmd, flags, ferr)
		}
		// The bytes go in UNDECODED: ParseWorkbook does the windows-1252
		// decoding, and the HTTP response declares no charset.
		w, werr := nepraparse.ParseWorkbook(raw, conflictBareFY(fy))
		if werr != nil {
			return nil, nil, nil, nil, nil, fmt.Errorf("parsing the %s workbook: %w", fy, werr)
		}
		content, cerr := genContentHash(w)
		if cerr != nil {
			return nil, nil, nil, nil, nil, cerr
		}
		artifacts = append(artifacts, newNepraArtifact(url, raw, content, contentType, len(w.Plants), 0))

		if sel.Surface != conflictSurfaceCapacity {
			// The column-order guard. Without it every GWh read could be a
			// utilisation and every number would look plausible and be wrong.
			if gerr := conflictsColumnOrderGuard(w); gerr != nil {
				return nil, nil, nil, nil, nil, fmt.Errorf("%s: %w", fy, gerr)
			}
			derived = append(derived, genSumEntries(w, fy)...)
			zlf, cen := genZeroLoadFactorEntries(w, fy)
			derived = append(derived, zlf...)
			census = append(census, cen)
		}
		if sel.Surface == "" || sel.Surface == conflictSurfaceCapacity {
			derived = append(derived, capacityEntry(w, fy))
		}
	}
	return derived, artifacts, skips, census, read, nil
}

// conflictsFetchError maps a fetch failure onto the typed exit ladder. A
// cliError from a leg keeps its own code.
func conflictsFetchError(cmd *cobra.Command, flags *rootFlags, err error) error {
	var typed *cliError
	if errors.As(err, &typed) {
		return err
	}
	return classifyAPIError(cmd.OutOrStdout(), err, flags)
}

// conflictsYearsToRead works out which documents the selection needs, from the
// selected entries' own recompute requirements.
func conflictsYearsToRead(sel conflictsSelection, ledger []conflictEntry) (per, gen []string) {
	want := map[string]bool{}
	if len(sel.FYs) > 0 {
		for _, fy := range sel.FYs {
			want[fy] = true
		}
	} else {
		for _, e := range conflictsFilter(ledger, sel) {
			if e.Computable == conflictNotComputable {
				continue
			}
			for _, fy := range e.RecomputeNeeds {
				want[nepraper.NormalizeFY(fy)] = true
			}
		}
	}
	perSet := map[string]bool{}
	genSet := map[string]bool{}
	for fy := range want {
		if sel.Surface == "" || sel.Surface == conflictSurfacePER {
			if src, ok := nepraper.SourceFor(fy); ok && src.Availability == nepraper.AvailabilityPublished {
				perSet[fy] = true
			}
		}
		if sel.Surface == "" || sel.Surface == conflictSurfaceGen || sel.Surface == conflictSurfaceCapacity {
			if conflictsGenYearReachable(fy) {
				genSet[fy] = true
			}
		}
	}
	for fy := range perSet {
		per = append(per, fy)
	}
	for fy := range genSet {
		gen = append(gen, fy)
	}
	sort.Strings(per)
	sort.Strings(gen)
	return per, gen
}

// conflictsWriteByteCost tells the caller what is about to be downloaded,
// BEFORE the first request.
//
// Binary responses bypass the client's response cache — responseCacheEnabled
// returns false whenever the binary header is set — so every --recompute run
// of the PER leg refetches in full. A year whose size this build has never
// measured is reported as unmeasured rather than estimated.
func conflictsWriteByteCost(cmd *cobra.Command, perYears, genYears []string) {
	w := cmd.ErrOrStderr()
	known := 0
	var unknown []string
	for _, fy := range perYears {
		if src, ok := nepraper.SourceFor(fy); ok && src.Bytes > 0 {
			known += src.Bytes
			continue
		}
		unknown = append(unknown, fy+" (PER)")
	}
	for _, fy := range genYears {
		if b, ok := conflictsWorkbookBytes[fy]; ok {
			known += b
			continue
		}
		unknown = append(unknown, fy+" (workbook)")
	}
	fmt.Fprintf(w, "--recompute will fetch %d PER PDF(s) and %d workbook(s): %d measured bytes",
		len(perYears), len(genYears), known)
	if len(unknown) > 0 {
		fmt.Fprintf(w, " plus %d document(s) whose size this build has never measured (%s)",
			len(unknown), strings.Join(unknown, ", "))
	}
	fmt.Fprintf(w, ".\nBinary responses bypass the client's response cache, so every --recompute run "+
		"refetches the PDFs in full. NEPRA serves them at a measured 68-164 KB/s.\n")
}

// conflictsReconcileAgainstLedger compares the derived entries against the
// shipped ledger.
//
// It never merges the two. A ledger entry the document does not reproduce
// keeps its ledger figures and gains a verdict; a derived entry with no ledger
// row is reported as a NEW FINDING rather than dropped, because a recompute
// that discovers a fifth FY2024-25 conflict has to say so.
func conflictsReconcileAgainstLedger(ledger, derived []conflictEntry, sel conflictsSelection,
	read map[string]bool) ([]conflictEntry, []conflictRecompute) {
	byID := map[string]int{}
	for i, d := range derived {
		if _, dup := byID[d.ID]; dup {
			// Two derived entries with one id is a bug in the id scheme, not
			// a data finding. Keep the first and say so rather than
			// overwriting silently.
			continue
		}
		byID[d.ID] = i
	}
	used := map[string]bool{}
	out := append([]conflictEntry(nil), ledger...)
	var rec []conflictRecompute

	for i := range out {
		e := out[i]
		if !conflictsSelects(e, sel) {
			continue
		}
		idx, ok := byID[e.ID]
		matchDetail := ""
		if !ok {
			// Fall back to the key, because a live_only entry whose table
			// labels the survey never recorded cannot have a matching id.
			for j, d := range derived {
				if used[d.ID] || d.Surface != e.Surface || d.Kind != e.Kind {
					continue
				}
				if d.Key != e.Key {
					continue
				}
				idx, ok = j, true
				matchDetail = "matched on key rather than id; the document's tables are " +
					d.A.Table + " and " + d.B.Table + " where the ledger recorded " +
					e.A.Table + " and " + e.B.Table
				break
			}
		}
		if !ok {
			verdict := conflictNotInDocument
			detail := "the documents read on this run produced no entry for this key; the ledger figures " +
				"are unchanged and are NOT confirmed by this run"
			if missing := conflictsMissingYears(e, read); len(missing) > 0 {
				verdict = conflictNotRead
				detail = "this entry needs " + strings.Join(missing, ", ") + ", which this run did not " +
					"read, so it could not be checked either way. It is NOT a failure to reproduce; widen " +
					"--fy or --surface to test it"
			}
			rec = append(rec, conflictRecompute{
				ID: e.ID, Surface: e.Surface, Kind: e.Kind,
				Verdict: verdict, Reproduced: false,
				LedgerComputable: e.Computable, LedgerA: e.A.Value, LedgerB: e.B.Value,
				Detail: detail,
			})
			continue
		}
		d := derived[idx]
		used[d.ID] = true
		r := conflictRecompute{
			ID: e.ID, Surface: e.Surface, Kind: e.Kind, LedgerComputable: e.Computable,
			LedgerA: e.A.Value, LedgerB: e.B.Value,
			ObservedA: d.A.Value, ObservedB: d.B.Value, ObservedRatio: d.Ratio,
			Detail: matchDetail,
		}
		if conflictsFiguresAgree(e.A.Value, d.A.Value) && conflictsFiguresAgree(e.B.Value, d.B.Value) {
			r.Verdict, r.Reproduced = conflictReproduced, true
			// A live_only entry that reproduces has now been read from the
			// document, so the entry says so instead of still claiming it is
			// only a quote.
			out[i].Computable = conflictVerified
			out[i].Derivation = d.Derivation
			out[i].ReproducedByThisBuild = true
			out[i].MeasuredFrom = "reproduced live from " + d.A.URL + " on this run"
		} else {
			r.Verdict = conflictFiguresDiffer
			if r.Detail != "" {
				r.Detail += "; "
			}
			r.Detail += "the document's figures differ from the ledger's; BOTH are reported and neither " +
				"replaces the other"
		}
		rec = append(rec, r)
	}

	for _, d := range derived {
		if used[d.ID] {
			continue
		}
		if _, inLedger := conflictsFindByID(ledger, d.ID); inLedger {
			continue
		}
		out = append(out, d)
		rec = append(rec, conflictRecompute{
			ID: d.ID, Surface: d.Surface, Kind: d.Kind, Verdict: conflictNewFinding, Reproduced: true,
			LedgerComputable: "", ObservedA: d.A.Value, ObservedB: d.B.Value, ObservedRatio: d.Ratio,
			Detail: "found in the document and ABSENT from the shipped ledger; it is added to this run's " +
				"entries rather than dropped, and the ledger is now known to be incomplete for this key",
		})
	}
	return out, rec
}

// conflictsMissingYears lists the documents an entry needs that this run did
// not read. It is what separates "the document contradicts the ledger" from
// "nobody looked".
func conflictsMissingYears(e conflictEntry, read map[string]bool) []string {
	var out []string
	for _, fy := range e.RecomputeNeeds {
		if !read[nepraper.NormalizeFY(fy)] {
			out = append(out, nepraper.NormalizeFY(fy))
		}
	}
	sort.Strings(out)
	return out
}

func conflictsFindByID(in []conflictEntry, id string) (conflictEntry, bool) {
	for _, e := range in {
		if e.ID == id {
			return e, true
		}
	}
	return conflictEntry{}, false
}

// conflictsFiguresAgree compares two published figures at the precision they
// were published to. The workbooks publish two decimals and the PERs at most
// three, so a difference under half of the last published digit is float
// representation, not disagreement.
func conflictsFiguresAgree(a, b *float64) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	scale := math.Max(math.Abs(*a), math.Abs(*b))
	return math.Abs(*a-*b) <= math.Max(0.005, scale*1e-9)
}
