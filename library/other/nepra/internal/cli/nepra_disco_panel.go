// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
// See .printing-press-patches/nepra-disco-panel.json.
//
// Wire types and pure builders for the `disco` reliability panel. NOTHING from
// internal/nepraper is marshalled directly here except nepraper.Value: none of
// Observation, Provenance, Conflict or Key carries a json tag, so the encoder
// would emit PascalCase keys ("ReportFY", "Prov", "AbsDiff") into a codebase
// whose every other envelope is snake_case. nepraper.Value IS passed through
// untouched, because its own MarshalJSON is what stops a non-numeric kind
// serialising as `"value":0` — the difference between "NEPRA published 0.00"
// and "NEPRA published nothing".

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraper"
)

// discoCitation is where one figure came from. It carries the CAPTION and the
// PDF PAGE, not only the table label, because the label is not a key: SAIFI is
// "TABLE 5", "Table 5", "Table 14", "Table 05" and "Table 05+06" across the
// seven reports, and its five-year twin is "TABLE 17", "TABLE 15", "Table 15",
// "Table 24" and "Table 17".
type discoCitation struct {
	ReportFY     string  `json:"report_fy"`
	Table        string  `json:"table,omitempty"`
	Caption      string  `json:"caption,omitempty"`
	Page         int     `json:"page"`
	RowY         float64 `json:"row_y,omitempty"`
	ColumnHeader string  `json:"column_header,omitempty"`
	// ColumnIndex is -1 for a table whose value column could not be
	// identified, which is the parser's sentinel and not column 0.
	ColumnIndex int    `json:"column_index"`
	Variant     string `json:"variant,omitempty"`
}

// discoExtra is a published column that is neither the value, the target nor
// the breach. It is kept so that no printed figure is silently discarded: the
// whole FY2014-15 complaints table lands here, because three of its columns
// match the caption equally well and the parser refuses to pick one.
type discoExtra struct {
	Header string         `json:"header"`
	Value  nepraper.Value `json:"value"`
}

// discoArtifactNote is a curated, reproducible corpus inconsistency joined onto
// a row by (period, entity, metric). Direction says whether this build can name
// the erroneous side; "unknowable" means it cannot.
type discoArtifactNote struct {
	Description string  `json:"description"`
	Ratio       float64 `json:"ratio"`
	Direction   string  `json:"direction"`
}

// discoConflictSide is one half of a same-key disagreement, with its own
// provenance. Both sides always reach the caller.
//
// Value is the quantity the enclosing conflict's Field names — the reported
// figure for "value", the allowed-in-tariff figure for "target", the verdict
// for "breach". Printing the reported figure for a TARGET or BREACH conflict
// would show two numbers that never disagreed and hide the two that did.
type discoConflictSide struct {
	Value       nepraper.Value `json:"value"`
	Table       string         `json:"table,omitempty"`
	Caption     string         `json:"caption,omitempty"`
	Page        int            `json:"page"`
	RowY        float64        `json:"row_y,omitempty"`
	ColumnIndex int            `json:"column_index"`
	Variant     string         `json:"variant,omitempty"`
}

// discoConflict is two published figures for one key that do not agree.
//
// There is deliberately no resolved value, no preferred side and no average.
// For the FY2024-25 MEPCO pair each figure is internally consistent with its
// own chart and the document holds no tie-breaker.
type discoConflict struct {
	PeriodFY string `json:"period_fy"`
	Entity   string `json:"entity"`
	Metric   string `json:"metric"`
	// Field names WHICH quantity disagreed: value, target or breach. The three
	// are different quantities and are never compared with each other.
	Field string `json:"field"`
	Class string `json:"class"`
	// Ratio is A/B and is null when undefined (a zero denominator, or a side
	// that is not numeric). It is never math.Inf: Go's encoder REFUSES a
	// non-finite float, so one would turn the whole envelope into an empty
	// document and report a real conflict as no conflict at all.
	Ratio                *float64          `json:"ratio"`
	RatioUndefinedReason string            `json:"ratio_undefined_reason,omitempty"`
	AbsDiff              float64           `json:"abs_diff"`
	A                    discoConflictSide `json:"a"`
	B                    discoConflictSide `json:"b"`
	Resolution           string            `json:"resolution"`
}

// discoRow is one figure as published. actual, target and breach are three
// separately typed quantities and are never merged: FY2018-19's T&D header is
// "Actual Reported (%) | Allowed in Tariff (%) | Breach of Target (%)", and a
// reader that treats the target as a value manufactures a 3547-vs-14 conflict.
type discoRow struct {
	ReportFY    string         `json:"report_fy"`
	PeriodFY    string         `json:"period_fy"`
	Entity      string         `json:"entity"`
	Metric      string         `json:"metric"`
	MetricTitle string         `json:"metric_title,omitempty"`
	Variant     string         `json:"variant,omitempty"`
	Actual      nepraper.Value `json:"actual"`
	// Target is Absent BY CONSTRUCTION for every five-year comparison column:
	// that is the table's shape, not a missing figure.
	Target nepraper.Value `json:"target"`
	// Breach is numeric in the early reports and a qualitative label
	// ("Far Away", "Near to Limit", "Away", "Within Limit") from FY2020-21 on.
	// A label is never coerced to 0.
	Breach                   nepraper.Value      `json:"breach"`
	Extras                   []discoExtra        `json:"extras"`
	Citation                 discoCitation       `json:"citation"`
	Restated                 bool                `json:"restated"`
	PeriodReportAvailability string              `json:"period_report_availability"`
	Secondhand               bool                `json:"secondhand"`
	SecondhandReason         string              `json:"secondhand_reason,omitempty"`
	Conflicted               bool                `json:"conflicted"`
	ConflictedWith           []discoConflictSide `json:"conflicted_with,omitempty"`
	KnownArtifact            *discoArtifactNote  `json:"known_artifact,omitempty"`
}

// discoTableStat describes one table the panel read.
type discoTableStat struct {
	Table              string   `json:"table"`
	Caption            string   `json:"caption"`
	Page               int      `json:"page"`
	Metric             string   `json:"metric"`
	Variant            string   `json:"variant"`
	Rows               int      `json:"rows"`
	Roster             []string `json:"roster"`
	RosterAnomaly      string   `json:"roster_anomaly,omitempty"`
	HasWeightedAverage bool     `json:"has_weighted_average"`
	// ValueColumn is read through Table.ValueColumnIndex, never off the bare
	// field: a zero-valued Table would otherwise answer "column 0" where the
	// sentinel for unknown is -1.
	ValueColumn           int    `json:"value_column"`
	ValueColumnIdentified bool   `json:"value_column_identified"`
	ValueColumnBasis      string `json:"value_column_basis"`
}

// discoDocumentStat is the fetched document against the measured corpus record.
// The *_measured fields are the byte and page counts this build recorded when
// the corpus was surveyed; the bare fields are what THIS fetch produced.
type discoDocumentStat struct {
	Pages         int    `json:"pages"`
	PagesMeasured int    `json:"pages_measured"`
	Bytes         int    `json:"bytes"`
	BytesMeasured int    `json:"bytes_measured"`
	Creator       string `json:"creator"`
	Producer      string `json:"producer"`
	CharCount     int    `json:"char_count"`
	LowTextPages  []int  `json:"low_text_pages"`
	Note          string `json:"note,omitempty"`
}

// discoCompleteness is the panel's assertion record. bytes_match and
// pages_match are POINTERS so that "not asserted" is null rather than false:
// a local --pdf of an unrecorded year has nothing to assert against, and
// reporting that as a failed match would be a fabricated finding.
type discoCompleteness struct {
	EntitiesExpected     int      `json:"entities_expected"`
	EntitiesSeen         int      `json:"entities_seen"`
	EntitiesAbsent       []string `json:"entities_absent"`
	EntitiesBeyondRoster []string `json:"entities_beyond_roster"`
	RosterAssertion      string   `json:"roster_assertion"`
	BytesMatch           *bool    `json:"bytes_match"`
	PagesMatch           *bool    `json:"pages_match"`
	AssertionsFailed     int      `json:"assertions_failed"`
	Failures             []string `json:"failures"`
}

// discoExclusion records how the report in hand treated an entity NEPRA does
// not evaluate. Kind is excluded | present_in_report | unattested — three
// materially different states that used to be one empty row.
type discoExclusion struct {
	Entity     string `json:"entity"`
	Kind       string `json:"kind"`
	StatedInFY string `json:"stated_in_fy,omitempty"`
	Source     string `json:"source,omitempty"`
	Reason     string `json:"reason"`
}

type discoNote struct {
	Page  int    `json:"page"`
	Table string `json:"table,omitempty"`
	Text  string `json:"text"`
}

type discoUnverified struct {
	PeriodFY         string         `json:"period_fy"`
	Metric           string         `json:"metric"`
	EntityAttributed bool           `json:"entity_attributed"`
	Value            nepraper.Value `json:"value"`
	Source           string         `json:"source"`
}

type discoSourceEntry struct {
	ReportFY     string `json:"report_fy"`
	Availability string `json:"availability"`
	URL          string `json:"url"`
	// Bytes and Pages are 0 for the unavailable year because nothing was ever
	// measured there. They are NOT a claim that the document is empty.
	Bytes        int    `json:"bytes"`
	Pages        int    `json:"pages"`
	Creator      string `json:"creator"`
	Producer     string `json:"producer,omitempty"`
	HasTextLayer bool   `json:"has_text_layer"`
	LowTextPages []int  `json:"low_text_pages"`
	Note         string `json:"note,omitempty"`
}

type discoMetricEntry struct {
	Token      string `json:"token"`
	Key        string `json:"key"`
	Title      string `json:"title"`
	Selectable bool   `json:"selectable"`
}

type discoUnrecordedYear struct {
	Label  string `json:"label"`
	Bytes  int    `json:"bytes"`
	Status string `json:"status"`
}

type discoCatalogue struct {
	AsOfDate         string                `json:"as_of_date"`
	ReportYears      []discoSourceEntry    `json:"report_years"`
	RosterEvaluated  []string              `json:"roster_evaluated"`
	ExcludedEntities []discoExclusion      `json:"excluded_entities"`
	Metrics          []discoMetricEntry    `json:"metrics"`
	Variants         []string              `json:"variants"`
	RefusedVariants  []discoRefusedVariant `json:"refused_variants"`
	UnrecordedYears  []discoUnrecordedYear `json:"reachable_but_unrecorded_report_years"`
	KnownArtifacts   []discoKnownArtifact  `json:"known_artifacts"`
	KnownUnverified  []discoUnverified     `json:"known_unverified"`
	RequestMade      bool                  `json:"request_made"`
}

type discoRefusedVariant struct {
	Variant string `json:"variant"`
	Reason  string `json:"reason"`
}

type discoKnownArtifact struct {
	PeriodFY    string  `json:"period_fy"`
	Entity      string  `json:"entity"`
	Metric      string  `json:"metric"`
	Description string  `json:"description"`
	Ratio       float64 `json:"ratio"`
	Direction   string  `json:"direction"`
}

type discoMeta struct {
	// Source is "live" or "file". RequestMade is false for every refusal that
	// is decided from the recorded corpus alone.
	Source                     string `json:"source"`
	RequestMade                bool   `json:"request_made"`
	ReportFY                   string `json:"report_fy"`
	ReportFYDefaulted          bool   `json:"report_fy_defaulted"`
	DetectedFY                 string `json:"detected_fy,omitempty"`
	DetectedFYMatchesRequested *bool  `json:"detected_fy_matches_requested"`
	Metric                     string `json:"metric"`
	MetricTitle                string `json:"metric_title,omitempty"`
	VariantFilter              string `json:"variant_filter"`
	PeriodFilter               string `json:"period_filter"`
	EntityFilter               string `json:"entity_filter"`
	TargetNote                 string `json:"target_note"`
	// Availability travels as a typed Value so an unobtainable year flows
	// through the same channel as a missing cell instead of becoming a zero.
	Availability     nepraper.Value   `json:"availability"`
	RosterEvaluated  []string         `json:"roster_evaluated"`
	ExcludedEntities []discoExclusion `json:"excluded_entities"`
	// Document is NULL when no document was obtained. A zero-valued block here
	// would read as "0 bytes over 0 pages", a measurement nobody took.
	Document                      *discoDocumentStat `json:"document"`
	Artifact                      *nepraArtifact     `json:"artifact,omitempty"`
	TablesRead                    []discoTableStat   `json:"tables_read"`
	Completeness                  discoCompleteness  `json:"completeness"`
	Conflicts                     []discoConflict    `json:"conflicts"`
	ConflictsInReport             int                `json:"conflicts_in_report"`
	ConflictsUndocumented         int                `json:"conflicts_undocumented"`
	ConflictsUndocumentedInReport int                `json:"conflicts_undocumented_in_report"`
	RowsBeforeLimit               int                `json:"rows_before_limit"`
	Truncated                     bool               `json:"truncated"`
	WeightedAverageScope          string             `json:"weighted_average_scope"`
	Notes                         []discoNote        `json:"notes"`
	ParserNotesCount              int                `json:"parser_notes_count"`
	KnownArtifactsMatched         int                `json:"known_artifacts_matched"`
}

// discoPayload is the panel envelope. weighted_average is a SEPARATE key from
// results: the "W. Av:" summary row is not an eleventh DISCO and must never be
// reachable by iterating results.
type discoPayload struct {
	Meta            discoMeta  `json:"meta"`
	Results         []discoRow `json:"results"`
	WeightedAverage []discoRow `json:"weighted_average"`
}

type discoUnavailableBlock struct {
	Kind            string `json:"kind"`
	Reason          string `json:"reason"`
	URL             string `json:"url"`
	SecondHandRoute string `json:"second_hand_route,omitempty"`
}

type discoRefusalPayload struct {
	Meta        discoMeta             `json:"meta"`
	Unavailable discoUnavailableBlock `json:"unavailable"`
	Results     []discoRow            `json:"results"`
}

// ---------------------------------------------------------------------------
// Enum resolution
// ---------------------------------------------------------------------------

// discoMetricToken maps the command's stable flag token to a nepraper metric.
// The TOKEN is the contract, not the nepraper key: a raw key such as
// `td_losses` is refused so the enum keeps meaning something.
type discoMetricToken struct {
	Token  string
	Metric nepraper.Metric
}

var discoMetricTokens = []discoMetricToken{
	{"tnd", nepraper.MetricTDLosses},
	{"recovery", nepraper.MetricRecovery},
	{"saifi", nepraper.MetricSAIFI},
	{"saidi", nepraper.MetricSAIDI},
	{"complaints", nepraper.MetricConsumerComplaints},
	{"safety", nepraper.MetricSafety},
}

const discoMetricAll = "all"

func discoMetricTokenList() []string {
	out := make([]string, 0, len(discoMetricTokens)+1)
	for _, m := range discoMetricTokens {
		out = append(out, m.Token)
	}
	return append(out, discoMetricAll)
}

// discoResolveMetric maps a flag token to a metric. The bool reports the `all`
// selector, which applies NO metric filter.
func discoResolveMetric(raw string) (nepraper.Metric, bool, error) {
	want := strings.ToLower(strings.TrimSpace(raw))
	if want == discoMetricAll {
		return "", true, nil
	}
	for _, m := range discoMetricTokens {
		if m.Token == want {
			return m.Metric, false, nil
		}
	}
	// Name the seven accepted tokens, and say plainly that a raw nepraper key
	// is not one of them so a caller who read the package docs is not left
	// guessing why `td_losses` failed.
	return "", false, fmt.Errorf(
		"unknown --metric %q; accepted tokens are %s.\n"+
			"These are flag tokens, not internal metric keys: pass --metric tnd, not --metric td_losses.\n"+
			"The other seven parameters NEPRA publishes in a PER (%s) have no token of their own and are\n"+
			"reachable only through --metric all, which applies no metric filter.",
		raw, strings.Join(discoMetricTokenList(), ", "),
		strings.Join(discoUntokenedMetrics(), ", "))
}

// discoUntokenedMetrics lists the nepraper metrics this command has no flag
// token for. Declaring them is the honest alternative to pretending the six
// tokens are everything a PER contains.
func discoUntokenedMetrics() []string {
	tokened := map[nepraper.Metric]bool{}
	for _, m := range discoMetricTokens {
		tokened[m.Metric] = true
	}
	var out []string
	for _, m := range nepraper.Metrics() {
		if !tokened[m] && m != nepraper.MetricUnknown {
			out = append(out, string(m))
		}
	}
	return out
}

var discoVariantTokens = map[string]string{
	"headline":   nepraper.VariantHeadline,
	"comparison": nepraper.VariantComparison,
	"with-lt":    nepraper.VariantWithLT,
	"without-lt": nepraper.VariantWithoutLT,
}

// discoChartVariantRefusal is why there is no --variant chart, with the
// measured consequence of pretending otherwise.
//
// The two TABLE rows below were re-measured from the live FY2024-25 PER by this
// build. The two FIGURE rows are the survey probe's readings and are attributed
// to it, because this parser cannot read a figure at all — which is the point.
const discoChartVariantRefusal = "--variant chart cannot be built from what ships, so it is refused rather than faked.\n" +
	"internal/nepraper assigns exactly four variants (headline, five_year_comparison, with_lt_interruptions,\n" +
	"without_lt_interruptions), and nepraper.Span keeps only text and geometry — it discards the font size that\n" +
	"the underlying PDF reader does expose, so a chart data label cannot be told apart from a table cell.\n" +
	"The consequence is measured: FY2024-25 carries FOUR different value sets for the same\n" +
	"IESCO/GEPCO/LESCO/K-Electric SAIFI cells — Table 05 (15.39, 49.49, 28.16, 68.46) and Table 17\n" +
	"(15.39, 49.49, 28.61, 68.64), both re-measured here from the live PDF, alongside Figure 08\n" +
	"(15.31, 49.41, 28.16, 68.64) and Figure 20 (15.31, 49.41, 28.61, 68.64) as read off the charts by the\n" +
	"corpus survey. Emitting a `chart` variant would mean GUESSING which spans belonged to the chart."

func discoVariantTokenList() []string {
	out := make([]string, 0, len(discoVariantTokens))
	for k := range discoVariantTokens {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// discoResolveVariant maps a flag token to a nepraper variant label. An empty
// token applies no filter.
func discoResolveVariant(raw string) (string, error) {
	want := strings.ToLower(strings.TrimSpace(raw))
	if want == "" {
		return "", nil
	}
	if want == "chart" {
		return "", fmt.Errorf("%s", discoChartVariantRefusal)
	}
	if v, ok := discoVariantTokens[want]; ok {
		return v, nil
	}
	return "", fmt.Errorf("unknown --variant %q; accepted tokens are %s",
		raw, strings.Join(discoVariantTokenList(), ", "))
}

// discoDefaultFY is the newest report year this build records as published.
// Sources() is sorted oldest-first, so the last published record wins. It is
// DERIVED, never hardcoded, so adding a report year moves the default with it.
func discoDefaultFY() string {
	fy := ""
	for _, s := range nepraper.Sources() {
		if s.Availability == nepraper.AvailabilityPublished {
			fy = s.FY
		}
	}
	return fy
}

func discoRecordedFYs() []string {
	var out []string
	for _, s := range nepraper.Sources() {
		out = append(out, s.FY)
	}
	return out
}

// discoReachableButUnrecorded are PERs the corpus survey reached (HTTP 200 with
// a Content-Length) but never downloaded, extracted or parsed. They are named
// so a caller learns the document EXISTS upstream while this build has no
// measured byte or page count to assert against — the opposite of claiming
// NEPRA published nothing.
var discoReachableButUnrecorded = []discoUnrecordedYear{
	{Label: "FY2013-14", Bytes: 3134331, Status: "reachable upstream (HTTP 200 + Content-Length seen by the corpus survey); never downloaded, extracted or parsed by this build"},
	{Label: "FY2015-16", Bytes: 1911376, Status: "reachable upstream (HTTP 200 + Content-Length seen by the corpus survey); never downloaded, extracted or parsed by this build"},
	{Label: "FY2016-17 (filename says '(FFinal)')", Bytes: 1765126, Status: "reachable upstream (HTTP 200 + Content-Length seen by the corpus survey); never downloaded, extracted or parsed by this build"},
	{Label: "FY2017-18 (filename says '(Final)')", Bytes: 1708069, Status: "reachable upstream (HTTP 200 + Content-Length seen by the corpus survey); never downloaded, extracted or parsed by this build"},
	{Label: "undated 'Performance Evaluation Report of DISCOs and K-Electric.pdf'", Bytes: 7929032, Status: "reachable upstream; carries no fiscal year in its filename, so it cannot be keyed by report year"},
	{Label: "Jul-Dec 2024 semiannual", Bytes: 4893527, Status: "reachable upstream; a SEMIANNUAL report, so it would collide with any annual fiscal-year key"},
}

func discoUnrecordedYearSummary() string {
	var labels []string
	for _, y := range discoReachableButUnrecorded {
		labels = append(labels, y.Label)
	}
	return strings.Join(labels, "; ")
}

// discoTargetNote states a limitation of the backing parser rather than hiding
// it: Provenance records the header of the VALUE column only, so a row cannot
// quote the target column's own wording.
const discoTargetNote = "NEPRA captions the target column 'Allowed in Tariff (%)' in the T&D loss tables and " +
	"'Target set by NEPRA' / 'Target by NEPRA (Min)' in the SAIFI/SAIDI tables. internal/nepraper records " +
	"the header of the VALUE column only (Provenance.ColumnHeader), so a row cannot cite the target column's " +
	"own wording verbatim. The target FIGURE is exact; only its header text is unavailable."

const discoUnknowableResolution = "none: each figure is internally consistent with its own chart and the document contains no tie-breaker"

func discoConflictResolution(class nepraper.ConflictClass) string {
	switch class {
	case nepraper.ConflictRounding:
		return "the document accounts for this: one figure printed to two precisions"
	case nepraper.ConflictDeclaredVariant:
		return "the document accounts for this: it declares two different definitions of the metric"
	case nepraper.ConflictKindMismatch:
		return "none: the two sides are not both numeric, so they cannot be compared as numbers"
	default:
		return discoUnknowableResolution
	}
}

// ---------------------------------------------------------------------------
// Row construction
// ---------------------------------------------------------------------------

type discoFilters struct {
	Metric    nepraper.Metric
	AllMetric bool
	Variant   string
	PeriodFY  string
	Entity    nepraper.Entity
}

func (f discoFilters) matches(o nepraper.Observation) bool {
	if !f.AllMetric && o.Metric != f.Metric {
		return false
	}
	if f.Variant != "" && o.Prov.Variant != f.Variant {
		return false
	}
	if f.PeriodFY != "" && o.PeriodFY != f.PeriodFY {
		return false
	}
	if f.Entity != "" && o.Entity != f.Entity {
		return false
	}
	return true
}

// discoObservationRow copies one Observation into the wire shape. It NEVER
// merges two observations for one key: two rows for one key is exactly what
// this command exists to show.
func discoObservationRow(o nepraper.Observation) discoRow {
	row := discoRow{
		ReportFY:    o.ReportFY,
		PeriodFY:    o.PeriodFY,
		Entity:      string(o.Entity),
		Metric:      string(o.Metric),
		MetricTitle: nepraper.MetricTitles[o.Metric],
		Variant:     o.Prov.Variant,
		Actual:      o.Value,
		Target:      o.Target,
		Breach:      o.Breach,
		Extras:      make([]discoExtra, 0, len(o.Extras)),
		Citation: discoCitation{
			ReportFY:     o.Prov.ReportFY,
			Table:        o.Prov.TableLabel,
			Caption:      o.Prov.TableCaption,
			Page:         o.Prov.Page,
			RowY:         o.Prov.RowY,
			ColumnHeader: o.Prov.ColumnHeader,
			ColumnIndex:  o.Prov.ColumnIndex,
			Variant:      o.Prov.Variant,
		},
	}
	for _, x := range o.Extras {
		row.Extras = append(row.Extras, discoExtra{Header: x.Header, Value: x.Value})
	}
	// Restated says the figure describes a year OTHER than the report it was
	// printed in. Secondhand is the stronger claim: the year's OWN report is
	// not obtainable by this build, so the restatement is the only route to it.
	// Deriving secondhand from period != report alone would wrongly tag
	// FY2020-21 — whose own PER is published and parseable — as secondhand.
	row.Restated = o.PeriodFY != "" && o.ReportFY != "" && o.PeriodFY != o.ReportFY
	avail := nepraper.AvailabilityUnknown
	if s, ok := nepraper.SourceFor(o.PeriodFY); ok {
		avail = s.Availability
	}
	row.PeriodReportAvailability = avail.String()
	if row.Restated && avail != nepraper.AvailabilityPublished {
		row.Secondhand = true
		row.SecondhandReason = nepraper.AvailabilityFor(o.PeriodFY).Reason
	}
	return row
}

// discoJoinKnownArtifacts attaches the curated corpus inconsistency for a row's
// key, when one is recorded. This is how a single-report comparison row can
// surface the cross-report 39733-vs-39.733 corruption without a second fetch —
// and it is labelled "inferred", because that is what the record says.
func discoJoinKnownArtifacts(rows []discoRow) int {
	byKey := map[nepraper.Key]nepraper.KnownArtifact{}
	for _, a := range nepraper.KnownArtifacts() {
		byKey[a.Key] = a
	}
	matched := 0
	for i := range rows {
		k := nepraper.Key{
			PeriodFY: rows[i].PeriodFY,
			Entity:   nepraper.Entity(rows[i].Entity),
			Metric:   nepraper.Metric(rows[i].Metric),
		}
		if a, ok := byKey[k]; ok {
			rows[i].KnownArtifact = &discoArtifactNote{
				Description: a.Description, Ratio: a.Ratio, Direction: a.Direction,
			}
			matched++
		}
	}
	return matched
}

// discoConflictSideOf projects the quantity named by field, so a target or
// breach conflict shows the figures that disagreed rather than the reported
// value, which may be identical on both sides.
func discoConflictSideOf(o nepraper.Observation, field string) discoConflictSide {
	v := o.Value
	switch field {
	case "target":
		v = o.Target
	case "breach":
		v = o.Breach
	}
	return discoConflictSide{
		Value:       v,
		Table:       o.Prov.TableLabel,
		Caption:     o.Prov.TableCaption,
		Page:        o.Prov.Page,
		RowY:        o.Prov.RowY,
		ColumnIndex: o.Prov.ColumnIndex,
		Variant:     o.Prov.Variant,
	}
}

// discoBuildConflicts converts the report's conflicts to the wire shape. The
// conflict pass runs over the WHOLE report before any variant filter, so a
// panel narrowed to one variant still tells the caller that the other table
// disagrees. Metric, period and entity filters ARE applied, because those are
// the population the caller asked about; the variant filter deliberately is
// not, since dropping it is how a contradiction gets hidden.
func discoBuildConflicts(all []nepraper.Conflict, f discoFilters) []discoConflict {
	out := make([]discoConflict, 0, len(all))
	for _, c := range all {
		if !f.AllMetric && c.Key.Metric != f.Metric {
			continue
		}
		if f.PeriodFY != "" && c.Key.PeriodFY != f.PeriodFY {
			continue
		}
		if f.Entity != "" && c.Key.Entity != f.Entity {
			continue
		}
		dc := discoConflict{
			PeriodFY:   c.Key.PeriodFY,
			Entity:     string(c.Key.Entity),
			Metric:     string(c.Key.Metric),
			Field:      c.Field,
			Class:      c.Class.String(),
			AbsDiff:    c.AbsDiff,
			A:          discoConflictSideOf(c.A, c.Field),
			B:          discoConflictSideOf(c.B, c.Field),
			Resolution: discoConflictResolution(c.Class),
		}
		if r, ok := c.RatioValue(); ok {
			v := r
			dc.Ratio = &v
		} else {
			dc.RatioUndefinedReason = "A/B is undefined here: the denominator is zero or a side is not numeric. abs_diff still carries the magnitude."
		}
		out = append(out, dc)
	}
	return out
}

// discoMarkConflicted flags every row whose key appears in the conflict set and
// attaches the sides this panel's filters did not emit, so a narrowed view can
// never present one figure as the report's answer.
func discoMarkConflicted(rows []discoRow, conflicts []discoConflict) {
	type key struct{ p, e, m string }
	sides := map[key][]discoConflictSide{}
	for _, c := range conflicts {
		k := key{c.PeriodFY, c.Entity, c.Metric}
		sides[k] = append(sides[k], c.A, c.B)
	}
	for i := range rows {
		k := key{rows[i].PeriodFY, rows[i].Entity, rows[i].Metric}
		cand, ok := sides[k]
		if !ok {
			continue
		}
		rows[i].Conflicted = true
		seen := map[string]bool{}
		for _, s := range cand {
			// Skip the side this row already IS, matched on PROVENANCE
			// identity (table, page, variant, row baseline, column) rather
			// than on the printed text: a target or breach conflict can carry
			// the same reported figure on both sides, and matching on the
			// value would then drop the row's counterpart as a duplicate.
			if s.Table == rows[i].Citation.Table && s.Page == rows[i].Citation.Page &&
				s.Variant == rows[i].Citation.Variant && s.RowY == rows[i].Citation.RowY &&
				s.ColumnIndex == rows[i].Citation.ColumnIndex {
				continue
			}
			sig := fmt.Sprintf("%s|%d|%s|%g|%d|%s", s.Table, s.Page, s.Variant, s.RowY, s.ColumnIndex, s.Value.Raw)
			if seen[sig] {
				continue
			}
			seen[sig] = true
			rows[i].ConflictedWith = append(rows[i].ConflictedWith, s)
		}
	}
}

// discoTableStats describes the tables that carry the selected metric. The
// variant filter is applied here too, so tables_read matches what was read.
func discoTableStats(rep *nepraper.Report, f discoFilters) []discoTableStat {
	out := make([]discoTableStat, 0, len(rep.Tables))
	roster := map[nepraper.Entity]bool{}
	for _, e := range nepraper.Roster() {
		roster[e] = true
	}
	for _, t := range rep.Tables {
		if !f.AllMetric && t.Metric != f.Metric {
			continue
		}
		if f.Variant != "" && t.Variant != f.Variant {
			continue
		}
		vc, vok := t.ValueColumnIndex()
		st := discoTableStat{
			Table: t.Label, Caption: t.Caption, Page: t.Page,
			Metric: string(t.Metric), Variant: t.Variant, Rows: t.RowCount,
			Roster: make([]string, 0, len(t.Roster)), HasWeightedAverage: t.HasWAvg,
			ValueColumn: vc, ValueColumnIdentified: vok, ValueColumnBasis: t.ValueColumnBasis,
		}
		var beyond, missing []string
		seen := map[nepraper.Entity]bool{}
		for _, e := range t.Roster {
			st.Roster = append(st.Roster, string(e))
			seen[e] = true
			if !roster[e] {
				beyond = append(beyond, string(e))
			}
		}
		for _, e := range nepraper.Roster() {
			if !seen[e] {
				missing = append(missing, string(e))
			}
		}
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, fmt.Sprintf("%d of the 10 evaluated entities absent from this table: %s",
				len(missing), strings.Join(missing, ", ")))
		}
		if len(beyond) > 0 {
			parts = append(parts, fmt.Sprintf("lists %s, which is not one of the ten NEPRA evaluates",
				strings.Join(beyond, ", ")))
		}
		st.RosterAnomaly = strings.Join(parts, "; ")
		out = append(out, st)
	}
	return out
}

// discoCompletenessOf builds the assertion record over the emitted rows.
func discoCompletenessOf(rows []discoRow, f discoFilters, truncated bool) discoCompleteness {
	c := discoCompleteness{
		EntitiesExpected:     len(nepraper.Roster()),
		EntitiesAbsent:       []string{},
		EntitiesBeyondRoster: []string{},
		Failures:             []string{},
	}
	inRoster := map[nepraper.Entity]bool{}
	for _, e := range nepraper.Roster() {
		inRoster[e] = true
	}
	seen := map[string]bool{}
	beyond := map[string]bool{}
	for _, r := range rows {
		if inRoster[nepraper.Entity(r.Entity)] {
			seen[r.Entity] = true
			continue
		}
		beyond[r.Entity] = true
	}
	for _, e := range nepraper.Roster() {
		if seen[string(e)] {
			c.EntitiesSeen++
		} else {
			c.EntitiesAbsent = append(c.EntitiesAbsent, string(e))
		}
	}
	c.EntitiesBeyondRoster = append(c.EntitiesBeyondRoster, sortedKeys(beyond)...)
	switch {
	case len(rows) == 0:
		// A zero-row panel is explained by its own refusal. Counting the
		// whole roster as ten failed assertions here would bury that reason
		// under noise it did not cause.
		c.RosterAssertion = "skipped: no rows matched the filters, so a 10-entity floor asserts nothing"
	case f.Entity != "":
		c.RosterAssertion = "skipped: --entity narrows the population to one licensee, so a 10-entity floor asserts nothing"
	case truncated:
		c.RosterAssertion = "skipped: --limit truncated the row set, so a missing entity may simply be past the cap"
	case len(c.EntitiesAbsent) > 0:
		c.RosterAssertion = "applied"
		c.AssertionsFailed++
		c.Failures = append(c.Failures, fmt.Sprintf(
			"roster: %d of the 10 evaluated entities produced no row (%s)",
			len(c.EntitiesAbsent), strings.Join(c.EntitiesAbsent, ", ")))
	default:
		c.RosterAssertion = "applied"
	}
	return c
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// discoAssertDocument compares the fetched document against the measured corpus
// record. This is the only in-band defence against the corrupt-PDF shape the
// survey measured: an append-on-retry fetch produced a 3,952,485-byte body from
// a 3,024,893-byte source, and file(1) still validated it as a PDF.
func discoAssertDocument(c *discoCompleteness, got, wantBytes, gotPages, wantPages int) {
	if wantBytes > 0 {
		ok := got == wantBytes
		c.BytesMatch = &ok
		if !ok {
			c.AssertionsFailed++
			c.Failures = append(c.Failures, fmt.Sprintf(
				"bytes: received %d, corpus record measured %d", got, wantBytes))
		}
	}
	if wantPages > 0 {
		ok := gotPages == wantPages
		c.PagesMatch = &ok
		if !ok {
			c.AssertionsFailed++
			c.Failures = append(c.Failures, fmt.Sprintf(
				"pages: page tree says %d, corpus record measured %d", gotPages, wantPages))
		}
	}
}

// discoExclusions reports how the report in hand treated each entity NEPRA does
// not evaluate. The three kinds are materially different and are never merged:
// the FY2014-15 report INCLUDES TESCO with real published figures, and quoting
// NEPRA's FY2024-25 exclusion sentence against it would back-date the claim by
// a decade.
func discoExclusions(rep *nepraper.Report) []discoExclusion {
	out := make([]discoExclusion, 0, 2)
	for _, e := range []nepraper.Entity{nepraper.EntityTESCO, nepraper.EntityBTPL} {
		v := nepraper.ExclusionReasonInFY(e, rep.FY)
		if v.Kind == nepraper.KindExcluded {
			out = append(out, discoExclusion{
				Entity: string(e), Kind: "excluded",
				StatedInFY: nepraper.TESCOExclusionStatedFY,
				Source:     nepraper.TESCOExclusionSource,
				Reason:     v.Reason,
			})
			continue
		}
		var tables []string
		seen := map[string]bool{}
		for _, o := range rep.Observations {
			if o.Entity != e || seen[o.Prov.TableLabel] {
				continue
			}
			seen[o.Prov.TableLabel] = true
			tables = append(tables, fmt.Sprintf("%s (PDF page %d)", o.Prov.TableLabel, o.Prov.Page))
		}
		if len(tables) > 0 {
			out = append(out, discoExclusion{
				Entity: string(e), Kind: "present_in_report",
				Reason: fmt.Sprintf("the %s report DOES list %s, in %s; the roster is per-table, not per-report",
					rep.FY, e, strings.Join(tables, ", ")),
			})
			continue
		}
		out = append(out, discoExclusion{
			Entity: string(e), Kind: "unattested",
			Reason: fmt.Sprintf("this build has no record of how the %s report treated %s, and %s produced no row in it. "+
				"That is different from both an exclusion and a report that says nothing.", rep.FY, e, e),
		})
	}
	return out
}

// discoNotesFor keeps the parser notes belonging to the tables this panel read,
// plus every report-level note. parser_notes_count reports the full total, so a
// filtered note list can never read as "the parser saw nothing else".
func discoNotesFor(rep *nepraper.Report, tables []discoTableStat) []discoNote {
	keep := map[string]bool{}
	for _, t := range tables {
		keep[fmt.Sprintf("%s|%d", t.Table, t.Page)] = true
	}
	out := make([]discoNote, 0, len(rep.Notes))
	for _, n := range rep.Notes {
		if n.Table != "" && !keep[fmt.Sprintf("%s|%d", n.Table, n.Page)] {
			continue
		}
		out = append(out, discoNote{Page: n.Page, Table: n.Table, Text: n.Text})
	}
	return out
}

func discoRosterStrings() []string {
	out := make([]string, 0, 10)
	for _, e := range nepraper.Roster() {
		out = append(out, string(e))
	}
	return out
}
