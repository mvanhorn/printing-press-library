// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// The 33-line TODO scaffold that used to live here is gone; the function name
// newNovelSourcesCmd is kept so root.go's existing registration keeps working.
// See .printing-press-patches/nepra-sources-catalogue.json.
//
// pp:data-source local

package cli

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// newNovelSourcesCmd builds the document catalogue.
//
// NEPRA cannot be enumerated. robots.txt advertises two sitemaps and both are
// HTTP 404 with a nine-byte body, /sitemap.xml is 404 too, the directory
// holding the workbooks answers 403 with no autoindex, and the three index
// pages that do exist are incomplete in BOTH directions. Every other command
// in this CLI therefore takes an argument you can only supply if you already
// know the answer: `generation year <fy>`, `reliability <path>`, `sir <year>`,
// `tariff <disco>`. This command is where those arguments come from.
//
// It makes NO request unless you ask it to. With no flags it prints the
// catalogue; --diff re-measures every CHEAP row and diffs the three index
// pages' href sets against it; --fetch takes one document and measures it.
func newNovelSourcesCmd(flags *rootFlags) *cobra.Command {
	var (
		kind    string
		diff    bool
		fetchID string
		strict  bool
	)

	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Catalogue every reachable published document with size, coverage window, text-layer flag and reachability state",
		Long: "Every published NEPRA document this build has measured, with the byte-exact path in the form\n" +
			"the command that fetches it actually accepts.\n\n" +
			"THE SITE CANNOT BE ENUMERATED. Both sitemaps in robots.txt are 404 with a nine-byte body,\n" +
			"/sitemap.xml is 404, the workbook directory answers 403 with no autoindex, and Main.htm names\n" +
			"five of the seven reachable generation years while two reachable surfaces are unlinked. So this\n" +
			"is a measured table, not a crawl, and every number in it cites the measurement.\n\n" +
			"AN HTTP 200 IS NOT EVIDENCE OF DATA here. The per-year workbook shell is 200 and byte-identical\n" +
			"at 9,838 B for three different fiscal years, and SIR Data 2025.htm is 200, 374 B, titled\n" +
			"'SIR Data 2024' and frames FY2023-24. Those rows are catalogued as decoys, and a generation row\n" +
			"must clear a 400,000-byte floor before --diff will call it reachable.\n\n" +
			"With no flags it prints the catalogue and makes no request.",
		Example: "  nepra-pp-cli sources\n" +
			"  nepra-pp-cli sources --kind gen\n" +
			"  nepra-pp-cli sources --kind per --agent\n" +
			"  nepra-pp-cli sources --diff --agent\n" +
			"  nepra-pp-cli sources --fetch per-fy2020-21",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "--kind=per",
			"pp:typed-exit-codes": "true",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Help-only branch. The catalogue is the useful answer and it
			//    costs nothing: no client is constructed and no request is
			//    made. This is what makes `sources` safe to call first, which
			//    is the whole point of the command.
			// len(args) == 0 is part of the condition on purpose: without it a
			// stray positional would be silently swallowed by the catalogue
			// print instead of reaching the usage error below.
			if len(args) == 0 && kind == "" && !diff && fetchID == "" {
				return sourcesRunCatalogue(cmd, flags, "", strict)
			}

			// 2. Dry run, before any validation that could fail.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "sources")
			}

			// 3. Usage errors.
			if len(args) > 0 {
				return usageErr(fmt.Errorf("sources takes no positional arguments; got %q. "+
					"Use --kind, --diff or --fetch", args[0]))
			}
			if err := sourcesValidateKind(kind); err != nil {
				return usageErr(err)
			}
			if diff && fetchID != "" {
				return usageErr(errors.New("--diff and --fetch are mutually exclusive: --diff re-measures " +
					"every cheap row and never downloads a document, while --fetch downloads exactly one"))
			}

			switch {
			case fetchID != "":
				return sourcesRunFetch(cmd, flags, kind, fetchID, strict)
			case diff:
				return sourcesRunDiff(cmd, flags, kind, strict)
			default:
				return sourcesRunCatalogue(cmd, flags, kind, strict)
			}
		},
	}

	// No backticks in these usage strings: cobra reads the first backquoted
	// word as the flag's value placeholder, which turned --kind into
	// "--kind events" in the help output.
	cmd.Flags().StringVar(&kind, "kind", "", "Restrict to one document family: gen, per, sir, fca or tariff. "+
		"Not 'events' -- the determination surfaces are catalogued by the events command, which owns their row floors")
	cmd.Flags().BoolVar(&diff, "diff", false, "Re-measure every CHEAP catalogued URL live and diff status and "+
		"byte count against the shipped baseline, then diff the index pages' href sets against the catalogue. Never fetches a PDF")
	cmd.Flags().StringVar(&fetchID, "fetch", "", "Fetch exactly one catalogued document by id and measure it "+
		"against the baseline. Honours --deliver file:<path>")
	cmd.Flags().BoolVar(&strict, "strict", false, "Exit non-zero when --diff finds drift or --fetch disagrees "+
		"with a baseline. Off by default so the catalogue stays readable on a site that legitimately changes")
	return cmd
}

// sourcesGap is something this command deliberately does NOT answer, with the
// measured reason. Declaring the holes in the machine-readable surface is the
// point: an agent that can read why a thing is impossible stops asking for it.
type sourcesGap struct {
	Subject string `json:"subject"`
	// Verdict is not_computable | unmeasured | owned_elsewhere | upstream_bug.
	Verdict string `json:"verdict"`
	Reason  string `json:"reason"`
	Instead string `json:"instead,omitempty"`
}

// sourcesDeclaredGaps is what `sources` cannot answer today and why. Every
// entry is a property of what ships, not a preference.
var sourcesDeclaredGaps = []sourcesGap{
	{
		Subject: "PDF reachability in --diff",
		Verdict: "not_computable",
		Reason: "internal/client has no Head() and no Range support, and ProbeGet issues a full GET and " +
			"discards the body, so a PDF's existence cannot be checked without downloading it. The SIR " +
			"corpus alone is 640,312,915 B at a measured 68,768-163,650 B/s, which is 65-155 minutes. " +
			"So every content_kind=pdf row comes back verdict not_probed. NEPRA serves accept-ranges: " +
			"bytes on every PDF, so a client Head() or a one-byte Range accessor would make this leg " +
			"cheap and honest.",
		Instead: "sources --fetch <id> for one document under the 64 MiB ceiling",
	},
	{
		Subject: "the text layer of any State of Industry Report",
		Verdict: "unmeasured",
		Reason: "all 22 sir rows ship text_layer: unmeasured. The claim that 'sir2023.pdf extracts 0 bytes' " +
			"names a file that appears nowhere in the probe evidence; what was measured is 'State of " +
			"Industry Report 2023.pdf' at 3,425,763 B, and nobody read its text layer. Reporting absent " +
			"would be asserting someone else's unsourced number.",
		Instead: "sources --fetch sir-2023, which extracts it and reports what it finds",
	},
	{
		Subject: "size or status of 12 of the 22 State of Industry Reports",
		Verdict: "unmeasured",
		Reason: "2004..2015 are on the index's 22-year run and nothing more. state: indexed_unmeasured is " +
			"the honest value; --diff does not probe them into existence because each probe is a " +
			"multi-megabyte download, and --fetch refuses them because the cost cannot be bounded first.",
	},
	{
		Subject: "a per-page byte or row floor for the eleven DISCO tariff pages",
		Verdict: "owned_elsewhere",
		Reason: "none was ever measured per page: the survey recorded 3,682 rows for the GROUP and the live " +
			"rebuild measured 3,661 determination rows + 23 furniture skipped across all eleven. The " +
			"group floor lives in the events catalogue where the row-floor invariant already lives, so " +
			"--kind tariff ships as 11 reference rows with bytes_as_of: null.",
		Instead: "events --disco <name>",
	},
	{
		Subject: "the shipped `sir <year>` fetch route",
		Verdict: "fixed",
		Reason: "WAS an upstream bug and is now corrected. resource_paths.go built " +
			"/publications/State%20of%20Industry%20Reports/sir{year}.pdf, which matched NO path this " +
			"catalogue measured and returns HTTP 404 with a 9-byte body for EVERY year; the published form " +
			"is .../State%20of%20Industry%20Report%20<year>.pdf, which this catalogue had measured all " +
			"along. Phase 5's live matrix caught it as the only failing command. spec.yaml, " +
			"resource_paths.go and the promoted command are all corrected (patch " +
			"nepra-sir-report-path), and `sir <year>` now returns the real document. NOTE the collateral " +
			"correction: the absorb manifest killed `sir --table` citing \"sir2023.pdf extracts 0 bytes; " +
			"no usable text layer\" — that was measured against the 9-byte 404, not against a PDF, so " +
			"that particular leg of the kill reason is unsupported. The kill itself still stands on its " +
			"other grounds (610 MB across ten reports, 331 MB for SIR 2025, and per-character kerning " +
			"that makes exact matching return false absences).",
		Instead: "sir <year> now works; sources --fetch sir-<year> still measures a report without downloading it twice",
	},
}

// sourcesMeta builds the envelope's meta block. see_also is computed from the
// OTHER catalogues at runtime so the numbers here can never drift from the
// catalogues they describe.
func sourcesMeta(source, kind string, rows []sourceDoc) map[string]any {
	measured := 0
	byKind := map[string]int{}
	for _, r := range rows {
		byKind[r.Kind]++
		if r.BytesAsOf != nil {
			measured++
		}
	}
	meta := map[string]any{
		"source":             source,
		"as_of_date":         sourcesAsOfDate,
		"reverified_date":    sourcesReverifiedDate,
		"kinds":              sourcesKinds,
		"documents":          len(rows),
		"measured_documents": measured,
		"counts_by_kind":     byKind,
		"enumerators":        sourcesEnumerators,
		"declared_gaps":      sourcesDeclaredGaps,
		"see_also": map[string]any{
			"events": map[string]any{
				"surfaces":   len(eventSurfaces),
				"as_of_date": eventsAsOfDate,
				"note": "the tariff-determination pages are catalogued by `events`, which asserts a ROW " +
					"floor per surface; `sources` asserts a BYTE floor per document and never re-lists them. " +
					"The eleven --kind tariff rows are cross-references that assert nothing.",
			},
			"scope": map[string]any{
				"command": "doctor --scope",
				"limits":  len(nepraScopeLimits),
				"note": "documents whose URL was never recorded — the 13 near-identical S.R.O.s of " +
					"13.01.2026, the frozen 'Notified Tariff 01-01-2019' schedule — are declared there, " +
					"not invented here.",
			},
		},
		"measured_documents_note": "a document with bytes_as_of: null was never sized. That is the absence " +
			"of a MEASUREMENT, not a zero and not a claim that NEPRA published nothing.",
	}
	if kind != "" {
		meta["kind"] = kind
	}
	return meta
}

// sourcesCompleteness is the assertion that keeps this catalogue and
// internal/nepraper from silently diverging, and it runs in CODE rather than
// only in tests because a regen can reintroduce the divergence at any time.
//
// It checks two things. First, that the per kind still tracks nepraper record
// for record. Second, and this is the load-bearing one, that every emitted
// fetch_arg is a value `reliability` will actually accept AND that
// "/Standards/" + arg reproduces the row's own path. The second half is what
// catches the real bug: validateEncodedSubPath would happily accept
// "M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf" — printable ASCII with
// no ".." — and `reliability` would then build /Standards/M&E/PER/... and
// return a 404 indistinguishable from a document NEPRA never published.
func sourcesCompleteness() []string {
	var problems []string
	per := sourcesPerRows()
	if want := len(nepraper.Sources()); len(per) != want {
		problems = append(problems, fmt.Sprintf(
			"the per kind emits %d rows against %d records in internal/nepraper. The two tables have "+
				"diverged; sources must JOIN nepraper, never copy it", len(per), want))
	}
	for _, row := range per {
		if row.FetchArg == "" {
			if row.FetchBlockedReason == "" {
				problems = append(problems, row.ID+
					" has no fetch route and no reason, which is indistinguishable from an oversight")
			}
			continue
		}
		if err := validateEncodedSubPath("path", row.FetchArg); err != nil {
			problems = append(problems, row.ID+" emits a fetch_arg `reliability` would refuse: "+err.Error())
		}
		if got := "/Standards/" + row.FetchArg; got != row.Path {
			problems = append(problems, row.ID+" fetch_arg round-trips to "+got+
				" but the measured path is "+row.Path+". `reliability` would fetch the wrong URL")
		}
	}
	if shadowed := sourcesShadowedEventPaths(); len(shadowed) > 0 {
		problems = append(problems, fmt.Sprintf(
			"%d path(s) are claimed by BOTH this catalogue and the events catalogue: %v. "+
				"Two catalogues claiming one URL under two fetch routes is the drift this command exists "+
				"to prevent", len(shadowed), shadowed))
	}
	return problems
}

// sourcesReportCompleteness prints any divergence loudly and, under --strict,
// makes it a non-zero exit.
func sourcesReportCompleteness(cmd *cobra.Command, strict bool) error {
	problems := sourcesCompleteness()
	if len(problems) == 0 {
		return nil
	}
	for _, p := range problems {
		fmt.Fprintf(cmd.ErrOrStderr(), "COMPLETENESS: %s\n", p)
	}
	if strict {
		return fmt.Errorf("%d catalogue completeness assertion(s) failed", len(problems))
	}
	return nil
}

// sourcesRunCatalogue prints the catalogue. It makes no request.
func sourcesRunCatalogue(cmd *cobra.Command, flags *rootFlags, kind string, strict bool) error {
	if err := sourcesReportCompleteness(cmd, strict); err != nil {
		return err
	}
	rows := sourcesDocsForKind(kind)
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		sourcesWriteHumanTable(cmd, rows, kind)
		return nil
	}
	return sourcesEmit(cmd, flags, sourcesMeta("catalogue", kind, rows), rows)
}

// sourcesRunDiff re-measures the catalogue against the live site.
func sourcesRunDiff(cmd *cobra.Command, flags *rootFlags, kind string, strict bool) error {
	if err := sourcesReportCompleteness(cmd, strict); err != nil {
		return err
	}
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	f := sourcesClientFetcher{c: c}

	rows := sourcesDocsForKind(kind)
	enums := sourcesEnumerators
	if kind != "" {
		// Under a --kind filter the sitemaps and robots.txt are not part of the
		// question; only the indexes that cover that kind are probed.
		enums = nil
	}
	probedRows, probedEnums, probed, drifted := sourcesProbeAll(cmd.Context(), f, rows, enums)
	// The href diff needs the WHOLE catalogue as its "already known" universe,
	// even under --kind: a document is new upstream or it is not, regardless of
	// which kind the caller asked to see.
	diffs := sourcesDiffIndexes(cmd.Context(), f, rows, sourcesDocs(), kind)

	meta := sourcesMeta("catalogue+live", kind, probedRows)
	meta["probed"] = probed
	meta["drifted"] = drifted
	meta["enumerator_diff"] = diffs
	if len(probedEnums) > 0 {
		meta["enumerators"] = probedEnums
	}
	meta["diff_note"] = "a probed 404 or 403 is DATA, not a failure: four gen rows and one directory are " +
		"catalogued precisely because they do not serve. index_only is a document NEPRA has published " +
		"that this build does not know about, and it is the most valuable line in this output — it is " +
		"never an error."

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		sourcesWriteDiffTable(cmd, probedRows, diffs, probed, drifted)
	} else if err := sourcesEmit(cmd, flags, meta, probedRows); err != nil {
		return err
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "DIFF: %s\n", sourcesDriftSummary(probed, drifted, diffs))
	if strict && (drifted > 0 || sourcesEnumDiffDrifted(diffs) > 0) {
		return fmt.Errorf("drift detected: %d of %d probed rows moved, %d of %d index pages disagree "+
			"with the catalogue", drifted, probed, sourcesEnumDiffDrifted(diffs), len(diffs))
	}
	return nil
}

// sourcesRunFetch retrieves exactly one document and measures it.
func sourcesRunFetch(cmd *cobra.Command, flags *rootFlags, kind, id string, strict bool) error {
	row, ok := sourceDocByID(id)
	if !ok {
		return usageErr(fmt.Errorf("unknown document id %q. Ids for --kind %s:\n  %v",
			id, sourcesFetchKindLabel(kind), sourcesFetchIDs(kind)))
	}
	if kind != "" && row.Kind != kind {
		return usageErr(fmt.Errorf("%s is kind %q, not %q; drop --kind or pass a matching id",
			id, row.Kind, kind))
	}
	if err := sourcesValidateFetch(row); err != nil {
		return usageErr(err)
	}
	if err := sourcesReportCompleteness(cmd, strict); err != nil {
		return err
	}

	c, err := flags.newClient()
	if err != nil {
		return err
	}
	header := client.HTMLResponseHeader
	if row.ContentKind == "pdf" {
		header = client.BinaryResponseHeader
	}
	body, err := c.GetWithHeaders(cmd.Context(), row.Path, nil, map[string]string{header: "true"})
	if err != nil {
		// Unlike --diff, a --fetch of one named document IS a not-found when it
		// 404s, so the typed exit codes apply here.
		return classifyAPIError(cmd.OutOrStdout(), err, flags)
	}

	// --deliver file:<path> wants the DOCUMENT, so the client's response goes
	// to the sink before any measurement is printed; stdout must not claim
	// success after a failed write.
	if handled, derr := handleBinaryResponseDelivery(cmd, flags, body); handled {
		return derr
	}

	raw := []byte(body)
	contentType := c.LastContentType()
	if unwrapped, ct, ok := client.UnwrapBinaryResponse(body); ok {
		raw = unwrapped
		if ct != "" {
			contentType = ct
		}
	}

	measured, refusedState, refusalReason := sourcesMeasureBody(row, raw, contentType)
	content, err := sourcesMarshalMeasurement(measured)
	if err != nil {
		return err
	}
	chosenBaseline, _ := sourcesBaselineFor(row)
	res := sourcesFetchResult{
		ID: row.ID, Kind: row.Kind, URL: nepraSourcesHost + row.Path,
		State:    "measured",
		Measured: measured,
		Baseline: chosenBaseline,
		// Rows is 1 and skipped 0: a document is one artifact, and the
		// provenance struct records that rather than leaving it implied.
		Artifact: newNepraArtifact(nepraSourcesHost+row.Path, raw, content, contentType, 1, 0),
	}
	if row.ContentKind == "pdf" {
		res.CharCountNote = sourcesCharCountNote
	}
	if refusedState != "" {
		res.State = refusedState
		res.RefusalReason = refusalReason
		res.BodyPrefix = sourcesBodyPrefix(raw)
	}

	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		sourcesWriteFetchSummary(cmd, row, res)
	} else {
		meta := sourcesMeta("live", row.Kind, []sourceDoc{row})
		meta["fetched"] = row.ID
		if err := sourcesEmit(cmd, flags, meta, []sourcesFetchResult{res}); err != nil {
			return err
		}
	}

	if msg := sourcesFetchDriftMessage(res); msg != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "BASELINE: %s\n", msg)
		if strict {
			return fmt.Errorf("%s", msg)
		}
	}
	if res.State != "measured" {
		fmt.Fprintf(cmd.ErrOrStderr(), "REFUSED: %s\n", res.RefusalReason)
		if strict {
			return fmt.Errorf("%s: %s", res.ID, res.RefusalReason)
		}
	}
	return nil
}

func sourcesFetchKindLabel(kind string) string {
	if kind == "" {
		return "(all)"
	}
	return kind
}

// sourcesEmit marshals the owned {meta, results} envelope and prints it.
func sourcesEmit(cmd *cobra.Command, flags *rootFlags, meta map[string]any, results any) error {
	payload, err := json.Marshal(results)
	if err != nil {
		return err
	}
	envelope, err := json.Marshal(map[string]any{
		"meta":    meta,
		"results": json.RawMessage(payload),
	})
	if err != nil {
		return err
	}
	wrapped, err := wrapPlatformStructuredOutput(envelope, flags, "results", true)
	if err != nil {
		return err
	}
	return printOutput(cmd.OutOrStdout(), wrapped, true)
}

// --- human output -------------------------------------------------------

func sourcesWriteHumanTable(cmd *cobra.Command, rows []sourceDoc, kind string) {
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(w, "ID\tKIND\tSTATE\tBYTES AS OF\tFETCH WITH")
	for _, r := range rows {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			r.ID, r.Kind, r.State, sourcesBytesCell(r), sourcesFetchCell(r))
	}
	_ = w.Flush()

	e := cmd.ErrOrStderr()
	measured := 0
	for _, r := range rows {
		if r.BytesAsOf != nil {
			measured++
		}
	}
	scope := "all five kinds"
	if kind != "" {
		scope = "--kind " + kind
	}
	fmt.Fprintf(e, "\n%d documents (%s), %d with a measured byte count, as of %s.\n",
		len(rows), scope, measured, sourcesAsOfDate)
	fmt.Fprintf(e, "A dash under BYTES AS OF means never measured — not zero. "+
		"Pass --diff to re-measure the cheap rows, or --fetch <id> for one document.\n")
	fmt.Fprintf(e, "%d declared gaps this command does NOT answer; see meta.declared_gaps under --json.\n",
		len(sourcesDeclaredGaps))
}

func sourcesBytesCell(r sourceDoc) string {
	if r.BytesAsOf == nil {
		if r.HTTPStatusAsOf != nil {
			return "- (HTTP " + itoa(*r.HTTPStatusAsOf) + ")"
		}
		return "-"
	}
	return commaBytes(*r.BytesAsOf)
}

func sourcesFetchCell(r sourceDoc) string {
	if r.FetchWith != "" {
		return r.FetchWith
	}
	if r.Fetchable {
		return "sources --fetch " + r.ID
	}
	// A dash here is not "we forgot": every one of these rows carries a
	// fetch_blocked_reason or a fetch_refusal saying which measurement
	// refuses it.
	return "-"
}

func sourcesWriteDiffTable(cmd *cobra.Command, rows []sourceDoc, diffs []sourcesEnumDiff, probed, drifted int) {
	w := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(w, "ID\tSTATE\tVERDICT\tBYTES NOW\tDELTA")
	for _, r := range rows {
		verdict, now, delta := "-", "-", "-"
		if r.Probe != nil {
			verdict = r.Probe.Verdict
			if r.Probe.BytesNow != nil {
				now = commaBytes(*r.Probe.BytesNow)
			} else if r.Probe.HTTPStatus != nil {
				now = "HTTP " + itoa(*r.Probe.HTTPStatus)
			}
			if r.Probe.DeltaBytes != nil {
				delta = commaBytes(*r.Probe.DeltaBytes)
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.ID, r.State, verdict, now, delta)
	}
	_ = w.Flush()

	e := cmd.ErrOrStderr()
	for _, d := range diffs {
		now := "unread"
		if d.HrefsNow != nil {
			now = itoa(*d.HrefsNow)
		}
		fmt.Fprintf(e, "\n%s: %s of %d hrefs, %d new upstream, %d catalogued paths dropped",
			d.Enumerator, now, d.HrefsAsOf, len(d.IndexOnly), len(d.CatalogueOnly))
		if d.Duplicates > 0 {
			fmt.Fprintf(e, ", %d duplicate hrefs", d.Duplicates)
		}
		fmt.Fprintln(e)
		for _, h := range d.IndexOnly {
			fmt.Fprintf(e, "  NEW UPSTREAM  %s\n", h.PathEncoded)
		}
		for _, p := range d.CatalogueOnly {
			fmt.Fprintf(e, "  DROPPED       %s\n", p)
		}
	}
	fmt.Fprintf(e, "\n%d rows probed, %d drifted. A 404 or 403 here is DATA, not a failure.\n", probed, drifted)
}

func sourcesWriteFetchSummary(cmd *cobra.Command, row sourceDoc, res sourcesFetchResult) {
	o := cmd.OutOrStdout()
	fmt.Fprintf(o, "%s  %s\n", res.ID, res.URL)
	fmt.Fprintf(o, "  state          %s\n", res.State)
	fmt.Fprintf(o, "  bytes          %s", commaBytes(res.Measured.Bytes))
	if res.Baseline != nil {
		fmt.Fprintf(o, "  (baseline %s, %s)", commaBytes(*res.Baseline), res.Measured.BaselineUsed)
	} else {
		fmt.Fprintf(o, "  (no baseline)")
	}
	fmt.Fprintln(o)
	if res.Measured.BaselineNote != "" {
		fmt.Fprintf(o, "  baseline_note  %s\n", res.Measured.BaselineNote)
	}
	if res.Measured.Pages != nil {
		fmt.Fprintf(o, "  pages          %d\n", *res.Measured.Pages)
	}
	if res.Measured.Creator != nil {
		fmt.Fprintf(o, "  creator        %q\n", *res.Measured.Creator)
	}
	if res.Measured.Producer != nil {
		fmt.Fprintf(o, "  producer       %q\n", *res.Measured.Producer)
	}
	if res.Measured.CharCount != nil {
		fmt.Fprintf(o, "  char_count     %s  (reported, never asserted)\n", commaBytes(*res.Measured.CharCount))
	}
	fmt.Fprintf(o, "  text_layer     %s\n", res.Measured.TextLayer)
	if res.Measured.LowTextPages != nil {
		fmt.Fprintf(o, "  low_text_pages %v\n", res.Measured.LowTextPages)
	}
	fmt.Fprintf(o, "  sha256_content %s\n", res.Artifact.SHA256Content)
	if res.RefusalReason != "" {
		fmt.Fprintf(o, "  refused        %s\n  body_prefix    %s\n", res.RefusalReason, res.BodyPrefix)
	}
}
