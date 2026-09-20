// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// HAND-AUTHORED implementation of the `verify` novel command. Not generated,
// and must survive `generate --force`.
// See .printing-press-patches/nepra-verify-fetch-gate.json.
// pp:data-source live
// Supported strategies: auto, local, live, or computed. Change this default deliberately.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// verifyReport is the whole document this command emits.
//
// Every top-level key is emitted unconditionally, staleness and crosscheck
// included. That is load-bearing rather than tidy: --agent's envelope
// flattens a two-key {meta, results:[...]} object down to the bare array,
// which would drop meta.verdict — the one field a caller must be able to read
// in every mode.
type verifyReport struct {
	Meta       verifyMeta            `json:"meta"`
	Results    []verifySurfaceResult `json:"results"`
	Staleness  *verifyStaleness      `json:"staleness"`
	Crosscheck *verifyCrosscheck     `json:"crosscheck"`
}

type verifyMeta struct {
	Source          string `json:"source"`
	ManifestAsOf    string `json:"manifest_as_of"`
	CheckedAt       string `json:"checked_at"`
	SurfacesChecked int    `json:"surfaces_checked"`
	Verdict         string `json:"verdict"`
	// ExitCode is the code this invocation will return, printed alongside
	// the document so a machine caller never has to infer it and so a
	// non-zero exit is never indistinguishable from a crash.
	ExitCode int    `json:"exit_code"`
	Note     string `json:"note"`
	// DeclaredGaps are the questions this command does not answer, with the
	// reason for each. Empty is a claim; a populated list is a disclosure.
	DeclaredGaps []verifyGap `json:"declared_gaps"`
}

const verifyStatusNote = "http_status is recorded only where it was actually observed. On a " +
	"successful fetch this client returns the body without the status code, so status_exact is " +
	"null and status_class is 2xx-3xx; on a failure the exact code comes from the transport's " +
	"typed error. A recorded 200 would be a value never seen."

// newNovelVerifyCmd builds the fetch, schema and staleness gate.
//
// The constructor name is fixed: root.go already registers newNovelVerifyCmd,
// and this file replaces the TODO scaffold in place rather than adding a
// second registration.
//
// WHAT THIS COMMAND IS FOR. An HTTP 200 from NEPRA is not evidence of data.
// Three catalogued bodies return like data and are not data — a 9-byte "Not
// Found", a 9,838-byte <frameset> shell that is byte-identical across three
// fiscal years, and a 374-byte stub titled "SIR Data 2024" that answers to
// "SIR Data 2025.htm" and frames FY2023-24 — and the only way to tell them
// from a workbook is to read the in-document band label and the 32-column
// fingerprint rather than the filename, and to hash the extracted rows rather
// than the response bytes.
//
// NOTE ON THE SHORT DESCRIPTION. The scaffold said "Assert every CACHED
// artifact". There is no artifact store in this CLI: no sync command exists,
// the local SQLite store holds only the teach loop's rows, and internal
// client's HTTP cache is opaque with no enumeration accessor. This is a FETCH
// gate, and the wording here says so. root.go's highlights block and SKILL.md
// still carry the old sentence and are generated; they need the same edit.
func newNovelVerifyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagFy         string
		flagSurface    string
		flagAll        bool
		flagManifest   bool
		flagCrosscheck string
		flagCheckStale bool
		flagStaleAfter int
	)

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Gate a published NEPRA surface on status, byte floor, decoys, charset, in-document year and the 32-column fingerprint",
		Long: `Fetch a catalogued NEPRA surface and refuse it unless every leg passes.

An HTTP 200 from this site is not evidence of data. Three catalogued bodies answer like data and
are not data: a 9-byte "Not Found", a 9,838-byte <frameset> shell that is byte-identical across
FY2017-18, FY2020-21 and FY2023-24, and a 374-byte stub titled "SIR Data 2024" that also answers
to "SIR Data 2025.htm" and frames FY2023-24. So the fiscal year is read from the in-document
<td colspan=26> band label and the 32-column header fingerprint, never from the filename, and the
checksum is taken over the EXTRACTED ROWS rather than the response bytes — two identical fetches
of one NEPRA page returned the same 74,431 bytes with different SHA-256s, so a raw-byte gate
would report drift on every run for a page that has not changed.

With no selector it prints the frozen expectation manifest and makes NO request.

Scope, stated rather than implied:
  - This is a FETCH gate, not a cache gate. This CLI has no artifact store to assert against.
  - Row, cell and plant counts are asserted only for FY2017-18, FY2020-21, FY2023-24 and the four
    Excel sheets. The other four reachable years have no fixture; their internals are computed and
    REPORTED under measured_but_not_asserted and cannot fail the gate. They genuinely differ:
    FY2018-19 and FY2019-20 are 32 physical columns wide, and FY2021-22 fails the Sum==sum(12
    months) identity on 9 of 125 published rows.
  - --crosscheck iea reports the NEPRA side and the reasons the two series cannot be subtracted.
    It does not read the IEA series in this build; see the declared gap in its output.

Exit codes:
  0  every leg passed, or the only findings were report-only
  2  usage error (bad flag combination, unparseable year, unknown surface, unusable source)
  3  the manifest records this fiscal year as never published, with the evidence
  5  a catalogued surface could not be reached at all — a 5xx, a DNS failure or
     a timeout. That is NOT a gate verdict: it says nothing about what NEPRA
     publishes, so it keeps its own code and meta.exit_code carries it
  6  a catalogued surface answered, and what it answered failed the gate`,
		Example: "  nepra-pp-cli verify\n" +
			"  nepra-pp-cli verify --manifest\n" +
			"  nepra-pp-cli verify --fy 2023-24 --agent\n" +
			"  nepra-pp-cli verify --fy 2023-24 --check-stale --stale-after 90\n" +
			"  nepra-pp-cli verify --surface fca --json",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--fy=2023-24",
			"pp:typed-exit-codes": "0,2,3,5,6",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// --crosscheck with no selector gates the manifest's newest
			// reachable year rather than falling through to the manifest
			// print, and the choice is recorded in meta rather than made
			// silently.
			crosscheckDefaulted := false
			if flagCrosscheck != "" && !flagManifest && flagFy == "" && flagSurface == "" && !flagAll {
				flagFy = verifyNewestPublishedFY
				crosscheckDefaulted = true
			}

			// (1) Help-only branch. The manifest is the useful zero-argument
			// answer and it costs no request, so a bare `verify` prints it.
			// This precedes the dry-run guard deliberately, which means
			// `verify --manifest --dry-run` prints the manifest.
			if flagManifest || (flagFy == "" && flagSurface == "" && !flagAll) {
				if err := verifyRejectManifestCombination(flagFy, flagSurface, flagAll, flagCrosscheck); err != nil {
					return verifyRefuse(cmd, flags, usageErr(err))
				}
				return verifyPrintManifest(cmd, flags)
			}

			// (2) Dry-run short-circuit, before any IO.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "verify")
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// (3) Validation.
			if len(args) > 0 {
				return verifyRefuse(cmd, flags, usageErr(
					fmt.Errorf("verify takes no positional arguments; got %q", args[0])))
			}
			targets, fy, err := verifySelectSurfaces(flagFy, flagSurface, flagAll)
			if err != nil {
				return verifyRefuse(cmd, flags, err)
			}
			if flagCrosscheck != "" {
				if err := verifyValidateCrosscheck(flagCrosscheck); err != nil {
					return verifyRefuse(cmd, flags, usageErr(err))
				}
			}
			if flagStaleAfter <= 0 {
				return verifyRefuse(cmd, flags, usageErr(
					fmt.Errorf("--stale-after must be a positive number of days; got %d. "+
						"A non-positive window would fail every run and say nothing", flagStaleAfter)))
			}
			if !flagCheckStale && cmd.Flags().Changed("stale-after") {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"note: --stale-after %d has no effect without --check-stale; the staleness "+
						"measurements are reported either way but only --check-stale lets them "+
						"reach the exit code.\n", flagStaleAfter)
			}
			// verify only reads the live site. There is no local artifact
			// store to gate against, so --data-source local cannot be
			// honoured and is refused rather than silently ignored.
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return verifyRefuse(cmd, flags, usageErr(
					fmt.Errorf("%w\nverify gates LIVE fetches: this CLI has no local artifact store "+
						"to assert against, and --crosscheck reaches a THIRD-PARTY host "+
						"(api.iea.org), never the local store", err)))
			}

			// (4) Real work.
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			note := verifyStatusNote
			if crosscheckDefaulted {
				note += fmt.Sprintf(" --crosscheck was given no --fy, so it gated the manifest's "+
					"newest reachable year, FY%s.", flagFy)
			}
			if cliutil.IsDogfoodEnv() && len(targets) > 1 {
				before := len(targets)
				targets = verifyCurtail(targets)
				note += fmt.Sprintf(" Running under the live-dogfood matrix, so the surface set was "+
					"bounded from %d to %d to fit the flat per-command budget. The requests that "+
					"were made are real; none was substituted.", before, len(targets))
			}

			report := verifyReport{
				Meta: verifyMeta{
					Source: "live", ManifestAsOf: verifyManifestAsOf,
					CheckedAt: time.Now().UTC().Format(time.RFC3339),
					Note:      note, DeclaredGaps: []verifyGap{},
				},
				Results: []verifySurfaceResult{},
			}

			for _, s := range targets {
				url := c.RequestBaseURL() + s.Path
				f := verifyFetch(ctx, c, s.Path)
				// A transport-level failure that is not a status the gate
				// models (5xx, DNS, timeout) is NOT a gate failure: it says
				// nothing about what NEPRA publishes. It keeps its own exit
				// code, and the document is printed first so a machine
				// caller can see which surface stopped the run.
				if f.Err != nil && !verifyIsGateableFailure(f) {
					report.Results = append(report.Results, verifyFetchFailureResult(s, url, f))
					report.Meta.Verdict = verifyFail
					report.Meta.SurfacesChecked = len(report.Results)
					report.Staleness = verifyStalenessPtr(flagCheckStale, flagStaleAfter)
					// BUILD THE TYPED ERROR FIRST AND HAND IT TO verifyEmit.
					//
					// Passing nil here left meta.exit_code at 0 — verifyEmit
					// only fills it when gate != nil — while this function
					// went on to return a non-zero error. meta.exit_code's own
					// doc says it exists "so a machine caller never has to
					// infer it and so a non-zero exit is never
					// indistinguishable from a crash", and that is exactly
					// what broke: with NEPRA unreachable the process exited 5
					// and the document said exit_code 0, so an agent reading
					// the field as instructed saw a clean pass. Fires on any
					// 5xx, DNS failure or timeout.
					gate := classifyAPIErrorOnly(f.Err)
					report.Meta.ExitCode = ExitCode(gate)
					if perr := verifyEmit(cmd, flags, &report, gate); perr != nil {
						return perr
					}
					return gate
				}
				switch s.Kind {
				case verifyKindWorkbook:
					report.Results = append(report.Results, verifyWorkbookLegs(s, url, f))
				default:
					report.Results = append(report.Results, verifySheetLegs(s, url, f))
				}
			}
			report.Meta.SurfacesChecked = len(report.Results)
			report.Meta.Verdict = verifyWorstVerdict(report.Results)

			// Staleness. The measurements are always reported; --check-stale
			// is what lets them reach the exit code.
			st := verifyBuildStaleness(verifyManifestAsOf, time.Now().UTC(), flagStaleAfter, flagCheckStale)
			if flagCheckStale {
				st.NextFYProbe = verifyProbeNextFY(ctx, c)
			} else {
				report.Meta.DeclaredGaps = append(report.Meta.DeclaredGaps, verifyGap{
					Gap: "whether a newer fiscal year has been published",
					Reason: "The next-year probe costs a request and was not asked for. Pass " +
						"--check-stale to make it, and to let the staleness measurements set " +
						"the exit code.",
				})
			}
			report.Staleness = &st

			// Crosscheck.
			if flagCrosscheck != "" {
				cc, err := verifyAttachCrosscheck(&report, fy)
				if err != nil {
					return err
				}
				report.Crosscheck = cc
				if cc != nil && cc.DeclaredGap != nil {
					report.Meta.DeclaredGaps = append(report.Meta.DeclaredGaps, *cc.DeclaredGap)
				}
			}

			// Assemble the exit code, then print, then return it. Under
			// --json the document reaches stdout AND the code is set: a gate
			// that exited non-zero with empty stdout would be
			// indistinguishable from a crash.
			var gate error
			if report.Meta.Verdict == verifyFail {
				gate = gateErr(fmt.Errorf("%d of %d catalogued surfaces failed the gate",
					verifyCountFailed(report.Results), len(report.Results)))
			}
			if gate == nil {
				gate = verifyStalenessGate(&st)
			}
			if gate != nil {
				report.Meta.ExitCode = ExitCode(gate)
				report.Meta.Verdict = verifyFail
			}
			if err := verifyEmit(cmd, flags, &report, gate); err != nil {
				return err
			}
			return gate
		},
	}

	cmd.Flags().StringVar(&flagFy, "fy", "",
		"Fiscal year to gate: 2017-18 through 2023-24 (\"FY 2023-24\" and \"2023-2024\" also parse). "+
			"The URL token is rebuilt from the parsed year, never echoed: the FY-prefixed form 404s on every year")
	cmd.Flags().StringVar(&flagSurface, "surface", "",
		"Gate one non-generation Excel sheet: fca, quarterly, sro or hydel")
	cmd.Flags().BoolVar(&flagAll, "all", false,
		"Gate every catalogued surface: 7 generation workbooks plus 4 Excel sheets, 11 requests and ~3.4 MB decoded. Mutually exclusive with --fy and --surface")
	cmd.Flags().BoolVar(&flagManifest, "manifest", false,
		"Print the frozen expectation manifest (paths, byte floors, expected year tokens, the 32-column fingerprint and every known decoy) and make no request")
	cmd.Flags().StringVar(&flagCrosscheck, "crosscheck", "",
		"Reconcile the summed Sum-GWh column against an external series. Only value: iea. Reports both sides and the reasons they cannot be subtracted; never emits a delta")
	cmd.Flags().BoolVar(&flagCheckStale, "check-stale", false,
		"Let the staleness measurements set the exit code, and probe whether a newer fiscal year has been published (one extra request). Off by default so a cron opts in")
	cmd.Flags().IntVar(&flagStaleAfter, "stale-after", verifyStaleAfterDefault,
		"Staleness window in days for the manifest's own measurement date (default 90). Only consulted with --check-stale")
	return cmd
}

// verifyRefuse ends an invocation on bad or unanswerable input.
//
// It writes a machine-readable {error, code, usage} document to STDOUT before
// returning, then preserves the typed exit code. An agent that reads only
// stdout would otherwise see nothing and could not tell a refusal from a
// crash — the same reason the JSON envelope accompanies a gate failure.
func verifyRefuse(cmd *cobra.Command, flags *rootFlags, err error) error {
	code := ExitCode(err)
	if flags != nil && flags.asJSON {
		if perr := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"error":   err.Error(),
			"code":    code,
			"verdict": "refused",
			"usage":   cmd.CommandPath() + " [--fy <fy> | --surface <id> | --all] [--manifest] [--crosscheck iea] [--check-stale] [--stale-after <days>]",
		}, flags); perr != nil {
			return perr
		}
	} else {
		_ = cmd.Usage()
	}
	return err
}

// verifyRejectManifestCombination refuses a manifest print that also fetched.
func verifyRejectManifestCombination(fy, surface string, all bool, crosscheck string) error {
	var with []string
	if fy != "" {
		with = append(with, "--fy")
	}
	if surface != "" {
		with = append(with, "--surface")
	}
	if all {
		with = append(with, "--all")
	}
	if crosscheck != "" {
		with = append(with, "--crosscheck")
	}
	if len(with) == 0 {
		return nil
	}
	return fmt.Errorf("--manifest cannot be combined with %s: --manifest makes NO request, and a "+
		"manifest print that also fetched would make the offline mode a lie. Run them separately",
		strings.Join(with, " or "))
}

// verifySelectSurfaces resolves the caller's selector.
func verifySelectSurfaces(fyArg, surfaceID string, all bool) ([]verifySurface, string, error) {
	selectors := 0
	if fyArg != "" {
		selectors++
	}
	if surfaceID != "" {
		selectors++
	}
	if all {
		selectors++
	}
	if selectors > 1 {
		return nil, "", usageErr(fmt.Errorf("--fy, --surface and --all are mutually exclusive; pick one"))
	}
	if all {
		return verifySurfaces, verifyNewestPublishedFY, nil
	}
	if surfaceID != "" {
		s, ok := verifySurfaceByID(surfaceID)
		if !ok || s.Kind != verifyKindSheet {
			return nil, "", usageErr(fmt.Errorf("unknown surface %q; the catalogued Excel sheets are %s. "+
				"For a generation workbook use --fy <year>",
				surfaceID, strings.Join(verifySheetIDs(), ", ")))
		}
		return []verifySurface{s}, "", nil
	}

	// A fiscal year. Parse it, then rebuild the path token from the parsed
	// value: substituting the raw argument is how "FY2023-24" reaches the
	// URL, and that form is an HTTP 404 with a 9-byte body on every year.
	fy, err := nepraparse.ParseFiscalYear(fyArg)
	if err != nil {
		return nil, "", usageErr(fmt.Errorf("cannot read %q as a fiscal year: %w\n"+
			"accepted forms: \"2023-24\", \"FY 2023-24\", \"2023-2024\"", fyArg, err))
	}
	if u, ok := verifyUnpublishedYearFor(fy.Label()); ok {
		return nil, "", notFoundErr(fmt.Errorf("the Detail-of-Generation series does not publish "+
			"FY%s. Recorded evidence, measured %s: %s\n"+
			"This is answered from the manifest rather than probed, so a 404 is not presented as a "+
			"discovery. Reachable years: %s. Pass --check-stale to probe whether a newer year has "+
			"appeared since", u.FY, u.AsOf, u.Evidence, strings.Join(verifyGenerationYears(), ", ")))
	}
	s, ok := verifySurfaceForFY(fy.Label())
	if !ok {
		return nil, "", usageErr(fmt.Errorf("FY%s is not in this manifest and is not recorded as "+
			"unpublished either, so there is no floor to gate it against. Catalogued years: %s",
			fy.Label(), strings.Join(verifyGenerationYears(), ", ")))
	}
	return []verifySurface{s}, fy.Label(), nil
}

func verifyValidateCrosscheck(value string) error {
	for _, s := range verifyCrosscheckSources {
		if strings.EqualFold(value, s) {
			return nil
		}
	}
	return fmt.Errorf("unknown --crosscheck source %q; the only known external series is %s "+
		"(%s). It is never silently ignored", value,
		strings.Join(verifyCrosscheckSources, ", "), verifyIEAURL)
}

// verifyCurtail bounds a wide surface set for the live-dogfood budget. It
// keeps the surfaces whose internals are actually asserted plus the smallest
// sheet, so the bounded run still exercises every leg.
func verifyCurtail(in []verifySurface) []verifySurface {
	keep := map[string]bool{"gen-2023-24": true, "hydel": true}
	var out []verifySurface
	for _, s := range in {
		if keep[s.ID] {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return in[:1]
	}
	return out
}

func verifyCountFailed(results []verifySurfaceResult) int {
	n := 0
	for _, r := range results {
		if r.Verdict == verifyFail {
			n++
		}
	}
	return n
}

// verifyIsGateableFailure reports whether a failed fetch is a statement about
// what NEPRA publishes (a 404 or another 4xx) rather than about the transport.
func verifyIsGateableFailure(f verifyFetchResult) bool {
	return f.StatusExact != nil && *f.StatusExact >= 400 && *f.StatusExact < 500
}

func verifyFetchFailureResult(s verifySurface, url string, f verifyFetchResult) verifySurfaceResult {
	res := newVerifySurfaceResult(s, url)
	res.add(verifyStatusLeg(f))
	res.skipRest("the surface could not be reached, so nothing about its content was measured",
		"byte_floor", "decoy_404_stub", "decoy_frameset", "charset",
		"year_token", "column_fingerprint", "structure", "sum_invariant", "column_order", "content_checksum")
	return res
}

func verifyStalenessPtr(gated bool, window int) *verifyStaleness {
	st := verifyBuildStaleness(verifyManifestAsOf, time.Now().UTC(), window, gated)
	return &st
}

// verifyStalenessGate turns the staleness measurements into an exit code.
//
// Only two things can fire: the manifest being older than the window, and the
// next-year probe finding a real new fiscal year. Days-since-coverage-end
// cannot, by design.
func verifyStalenessGate(st *verifyStaleness) error {
	if st == nil || !st.Gated {
		// The measurements are always reported; only --check-stale lets them
		// reach the exit code. Checking the flag HERE rather than at the call
		// site means an ungated measurement cannot leak into the code through
		// a future second caller.
		return nil
	}
	if st.ManifestVerdict == verifyFail {
		return gateErr(fmt.Errorf("the expectation manifest was measured %d days ago (%s) and the "+
			"staleness window is %d days: its floors and internals may no longer describe what "+
			"NEPRA publishes", st.ManifestAgeDays, st.ManifestMeasuredAt, st.WindowDays))
	}
	if st.NextFYProbe != nil && st.NextFYProbe.Verdict == verifyFail {
		return gateErr(fmt.Errorf("the next-fiscal-year probe failed: %s", st.NextFYProbe.Note))
	}
	return nil
}

// verifyAttachCrosscheck computes the NEPRA side of the external comparison.
func verifyAttachCrosscheck(report *verifyReport, fy string) (*verifyCrosscheck, error) {
	if fy == "" {
		// --crosscheck with no --fy defaults to the manifest's newest
		// reachable year, and says so rather than picking silently.
		fy = verifyNewestPublishedFY
		report.Meta.Note += fmt.Sprintf(" --crosscheck was given no --fy, so it used the manifest's "+
			"newest reachable year, FY%s.", fy)
	}
	// The sigma is computed from the workbook this run actually fetched, so
	// the number always belongs to bytes whose provenance is in the payload.
	for _, r := range report.Results {
		if r.Kind != verifyKindWorkbook || r.FY == nil || *r.FY != fy {
			continue
		}
		if r.Verdict == verifyFail {
			return &verifyCrosscheck{
				Source: "iea", URL: verifyIEAURL, Verdict: verifyReported,
				DeclaredGap: &verifyGap{
					Gap: "the NEPRA side of the crosscheck",
					Reason: "FY" + fy + " failed the gate on this run, so its numbers were not " +
						"summed. A total taken from a body that failed the schema gate would be " +
						"a number with no standing.",
				},
				Delta:      verifyDelta{Reason: "no NEPRA side to compare"},
				GapReasons: verifyCrosscheckGapReasons(fy),
			}, nil
		}
		if r.sigma == nil {
			return &verifyCrosscheck{
				Source: "iea", URL: verifyIEAURL, Verdict: verifyReported,
				DeclaredGap: &verifyGap{
					Gap:    "the NEPRA side of the crosscheck",
					Reason: "FY" + fy + " was read but produced no summable panel on this run.",
				},
				Delta:      verifyDelta{Reason: "no NEPRA side to compare"},
				GapReasons: verifyCrosscheckGapReasons(fy),
			}, nil
		}
		return verifyBuildCrosscheck(fy, *r.sigma), nil
	}
	return &verifyCrosscheck{
		Source: "iea", URL: verifyIEAURL, Verdict: verifyReported,
		DeclaredGap: &verifyGap{
			Gap: "the NEPRA side of the crosscheck",
			Reason: "FY" + fy + " was not among the surfaces this run fetched, and this command " +
				"will not sum a year it did not read. Add --fy " + fy + ".",
		},
		Delta:      verifyDelta{Reason: "no NEPRA side to compare"},
		GapReasons: verifyCrosscheckGapReasons(fy),
	}, nil
}

// verifyEmit prints the document, then the human summary, in that order.
func verifyEmit(cmd *cobra.Command, flags *rootFlags, report *verifyReport, gate error) error {
	if report.Meta.ExitCode == 0 && gate != nil {
		report.Meta.ExitCode = ExitCode(gate)
	}
	if report.Meta.Verdict == "" {
		report.Meta.Verdict = verifyPass
	}
	agentMeta := map[string]any{
		"source":         report.Meta.Source,
		"verdict":        report.Meta.Verdict,
		"exit_code":      report.Meta.ExitCode,
		"manifest_as_of": report.Meta.ManifestAsOf,
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		verifyLegSummary(cmd.ErrOrStderr(), report.Results)
		fmt.Fprintf(cmd.ErrOrStderr(), "%s: %d surface(s), manifest measured %s\n",
			strings.ToUpper(report.Meta.Verdict), report.Meta.SurfacesChecked, report.Meta.ManifestAsOf)
		for _, g := range report.Meta.DeclaredGaps {
			fmt.Fprintf(cmd.ErrOrStderr(), "DECLARED GAP: %s — %s\n", g.Gap, g.Reason)
		}
	}
	raw, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, agentMeta)
}
