// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-disco-panel.json.
//
// pp:data-source live
// PER PDFs are never synced into the SQLite store, so there is no local
// panel to serve and --data-source local is refused rather than answered
// emptily. Repeated from disco.go so the annotation sits in the file that
// declares the cobra.Command, which is where the surface scanner resolves it.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// discoAsOfDate is the date the corpus byte and page counts were measured. It
// is printed alongside every assertion so a mismatch can be dated.
const discoAsOfDate = "2026-09-10"

// init registers the panel as a novel command in its own right, so that a
// `generate --force` which refreshes internal/cli/disco.go back into a TODO
// scaffold cannot take the implementation with it: addNovelCommandIfAbsent
// prefers an implemented command over an annotated scaffold of the same name.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newNepraDiscoCmd(flags))
	})
}

// newNepraDiscoCmd builds the per-DISCO reliability panel.
//
// It is the first consumer of internal/nepraper, and it is deliberately not a
// "give me MEPCO's SAIDI" command: for FY2024-25 that question has TWO
// published answers, 3547.00 in Table 06 (PDF page 15) and 1182.56 in Table 18
// (PDF page 29), each internally consistent with its own chart, with no
// tie-breaker anywhere in the document. Both reach the caller with their own
// page-level citation. Nothing here averages, prefers or de-duplicates them.
func newNepraDiscoCmd(flags *rootFlags) *cobra.Command {
	var (
		flagMetric  string
		flagFY      string
		flagVariant string
		flagPeriod  string
		flagEntity  string
		flagPDF     string
		flagStrict  bool
		flagLimit   int
	)

	cmd := &cobra.Command{
		Use:   "disco",
		Short: "Per-DISCO reliability panel: actual, allowed-in-tariff and breach as three separate quantities",
		Long: `disco reads one NEPRA Performance Evaluation Report (PER) and returns every figure it
published for the selected parameter, per distribution licensee, with a page-level citation.

actual, target (what NEPRA allowed in tariff or set as a target) and breach are THREE
separately typed fields on every row. They are never merged and an absent one never reads
as zero: a five-year comparison column publishes no target by construction, and the breach
column is numeric in the early reports but a label ("Far Away", "Near to Limit", "Away")
from FY2020-21 on.

Ten licensees are evaluated, not eleven. NEPRA excludes TESCO from the PER on the record;
--entity TESCO returns that stated exclusion, never an empty row and never a zero. The
"W. Av:" summary row is returned under its own weighted_average key and can never be
reached by iterating results.

Where one report contradicts itself the panel hands over BOTH sides. FY2024-25 prints
MEPCO's SAIDI as 3547.00 (Table 06, PDF page 15) and as 1182.56 (Table 18, PDF page 29) —
a ratio of 2.9994250 — and SAIFI as 30.67 against 10.23. Narrowing with --variant still
reports the suppressed side's table, page and figure.

With no --metric this prints the report-year catalogue and makes NO request.

Exit codes:
  0  the panel was produced, or the catalogue was printed
  2  usage error: unknown --metric, --variant or --entity; a --fy this build has no record
     of; a negative --limit; or --data-source local, for which no local PER store exists
  3  nothing to return, with the reason: FY2023-24's PER 404s on NEPRA's own index, or the
     requested metric/variant/period/entity combination matched no published row`,
		Example: `  nepra-pp-cli disco
  nepra-pp-cli disco --metric saidi --fy 2024-25 --agent
  nepra-pp-cli disco --metric tnd --fy 2024-25 --agent
  nepra-pp-cli disco --metric saifi --entity MEPCO --json
  nepra-pp-cli disco --metric saidi --variant comparison --period 2023-24 --json
  nepra-pp-cli disco --metric complaints --fy 2014-15 --json
  nepra-pp-cli disco --metric saidi --pdf ./per-2024-25.pdf --fy 2024-25 --strict`,
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--metric=saidi;--fy=2024-25",
			"pp:typed-exit-codes": "0,2,3",
			"pp:novel-hand-coded": "true",
		},
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			// (1) Help-only branch. The catalogue is genuinely useful and
			// costs nothing: no request is made.
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return discoPrintCatalogue(cmd, flags)
			}
			// (2) Dry-run guard, before any IO and after no validation.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "disco")
			}
			// (3) Bounded context.
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			// (4) Validation.
			if len(args) > 0 {
				return usageErr(fmt.Errorf("disco takes no positional arguments; got %q", args[0]))
			}
			// There is no local PER store: openStoreForRead serves the SQLite
			// resources table and no PER resource is ever synced into it, so a
			// caller must not be told a local panel exists.
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(flagMetric) == "" {
				if flagPDF == "" && flagEntity == "" && flagVariant == "" && flagPeriod == "" {
					return discoPrintCatalogue(cmd, flags)
				}
				return discoMissingMetric(cmd, flags)
			}

			opts, err := discoResolveOptions(flagMetric, flagVariant, flagPeriod, flagEntity, flagFY, flagPDF, flagLimit)
			if err != nil {
				return usageErr(err)
			}
			return discoRun(ctx, cmd, flags, opts, flagStrict)
		},
	}

	cmd.Flags().StringVar(&flagMetric, "metric", "", "Reliability parameter to panel: "+strings.Join(discoMetricTokenList(), ", ")+". Case-insensitive. With no --metric the report-year catalogue is printed and no request is made")
	cmd.Flags().StringVar(&flagFY, "fy", "", "Report year to read, as 2024-25 or FY2024-25. Defaults to the newest year this build records as published, and the payload says so")
	cmd.Flags().StringVar(&flagVariant, "variant", "", "Keep only rows from tables of this kind: "+strings.Join(discoVariantTokenList(), ", ")+". Omit for every variant the report published")
	cmd.Flags().StringVar(&flagPeriod, "period", "", "Keep only rows describing this fiscal year. Distinct from --fy: --fy is the document, --period is the year the figure describes")
	cmd.Flags().StringVar(&flagEntity, "entity", "", "One distribution licensee: PESCO, IESCO, GEPCO, FESCO, LESCO, MEPCO, QESCO, SEPCO, HESCO, K-Electric (also TESCO and BTPL, which each appear in one table of one report)")
	cmd.Flags().StringVar(&flagPDF, "pdf", "", "Parse a PER PDF already on disk instead of fetching it. PDFs are never response-cached, so this avoids a 1-3 MB refetch; pair it with 'reliability <path> --deliver file:per.pdf'")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Exit 1 when a completeness assertion fails (byte or page count off the measured record, or a table short of the ten evaluated entities). Without it the shortfall is still printed to stderr and recorded in meta.completeness — never silent")
	cmd.Flags().IntVar(&flagLimit, "limit", 0, "Keep at most this many result rows after filtering. Truncation sets meta.truncated and meta.rows_before_limit")
	return cmd
}

// discoOptions is the validated request.
type discoOptions struct {
	Filters       discoFilters
	MetricToken   string
	ReportFY      string
	ReportDefault bool
	PDFPath       string
	Limit         int
}

var discoFYPattern = regexp.MustCompile(`^FY\d{4}-\d{2}$`)

func discoResolveOptions(metric, variant, period, entity, fy, pdf string, limit int) (discoOptions, error) {
	var o discoOptions
	m, all, err := discoResolveMetric(metric)
	if err != nil {
		return o, err
	}
	o.Filters.Metric, o.Filters.AllMetric = m, all
	o.MetricToken = strings.ToLower(strings.TrimSpace(metric))

	if o.Filters.Variant, err = discoResolveVariant(variant); err != nil {
		return o, err
	}
	if p := strings.TrimSpace(period); p != "" {
		norm := nepraper.NormalizeFY(p)
		if !discoFYPattern.MatchString(norm) {
			return o, fmt.Errorf("--period %q is not a fiscal year; use 2020-21, 2020-2021 or FY2020-21", period)
		}
		o.Filters.PeriodFY = norm
	}
	if e := strings.TrimSpace(entity); e != "" {
		res, ok := nepraper.LookupEntity(e)
		if !ok {
			return o, fmt.Errorf(
				"unknown --entity %q; NEPRA evaluates ten licensees: %s.\n"+
					"TESCO and BTPL are also resolvable: TESCO is excluded from the PER on the record and BTPL appears\n"+
					"in exactly one table of one report (FY2014-15's complaints table).",
				entity, strings.Join(discoRosterStrings(), ", "))
		}
		if res == nepraper.EntityWeightedAverage {
			return o, fmt.Errorf(
				"--entity %q resolves to the \"W. Av:\" summary row, which is not a licensee and is never returned as an "+
					"eleventh DISCO.\nIt is already in every panel under the top-level weighted_average key; read it there.",
				entity)
		}
		o.Filters.Entity = res
	}
	o.PDFPath = strings.TrimSpace(pdf)
	if limit < 0 {
		return o, fmt.Errorf("--limit must be zero or positive; got %d (0 means no cap)", limit)
	}
	o.Limit = limit

	if f := strings.TrimSpace(fy); f != "" {
		norm := nepraper.NormalizeFY(f)
		if !discoFYPattern.MatchString(norm) {
			return o, fmt.Errorf("--fy %q is not a fiscal year; use 2024-25, 2024-2025 or FY2024-25", fy)
		}
		o.ReportFY = norm
	} else if o.PDFPath == "" {
		// A local file NEVER gets the newest-published default: that would
		// stamp FY2024-25 onto an FY2018-19 document. It is detected from the
		// document's own running header instead, or refused.
		o.ReportFY = discoDefaultFY()
		o.ReportDefault = true
	}
	return o, nil
}

// discoRun acquires, parses, asserts and renders. It is the only function here
// that touches the network.
func discoRun(ctx context.Context, cmd *cobra.Command, flags *rootFlags, o discoOptions, strict bool) error {
	source := "live"
	var raw []byte
	var artifactURL, contentType string
	var record nepraper.SourceRecord
	requestMade := false

	if o.PDFPath != "" {
		source = "file"
		b, err := os.ReadFile(o.PDFPath)
		if err != nil {
			return usageErr(fmt.Errorf("--pdf %q could not be read: %w", o.PDFPath, err))
		}
		raw, artifactURL, contentType = b, o.PDFPath, "application/pdf"
	} else {
		rec, ok := nepraper.SourceFor(o.ReportFY)
		if !ok {
			return usageErr(fmt.Errorf(
				"no record of a %s Performance Evaluation Report in this build.\n"+
					"Recorded report years: %s.\n"+
					"These further PERs were reached upstream by the corpus survey but never downloaded, extracted or\n"+
					"parsed, so this build has no measured byte or page count to assert them against: %s.\n"+
					"They exist on NEPRA's site; they are not in this table. Use --pdf <path> to parse one you hold.",
				o.ReportFY, strings.Join(discoRecordedFYs(), ", "), discoUnrecordedYearSummary()))
		}
		record = rec
		if rec.Availability == nepraper.AvailabilityUnavailable {
			return discoRefuseUnavailable(cmd, flags, o, rec)
		}
		c, cerr := flags.newClient()
		if cerr != nil {
			return cerr
		}
		// The path is used VERBATIM from the corpus record: it is already
		// absolute and already percent-encoded, trailing %20 included, and
		// re-escaping it turns FY2020-21's 200 into a 404. The
		// binary-response header is what makes the client hand back the raw
		// document instead of trying to decode it as JSON; it also drops the
		// whole-call timeout (a cold CDN MISS on a 3 MB PER needs it) and
		// deliberately bypasses the response cache, so every live invocation
		// re-downloads 1.0-3.0 MB. --pdf is the way out of that.
		body, gerr := c.GetWithHeaders(ctx, rec.URLPath, map[string]string{},
			map[string]string{client.BinaryResponseHeader: "true"})
		if gerr != nil {
			return classifyAPIError(cmd.OutOrStdout(), gerr, flags)
		}
		requestMade = true
		decoded, ct, ok := client.UnwrapBinaryResponse(body)
		if !ok {
			return notFoundErr(fmt.Errorf(
				"%s did not return a binary document envelope for %s.\n"+
					"That is the HTML-decoy shape: NEPRA's own index links to PERs that answer with a short text/html\n"+
					"body under a non-200, and parsing one as a PDF would yield an empty report that looks like no data.",
				o.ReportFY, rec.URL()))
		}
		raw, artifactURL, contentType = decoded, rec.URL(), ct
		if contentType == "" {
			contentType = c.LastContentType()
		}
	}

	doc, err := nepraper.ExtractText(raw)
	if err != nil {
		if errors.Is(err, nepraper.ErrNotPDF) {
			msg := fmt.Errorf(
				"%s is not a PDF (no %%PDF header) at %d bytes: %w.\n"+
					"This guard exists because NEPRA's index links a PER that answers with a 9-byte 'Not Found' body; "+
					"without it that body parses as an empty report and reads as 'the report says nothing'.",
				artifactURL, len(raw), err)
			if source == "file" {
				return usageErr(msg)
			}
			return notFoundErr(msg)
		}
		return fmt.Errorf("extracting %s: %w", artifactURL, err)
	}

	if o.ReportFY == "" {
		detected, ok := nepraper.DetectFY(doc)
		if !ok {
			return usageErr(fmt.Errorf(
				"--fy is required with --pdf when the document's own running header carries no fiscal year.\n"+
					"%s yielded no detectable year, and the newest-published default is deliberately NOT applied to a\n"+
					"local file: it would stamp %s onto a document from another year.",
				o.PDFPath, discoDefaultFY()))
		}
		o.ReportFY = detected
		if rec, ok := nepraper.SourceFor(o.ReportFY); ok {
			record = rec
		}
	} else if o.PDFPath != "" {
		if rec, ok := nepraper.SourceFor(o.ReportFY); ok {
			record = rec
		}
	}

	rep, err := nepraper.ParseReliability(doc, o.ReportFY)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", o.ReportFY, err)
	}

	payload := discoBuildPayload(rep, doc, len(raw), o, source, requestMade, record)

	// Provenance for the exact bytes the rows came from. sha256_content is over
	// the FULL unfiltered observation set, so it is a stable identity for this
	// document's extraction rather than a hash of one query's answer. For a PDF
	// body sha256_raw IS also a usable document identity: the Cloudflare
	// data-cfemail rotation that makes a raw hash useless on this site's HTML
	// surfaces does not touch a PDF. Do not delete it.
	content, err := json.Marshal(struct {
		Observations     []nepraper.Observation `json:"observations"`
		WeightedAverages []nepraper.Observation `json:"weighted_averages"`
	}{rep.Observations, rep.WeightedAverages})
	if err != nil {
		return err
	}
	art := newNepraArtifact(artifactURL, raw, content, contentType, len(rep.Observations), 0)
	payload.Meta.Artifact = &art

	human := wantsHumanTable(cmd.OutOrStdout(), flags)
	if human {
		discoWriteHuman(cmd, payload)
	}
	// A failed assertion is printed loudly whether or not --strict was passed,
	// and in every output mode. It is never silent.
	discoWriteAssertions(cmd, payload.Meta.Completeness)

	if len(payload.Results) == 0 {
		return discoRefuseEmpty(cmd, flags, rep, o, payload, human)
	}
	if !human {
		if err := discoEmit(cmd, flags, payload); err != nil {
			return err
		}
	}
	if strict && payload.Meta.Completeness.AssertionsFailed > 0 {
		return fmt.Errorf("%d completeness assertion(s) failed for %s: %s",
			payload.Meta.Completeness.AssertionsFailed, o.ReportFY,
			strings.Join(payload.Meta.Completeness.Failures, "; "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// Catalogue: the no-selector branch. Makes NO request.
// ---------------------------------------------------------------------------

// discoPrintCatalogue lists the report years, the roster, the metric enum and
// every refusal this command can make, without touching the network. It is the
// answer to "what can I ask for", and it names the report years NEPRA has that
// this build does not, so a caller is never told a document does not exist.
func discoPrintCatalogue(cmd *cobra.Command, flags *rootFlags) error {
	cat := discoCatalogue{
		AsOfDate:        discoAsOfDate,
		RosterEvaluated: discoRosterStrings(),
		Variants:        discoVariantTokenList(),
		RefusedVariants: []discoRefusedVariant{{Variant: "chart", Reason: discoChartVariantRefusal}},
		UnrecordedYears: discoReachableButUnrecorded,
		RequestMade:     false,
	}
	for _, s := range nepraper.Sources() {
		e := discoSourceEntry{
			ReportFY: s.FY, Availability: s.Availability.String(), URL: s.URL(),
			Bytes: s.Bytes, Pages: s.Pages, Creator: s.Creator,
			HasTextLayer: s.HasTextLayer, LowTextPages: s.LowTextPages, Note: s.Note,
		}
		if e.LowTextPages == nil {
			e.LowTextPages = []int{}
		}
		cat.ReportYears = append(cat.ReportYears, e)
	}
	// The exclusion block is stated for the ONE report year that carries the
	// wording, never for the corpus as a whole: FY2014-15 includes TESCO.
	cat.ExcludedEntities = []discoExclusion{{
		Entity: string(nepraper.EntityTESCO), Kind: "excluded",
		StatedInFY: nepraper.TESCOExclusionStatedFY,
		Source:     nepraper.TESCOExclusionSource,
		Reason:     nepraper.TESCOExclusion,
	}, {
		Entity: string(nepraper.EntityBTPL), Kind: "present_in_report",
		Reason: "BTPL appears in exactly one table of one report (FY2014-15's 12-entity complaints table), " +
			"which is why the roster is per-table and not per-report",
	}}
	tokened := map[nepraper.Metric]string{}
	for _, m := range discoMetricTokens {
		tokened[m.Metric] = m.Token
	}
	for _, m := range nepraper.Metrics() {
		cat.Metrics = append(cat.Metrics, discoMetricEntry{
			Token: tokened[m], Key: string(m), Title: nepraper.MetricTitles[m],
			Selectable: tokened[m] != "",
		})
	}
	for _, a := range nepraper.KnownArtifacts() {
		cat.KnownArtifacts = append(cat.KnownArtifacts, discoKnownArtifact{
			PeriodFY: a.Key.PeriodFY, Entity: string(a.Key.Entity), Metric: string(a.Key.Metric),
			Description: a.Description, Ratio: a.Ratio, Direction: a.Direction,
		})
	}
	for _, u := range nepraper.KnownUnverified() {
		cat.KnownUnverified = append(cat.KnownUnverified, discoUnverified{
			PeriodFY: u.PeriodFY, Metric: string(u.Metric),
			EntityAttributed: u.EntityAttributed(),
			Value:            u.UnverifiedValue(), Source: u.Source,
		})
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.ErrOrStderr()
		fmt.Fprintf(w, "%d recorded report years (%d published, %d unavailable). Byte and page counts measured %s.\n",
			len(cat.ReportYears), discoPublishedCount(), len(cat.ReportYears)-discoPublishedCount(), discoAsOfDate)
		fmt.Fprintf(w, "Pass --metric <%s> to read a panel. Default report year: %s.\n",
			strings.Join(discoMetricTokenList(), "|"), discoDefaultFY())
		fmt.Fprintf(w, "%d further PERs are reachable on NEPRA's site but unmeasured here; see reachable_but_unrecorded_report_years.\n",
			len(cat.UnrecordedYears))
		tw := newTabWriter(cmd.OutOrStdout())
		fmt.Fprintln(tw, "REPORT FY\tAVAILABILITY\tBYTES\tPAGES\tCREATOR/PRODUCER\tNOTE")
		for _, e := range cat.ReportYears {
			bytesCell, pagesCell := fmt.Sprintf("%d", e.Bytes), fmt.Sprintf("%d", e.Pages)
			if e.Availability != nepraper.AvailabilityPublished.String() {
				// Never print 0 where nothing was ever measured.
				bytesCell, pagesCell = "not measured", "not measured"
			}
			creator := e.Creator
			if creator == "" {
				creator = "(no /Creator)"
			}
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", e.ReportFY, e.Availability,
				bytesCell, pagesCell, creator, truncate(e.Note, 60))
		}
		if err := tw.Flush(); err != nil {
			return err
		}
		return nil
	}
	sections, err := json.Marshal(cat)
	if err != nil {
		return err
	}
	// The catalogue is a multi-section REPORT, so it takes the same
	// {meta, results:{sections}} shape as verify, fleet and conflicts rather
	// than being flattened into a row array it is not. It previously emitted
	// a bare top-level object with no envelope at all, so it carried neither
	// .meta nor .results and matched none of this CLI's documented shapes.
	payload, err := json.Marshal(map[string]any{
		"meta": map[string]any{
			"source":       "catalogue",
			"as_of_date":   cat.AsOfDate,
			"requests":     0,
			"request_made": cat.RequestMade,
			"report_years": len(cat.ReportYears),
			"note": "the PER catalogue was measured on " + cat.AsOfDate + ". results is a keyed " +
				"report: iterate a named section such as results.report_years[], not results[].",
		},
		"results": json.RawMessage(sections),
	})
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(payload, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

func discoPublishedCount() int {
	n := 0
	for _, s := range nepraper.Sources() {
		if s.Availability == nepraper.AvailabilityPublished {
			n++
		}
	}
	return n
}

// discoMissingMetric refuses a selector-without-metric invocation. The JSON
// {error, usage} envelope goes to stdout first so a machine caller sees the
// reason and still gets exit 2.
func discoMissingMetric(cmd *cobra.Command, flags *rootFlags) error {
	msg := fmt.Sprintf("--metric is required when any of --pdf, --entity, --variant or --period is set; accepted tokens are %s",
		strings.Join(discoMetricTokenList(), ", "))
	if wantsMachineOutput(flags) {
		if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"error": msg,
			"usage": cmd.CommandPath() + " --metric <" + strings.Join(discoMetricTokenList(), "|") + ">",
		}, flags); err != nil {
			return err
		}
	}
	return usageErr(errors.New(msg))
}

// ---------------------------------------------------------------------------
// Refusals
// ---------------------------------------------------------------------------

// discoRefuseUnavailable answers for a report year NEPRA links but does not
// serve. It makes NO request, prints the recorded reason and URL, names the
// second-hand route through a later report's comparison columns, and exits 3.
// This is emphatically not "no data": the distinction is the whole point when a
// caller reasons about a gap in a time series.
func discoRefuseUnavailable(cmd *cobra.Command, flags *rootFlags, o discoOptions, rec nepraper.SourceRecord) error {
	avail := nepraper.AvailabilityFor(rec.FY)
	route := ""
	if newest := discoDefaultFY(); newest != "" && newest != rec.FY {
		route = fmt.Sprintf("disco --metric %s --fy %s --variant comparison --period %s returns the %s column of the %s report, tagged secondhand",
			o.MetricToken, strings.TrimPrefix(newest, "FY"), strings.TrimPrefix(rec.FY, "FY"), rec.FY, newest)
	}
	payload := discoRefusalPayload{
		Meta: discoMeta{
			Source: "live", RequestMade: false,
			ReportFY: rec.FY, ReportFYDefaulted: o.ReportDefault,
			Metric: o.MetricToken, VariantFilter: o.Filters.Variant,
			PeriodFilter: o.Filters.PeriodFY, EntityFilter: string(o.Filters.Entity),
			TargetNote: discoTargetNote, Availability: avail,
			RosterEvaluated: discoRosterStrings(),
			// Nothing was fetched, so the document block stays NULL and
			// nothing is asserted. Zeroes here would claim a measurement
			// nobody took.
			Document:         nil,
			ExcludedEntities: []discoExclusion{},
			Completeness: discoCompleteness{
				EntitiesExpected: len(nepraper.Roster()),
				EntitiesAbsent:   []string{}, EntitiesBeyondRoster: []string{},
				RosterAssertion: "skipped: the report could not be obtained, so no row set exists to assert against",
				Failures:        []string{},
			},
			Conflicts: []discoConflict{}, TablesRead: []discoTableStat{},
			Notes: []discoNote{},
		},
		Unavailable: discoUnavailableBlock{
			Kind: avail.Kind.String(), Reason: avail.Reason,
			URL: rec.URL(), SecondHandRoute: route,
		},
		Results: []discoRow{},
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		w := cmd.ErrOrStderr()
		fmt.Fprintf(w, "UNAVAILABLE %s: %s\n", rec.FY, avail.Reason)
		if route != "" {
			fmt.Fprintf(w, "second-hand route: %s\n", route)
		}
		fmt.Fprintln(w, "No request was made. This is a hole in the SOURCE, not an empty result.")
		return notFoundErr(fmt.Errorf(
			"%s is UNAVAILABLE: NEPRA links this PER from its own index and serves HTTP 404 for it.", rec.FY))
	}
	if !flags.quiet {
		out, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		wrapped, err := wrapPlatformStructuredOutput(out, flags, "results", true)
		if err != nil {
			return err
		}
		if err := printOutput(cmd.OutOrStdout(), wrapped, true); err != nil {
			return err
		}
	}
	return notFoundErr(fmt.Errorf(
		"%s is UNAVAILABLE: NEPRA links this PER from its own index and serves HTTP 404 for it. "+
			"See the unavailable block for the URL and the second-hand route.", rec.FY))
}

// discoRefuseEmpty is the guarantee that a zero-row panel always says WHY. An
// empty results array printed as success is indistinguishable from a report
// that published nothing, and this corpus has at least four different reasons
// a row set can be empty.
func discoRefuseEmpty(cmd *cobra.Command, flags *rootFlags, rep *nepraper.Report, o discoOptions, payload discoPayload, human bool) error {
	var b strings.Builder
	fmt.Fprintf(&b, "no published row for %s in the %s report", o.MetricToken, o.ReportFY)
	if o.Filters.Variant != "" {
		fmt.Fprintf(&b, " under variant %s", o.Filters.Variant)
	}
	if o.Filters.PeriodFY != "" {
		fmt.Fprintf(&b, " for period %s", o.Filters.PeriodFY)
	}
	if o.Filters.Entity != "" {
		fmt.Fprintf(&b, " for %s", o.Filters.Entity)
	}
	b.WriteString(".\n")

	// (a) Which variants DID carry this metric — the FY2022-23 trap, where both
	// SAIFI tables are captioned with/without LT so nothing is ever "headline".
	variants, metrics, periods, entities := discoWhatTheReportCarried(rep, o)
	if len(variants) > 0 {
		fmt.Fprintf(&b, "  this report DOES publish %s, under variant(s): %s\n", o.MetricToken, strings.Join(variants, ", "))
	} else if !o.Filters.AllMetric {
		fmt.Fprintf(&b, "  this report published no %s table at all; the parameters it did carry are: %s\n",
			o.MetricToken, strings.Join(metrics, ", "))
	}
	if o.Filters.PeriodFY != "" && len(periods) > 0 {
		fmt.Fprintf(&b, "  periods actually present for this selection: %s\n", strings.Join(periods, ", "))
	}
	if o.Filters.Entity != "" {
		if len(entities) > 0 {
			fmt.Fprintf(&b, "  entities actually present for this selection: %s\n", strings.Join(entities, ", "))
		}
		if v := nepraper.ExclusionReasonInFY(o.Filters.Entity, rep.FY); v.Kind != nepraper.KindExcluded {
			fmt.Fprintf(&b, "  this build has no record of how the %s report treated %s. That is not a statement that %s "+
				"was excluded, and not a statement that it reported nothing.\n", rep.FY, o.Filters.Entity, o.Filters.Entity)
		}
	}
	// (b) The corpus note for this report year, verbatim. For FY2014-15 this is
	// the answer: its SAIFI/SAIDI/T&D/recovery figures are chart images.
	if rec, ok := nepraper.SourceFor(o.ReportFY); ok && rec.Note != "" {
		fmt.Fprintf(&b, "  recorded note for %s: %q\n", rec.FY, rec.Note)
	}
	fmt.Fprintf(&b, "  %d table(s) matched the filters and the parser recorded %d note(s) for this report; "+
		"see meta.tables_read and meta.notes.", len(payload.Meta.TablesRead), payload.Meta.ParserNotesCount)

	reason := b.String()
	payload.Meta.Completeness.RosterAssertion = "skipped: no rows matched the filters, so a 10-entity floor asserts nothing"
	if human {
		// The reason is NOT written here: cobra prints the returned error to
		// stderr, and printing it twice reads as two separate findings.
		return notFoundErr(errors.New(reason))
	}
	if !flags.quiet {
		out, err := json.Marshal(struct {
			discoPayload
			// Refusal is the reason a zero-row panel is empty. An empty
			// results array printed as success is indistinguishable from a
			// report that published nothing, so this field is never omitted.
			Refusal string `json:"refusal"`
		}{payload, reason})
		if err != nil {
			return err
		}
		wrapped, err := wrapPlatformStructuredOutput(out, flags, "results", true)
		if err != nil {
			return err
		}
		if err := printOutput(cmd.OutOrStdout(), wrapped, true); err != nil {
			return err
		}
	}
	return notFoundErr(errors.New(reason))
}

// discoEmit writes the machine payload.
//
// --json and --agent get the whole {meta, results, weighted_average} envelope,
// because meta is where the provenance, the completeness assertions and BOTH
// sides of every conflict live. The row-shaped modes (--csv, --plain, --quiet,
// --select) get the rows, since a CSV of a nested object is not a thing — and
// they get a stderr notice naming what the envelope carried, so a contradiction
// is never lost just because the caller asked for a flat table.
func discoEmit(cmd *cobra.Command, flags *rootFlags, payload discoPayload) error {
	rowShaped := flags.csv || flags.plain || flags.quiet || flags.selectFields != ""
	if !rowShaped {
		out, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		wrapped, err := wrapPlatformStructuredOutput(out, flags, "results", true)
		if err != nil {
			return err
		}
		return printOutput(cmd.OutOrStdout(), wrapped, true)
	}
	discoWriteRowModeNotice(cmd, payload)
	if flags.quiet {
		// --quiet governs stdout. The stderr notice above and any
		// COMPLETENESS line still print, so silence never hides a shortfall.
		return nil
	}
	rows, err := json.Marshal(discoFlatten(payload.Results))
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), rows, flags,
		map[string]any{"source": payload.Meta.Source})
}

// discoFlatRow is the row-shaped projection for --csv, --plain and --select.
//
// Every typed Value becomes a PAIR of columns: the text exactly as NEPRA
// printed it, and its kind. Flattening to the text alone would make "not
// reported", "excluded on the record" and a real published 0.00 all look like
// an empty cell, which is the one conflation this whole command exists to
// prevent. The kind column is what keeps them apart in a spreadsheet.
type discoFlatRow struct {
	ReportFY                 string  `json:"report_fy"`
	PeriodFY                 string  `json:"period_fy"`
	Entity                   string  `json:"entity"`
	Metric                   string  `json:"metric"`
	Variant                  string  `json:"variant"`
	Actual                   string  `json:"actual"`
	ActualKind               string  `json:"actual_kind"`
	Target                   string  `json:"target"`
	TargetKind               string  `json:"target_kind"`
	Breach                   string  `json:"breach"`
	BreachKind               string  `json:"breach_kind"`
	Table                    string  `json:"table"`
	Page                     int     `json:"page"`
	ColumnHeader             string  `json:"column_header"`
	Caption                  string  `json:"caption"`
	Restated                 bool    `json:"restated"`
	Secondhand               bool    `json:"secondhand"`
	PeriodReportAvailability string  `json:"period_report_availability"`
	Conflicted               bool    `json:"conflicted"`
	ConflictedWith           int     `json:"conflicted_with"`
	KnownArtifactRatio       float64 `json:"known_artifact_ratio,omitempty"`
	Extras                   int     `json:"extras"`
}

func discoFlatten(rows []discoRow) []discoFlatRow {
	out := make([]discoFlatRow, 0, len(rows))
	for _, r := range rows {
		f := discoFlatRow{
			ReportFY: r.ReportFY, PeriodFY: r.PeriodFY, Entity: r.Entity,
			Metric: r.Metric, Variant: r.Variant,
			Actual: discoFlatText(r.Actual), ActualKind: r.Actual.Kind.String(),
			Target: discoFlatText(r.Target), TargetKind: r.Target.Kind.String(),
			Breach: discoFlatText(r.Breach), BreachKind: r.Breach.Kind.String(),
			Table: r.Citation.Table, Page: r.Citation.Page,
			ColumnHeader: r.Citation.ColumnHeader, Caption: r.Citation.Caption,
			Restated: r.Restated, Secondhand: r.Secondhand,
			PeriodReportAvailability: r.PeriodReportAvailability,
			Conflicted:               r.Conflicted, ConflictedWith: len(r.ConflictedWith),
			Extras: len(r.Extras),
		}
		if r.KnownArtifact != nil {
			f.KnownArtifactRatio = r.KnownArtifact.Ratio
		}
		out = append(out, f)
	}
	return out
}

// discoFlatText is the source text as printed, with the canonical label
// preferred for a recognised verdict. It is EMPTY only when the source really
// printed nothing, and the paired *_kind column says which case that is.
func discoFlatText(v nepraper.Value) string {
	if v.Kind == nepraper.KindQualitative && v.Label != "" {
		return v.Label
	}
	return v.Raw
}

// discoWriteRowModeNotice tells a flat-output caller what the envelope held.
// Silence here would let a --csv reader believe a single figure was the
// report's answer when the same report publishes another.
func discoWriteRowModeNotice(cmd *cobra.Command, payload discoPayload) {
	w := cmd.ErrOrStderr()
	url := ""
	if payload.Meta.Artifact != nil {
		url = "  " + payload.Meta.Artifact.URL
	}
	fmt.Fprintf(w, "%s %s: %d row(s) from %d table(s)%s\n",
		payload.Meta.ReportFY, payload.Meta.Metric, len(payload.Results), len(payload.Meta.TablesRead), url)
	if n := len(payload.Meta.Conflicts); n > 0 {
		fmt.Fprintf(w, "%d same-key conflict(s) apply to this selection and are NOT in a flat row set; "+
			"re-run with --json to read meta.conflicts, which carries both sides with their pages.\n", n)
	}
	if n := len(payload.WeightedAverage); n > 0 {
		fmt.Fprintf(w, "%d weighted-average row(s) omitted from the flat output: they are NOT a licensee. "+
			"Re-run with --json to read them under weighted_average.\n", n)
	}
}

// discoWhatTheReportCarried reports what the report DID publish near the
// caller's selection, so a refusal is diagnostic rather than a dead end.
func discoWhatTheReportCarried(rep *nepraper.Report, o discoOptions) (variants, metrics, periods, entities []string) {
	seenVariant, seenMetric, seenPeriod, seenEntity := map[string]bool{}, map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, t := range rep.Tables {
		if !seenMetric[string(t.Metric)] {
			seenMetric[string(t.Metric)] = true
			metrics = append(metrics, string(t.Metric))
		}
		if (o.Filters.AllMetric || t.Metric == o.Filters.Metric) && !seenVariant[t.Variant] {
			seenVariant[t.Variant] = true
			variants = append(variants, t.Variant)
		}
	}
	for _, ob := range rep.Observations {
		if !o.Filters.AllMetric && ob.Metric != o.Filters.Metric {
			continue
		}
		if o.Filters.Variant != "" && ob.Prov.Variant != o.Filters.Variant {
			continue
		}
		if !seenPeriod[ob.PeriodFY] {
			seenPeriod[ob.PeriodFY] = true
			periods = append(periods, ob.PeriodFY)
		}
		if !seenEntity[string(ob.Entity)] {
			seenEntity[string(ob.Entity)] = true
			entities = append(entities, string(ob.Entity))
		}
	}
	return variants, metrics, periods, entities
}

// ---------------------------------------------------------------------------
// Human rendering
// ---------------------------------------------------------------------------

// discoWriteAssertions prints every failed completeness assertion loudly to
// stderr whether or not --strict was passed. A shortfall is never silent.
func discoWriteAssertions(cmd *cobra.Command, c discoCompleteness) {
	for _, f := range c.Failures {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"COMPLETENESS: %s. Measured on %s; a mismatch is how the documented append-on-retry "+
				"corruption presents (a 3,952,485-byte body from a 3,024,893-byte source that file(1) still "+
				"validated as a PDF). Do not treat the result as complete.\n", f, discoAsOfDate)
	}
}

func discoWriteHuman(cmd *cobra.Command, p discoPayload) {
	w := cmd.ErrOrStderr()
	fy := p.Meta.ReportFY
	if p.Meta.ReportFYDefaulted {
		fy += " (defaulted to the newest year recorded as published)"
	}
	fmt.Fprintf(w, "report %s  metric %s", fy, p.Meta.Metric)
	if p.Meta.MetricTitle != "" {
		fmt.Fprintf(w, "  %q", p.Meta.MetricTitle)
	}
	fmt.Fprintln(w)
	if p.Meta.Artifact != nil {
		fmt.Fprintf(w, "source %s  %s\n", p.Meta.Source, p.Meta.Artifact.URL)
	}
	fmt.Fprintf(w, "document %d B / %d pages", p.Meta.Document.Bytes, p.Meta.Document.Pages)
	if p.Meta.Document.BytesMeasured > 0 {
		fmt.Fprintf(w, "  (corpus record: %d B / %d pages)", p.Meta.Document.BytesMeasured, p.Meta.Document.PagesMeasured)
	}
	fmt.Fprintf(w, "  %d chars\n", p.Meta.Document.CharCount)
	if p.Meta.DetectedFY != "" && p.Meta.DetectedFYMatchesRequested != nil && !*p.Meta.DetectedFYMatchesRequested {
		fmt.Fprintf(w, "NOTE the document's own running header says %s, not %s. Recorded, not overridden.\n",
			p.Meta.DetectedFY, p.Meta.ReportFY)
	}
	for _, t := range p.Meta.TablesRead {
		vc := "value column not identified"
		if t.ValueColumnIdentified {
			vc = fmt.Sprintf("value column %d", t.ValueColumn)
		}
		fmt.Fprintf(w, "  %-9s p%-3d %-24s %2d rows  %s  %q\n", t.Table, t.Page, t.Variant, t.Rows, vc, truncate(t.Caption, 52))
		if t.RosterAnomaly != "" {
			fmt.Fprintf(w, "      roster: %s\n", t.RosterAnomaly)
		}
	}
	if p.Meta.EntityFilter != "" {
		fmt.Fprintf(w, "entity filter %s: %d row(s)", p.Meta.EntityFilter, len(p.Results))
	} else {
		fmt.Fprintf(w, "roster %d/%d evaluated entities present", p.Meta.Completeness.EntitiesSeen, p.Meta.Completeness.EntitiesExpected)
	}
	if len(p.Meta.Completeness.EntitiesBeyondRoster) > 0 {
		fmt.Fprintf(w, "  plus %s (not among the ten NEPRA evaluates)", strings.Join(p.Meta.Completeness.EntitiesBeyondRoster, ", "))
	}
	fmt.Fprintf(w, "  |  %d conflict(s) in this report, %d undocumented\n",
		p.Meta.ConflictsInReport, p.Meta.ConflictsUndocumentedInReport)
	if p.Meta.Truncated {
		fmt.Fprintf(w, "TRUNCATED by --limit: %d of %d rows shown\n", len(p.Results), p.Meta.RowsBeforeLimit)
	}

	out := cmd.OutOrStdout()
	if len(p.Results) > 0 {
		tw := newTabWriter(out)
		fmt.Fprintln(tw, "ENTITY\tPERIOD\tACTUAL\tTARGET\tBREACH\tVARIANT\tTABLE\tPAGE\tFLAGS")
		for _, r := range p.Results {
			fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Entity, r.PeriodFY, discoCell(r.Actual), discoCell(r.Target), discoCell(r.Breach),
				discoDash(r.Variant), discoDash(r.Citation.Table), discoPageCell(r.Citation.Page), discoFlags(r))
		}
		_ = tw.Flush()
	}
	if len(p.WeightedAverage) > 0 {
		fmt.Fprintln(out, strings.Repeat("-", 78))
		fmt.Fprintln(out, "weighted average (NEPRA's \"W. Av:\" summary row - NOT an eleventh DISCO)")
		tw2 := newTabWriter(out)
		fmt.Fprintln(tw2, "ENTITY\tPERIOD\tACTUAL\tTARGET\tBREACH\tVARIANT\tTABLE\tPAGE")
		for _, r := range p.WeightedAverage {
			fmt.Fprintf(tw2, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				r.Entity, r.PeriodFY, discoCell(r.Actual), discoCell(r.Target), discoCell(r.Breach),
				discoDash(r.Variant), discoDash(r.Citation.Table), discoPageCell(r.Citation.Page))
		}
		_ = tw2.Flush()
	}
	// Extras are the columns the parser could NOT resolve to a single value.
	// They are printed rather than hidden behind an "N extra col(s)" flag: for
	// FY2014-15's complaints table three columns match the caption equally
	// well, so every published figure lands here and the actual reads absent.
	// Dropping them from the terminal would look like a table with no numbers.
	var extraLines int
	for _, r := range p.Results {
		if len(r.Extras) == 0 {
			continue
		}
		if extraLines == 0 {
			fmt.Fprintln(out, strings.Repeat("-", 78))
			fmt.Fprintln(out, "extra columns (published figures the parser would not name as THE value)")
		}
		for _, x := range r.Extras {
			if extraLines >= discoHumanExtraCap {
				break
			}
			fmt.Fprintf(out, "  %-11s %-52s %s\n", r.Entity, truncate(x.Header, 52), discoCell(x.Value))
			extraLines++
		}
		if extraLines >= discoHumanExtraCap {
			fmt.Fprintf(out, "  ... more extra columns not shown; all of them are in results[].extras under --json\n")
			break
		}
	}
	// Conflicts are capped in the terminal only. The machine envelope always
	// carries every one of them: a suppressed conflict is a hidden
	// contradiction, and the count below says exactly how many were not shown.
	shown := p.Meta.Conflicts
	if len(shown) > discoHumanConflictCap {
		shown = shown[:discoHumanConflictCap]
	}
	for _, c := range shown {
		ratio := "undefined (zero denominator or a non-numeric side)"
		if c.Ratio != nil {
			ratio = fmt.Sprintf("%.7f", *c.Ratio)
		}
		fmt.Fprintf(w, "CONFLICT %s/%s/%s on %s [%s]: %s (%s p%d, %s) vs %s (%s p%d, %s)  ratio %s  |A-B| %s  resolution: %s\n",
			c.PeriodFY, c.Entity, c.Metric, c.Field, c.Class,
			discoCell(c.A.Value), c.A.Table, c.A.Page, c.A.Variant,
			discoCell(c.B.Value), c.B.Table, c.B.Page, c.B.Variant,
			ratio, discoNum(c.AbsDiff), c.Resolution)
	}
	if len(p.Meta.Conflicts) > len(shown) {
		fmt.Fprintf(w, "... %d more conflict(s) for this selection; all %d are in meta.conflicts under --json\n",
			len(p.Meta.Conflicts)-len(shown), len(p.Meta.Conflicts))
	}
}

// discoHumanConflictCap bounds the terminal conflict list. FY2022-23 SAIFI
// alone produces 30 for one metric.
const discoHumanConflictCap = 10

// discoHumanExtraCap bounds the terminal extras list. FY2014-15's complaints
// table alone publishes 48 of them.
const discoHumanExtraCap = 48

func discoDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// discoPageCell never prints page 0: the only rows with no page are the
// synthesized exclusion rows, which are stated in prose rather than a table.
func discoPageCell(page int) string {
	if page <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d", page)
}

// discoNum trims binary float noise (18.950000000000003) without rounding a
// real figure away.
func discoNum(v float64) string { return fmt.Sprintf("%.10g", v) }

// discoCell renders a Value for a terminal WITHOUT ever printing a bare 0 for
// a non-numeric kind. An absent cell reads "not reported", an excluded entity
// reads "excluded", a truncated chart label keeps its raw text.
func discoCell(v nepraper.Value) string {
	switch v.Kind {
	case nepraper.KindNumeric:
		if v.Raw != "" {
			return v.Raw
		}
		return fmt.Sprintf("%g", v.Num)
	case nepraper.KindQualitative:
		if v.Label != "" {
			return v.Label
		}
		return v.Raw
	case nepraper.KindUnverified:
		return v.Raw + " (unverified)"
	case nepraper.KindExcluded:
		return "excluded on the record"
	case nepraper.KindUnavailable:
		return "source unavailable"
	default:
		return "not reported"
	}
}

func discoFlags(r discoRow) string {
	var f []string
	if r.Conflicted {
		f = append(f, "CONFLICTED")
	}
	if r.Restated {
		f = append(f, "restated")
	}
	if r.Secondhand {
		f = append(f, "secondhand")
	}
	if r.KnownArtifact != nil {
		f = append(f, "known-artifact")
	}
	if len(r.Extras) > 0 {
		f = append(f, fmt.Sprintf("%d extra col(s)", len(r.Extras)))
	}
	return strings.Join(f, ",")
}

// ---------------------------------------------------------------------------
// Payload assembly (pure: no network, no filesystem)
// ---------------------------------------------------------------------------

// discoBuildPayload assembles the envelope from a parsed report.
func discoBuildPayload(rep *nepraper.Report, doc *nepraper.Doc, rawBytes int, o discoOptions,
	source string, requestMade bool, record nepraper.SourceRecord) discoPayload {

	tables := discoTableStats(rep, o.Filters)

	results := make([]discoRow, 0, len(rep.Observations))
	for _, ob := range rep.Observations {
		if o.Filters.matches(ob) {
			results = append(results, discoObservationRow(ob))
		}
	}
	// An entity NEPRA excludes on the record gets its stated exclusion rather
	// than an empty row or a zero. This is the ONLY synthesized row this
	// command emits, and it comes from nepraper's own year-scoped lookup: the
	// FY2014-15 report INCLUDES TESCO with real published figures, so that
	// year returns those instead and never the FY2024-25 quote.
	if len(results) == 0 && o.Filters.Entity != "" {
		if v := nepraper.ExclusionReasonInFY(o.Filters.Entity, rep.FY); v.Kind == nepraper.KindExcluded {
			metric := o.Filters.Metric
			if o.Filters.AllMetric {
				// The exclusion is whole-report, not per-metric, so no metric
				// is asserted here rather than one being invented.
				metric = ""
			}
			for _, ob := range nepraper.Lookup(rep, nepraper.Key{
				PeriodFY: rep.FY, Entity: o.Filters.Entity, Metric: metric,
			}) {
				results = append(results, discoObservationRow(ob))
			}
		}
	}

	weighted := make([]discoRow, 0, len(rep.WeightedAverages))
	for _, ob := range rep.WeightedAverages {
		wf := o.Filters
		wf.Entity = "" // the summary row is not an entity; --entity cannot select it
		if wf.matches(ob) {
			weighted = append(weighted, discoObservationRow(ob))
		}
	}

	// The conflict pass runs over the WHOLE report before the variant filter,
	// so a panel narrowed to one variant still reports that the other table
	// disagrees, and never presents the surviving figure as the answer.
	allConflicts := nepraper.Conflicts(rep)
	undocumented := nepraper.UndocumentedConflicts(rep)
	conflicts := discoBuildConflicts(allConflicts, o.Filters)
	discoMarkConflicted(results, conflicts)
	discoMarkConflicted(weighted, conflicts)
	undocumentedShown := 0
	for _, c := range conflicts {
		if c.Class == nepraper.ConflictUndocumented.String() || c.Class == nepraper.ConflictTransposition.String() {
			undocumentedShown++
		}
	}

	rowsBefore := len(results)
	truncated := false
	if o.Limit > 0 && len(results) > o.Limit {
		results = results[:o.Limit]
		truncated = true
	}

	matched := discoJoinKnownArtifacts(results) + discoJoinKnownArtifacts(weighted)

	completeness := discoCompletenessOf(results, o.Filters, truncated)
	discoAssertDocument(&completeness, rawBytes, record.Bytes, doc.NumPages, record.Pages)

	var detectedMatches *bool
	if rep.DetectedFY != "" {
		m := rep.DetectedFY == o.ReportFY
		detectedMatches = &m
	}

	meta := discoMeta{
		Source:                     source,
		RequestMade:                requestMade,
		ReportFY:                   o.ReportFY,
		ReportFYDefaulted:          o.ReportDefault,
		DetectedFY:                 rep.DetectedFY,
		DetectedFYMatchesRequested: detectedMatches,
		Metric:                     o.MetricToken,
		VariantFilter:              o.Filters.Variant,
		PeriodFilter:               o.Filters.PeriodFY,
		EntityFilter:               string(o.Filters.Entity),
		TargetNote:                 discoTargetNote,
		Availability:               nepraper.AvailabilityFor(o.ReportFY),
		RosterEvaluated:            discoRosterStrings(),
		ExcludedEntities:           discoExclusions(rep),
		Document: &discoDocumentStat{
			Pages: doc.NumPages, PagesMeasured: record.Pages,
			Bytes: rawBytes, BytesMeasured: record.Bytes,
			Creator: doc.Creator, Producer: doc.Producer,
			CharCount: doc.CharCount(), LowTextPages: doc.LowTextPages(20),
			Note: record.Note,
		},
		TablesRead:                    tables,
		Completeness:                  completeness,
		Conflicts:                     conflicts,
		ConflictsInReport:             len(allConflicts),
		ConflictsUndocumented:         undocumentedShown,
		ConflictsUndocumentedInReport: len(undocumented),
		RowsBeforeLimit:               rowsBefore,
		Truncated:                     truncated,
		WeightedAverageScope: "the \"W. Av:\" summary rows for the selected metric, variant and period. " +
			"--entity and --limit do not apply to them, and they are never in results",
		Notes:                 discoNotesFor(rep, tables),
		ParserNotesCount:      len(rep.Notes),
		KnownArtifactsMatched: matched,
	}
	if !o.Filters.AllMetric {
		meta.MetricTitle = nepraper.MetricTitles[o.Filters.Metric]
	}
	if meta.Document.LowTextPages == nil {
		meta.Document.LowTextPages = []int{}
	}
	return discoPayload{Meta: meta, Results: results, WeightedAverage: weighted}
}
