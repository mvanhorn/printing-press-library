// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// DATA ONLY. Every number in this file was MEASURED — either against the
// committed workbook fixtures in internal/cli/testdata (which are byte copies
// of the live sheets: the FY2023-24 fixture decodes to 493,187 bytes with md5
// 044cc6caaa71d0c17502892d6f275f75, identical to the live fetch on
// 2026-09-10) or against a live probe recorded in the comment beside it.
// A year this session could not measure carries NO floor at all rather than a
// zero, because a floor of 0 asserts something.

package cli

// capacityAsOfSurface is one 30-June balance-sheet date and what is known
// about the workbook behind it.
//
// NEPRA publishes plant capacity once per fiscal year as two integer columns
// per plant. There is no in-year capacity series anywhere in the document, so
// a 30-June date is the ONLY date a capacity figure from this source can
// honestly carry.
type capacityAsOfSurface struct {
	// AsOf is the balance-sheet date, always YYYY-06-30.
	AsOf string `json:"as_of"`
	// FiscalYear is the label the workbook's own header band uses.
	FiscalYear string `json:"fiscal_year"`
	// State is "reachable" or "unavailable".
	State string `json:"state"`
	// Reason is set only for an unavailable year and says what was probed.
	Reason string `json:"reason,omitempty"`
	// PlantRowFloor and DecodedByteFloor are MEASURED lower bounds, valid
	// only when FloorsMeasured is true. They are floors, not equalities: a
	// republished sheet may grow.
	PlantRowFloor    int `json:"plant_row_floor,omitempty"`
	DecodedByteFloor int `json:"decoded_byte_floor,omitempty"`
	// FloorsMeasured says the two floors above were MEASURED. When it is
	// false both floor keys are OMITTED entirely rather than emitted as 0,
	// because a floor of 0 asserts something. Every reachable year now has
	// a committed fixture, so it is true for all seven; the field stays
	// because a NEW fiscal year appears with no fixture and must not
	// silently inherit a zero floor.
	FloorsMeasured bool `json:"floors_measured"`
	// Offline reports that the catalogue row itself needs no request.
	Offline bool `json:"offline"`
}

// capacityAsOfCatalogue is every 30-June date this command will discuss.
//
// SEVEN years are reachable, FY2017-18..FY2023-24. FY2016-17, FY2024-25 and
// FY2025-26 are 404s under the generation path and are kept here so that
// "unavailable" is a recorded fact with a reason rather than a missing row.
//
// Every reachable year has a committed fixture under internal/cli/testdata,
// captured live on 2026-09-10, and both floors below were measured from it.
//
// The research's per-year byte-size list (418,887 / 421,591 / 426,282 /
// 455,640 / 490,461 / 493,187 / 516,219) is SORTED ASCENDING, not
// year-ordered, so a positional read misattributes EVERY year. The measured
// year order is 426,282 / 418,887 / 421,591 / 455,640 / 516,219 / 490,461 /
// 493,187 — FY2017-18 is the third value in the sorted list and FY2021-22 is
// the largest file of the seven.
//
// DecodedByteFloor is measured from a plain fetch with Accept: */*. This
// CLI's own client always sends
// `Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8`
// (internal/client/client.go sets it unconditionally; the application/json
// branch below it only fires when Accept is empty, which it never is), and
// Cloudflare then injects a ~361-byte RUM beacon <script> after </html> for
// an HTML accept. MEASURED: FY2023-24 arrives as 493,548 bytes through this
// client against 493,187 through Accept: */*, byte-identical up to that
// injection. So the live body normally EXCEEDS the floor rather than
// equalling it, which is why this is a floor and not an equality.
//
// An earlier version of this comment said the client sends
// `Accept: application/json`. It does not, and the sibling explanation in
// nepra_gen_catalogue.go had it right all along — two files explaining the
// same measured 361 bytes with two mutually exclusive request headers.
var capacityAsOfCatalogue = []capacityAsOfSurface{
	{
		AsOf: "2017-06-30", FiscalYear: "2016-17", State: "unavailable", Offline: true,
		Reason: "the generation workbook path 404s for FY2016-17: NEPRA's Detail of Generation series begins at FY2017-18",
	},
	{
		AsOf: "2018-06-30", FiscalYear: "2017-18", State: "reachable", Offline: true,
		PlantRowFloor: 108, DecodedByteFloor: 426282, FloorsMeasured: true,
	},
	{
		AsOf: "2019-06-30", FiscalYear: "2018-19", State: "reachable", Offline: true,
		PlantRowFloor: 108, DecodedByteFloor: 418887, FloorsMeasured: true,
	},
	{
		AsOf: "2020-06-30", FiscalYear: "2019-20", State: "reachable", Offline: true,
		PlantRowFloor: 108, DecodedByteFloor: 421591, FloorsMeasured: true,
	},
	{
		AsOf: "2021-06-30", FiscalYear: "2020-21", State: "reachable", Offline: true,
		PlantRowFloor: 108, DecodedByteFloor: 455640, FloorsMeasured: true,
	},
	{
		AsOf: "2022-06-30", FiscalYear: "2021-22", State: "reachable", Offline: true,
		PlantRowFloor: 125, DecodedByteFloor: 516219, FloorsMeasured: true,
	},
	{
		AsOf: "2023-06-30", FiscalYear: "2022-23", State: "reachable", Offline: true,
		PlantRowFloor: 130, DecodedByteFloor: 490461, FloorsMeasured: true,
	},
	{
		AsOf: "2024-06-30", FiscalYear: "2023-24", State: "reachable", Offline: true,
		PlantRowFloor: 133, DecodedByteFloor: 493187, FloorsMeasured: true,
	},
	{
		AsOf: "2025-06-30", FiscalYear: "2024-25", State: "unavailable", Offline: true,
		// LIVE-PROBED 2026-09-10 with the product UA: the sheet path returns
		// HTTP 404 with a 9-byte "Not Found" body, while
		// /publications/State of Industry Reports/Detail of Generation/SIR Data 2025.htm
		// returns HTTP 200 — 374 bytes, md5 37c27f4decfbad45c80078a997f13c19,
		// BYTE-IDENTICAL to SIR Data 2024.htm, titled "SIR Data 2024" and
		// framing List%20of%20Companies%20Genenration%20wise%202023-24.htm.
		Reason: "no FY2024-25 workbook exists: the sheet path is a 9-byte HTTP 404 (live-probed 2026-09-10). " +
			"BEWARE THE DECOY: `SIR Data 2025.htm` returns HTTP 200 but is 374 bytes, md5 " +
			"37c27f4decfbad45c80078a997f13c19, byte-identical to `SIR Data 2024.htm`, titled \"SIR Data 2024\", " +
			"and frames the FY2023-24 sheet. A filename-driven fetch would republish FY2023-24 as FY2024-25.",
	},
	{
		AsOf: "2026-06-30", FiscalYear: "2025-26", State: "unavailable", Offline: true,
		Reason: "FY2025-26 has not ended and no workbook is published: the generation path 404s under every naming variant tested",
	},
}

// capacityAsOfByDate looks up one catalogue row.
func capacityAsOfByDate(asOf string) (capacityAsOfSurface, bool) {
	for _, s := range capacityAsOfCatalogue {
		if s.AsOf == asOf {
			return s, true
		}
	}
	return capacityAsOfSurface{}, false
}

// capacityLocalAsOfDates are the only balance-sheet dates the offline path can
// answer: nepraxwalk.ObservedFYs() is ["2017-18","2023-24"] and FY2020-21 is
// deliberately HELD OUT of the crosswalk.
var capacityLocalAsOfDates = []string{"2018-06-30", "2024-06-30"}

// Ledger labels. The two derived labels are computed from the bytes fetched in
// this run; the three unavailable ones are figures NEPRA published that no
// shipped code path in this module can re-derive.
const (
	capacityLedgerWorkbookActive       = "workbook_active"
	capacityLedgerWorkbookAllPublished = "workbook_all_published"
)

// capacityLedgerDefinitions are the prose definitions of the derived rows,
// kept beside the unavailable rows so the ledger reads as one table.
var capacityLedgerDefinitions = map[string]string{
	capacityLedgerWorkbookActive:       "plants whose twelve-month generation block is ordinary published data",
	capacityLedgerWorkbookAllPublished: "every plant row publishing a capacity number, whatever its status",
}

// capacityUnavailableLedger are published national capacity figures that this
// CLI CANNOT MEASURE.
//
// Each carries the figure as a verbatim `published_as` STRING and NO numeric
// field, so no consumer can add a number this CLI never measured into a
// total. This is the discipline internal/nepraper already uses for the
// truncated SAIDI label (RawLabel "19,535." with no Num) and that
// nepraparse.Value's MarshalJSON uses to omit `value` on every non-numeric
// state.
// The two SIR rows quote FY2023-24 stock figures, so they are attached to
// the FY2023-24 report ONLY. Emitting them under a FY2022-23 heading would
// invite a reader to treat a 2023-24 figure as a 2022-23 one, which is the
// same dating error the --as-of grammar refuses.
const capacitySIRLedgerFY = "2023-24"

var capacityUnavailableLedger = []capacityLedgerRow{
	{
		Label:       "sir_2024_cppag_system",
		State:       "unavailable",
		PublishedAs: "42,512 MW",
		Reason: "this figure exists only inside sir2024.pdf and this CLI ships no State-of-Industry-Report table " +
			"extractor: internal/nepraper parses Performance Evaluation Reports only, and its roster is the 10 " +
			"evaluated DISCOs. The string is a citation of what the source states, NOT a measurement this CLI made.",
		Source:  "NEPRA State of Industry Report 2024, pp. 88-89",
		Instead: "`nepra-pp-cli sir 2024` fetches the PDF; no shipped command reads a table out of it",
	},
	{
		Label:       "sir_2024_including_k_electric",
		State:       "unavailable",
		PublishedAs: "45,888 MW",
		Reason: "same artifact and same gap. NEPRA states the scope difference itself, verbatim at SIR 2024 p.88: " +
			"\"These figures do not include KE's own generation and purchases from IPPs.\"",
		Source:  "NEPRA State of Industry Report 2024, pp. 88-89",
		Instead: "`nepra-pp-cli sir 2024`",
	},
	{
		Label: "licence_register_gross",
		// STILL "unavailable", and the distinction is the point: `licence
		// --all` now reads the whole register, but THIS FIGURE remains
		// irreproducible. 47,559.97 is reachable only by making two bad
		// guesses (see Reason), so the ledger must not claim this CLI can
		// recompute it. What IS now available is a different, honest partial
		// total, and Instead says how to get it.
		State: "unavailable",
		// Verbatim, as published. No narration belongs in this field: the
		// human ledger renders it directly as `published as <this>`.
		PublishedAs: "47,559.97 MW over 331 of 335 entities",
		Reason: "THE REGISTER IS NOW READABLE BUT THIS FIGURE IS NOT REPRODUCIBLE. " +
			"`licence --all` reads all 21 key/value register pages and 379 entities, of which 361 publish a " +
			"capacity in a unit that converts to MW. The remaining 18 are the reason there is no headline figure: " +
			"10 publish MWp, a solar PEAK DC rating that is a different physical quantity from an AC megawatt with " +
			"no NEPRA-published conversion; 6 do not parse as <number><unit> — and note those DO carry a " +
			"unit (\"12:00 MW\", \"02:00 MW\", \"3s 6 MW\", \"110 MW Gross ISO\"), it is the NUMBER " +
			"that is unreadable; and 2 publish no capacity key. The probe's 47,559.97 is " +
			"reproducible ONLY by reading \"12:00 MW\" as 12 and \"3s 6 MW\" as 3, and the latter entity's own " +
			"plant-detail row reads 1x16MW + 1x20MW, i.e. 36 MW, so that guess is out by 12x. Separately, one " +
			"entity has its key/value pairs transposed upstream and publishes a technology string in its capacity " +
			"cell.",
		Source: "NEPRA licence register, all 21 pages re-measured 11 Sep 2026",
		Instead: "`nepra-pp-cli licence --all --json` and sum gross_capacity_mw yourself, reading the four " +
			"population counts in meta first: entities_with_parsed_capacity, entities_with_non_mw_unit, " +
			"entities_with_unreadable_capacity and entities_with_no_capacity_key partition the set exactly",
	},
}

// capacityUnavailableLedgerFor returns the unavailable rows that belong in a
// report for this fiscal year.
//
// A figure this build has NEVER READ carries no published_as at all — only
// the reason it is absent. Reusing the FY2023-24 SIR strings under another
// year's heading would be inventing an attribution.
func capacityUnavailableLedgerFor(fy string) []capacityLedgerRow {
	out := make([]capacityLedgerRow, 0, len(capacityUnavailableLedger))
	if fy != capacitySIRLedgerFY {
		out = append(out, capacityLedgerRow{
			Label: "sir_" + fy + "_system",
			State: "unavailable",
			// NO published_as: this build has read no SIR figure for this
			// year, and quoting the FY2023-24 one here would misdate it.
			Reason: "no State-of-Industry-Report system-capacity figure for FY" + fy + " has been read by this build. " +
				"internal/nepraper is a Performance-Evaluation-Report extractor with no SIR table in its registry, " +
				"so there is no code path that could produce one, and the FY" + capacitySIRLedgerFY + " figures " +
				"(42,512 / 45,888 MW) are NOT quoted here because they describe a different year's stock.",
			Source:  "absence of a reading, not a statement that NEPRA published nothing",
			Instead: "`nepra-pp-cli sir <year>` fetches the PDF; no shipped command reads a table out of it",
		})
	}
	for _, r := range capacityUnavailableLedger {
		switch r.Label {
		case "sir_2024_cppag_system", "sir_2024_including_k_electric":
			if fy != capacitySIRLedgerFY {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// capacitySystemBasisCPPAG is the whole-file scope attribution for --by system.
//
// There is NO per-row system column anywhere in the workbook and no system
// enum anywhere in this module. What is honestly derivable is a WHOLE-FILE
// attribution on NEPRA's own record, a per-row export disposition that carries
// zero megawatts because the sentinel occupies the capacity column, and a
// declared absence for K-Electric's own fleet.
const capacitySystemBasisCPPAG = "WHOLE-FILE attribution, not a per-row column. NEPRA SIR 2024 p.88, verbatim: " +
	"\"These figures do not include KE's own generation and purchases from IPPs.\" Every row in this workbook is " +
	"CPPA-G/XWDISCO-dispatched."

// capacitySystemBasisExport explains why the export bucket can never carry MW.
const capacitySystemBasisExport = "rows whose Installed Capacity (MW) cell holds the literal sentinel " +
	"\"Export to K.Electric\". Because the sentinel OCCUPIES the capacity column those rows publish no megawatt " +
	"figure at all, so this bucket can never carry MW: `measured` is false, never a zero. 0 such rows in " +
	"FY2023-24; 3 in FY2020-21 (Tenaga Generasi, Hydrochina Dawood HDPPL, Zephyr Power - all WIND)."

// Bucket bases for the four status members.
const (
	capacityBasisActive = "the twelve-month generation block is ordinary published data"

	capacityBasisDelicensed = "all 26 monthly cells read DELICENSED in a single <td colspan=26>. The plant KEEPS a " +
		"valid published capacity: Kotri Power Station publishes 174 MW installed / 120 MW dependable while " +
		"generating nothing."

	capacityBasisDecommissioned = "all 26 monthly cells read DECOMMISSIONED. NEPRA publishes no legend " +
		"distinguishing this from DELICENSED; the two are kept apart because the source keeps them apart, not " +
		"because this CLI knows the difference."

	capacityBasisListedNoData = "capacity published, NO status token and NO monthly data: an EMPTY <td colspan=26> " +
		"(FY2023-24) or 26 individual blanks (FY2017-18, FY2020-21). Whether this means idle, delicensed or an " +
		"omission is not stated anywhere in the source, so operating status is UNKNOWN and these megawatts are in " +
		"NEITHER the operating nor the non-operating total."

	capacityBasisUndetermined = "the monthly block publishes NO measurement and is NEITHER a recognised status " +
		"sentinel NOR a plain blank: the source put text there that this build does not model, and the verbatim " +
		"text is in unmodelled_cell_text. MEASURED in FY2022-23, where Reshma Power Generation and Gulf Powergen " +
		"each carry `<td colspan=26>DELICENSE</td>` — the upstream typo, no trailing D — in the SAME file that " +
		"spells the other eleven \"DELICENSED\". Calling these plants active would present 181.00 MW of " +
		"delicensed capacity as generating; calling them delicensed would be this CLI deciding what NEPRA meant. " +
		"So they are neither, and their megawatts are in no operating total."

	capacityBasisReportingOrSilent = "the crosswalk records one block_status per plant-year over the vocabulary " +
		"{numeric, delicensed, decommissioned} and no per-month cell states, so a plant that reported twelve months " +
		"and a plant whose whole block is blank are INDISTINGUISHABLE offline. This bucket therefore mixes them."
)

// capacityLocalEnumGapReason is the mandatory declaration on the offline path.
const capacityLocalEnumGapReason = "internal/nepraxwalk/crosswalk.json stores one block_status per observation over " +
	"the vocabulary {numeric, delicensed, decommissioned} and does NOT store per-month cell states, so a plant whose " +
	"entire monthly block is blank is indistinguishable from a fully-reporting plant. Reshma Power Generation and " +
	"Gulf Powergen both carry block_status \"numeric\" for FY2023-24, so their 181.00 MW sits inside " +
	"reporting_or_silent and cannot be split out offline. Pass --data-source live for the four-member enum."

// capacityLocalDependableGap says why the offline path has no dependable
// column at all. It emits NO dependable key rather than a zero.
const capacityLocalDependableGap = "crosswalk.json's Observation carries installed_capacity_mw_raw only — there is " +
	"no dependable-capacity field — so the offline path omits dependable entirely. An absent column is not zero " +
	"megawatts."
