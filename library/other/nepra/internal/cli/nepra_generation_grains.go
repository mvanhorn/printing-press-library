// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-generation-grains.json.
//
// pp:data-source live
// Supported strategies: auto, local, live, or computed. `live` matches the
// sibling `gen`, not the sibling `generation year`: there is no local store
// for this panel, so --data-source local is refused rather than quietly
// served from the network.
//
// WHY THIS FILE EXISTS. The absorb manifest approved three rows against the
// command path `generation plants` — rows 11 and 12 as the typed endpoint
// surface and row 13 as the --technology / --fuel behaviour on top of it.
// The generator emitted `generation index` and `generation year` instead, so
// the approved path resolved to nothing and the Phase 3 completion gate
// failed on it. These two subcommands are that path, built as thin FILTERED
// VIEWS over the same fetch, parse and shapers `gen` already uses: the
// monthly grain calls genRows unchanged, so an unfiltered `generation
// monthly --fy X` is byte-identical to `gen --fy X`.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// init attaches both grains to the GENERATED `generation` parent.
//
// internal/cli/generation.go carries a "DO NOT EDIT" header, so the children
// are added through the novel hook rather than by editing it: root.go runs
// the hook loop after every generated AddCommand, so the parent exists by
// then, and a `generate --force` cannot drop the wiring.
//
// THE Find GUARD IS THREE CONDITIONS, NOT ONE. cobra's Find returns a nil
// error for a missing NESTED name — root.Find([]string{"generation","nope"})
// returns (generation, ["nope"], nil) — and on a missing TOP-LEVEL name it
// returns the ROOT command with the error set. So a hook that checks only
// err would publish these as stray top-level commands, and one that checks
// only the remainder would attach them to root. Both are silent.
func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		parent, rest, err := root.Find([]string{"generation"})
		if err != nil || parent == nil || parent == root || len(rest) != 0 {
			return
		}
		addNovelCommandIfAbsent(parent, newGenerationPlantsCmd(flags))
		addNovelCommandIfAbsent(parent, newGenerationMonthlyCmd(flags))
	})
}

// The two grains this file serves.
const (
	genGrainPlant      = "plant"
	genGrainPlantMonth = "plant-month"
	// genGrainPlantMonthFiltered is a DIFFERENT grain string on purpose.
	// genAssertions gates rows_equal_plants_times_12 on the literal
	// "plant-month" and computes Expected from the WHOLE workbook, so a
	// filtered run under that grain would report a failed assertion — and
	// exit 1 under --strict — on an answer that is entirely correct.
	genGrainPlantMonthFiltered = "plant-month-filtered"
)

// genMonthsPerPlant is how many rows genRows emits per plant.
//
// It is NOT nepraparse.MonthlyCells/2. MonthlyCells is PairCount*2 == 26 and
// PairCount is 13, because the thirteenth pair is the annual Sum column, not
// a month. Deriving twelve from that constant yields thirteen.
const genMonthsPerPlant = 12

// genPlantRow is one row per plant: the plant-level half of genRow, with the
// month, period and utilisation columns dropped because they make no sense at
// this grain.
//
// NO FIELD CARRIES omitempty, for the same reason genRow's doc gives: the
// generated compactListFields drops any key present on fewer than ~80% of
// rows, and a year like FY2017-18 (11 of 108 plants publishing no capacity
// at all) sits close enough to that line to have the whole installed_mw
// column stripped from --compact and --agent output.
type genPlantRow struct {
	FY              string `json:"fy"`
	SNo             int    `json:"sno"`
	Plant           string `json:"plant"`
	Technology      string `json:"technology"`
	TechnologyKnown bool   `json:"technology_known"`
	Fuel            string `json:"fuel"`
	FuelKnown       bool   `json:"fuel_known"`
	// InstalledMW and DependableMW are nil unless the capacity cell is a
	// number, and a status sentinel in the MONTHLY block does NOT null them:
	// Kotri keeps 174/120 MW while all 26 of its monthly cells read
	// DELICENSED. The converse bites too — FY2017-18's eleven listed_no_data
	// rows publish no capacity number at all, and three FY2020-21 rows carry
	// "Export to K.Electric" IN the Installed Capacity column. Emitting 0 in
	// either case would be a fabrication, so the adjacent *_state column
	// carries the reason and the number stays absent.
	InstalledMW       *float64 `json:"installed_mw"`
	InstalledMWState  string   `json:"installed_mw_state"`
	DependableMW      *float64 `json:"dependable_mw"`
	DependableMWState string   `json:"dependable_mw_state"`
	// RowClass is genRowClass's five-step derivation over the whole row. It
	// is NOT p.Status: a numeric block status does not mean the plant
	// reported, which is why Reshma and Gulf Powergen carry block_status
	// numeric with all 26 monthly cells blank.
	RowClass    string `json:"row_class"`
	BlockStatus string `json:"block_status"`
	// SumGWhReported is the plant's PUBLISHED annual Sum, never recomputed.
	SumGWhReported      *float64 `json:"sum_gwh_reported"`
	SumGWhReportedState string   `json:"sum_gwh_reported_state"`
	// SumReconciles is nil when the row was not eligible for the identity
	// check at all. An ineligible row is not a failing row.
	SumReconciles *bool `json:"sum_reconciles"`
	// MonthsReported counts the twelve monthly GWh cells that carried a
	// NUMBER, which includes a measured 0.00. It is the plant-grain answer
	// to "how much of this year did this plant actually report", and it is a
	// count of cells, never a sum of them.
	MonthsReported int `json:"months_reported"`
	// Note carries the verbatim text of any cell this parse could not model,
	// plus any residue past logical column 31. "" when there is none.
	Note      string `json:"note"`
	SourceURL string `json:"source_url"`
}

// genPlantRowHeader is the csv/tsv column order and matches the struct field
// order above.
//
// Written out literally for the reason genRowHeader gives: the shipped
// printCSV alphabetises keys with sort.Strings, and printAutoTable silently
// truncates a >6-column table to its first six headers.
var genPlantRowHeader = []string{
	"fy", "sno", "plant", "technology", "technology_known", "fuel", "fuel_known",
	"installed_mw", "installed_mw_state", "dependable_mw", "dependable_mw_state",
	"row_class", "block_status", "sum_gwh_reported", "sum_gwh_reported_state",
	"sum_reconciles", "months_reported", "note", "source_url",
}

// Cells renders the row in genPlantRowHeader order. An absent number is the
// EMPTY STRING, never "0" and never "null"; a nil sum_reconciles is "" and
// never "false".
func (r genPlantRow) Cells() []string {
	return []string{
		r.FY,
		genFormatInt(r.SNo),
		r.Plant,
		r.Technology,
		genFormatBool(r.TechnologyKnown),
		r.Fuel,
		genFormatBool(r.FuelKnown),
		genFormatFloatPtr(r.InstalledMW),
		r.InstalledMWState,
		genFormatFloatPtr(r.DependableMW),
		r.DependableMWState,
		r.RowClass,
		r.BlockStatus,
		genFormatFloatPtr(r.SumGWhReported),
		r.SumGWhReportedState,
		genFormatBoolPtr(r.SumReconciles),
		genFormatInt(r.MonthsReported),
		r.Note,
		r.SourceURL,
	}
}

// genPlantRows shapes the selected plants into one row each, reusing the
// primitives `gen` already uses so the two grains cannot disagree about a
// plant's class, capacity or reconcilability.
//
// reconcile is genReconcileByRow over the WHOLE workbook: the Sum identity is
// a property of the published row, not of the selection.
func genPlantRows(w *nepraparse.Workbook, plants []nepraparse.Plant, sourceURL string) []genPlantRow {
	reconcile := genReconcileByRow(w)
	out := make([]genPlantRow, 0, len(plants))
	for _, p := range plants {
		months := 0
		for i := range p.Months {
			if p.Months[i].Generation.State() == nepraparse.StateNumeric {
				months++
			}
		}
		out = append(out, genPlantRow{
			FY:                  w.FiscalYear.Label(),
			SNo:                 p.SNo,
			Plant:               p.Name,
			Technology:          p.Technology,
			TechnologyKnown:     p.TechnologyKnown,
			Fuel:                p.Fuel,
			FuelKnown:           p.FuelKnown,
			InstalledMW:         genFloatPtr(p.InstalledCapacity),
			InstalledMWState:    p.InstalledCapacity.State().String(),
			DependableMW:        genFloatPtr(p.DependableCapacity),
			DependableMWState:   p.DependableCapacity.State().String(),
			RowClass:            genRowClass(p),
			BlockStatus:         p.Status.String(),
			SumGWhReported:      genFloatPtr(p.Total.Generation),
			SumGWhReportedState: p.Total.Generation.State().String(),
			SumReconciles:       reconcile[p.RowIndex],
			MonthsReported:      months,
			Note:                genPlantNote(p),
			SourceURL:           sourceURL,
		})
	}
	return out
}

// genPlantNote reports any cell in the plant's identity or capacity columns
// that nepraparse could not model, plus anything found past logical column
// 31. It is the plant-grain sibling of genRowNote, which takes a monthly
// observation and so cannot be used here.
func genPlantNote(p nepraparse.Plant) string {
	var parts []string
	for _, c := range []struct {
		label string
		v     nepraparse.Value
	}{
		{"installed_mw", p.InstalledCapacity},
		{"dependable_mw", p.DependableCapacity},
		{"sum_gwh_reported", p.Total.Generation},
	} {
		if c.v.State() == nepraparse.StateUnknownText {
			parts = append(parts, fmt.Sprintf("unmodelled %s text %q", c.label, c.v.Raw()))
		}
	}
	if len(p.Residue) > 0 {
		parts = append(parts, fmt.Sprintf("%d cell(s) past logical column %d: %q",
			len(p.Residue), nepraparse.LogicalColumns-1, p.Residue))
	}
	return joinNotes(parts)
}

// genPlantFilter is the resolved --technology / --fuel selection.
type genPlantFilter struct {
	Technology string
	Fuel       string
}

// Active reports whether anything was actually selected.
func (f genPlantFilter) Active() bool { return f.Technology != "" || f.Fuel != "" }

// genFilterPlants selects the plants matching the filter.
//
// MATCHING IS PER-COLUMN AND EXACT, case-insensitive at most. It is
// deliberately NOT a substring, prefix or whitespace-normalising match,
// because the published vocabularies defeat all three: technology carries
// both "THERMAL" and "THERMAL- COAL" (the space after the hyphen is NEPRA's
// and is reproduced verbatim), so a prefix match silently folds 8 more plants
// into THERMAL, and a whitespace-normalising match would accept
// "THERMAL-COAL", a string NEPRA never published. WIND, SOLAR and NUCLEAR
// appear in BOTH the technology and fuel vocabularies, which is why the two
// flags match against their own column only and are ANDed.
//
// Validation is against the values THIS YEAR publishes, not only against
// nepraparse.KnownTechnologies / KnownFuels: a value outside the measured
// vocabulary is kept and filterable rather than folded away, the same
// contract genRollup states for an out-of-vocabulary group.
func genFilterPlants(w *nepraparse.Workbook, f genPlantFilter) ([]nepraparse.Plant, error) {
	if f.Technology != "" {
		if err := genFilterKnown("--technology", f.Technology, w.Technologies(), nepraparse.KnownTechnology); err != nil {
			return nil, err
		}
	}
	if f.Fuel != "" {
		if err := genFilterKnown("--fuel", f.Fuel, w.Fuels(), nepraparse.KnownFuel); err != nil {
			return nil, err
		}
	}
	out := make([]nepraparse.Plant, 0, len(w.Plants))
	for _, p := range w.Plants {
		if f.Technology != "" && !genFilterMatches(p.Technology, f.Technology) {
			continue
		}
		if f.Fuel != "" && !genFilterMatches(p.Fuel, f.Fuel) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// genFilterMatches is exact first, then case-insensitive over the whole
// value. Never a substring.
func genFilterMatches(published, want string) bool {
	return published == want || strings.EqualFold(published, want)
}

// genFilterKnown refuses a value this fiscal year does not publish, and says
// what it does publish. The message distinguishes "not in this year" from
// "not a NEPRA value at all", because those are different findings: the first
// is a real answer about the year, the second is a typo.
func genFilterKnown(flag, want string, published []string, known func(string) bool) error {
	for _, v := range published {
		if genFilterMatches(v, want) {
			return nil
		}
	}
	vocab := append([]string(nil), published...)
	sort.Strings(vocab)
	suffix := ""
	if !known(want) {
		suffix = fmt.Sprintf("; %q is not in NEPRA's published vocabulary in any sampled year either", want)
	}
	return fmt.Errorf("%s %q is not published in this fiscal year%s. This year publishes: %s",
		flag, want, suffix, strings.Join(vocab, ", "))
}

// newGenerationPlantsCmd and newGenerationMonthlyCmd each carry a LITERAL
// `Use:` string, and that is a requirement rather than a style choice.
//
// verify-skill reconstructs the command tree from the literal Use values it
// finds in internal/cli/*.go. The shared builder below originally set
// `Use: use` from a variable, and the scanner then reported
// "nepra-pp-cli generation plants: command path not found in
// internal/cli/*.go" — a false finding about a command that resolves at
// runtime and passes the Phase 3 gate. Setting the name here, literally,
// makes the static and runtime trees agree.
func newGenerationPlantsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "plants",
	}
	return genConfigureGrain(cmd, flags, genGrainPlant)
}

func newGenerationMonthlyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "monthly",
	}
	return genConfigureGrain(cmd, flags, genGrainPlantMonth)
}

// newGenerationGrainCmd builds whichever of the two grains was asked for.
//
// The two commands differ only in their row builder, their header and their
// prose: everything from the no-arg branch through the fetch, the parse and
// the completeness report is `gen`'s step order, in `gen`'s order, so a
// divergence between the three commands is a code change rather than a drift.
func genConfigureGrain(cmd *cobra.Command, flags *rootFlags, grain string) *cobra.Command {
	var (
		flagFy         string
		flagFormat     string
		flagTechnology string
		flagFuel       string
		flagStrict     bool
	)

	short := "One row per plant: installed and dependable capacity, technology, fuel and status for a fiscal year"
	long := "The plant-grain view of NEPRA's generation workbook: one row per plant, not twelve.\n\n" +
		"Capacity is INDEPENDENT of the monthly block, in both directions, and the *_state columns are\n" +
		"what carry that. Kotri publishes 174 MW installed / 120 MW dependable while all 26 of its monthly\n" +
		"cells read DELICENSED, so a delicensed plant keeps a real capacity. Conversely FY2017-18's eleven\n" +
		"listed_no_data plants publish NO capacity number, and three FY2020-21 plants publish\n" +
		"\"Export to K.Electric\" in the Installed Capacity column itself. A 0 in any of those cases would\n" +
		"be invented, so the number is absent and installed_mw_state says why.\n\n" +
		"NO CAPACITY FACTOR AND NO UTILISATION IS EMITTED, at this grain or any other. This is the command\n" +
		"where a load factor looks easiest — capacity in MW and generation in GWh sit on one row — and it\n" +
		"is exactly where it would be wrong: the honest denominator is hours-per-month, which this corpus\n" +
		"does not publish anywhere. meta.refusals says so in the output."
	example := "  nepra-pp-cli generation plants --fy 2023-24\n" +
		"  nepra-pp-cli generation plants --fy 2023-24 --technology HYDEL\n" +
		"  nepra-pp-cli generation plants --fy 2023-24 --fuel RLNG --format csv\n" +
		"  nepra-pp-cli generation plants --fy 2020-21 --format jsonl --strict"
	if grain == genGrainPlantMonth {
		short = "Twelve rows per plant: the monthly GWh and utilisation cells for a fiscal year"
		long = "The plant-month view of NEPRA's generation workbook: twelve rows per plant, whatever the\n" +
			"plant's state, so a delicensed or never-reporting plant cannot vanish from the extract.\n\n" +
			"With no --technology or --fuel this emits exactly what `nepra-pp-cli gen --fy <year>` emits, from\n" +
			"the same builder — the two are the same rows reached by two command paths. The filters are the\n" +
			"difference, and they select PLANTS, so a matched plant keeps all twelve of its rows.\n\n" +
			"A blank NEVER becomes a zero: a measured 0.00 GWh stays distinguishable from NOT REPORTED and\n" +
			"from DELICENSED, and every numeric field carries an adjacent *_state column.\n\n" +
			"NO AGGREGATE UTILISATION IS EMITTED. The per-month \"% age\" cell is carried verbatim and is\n" +
			"never summed or averaged: the file's own Sum column proves why, reconciling 118/118 on GWh and\n" +
			"only 3/118 on \"% age\"."
		example = "  nepra-pp-cli generation monthly --fy 2023-24\n" +
			"  nepra-pp-cli generation monthly --fy 2023-24 --technology HYDEL\n" +
			"  nepra-pp-cli generation monthly --fy 2023-24 --fuel COAL --format csv\n" +
			"  nepra-pp-cli generation monthly --fy 2020-21 --format jsonl --strict"
	}

	cmd.Short = short
	cmd.Long = long
	cmd.Example = example
	cmd.Annotations = map[string]string{
		// A pure read: one HTTP GET of one published document.
		"mcp:read-only": "true",
		"pp:happy-args": "--fy=2023-24",
		// 0 success, 1 a --strict completeness failure, 2 a usage refusal,
		// 3 a fiscal year this path does not publish, 5 a fatal parse.
		// There is no 4: this surface needs no credential.
		"pp:typed-exit-codes": "0,1,2,3,5",
		"pp:novel-hand-coded": "true",
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		// 1. Help-only branch. The catalogue is the useful answer to a
		// bare invocation and it costs no request.
		if genHelpOnly(cmd, flagFy, flagFormat, "", flagStrict) &&
			flagTechnology == "" && flagFuel == "" {
			return genPrintCatalogue(cmd, flags)
		}

		// 2. Dry-run, BEFORE any validation. The verify pipeline probes
		// hand-written commands with --dry-run, and cobra runs Args: and
		// MarkFlagRequired before RunE — which is why --fy is validated
		// by hand below and MarkFlagRequired is deliberately not called.
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, cmd.CommandPath())
		}

		// 3. Usage errors.
		if err := validateDataSourceStrategy(flags, "live"); err != nil {
			return usageErr(err)
		}
		format, err := genResolveFormat(flagFormat, flags, cmd.OutOrStdout())
		if err != nil {
			return usageErr(err)
		}
		filter := genPlantFilter{
			Technology: strings.TrimSpace(flagTechnology),
			Fuel:       strings.TrimSpace(flagFuel),
		}
		// A flag that was SET but is blank is a mistake, not match-all:
		// Plant.Technology can legitimately be "", and treating the two
		// the same would silently answer a different question.
		for _, f := range []string{"technology", "fuel"} {
			if cmd.Flags().Changed(f) && strings.TrimSpace(cmd.Flags().Lookup(f).Value.String()) == "" {
				return usageErr(fmt.Errorf("--%s was given but is blank; omit it to select every plant", f))
			}
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
				"were %s and this path published nothing else", fy.Label(), genAsOfDate, genReachableList()))
		}
		if !year.Reachable {
			return notFoundErr(fmt.Errorf("FY%s is not published at this path: %s. Reachable years are %s",
				year.Label, year.Note, genReachableList()))
		}

		// 4. Fetch. One GET of one document. genSheetPath, never
		// fmt.Sprintf and never replacePathParam: six of the seven %20
		// sequences in the template parse as format verbs, and
		// url.PathEscape would double-encode a path that is already
		// percent-encoded.
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

		// 5. Parse, ALWAYS with the caller's non-empty label. That label
		// is the only thing standing between a stale copy and a whole
		// year dated wrongly: the per-year parent .htm is a 9,838-byte
		// frameset that is byte-identical across three sampled years.
		w, err := nepraparse.ParseWorkbook(body, year.Label)
		if err != nil {
			return genClassifyParseError(err, year.Label, len(body))
		}

		// 6. Select, then shape.
		selected, err := genFilterPlants(w, filter)
		if err != nil {
			return usageErr(err)
		}
		var (
			rowCells  [][]string
			payload   any
			header    []string
			metaGrain = grain
		)
		switch grain {
		case genGrainPlant:
			pr := genPlantRows(w, selected, sourceURL)
			payload = pr
			header = genPlantRowHeader
			for _, r := range pr {
				rowCells = append(rowCells, r.Cells())
			}
		default:
			// Reuse genRows unchanged so an unfiltered run is
			// byte-identical to `gen`, then keep only the selected
			// plants' rows. Selection is by plant, so a matched plant
			// keeps all twelve.
			keep := make(map[string]bool, len(selected))
			for _, p := range selected {
				keep[p.Name] = true
			}
			all := genRows(w, sourceURL)
			mr := make([]genRow, 0, len(all))
			for _, r := range all {
				if keep[r.Plant] {
					mr = append(mr, r)
				}
			}
			payload = mr
			header = genRowHeader
			for _, r := range mr {
				rowCells = append(rowCells, r.Cells())
			}
			if filter.Active() {
				metaGrain = genGrainPlantMonthFiltered
			}
		}

		meta := genGrainMeta(w, year, grain, metaGrain, filter, selected,
			len(rowCells), body, sourceURL, c.LastContentType(), genRowsJSON(payload))

		// 7. Rows first, then the assertions, so a truncated pipe loses
		// the commentary and not the data.
		if err := genWrite(cmd, flags, format, header, rowCells, meta, payload); err != nil {
			return err
		}
		short := genReport(cmd, meta, format)
		if len(short) > 0 && flagStrict {
			return fmt.Errorf("%d of %d completeness assertions failed for FY%s; see the COMPLETENESS lines on stderr",
				len(short), len(meta.Assertions), year.Label)
		}
		return nil
	}

	cmd.Flags().StringVar(&flagFy, "fy", "", "Fiscal year as published, e.g. 2023-24. Reachable: "+
		"2017-18 through 2023-24 (FY 2023-24 and 2023-2024 also parse). A year outside that window exits 3 "+
		"rather than returning an empty result, because unpublished and empty are different findings")
	cmd.Flags().StringVar(&flagFormat, "format", "", "Row serialisation: csv, tsv, json or jsonl. "+
		"Default: csv with --csv, tsv with --plain, json with --json/--agent or a piped stdout, else tsv")
	cmd.Flags().StringVar(&flagTechnology, "technology", "", "Keep only plants whose published Technology "+
		"equals this value. Matched EXACTLY on the whole cell (case-insensitive), never as a substring: "+
		"\"THERMAL\" and \"THERMAL- COAL\" are different technologies and the space after the hyphen is "+
		"NEPRA's own. A value this fiscal year does not publish exits 2 and lists what it does publish")
	cmd.Flags().StringVar(&flagFuel, "fuel", "", "Keep only plants whose published Fuel equals this value, "+
		"on the same exact-match rule as --technology. Combined with --technology it is an AND. Note WIND, "+
		"SOLAR and NUCLEAR appear in BOTH vocabularies, so each flag matches its own column only")
	cmd.Flags().BoolVar(&flagStrict, "strict", false, "Exit non-zero when a completeness assertion fails. "+
		"The assertions print to stderr either way; --strict only changes the exit code")
	return cmd
}

// genGrainAssertion is the row-count identity for whichever grain ran.
//
// It exists because genAssertions' own rows_equal_plants_times_12 is gated on
// the literal grain "plant-month" and measured against the WHOLE workbook, so
// neither grain here is covered by it once a filter is active. Following the
// genAssertion contract, a check with nothing to assert against reports
// Asserts:false rather than passing silently.
func genGrainAssertion(grain string, rows, matched int) genAssertion {
	if grain == genGrainPlant {
		return genAssertion{
			Name: "rows_equal_plants_matched", Asserts: true, OK: rows == matched,
			Expected: genIntPtr(matched), Actual: genIntPtr(rows),
			Detail: "at the plant grain every selected plant gets exactly one row, whatever its class",
		}
	}
	return genAssertion{
		Name: "rows_equal_plants_matched_times_12", Asserts: true,
		OK:       rows == matched*genMonthsPerPlant,
		Expected: genIntPtr(matched * genMonthsPerPlant), Actual: genIntPtr(rows),
		Detail: "selection is by plant, so a matched plant keeps all twelve of its months whatever its class",
	}
}

// genGrainMeta assembles the meta block for either grain.
//
// IT IS A SEPARATE FUNCTION SO THE CALL SITE IS TESTABLE, and that is not
// tidiness. A mutation that measured the grain assertion against the whole
// workbook instead of the SELECTION survived the first test pass: the test
// called genGrainAssertion directly with the right arguments and so never
// exercised the wiring, which is the only place that bug can live. This is
// the same shape as the capacity command's `bucket()` defect, where a test
// pinned the implementation detail rather than the semantic.
//
// Meta is built from the UNFILTERED workbook. Handing genBuildMeta a filtered
// copy would compute the percent-then-GWh proof, the Sum identity and the
// census over the subset: on FY2023-24 `--technology Coal` is ONE delicensed
// plant, which leaves the column order unprovable and would fail an assertion
// on an entirely correct answer.
func genGrainMeta(
	w *nepraparse.Workbook,
	year genYear,
	grain, metaGrain string,
	filter genPlantFilter,
	selected []nepraparse.Plant,
	rows int,
	body []byte,
	sourceURL, contentType string,
	content []byte,
) genMeta {
	meta := genBuildMeta(w, year, metaGrain, rows, body, sourceURL, contentType, content)
	meta.FilterTechnology = filter.Technology
	meta.FilterFuel = filter.Fuel
	meta.PlantsMatched = len(selected)
	// The artifact's content hash is over the EMITTED rows, so it is not
	// comparable across different filters. Record the plants the filter
	// excluded rather than leaving skipped at 0, whose own doc says an
	// unexplained non-zero is a reason to distrust the row count.
	meta.Artifact.SkippedRows = len(w.Plants) - len(selected)
	meta.Assertions = append(meta.Assertions, genGrainAssertion(grain, rows, len(selected)))
	return meta
}
