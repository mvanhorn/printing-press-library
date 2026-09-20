// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// The RunE body below is an implementation, not a TODO scaffold: `generate
// --force` preserves implemented bodies. The pure shapers live in
// nepra_gen_rows.go, the meta block in nepra_gen_meta.go and the seven-year
// catalogue in nepra_gen_catalogue.go.
// See .printing-press-patches/nepra-gen-plant-month-extract.json.
// pp:data-source live

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// The four row serialisations.
const (
	genFormatCSV   = "csv"
	genFormatTSV   = "tsv"
	genFormatJSON  = "json"
	genFormatJSONL = "jsonl"
)

var genFormats = []string{genFormatCSV, genFormatTSV, genFormatJSON, genFormatJSONL}

// newNovelGenCmd builds the plant-month generation extract.
//
// This is the only machine-readable form of NEPRA's plant-by-month generation
// table. The published artefact is one fiscal year of Excel "Save as Web Page"
// output — 493,187 windows-1252 bytes for FY2023-24, with the charset declared
// only inside the document — and the whole value of turning it into rows is
// that three different kinds of nothing stay apart:
//
//	KAPCO           1,601 MW installed, a MEASURED 0.00 GWh in all twelve months
//	Reshma             97 MW installed, NO monthly data published at all
//	HUBCO           1,292 MW installed, DELICENSED across the whole block
//
// Collapse those and you have invented twelve months of zero generation for a
// plant that reported none, or deleted a real zero, or lost the reason a
// licensed plant stopped reporting. The parse is nepraparse's — five cell
// states, a 32-column fingerprint, a cell-state census and a
// percent-then-GWh proof re-derived from the numbers on every run.
//
// The command name comes from the manifest. It is kept so the generated
// registration in root.go keeps working.
func newNovelGenCmd(flags *rootFlags) *cobra.Command {
	var (
		flagFy     string
		flagFormat string
		flagRollup string
		flagStrict bool
	)

	cmd := &cobra.Command{
		Use:   "gen",
		Short: "Pull every power plant's monthly generation and utilisation for a fiscal year as rows you can pipe",
		Long: "One fiscal year of NEPRA's plant-by-month generation table, as tidy rows.\n\n" +
			"Twelve rows per plant — 1,596 for FY2023-24's 133 plants — whatever the plant's state, so a\n" +
			"delicensed or never-reporting plant cannot vanish from the extract. Every numeric field carries\n" +
			"an adjacent *_state column, and a blank NEVER becomes a zero: a measured 0.00 GWh (KAPCO, all\n" +
			"twelve months) stays distinguishable from NOT REPORTED (Reshma, 97 MW, no monthly data) and from\n" +
			"DELICENSED (HUBCO, 1,292 MW). Capacity is independent of that: Kotri keeps 174/120 MW while all\n" +
			"26 of its monthly cells read DELICENSED.\n\n" +
			"NO AGGREGATE UTILISATION IS EMITTED, in any grain — a utilisation is a ratio, and the file's own\n" +
			"Sum column proves it (the annual-total identity holds 118/118 on GWh and 3/118 on \"% age\").\n" +
			"meta.refusals lists that and every other question this command declines, with the reason.\n\n" +
			"Live-only: there is no local store for this panel, so --data-source local is refused rather than\n" +
			"quietly served from the network. With no --fy it prints the seven-year catalogue and makes no request.",
		Example: "  nepra-pp-cli gen\n" +
			"  nepra-pp-cli gen --fy 2023-24\n" +
			"  nepra-pp-cli gen --fy 2023-24 --format csv > fy2023-24.csv\n" +
			"  nepra-pp-cli gen --fy 2023-24 --rollup technology --format csv\n" +
			"  nepra-pp-cli gen --fy 2020-21 --format jsonl --strict",
		Annotations: map[string]string{
			// A pure read: one HTTP GET of one published document, no
			// writes anywhere. The scaffold said false, which was wrong.
			"mcp:read-only": "true",
			"pp:happy-args": "--fy=2023-24",
			// The codes this command can actually return, in the generated
			// commands' own comma form: 0 success, 1 a --strict completeness
			// failure, 2 a usage refusal, 3 a fiscal year this path does not
			// publish, 5 a fatal parse (schema drift, wrong year, no table).
			// There is no 4: this surface needs no credential.
			"pp:typed-exit-codes": "0,1,2,3,5",
			"pp:novel-hand-coded": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// 1. Help-only branch. The catalogue is the useful answer to a
			// bare `gen` and it costs no request: seven reachable years with
			// their measured byte and row floors, and the four recorded
			// 404s.
			if genHelpOnly(cmd, flagFy, flagFormat, flagRollup, flagStrict) {
				return genPrintCatalogue(cmd, flags)
			}

			// 2. Dry-run, BEFORE any validation. The verify pipeline probes
			// hand-written commands with --dry-run, and cobra runs Args: and
			// MarkFlagRequired before RunE — which is why --fy is validated
			// here by hand and cmd.MarkFlagRequired is deliberately not
			// called.
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "gen")
			}

			// 3. Usage errors.
			if err := validateDataSourceStrategy(flags, "live"); err != nil {
				return usageErr(err)
			}
			format, err := genResolveFormat(flagFormat, flags, cmd.OutOrStdout())
			if err != nil {
				return usageErr(err)
			}
			rollup, err := genResolveRollup(flagRollup)
			if err != nil {
				return usageErr(err)
			}
			if strings.TrimSpace(flagFy) == "" {
				return usageErr(fmt.Errorf("--fy is required and must not be blank; reachable years are %s "+
					"(run `%s` with no flags for the catalogue)", genReachableList(), cmd.CommandPath()))
			}
			fy, err := nepraparse.ParseFiscalYear(flagFy)
			if err != nil {
				return usageErr(fmt.Errorf("--fy %q is not a fiscal year: %w; accepted forms are 2023-24, "+
					"FY 2023-24 and 2023-2024, and the reachable years are %s", flagFy, err, genReachableList()))
			}
			year, known := genYearByLabel(fy.Label())
			if !known {
				return notFoundErr(fmt.Errorf("FY%s is outside the recorded window; on %s the reachable years "+
					"were %s and this path published nothing else. If NEPRA has since added it, "+
					"`%s generation year %s` fetches the raw workbook so you can see for yourself",
					fy.Label(), genAsOfDate, genReachableList(), rootCommandName(cmd), fy.Label()))
			}
			if !year.Reachable {
				return notFoundErr(fmt.Errorf("FY%s is not published at this path: %s. Reachable years are %s. "+
					"If NEPRA has since published it, `%s generation year %s` fetches the raw workbook",
					year.Label, year.Note, genReachableList(), rootCommandName(cmd), year.Label))
			}

			// 4. Fetch. One GET of one document.
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := genSheetPath(year.Label)
			sourceURL := c.RequestBaseURL() + path
			body, err := c.GetWithHeaders(cmd.Context(), path, map[string]string{},
				map[string]string{client.HTMLResponseHeader: "true"})
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}

			// 5. Parse. ALWAYS with the caller's non-empty label: that is the
			// only thing standing between a stale copy and a whole year dated
			// wrongly. `SIR Data 2025.htm` is byte-identical to `SIR Data
			// 2024.htm` and frames FY2023-24, and the per-year parent .htm is
			// a 9,838-byte frameset that is byte-identical across three
			// sampled years. ErrFiscalYearMismatch is what catches that.
			w, err := nepraparse.ParseWorkbook(body, year.Label)
			if err != nil {
				return genClassifyParseError(err, year.Label, len(body))
			}

			// 6. Shape.
			grain := "plant-month"
			var rowCells [][]string
			var payload any
			groups := 0
			if rollup != "" {
				grain = "group-month"
				rr := genRollup(w, rollup, sourceURL)
				payload = rr
				groups = len(rr) / 12
				for _, r := range rr {
					rowCells = append(rowCells, r.Cells())
				}
			} else {
				pr := genRows(w, sourceURL)
				payload = pr
				for _, r := range pr {
					rowCells = append(rowCells, r.Cells())
				}
			}
			content := genRowsJSON(payload)
			meta := genBuildMeta(w, year, grain, len(rowCells), body, sourceURL, c.LastContentType(), content)
			meta.Rollup = rollup
			meta.Groups = groups

			// 7. Write the rows, then the assertions — in that order, so a
			// truncated pipe loses the commentary and not the data.
			header := genRowHeader
			if rollup != "" {
				header = genRollupHeader
			}
			if err := genWrite(cmd, flags, format, header, rowCells, meta, payload); err != nil {
				return err
			}

			short := genReport(cmd, meta, format)
			if len(short) > 0 && flagStrict {
				return fmt.Errorf("%d of %d completeness assertions failed for FY%s; see the COMPLETENESS lines on stderr",
					len(short), len(meta.Assertions), year.Label)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flagFy, "fy", "", "Fiscal year as published, e.g. 2023-24. Reachable: "+
		"2017-18 through 2023-24 (FY 2023-24 and 2023-2024 also parse). A year outside that window exits 3 "+
		"rather than returning an empty result, because unpublished and empty are different findings")
	cmd.Flags().StringVar(&flagFormat, "format", "", "Row serialisation: csv, tsv, json or jsonl. "+
		"Default: csv with --csv, tsv with --plain, json with --json/--agent or a piped stdout, else tsv. "+
		"csv/tsv put the header and rows on stdout and the summary on stderr; json emits one {meta,results} "+
		"envelope; jsonl emits one row per line with meta on stderr. NOTE: --compact and --select do not "+
		"apply to the json envelope, which this command marshals itself. Published thousands separators and "+
		"trailing zeros are not reproduced (\"1,601.00\" prints as 1601); the value is unchanged")
	cmd.Flags().StringVar(&flagRollup, "rollup", "", "Aggregate the year into group-month rows by "+
		"technology (9 groups in every sampled year) or fuel (16). Sums GWh over the cells that carried a "+
		"number and counts every cell it could not sum, by reason. Emits NO utilisation figure of any kind, "+
		"and reports gwh_measured=false with no number for a group whose cells were all non-numeric")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Exit non-zero when a completeness assertion fails. "+
		"The assertions print to stderr either way; --strict only changes the exit code, which is what makes "+
		"this command usable as a cron gate")
	return cmd
}

// genHelpOnly reports whether the invocation asked for nothing in particular,
// in which case the catalogue is the answer and no request is made.
func genHelpOnly(cmd *cobra.Command, fy, format, rollup string, strict bool) bool {
	if cmd.Flags().Changed("fy") || strings.TrimSpace(fy) != "" {
		return false
	}
	return format == "" && rollup == "" && !strict
}

// rootCommandName returns the binary name for use in a suggestion.
func rootCommandName(cmd *cobra.Command) string {
	return cmd.Root().Name()
}

// genResolveFormat picks the row serialisation, first match wins.
//
// It never falls back silently on a bad value: a caller who asked for --format
// xml gets exit 2 rather than a TSV file they did not ask for and will not
// notice until it is in a spreadsheet.
func genResolveFormat(explicit string, flags *rootFlags, out io.Writer) (string, error) {
	if e := strings.TrimSpace(explicit); e != "" {
		for _, f := range genFormats {
			if strings.EqualFold(e, f) {
				return f, nil
			}
		}
		return "", fmt.Errorf("--format %q is not one of %s", explicit, strings.Join(genFormats, ", "))
	}
	switch {
	case flags == nil:
		return genFormatTSV, nil
	case flags.csv:
		return genFormatCSV, nil
	case flags.plain:
		return genFormatTSV, nil
	case flags.asJSON:
		// --agent sets asJSON in the root PersistentPreRun, so this covers
		// both.
		return genFormatJSON, nil
	case !isTerminal(out):
		return genFormatJSON, nil
	default:
		return genFormatTSV, nil
	}
}

// genResolveRollup validates the rollup selector.
func genResolveRollup(v string) (string, error) {
	switch s := strings.TrimSpace(strings.ToLower(v)); s {
	case "":
		return "", nil
	case genRollupTechnology, genRollupFuel:
		return s, nil
	default:
		return "", fmt.Errorf("--rollup %q is not one of %s, %s; there is deliberately no plant rollup "+
			"(the default grain already is per plant) and no province or system rollup (NEPRA publishes "+
			"neither in this file)", v, genRollupTechnology, genRollupFuel)
	}
}

// genClassifyParseError maps a parse failure onto an exit code.
//
// Schema drift, a missing header band and a missing table are all exit 5 and
// all FATAL: a shifted or renamed column produces numbers that still look
// right, so continuing would hand back a corrupted panel that nobody would
// question. A fiscal-year mismatch is the same severity for the same reason —
// the numbers would be fine and the DATE would be wrong.
func genClassifyParseError(err error, label string, bodyBytes int) error {
	switch {
	case errors.Is(err, nepraparse.ErrFiscalYearMismatch):
		return apiErr(fmt.Errorf("the workbook served for FY%s says it covers a different year: %w. "+
			"This is the stale-copy guard and it is doing its job: NEPRA republishes byte-identical "+
			"copies of one year's file under two years' names, so the parse refuses rather than "+
			"date a whole year wrongly", label, err))
	case errors.Is(err, nepraparse.ErrSchemaDrift):
		return apiErr(fmt.Errorf("the FY%s header band no longer matches the measured 32-column schema, "+
			"so NO rows were emitted: %w. A shifted or renamed column produces numbers that still look "+
			"plausible; the per-column difference is in the message above and must be read before the "+
			"schema constants are changed", label, err))
	case errors.Is(err, nepraparse.ErrNoHeaderBand):
		return apiErr(fmt.Errorf("could not locate the FY%s three-row header band in %d bytes: %w. "+
			"These files carry ZERO <th>, so the band is found by its thirteen %q leaf cells; none were "+
			"found, which means this is not the workbook", label, bodyBytes, err, nepraparse.MetricPct))
	case errors.Is(err, nepraparse.ErrNoTable):
		return apiErr(fmt.Errorf("the FY%s response carried no table rows at all in %d bytes: %w. "+
			"A 9-byte body here is NEPRA's 404; a larger one with no table is a frameset wrapper rather "+
			"than the sheet", label, bodyBytes, err))
	case errors.Is(err, nepraparse.ErrFiscalYear):
		return apiErr(fmt.Errorf("the FY%s workbook's own header band does not name a readable fiscal "+
			"year: %w. The band cell is the only in-document statement of which year the file covers, "+
			"so an unreadable one cannot be worked around", label, err))
	default:
		return err
	}
}

// genWrite emits the rows in the resolved format.
func genWrite(cmd *cobra.Command, flags *rootFlags, format string, header []string, rows [][]string, meta genMeta, payload any) error {
	out := cmd.OutOrStdout()
	switch format {
	case genFormatJSON:
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		envelope := map[string]any{"meta": meta, "results": json.RawMessage(body)}
		blob, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return err
		}
		return printOutput(out, blob, true)
	case genFormatJSONL:
		// One row per line on stdout; the meta block goes to stderr so a
		// `while read` loop sees rows only.
		items, err := genRowJSONLines(payload)
		if err != nil {
			return err
		}
		for _, line := range items {
			if _, err := fmt.Fprintf(out, "%s\n", line); err != nil {
				return err
			}
		}
		blob, err := json.MarshalIndent(meta, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(cmd.ErrOrStderr(), "%s\n", blob)
		return err
	case genFormatCSV:
		writeCSVRow(out, header)
		for _, r := range rows {
			writeCSVRow(out, r)
		}
		// A spreadsheet treats a cell beginning =, +, - or @ as a FORMULA,
		// so upstream text reaching a .csv can execute on open. This command
		// REPORTS it and does not repair it, which is the same contract it
		// keeps for every other published value: prefixing a quote would
		// alter a NEPRA string, and this CLI does not alter NEPRA strings.
		// The warning goes to stderr, where the completeness assertions
		// already go, so a caller reading those sees it too.
		//
		// MEASURED across FY2017-18, FY2020-21, FY2023-24 and the whole
		// licence register: ZERO cells currently begin with one of those
		// characters, so this is a guard against future upstream content,
		// not a description of today's.
		genWarnFormulaCells(cmd.ErrOrStderr(), header, rows)
		return nil
	case genFormatTSV:
		return genWriteTSV(out, header, rows)
	default:
		// Unreachable: genResolveFormat is the only producer of this value.
		return usageErr(fmt.Errorf("--format %q is not one of %s", format, strings.Join(genFormats, ", ")))
	}
}

// genWriteTSV writes tab-separated rows.
//
// Tab safety is PROVEN, not assumed: nepraparse.collapseText turns every
// whitespace rune — tab, CR, LF and NBSP included — into a single space and
// trims, so no cell can carry a tab. The guard below still runs, because a
// broken row that reads as a valid row is exactly the failure this command
// exists to prevent, and refusing beats emitting one.
func genWriteTSV(out io.Writer, header []string, rows [][]string) error {
	if _, err := fmt.Fprintln(out, strings.Join(header, "\t")); err != nil {
		return err
	}
	for i, r := range rows {
		for j, cell := range r {
			if strings.ContainsAny(cell, "\t\r\n") {
				return fmt.Errorf("row %d column %q contains a tab, CR or LF (%q), which would silently "+
					"split the row; re-run with --format csv or --format json", i+1, header[j], cell)
			}
		}
		if _, err := fmt.Fprintln(out, strings.Join(r, "\t")); err != nil {
			return err
		}
	}
	return nil
}

// genRowJSONLines marshals each row compactly, one per line.
func genRowJSONLines(payload any) ([]string, error) {
	switch rows := payload.(type) {
	case []genRow:
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			b, err := json.Marshal(r)
			if err != nil {
				return nil, err
			}
			out = append(out, string(b))
		}
		return out, nil
	case []genRollupRow:
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			b, err := json.Marshal(r)
			if err != nil {
				return nil, err
			}
			out = append(out, string(b))
		}
		return out, nil
	case []genPlantRow:
		// `generation plants`. Without this case the type falls to the
		// default below and --format jsonl fails with "unsupported payload"
		// on an otherwise correct run.
		out := make([]string, 0, len(rows))
		for _, r := range rows {
			b, err := json.Marshal(r)
			if err != nil {
				return nil, err
			}
			out = append(out, string(b))
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unsupported payload %T", payload)
	}
}

// genReport prints the completeness assertions and the summary to stderr, in
// every format and whether or not --strict was set, and returns the failures.
//
// Stderr is deliberate: the rows are the product and must stay pipeable, but a
// completeness claim that only a --json reader ever sees is a claim most
// callers will never read.
func genReport(cmd *cobra.Command, meta genMeta, format string) []genAssertion {
	w := cmd.ErrOrStderr()
	if format != genFormatJSONL {
		genWriteSummary(w, meta)
	}
	for _, a := range meta.Assertions {
		switch {
		case !a.Asserts:
			fmt.Fprintf(w, "assert  %-42s NOT ASSERTED  %s\n", a.Name, a.Detail)
		case a.OK:
			fmt.Fprintf(w, "assert  %-42s ok            %s\n", a.Name, genAssertionValues(a))
		default:
			fmt.Fprintf(w, "assert  %-42s FAILED        %s\n", a.Name, genAssertionValues(a))
		}
	}
	short := genShortfalls(meta.Assertions)
	for _, a := range short {
		asOf := a.AsOf
		if asOf == "" {
			asOf = "this run"
		}
		fmt.Fprintf(w, "COMPLETENESS: %s failed — %s. Measured on %s. Do not treat this extract as complete.\n",
			a.Name, a.Detail, asOf)
	}
	return short
}

func genAssertionValues(a genAssertion) string {
	switch {
	case a.Expected != nil && a.Actual != nil:
		return fmt.Sprintf("expected %d, got %d", *a.Expected, *a.Actual)
	case a.Actual != nil:
		return fmt.Sprintf("measured %d", *a.Actual)
	default:
		return a.Detail
	}
}

// genWriteSummary is the one-screen human summary.
func genWriteSummary(w io.Writer, meta genMeta) {
	fmt.Fprintf(w, "FY%s  %s  %s%s\n", meta.FY, meta.Grain, meta.Charset, genDeclaredSuffix(meta.CharsetDeclared))
	fmt.Fprintf(w, "%d rows from %d plants  %d bytes  %d table rows  %d raw cells  width %d logical / %d physical\n",
		meta.Rows, meta.Plants, meta.Artifact.Bytes, meta.TableRows, meta.RawCells, meta.LogicalColumns, meta.PhysicalWidth)
	rc := meta.RowClasses
	fmt.Fprintf(w, "row classes: %d active, %d delicensed, %d decommissioned, %d listed_no_data, %d export_to_k_electric, %d unmodelled_block\n",
		rc.Active, rc.Delicensed, rc.Decommissioned, rc.ListedNoData, rc.ExportToKElectric, rc.UnmodelledBlock)
	fmt.Fprintf(w, "of the %d active plants %d reconcile against the published Sum and %d publish a partly-blank block and cannot be summed at all\n",
		rc.Active, meta.Reconcilability.ActiveAndEligible, meta.Reconcilability.ActiveNotEligible)
	c := meta.Census
	fmt.Fprintf(w, "monthly cells: %d total = %d present + %d not_reported + %d delicensed + %d decommissioned + %d export + %d unknown_text  (balanced=%t)\n",
		c.MonthlyCellsTotal, c.Present, c.NotReported, c.Delicensed, c.Decommissioned, c.ExportToKElectric, c.UnknownText, c.Balanced)
	fmt.Fprintf(w, "of the present cells %d read exactly 0.00 and are MEASUREMENTS; %d rows mix a blank with a zero\n",
		c.MeasuredZeros, c.MixedBlankZeroRows)
	fmt.Fprintf(w, "column order: %s\n", meta.ColumnOrder.Explanation)
	fmt.Fprintf(w, "sum check (%s, tolerance %.2f): %d/%d reconcile, %d skipped\n",
		meta.SumCheck.Field, meta.SumCheck.Tolerance, meta.SumCheck.Passed, meta.SumCheck.Eligible, meta.SumCheck.Skipped)
	for _, m := range meta.SumCheck.Mismatches {
		fmt.Fprintf(w, "  PUBLISHED MISMATCH, reported not corrected: %s months=%.2f sum=%.2f delta=%+.2f\n",
			m.Plant, m.MonthlySum, m.Reported, m.Delta)
	}
	fmt.Fprintf(w, "capacity: total %s | active %s | non-operating %s | listed_no_data %s | export %s | unmodelled_block %s\n",
		genMWString(meta.CapacityMW.PublishedTotal), genMWString(meta.CapacityMW.Active),
		genMWString(meta.CapacityMW.NonOperating), genMWString(meta.CapacityMW.ListedNoData),
		genMWString(meta.CapacityMW.ExportToKElectric), genMWString(meta.CapacityMW.UnmodelledBlock))
	for _, warn := range meta.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
	for _, r := range meta.Refusals {
		fmt.Fprintf(w, "refusal: %s\n", r)
	}
}

func genDeclaredSuffix(declared bool) string {
	if declared {
		return " (declared in the document; the HTTP header declares none)"
	}
	return " (ASSUMED — the document declared no charset)"
}

// genMWString renders a capacity sum without printing a number that is not a
// measurement.
func genMWString(m genMW) string {
	if !m.Measured || m.MW == nil {
		return fmt.Sprintf("<not measured over %d plants>", m.PlantsNotNumeric)
	}
	s := fmt.Sprintf("%.1f MW/%d plants", *m.MW, m.Plants)
	if m.PlantsNotNumeric > 0 {
		s += fmt.Sprintf(" (+%d plants whose capacity cell is not a number)", m.PlantsNotNumeric)
	}
	return s
}

// genPrintCatalogue lists the fiscal years without making a request.
func genPrintCatalogue(cmd *cobra.Command, flags *rootFlags) error {
	type entry struct {
		FY              string `json:"fy"`
		Reachable       bool   `json:"reachable"`
		DecodedBytes    *int   `json:"decoded_bytes,omitempty"`
		PlantRows       *int   `json:"plant_rows,omitempty"`
		PlantRowsPinned bool   `json:"plant_rows_pinned"`
		StatusRows      *int   `json:"status_rows,omitempty"`
		Path            string `json:"path,omitempty"`
		AsOfDate        string `json:"as_of_date"`
		Note            string `json:"note"`
	}
	out := make([]entry, 0, len(genYears))
	for _, y := range genYears {
		e := entry{FY: y.Label, Reachable: y.Reachable, PlantRowsPinned: y.PlantRowsPinned,
			AsOfDate: genAsOfDate, Note: y.Note}
		if y.Reachable {
			e.Path = genSheetPath(y.Label)
			e.DecodedBytes = genIntPtr(y.DecodedBytes)
		}
		if y.PlantRows > 0 {
			e.PlantRows = genIntPtr(y.PlantRows)
		}
		if y.StatusRows >= 0 {
			e.StatusRows = genIntPtr(y.StatusRows)
		}
		out = append(out, e)
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"%d fiscal years catalogued, %d reachable. Pass --fy <year> to read one.\n"+
				"Byte counts and the two 404s were measured on %s. EVERY reachable year carries a plant-row\n"+
				"count; plant_rows_pinned says whether a committed fixture re-derives it on each test run or\n"+
				"whether it was measured once against the live bytes.\n",
			len(genYears), len(genReachableLabels()), genAsOfDate)
	}
	rows, err := json.Marshal(out)
	if err != nil {
		return err
	}
	// Same {meta, results} envelope every other grain of this command emits.
	// This path used to return a bare top-level array, so `.results[]` found
	// nothing and the as-of date and 404 count reached humans only through the
	// stderr line above.
	payload, err := json.Marshal(map[string]any{
		"meta": map[string]any{
			"source":       "catalogue",
			"as_of_date":   genAsOfDate,
			"fiscal_years": len(genYears),
			"reachable":    len(genReachableLabels()),
			"requests":     0,
			"note": "byte counts and the two unreachable years were measured on " + genAsOfDate +
				". plant_rows_pinned says whether a committed fixture re-derives the count on each " +
				"test run or whether it was measured once against the live bytes.",
		},
		"results": json.RawMessage(rows),
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

// genWarnFormulaCells names every emitted cell a spreadsheet would treat as a
// formula. It alters nothing.
func genWarnFormulaCells(w io.Writer, header []string, rows [][]string) {
	flagged := 0
	for i, r := range rows {
		for j, c := range r {
			if !genLooksLikeFormula(c) {
				continue
			}
			flagged++
			if flagged <= 5 {
				name := "column " + strconv.Itoa(j)
				if j < len(header) {
					name = header[j]
				}
				fmt.Fprintf(w, "CSV FORMULA CELL: row %d, %s begins %q and a spreadsheet will evaluate it "+
					"as a formula on open. The value is emitted VERBATIM and not altered, because it is "+
					"what NEPRA published: %q\n", i+1, name, c[:1], c)
			}
		}
	}
	if flagged > 5 {
		fmt.Fprintf(w, "CSV FORMULA CELL: %d further cells also begin with a formula character.\n", flagged-5)
	}
}

// genLooksLikeFormula reports whether a spreadsheet would evaluate this cell.
func genLooksLikeFormula(c string) bool {
	if c == "" {
		return false
	}
	switch c[0] {
	case '=', '+', '@', '\t', '\r':
		return true
	case '-':
		// A negative NUMBER is data, not a formula. Only flag a leading '-'
		// that is followed by something non-numeric, which is the shape that
		// actually evaluates.
		if len(c) > 1 && (c[1] >= '0' && c[1] <= '9' || c[1] == '.') {
			return false
		}
		return true
	}
	return false
}
