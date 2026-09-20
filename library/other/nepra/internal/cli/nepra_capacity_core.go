// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.
//
// The pure, client-free core of `capacity`. Every number this command prints
// is produced here, so every number is unit-testable without a network.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraxwalk"
)

// capacityStatus is the four-member plant-status enum this command exists to
// produce. It does NOT exist in any shipped type: nepraparse models the
// monthly block's cell states and nepraxwalk models one block_status per
// plant-year, and neither of them can tell a plant whose whole monthly block
// is BLANK from one that reported twelve months. That fourth member is
// derived here, in capacityStatusOf.
type capacityStatus string

const (
	// capStatusActive: the monthly block is ordinary published data.
	capStatusActive capacityStatus = "active"
	// capStatusDelicensed: all 26 monthly cells read DELICENSED.
	capStatusDelicensed capacityStatus = "delicensed"
	// capStatusDecommissioned: all 26 monthly cells read DECOMMISSIONED.
	capStatusDecommissioned capacityStatus = "decommissioned"
	// capStatusListedNoData: a capacity number is published, there is NO
	// status token and there is NO monthly data. Operating status is
	// genuinely unknown and the source carries no legend, so this member's
	// megawatts belong to NEITHER the operating nor the non-operating total.
	capStatusListedNoData capacityStatus = "listed_no_data"

	// capStatusExportToKElectric: the plant's INSTALLED CAPACITY cell holds
	// the sentinel "Export to K.Electric" instead of a number, and its
	// monthly block is blank.
	//
	// It exists because without it those rows land in listed_no_data, whose
	// own basis text says "capacity published, NO status token and NO monthly
	// data" — the exact opposite of what is true for them: NO capacity number
	// is published and a status token IS present. Three FY2020-21 plants are
	// affected (Tenaga Generasi, Hydrochina Dawood, Zephyr Power), and the
	// sibling `gen` command has classified them correctly all along:
	// genRowClass's step 2 exists for precisely this case, and its comment
	// says these rows "would otherwise be misreported as listed_no_data —
	// losing the one thing NEPRA did say about them".
	//
	// THE COMPLETENESS ASSERTION COULD NOT CATCH IT. The check is
	// perStatus[listed_no_data] == Census.FullyNotReportedRows, and
	// Census counts the same three rows the same wrong way, so the identity
	// held while both sides were wrong together.
	capStatusExportToKElectric capacityStatus = "export_to_k_electric"

	// capStatusReportingOrSilent is the OFFLINE-ONLY collapse of active and
	// listed_no_data. It exists because the crosswalk cannot separate them.
	capStatusReportingOrSilent capacityStatus = "reporting_or_silent"
	// capStatusUndetermined is a plant whose monthly block contains NO
	// published measurement at all and is NOT a recognised sentinel or a
	// plain blank. It exists because FY2022-23 publishes one:
	// Reshma Power Generation and Gulf Powergen both carry
	// `<td colspan=26>DELICENSE</td>` — the upstream typo, no trailing D —
	// in the SAME file that spells the other eleven "DELICENSED".
	// nepraparse.statusFor matches "DELICENSED" only, so those 26 expanded
	// cells classify as StateUnknownText, blockStatus() returns
	// StateNumeric, and calling the plant `active` would present 181.00 MW
	// of delicensed capacity as generating. This member refuses to guess
	// which sentinel it is; it says the source published something this
	// build does not model, and carries the verbatim text.
	capStatusUndetermined capacityStatus = "status_undetermined"
	// capStatusUnrecorded is emitted only if a curated block_status is
	// missing or unrecognised — a data regression, not a published state. It
	// is never folded into a reporting bucket.
	capStatusUnrecorded capacityStatus = "status_unrecorded"
)

// capacityLiveStatusEnum is the four-member enum, in report order.
var capacityLiveStatusEnum = []capacityStatus{
	capStatusActive, capStatusDelicensed, capStatusDecommissioned, capStatusListedNoData,
}

// capacityExtraStatusEnum are members that appear ONLY when the source
// forces them. They are appended in this fixed order — never map order — so
// the output is diffable between runs.
var capacityExtraStatusEnum = []capacityStatus{
	capStatusExportToKElectric, capStatusUndetermined, capStatusUnrecorded,
}

// capacityLocalStatusEnum is the three members the offline path can produce.
var capacityLocalStatusEnum = []capacityStatus{
	capStatusReportingOrSilent, capStatusDelicensed, capStatusDecommissioned,
}

// capacityGroupings are the accepted --by values.
var capacityGroupings = []string{"status", "system", "technology"}

// capacityMW is a megawatt sum that knows whether anything measured it.
//
// The fields are unexported and the JSON is hand-written for the same reason
// nepraparse.Value's are: a plain struct would marshal an unpopulated sum as
// {"mw":0} and a consumer could not tell "no plant in this bucket published a
// number" from "these plants published zero megawatts". `mw` is OMITTED
// entirely when nothing was measured.
//
// DELIBERATE DEVIATION from nepraxwalk's own rule (coverage.go's
// `measurable := ObservedInFY > 0`): measured here is set from
// PLANTS-WITH-A-NUMBER, not plants-in-bucket. Otherwise the FY2020-21
// export_to_k_electric bucket — 3 plants, 0 published numbers, because the
// sentinel occupies the capacity column — would read as three plants with
// zero megawatts.
type capacityMW struct {
	measured bool
	mw       float64
	plants   int
}

func (c capacityMW) MarshalJSON() ([]byte, error) {
	out := struct {
		Measured bool     `json:"measured"`
		MW       *float64 `json:"mw,omitempty"`
		Plants   int      `json:"plants"`
	}{Measured: c.measured, Plants: c.plants}
	if c.measured {
		n := c.mw
		out.MW = &n
	}
	return json.Marshal(out)
}

// Float64 returns the sum and whether anything measured it. There is no
// accessor that returns a bare float.
func (c capacityMW) Float64() (float64, bool) {
	if !c.measured {
		return 0, false
	}
	return c.mw, true
}

// Plants is the number of plants that published a NUMBER in this sum, which
// is not in general the number of plants in the bucket.
func (c capacityMW) Plants() int { return c.plants }

// add folds one plant's cell into the sum.
func (c *capacityMW) add(v nepraparse.Value) {
	n, ok := v.Float64()
	if !ok {
		return
	}
	c.mw += n
	c.plants++
	c.measured = true
}

// capacityCellCounts records WHY a capacity cell held no number, per plant.
// A plant is counted in a bucket if EITHER of its two capacity cells is in
// that state, so in FY2020-21 the three export rows appear in both
// StatusCell (installed sentinel) and NotReported (dependable blank).
type capacityCellCounts struct {
	NotReported int `json:"capacity_not_reported_plants"`
	StatusCell  int `json:"capacity_status_cell_plants"`
	UnknownText int `json:"capacity_unknown_text_plants"`
}

func (c *capacityCellCounts) observe(vals ...nepraparse.Value) {
	var notReported, status, unknown bool
	for _, v := range vals {
		switch st := v.State(); {
		case st == nepraparse.StateNumeric:
		case st == nepraparse.StateNotReported:
			notReported = true
		case st.Status():
			status = true
		default:
			unknown = true
		}
	}
	if notReported {
		c.NotReported++
	}
	if status {
		c.StatusCell++
	}
	if unknown {
		c.UnknownText++
	}
}

// capacityBucket is one row of the answer.
//
// Plants, the two sums and the three cell counts are POINTERS so that the
// k_electric_own_fleet entry — a declared absence from the dataset — can
// carry no numeric key at all. Absent is not zero.
type capacityBucket struct {
	Key string `json:"key"`
	// State is set only for a bucket that is not a measurement, currently
	// just "absent_from_dataset".
	State string `json:"state,omitempty"`
	// Known reports membership of nepraparse.KnownTechnologies. Set on
	// --by technology only.
	Known *bool `json:"known,omitempty"`
	// Plants is plants IN the bucket, which is not the same as the plants
	// inside each sum.
	Plants *int `json:"plants,omitempty"`
	// Operating is OMITTED when operating status is unknown. Emitting
	// `operating:false` beside `operating_known:false` invites exactly the
	// misreading this command exists to prevent.
	Operating      *bool `json:"operating,omitempty"`
	OperatingKnown *bool `json:"operating_known,omitempty"`

	Installed  *capacityMW `json:"installed,omitempty"`
	Dependable *capacityMW `json:"dependable,omitempty"`
	// DependableGap explains an absent dependable sum.
	DependableGap string `json:"dependable_unavailable,omitempty"`

	// capacityCellCounts is EMBEDDED so its three counts appear at bucket
	// level. A nil embedded pointer emits none of them, which is how the
	// k_electric_own_fleet absence carries no numeric key at all.
	*capacityCellCounts

	Basis string `json:"basis,omitempty"`
	// UnmodelledCellText is every distinct VERBATIM string the source put in
	// a monthly block this build cannot classify, sorted. It is the evidence
	// behind capStatusUndetermined and is never interpreted.
	UnmodelledCellText []string `json:"unmodelled_cell_text,omitempty"`
	// The absence fields, populated verbatim from nepraxwalk.AbsentParents().
	AbsenceNote string   `json:"absence_note,omitempty"`
	ProbeTokens []string `json:"probe_tokens,omitempty"`
	Caveat      string   `json:"caveat,omitempty"`
	// ByStatus is the nested four-way split on --by technology.
	ByStatus []capacityBucket `json:"by_status,omitempty"`
}

// capacityTotals is the whole-file total, independent of --by.
type capacityTotals struct {
	PlantRows     int         `json:"plant_rows"`
	Installed     capacityMW  `json:"installed"`
	Dependable    *capacityMW `json:"dependable,omitempty"`
	DependableGap string      `json:"dependable_unavailable,omitempty"`
	capacityCellCounts
}

// capacityLedgerRow is one line of the reconciliation ledger.
//
// A "derived" row carries numbers this run computed from the bytes it
// fetched. An "unavailable" row carries the figure NEPRA published as a
// verbatim STRING in PublishedAs and NO numeric field, so nobody can add a
// figure this CLI never measured into a total.
type capacityLedgerRow struct {
	Label        string   `json:"label"`
	State        string   `json:"state"`
	InstalledMW  *float64 `json:"installed_mw,omitempty"`
	DependableMW *float64 `json:"dependable_mw,omitempty"`
	Plants       *int     `json:"plants,omitempty"`
	PublishedAs  string   `json:"published_as,omitempty"`
	Definition   string   `json:"definition,omitempty"`
	Reason       string   `json:"reason,omitempty"`
	Source       string   `json:"source"`
	Instead      string   `json:"instead,omitempty"`
	// Caveat qualifies a DERIVED row: the number is real but something
	// about it is narrower than the label suggests.
	Caveat string `json:"caveat,omitempty"`
}

// capacityEnumGap declares a status-enum member the current data path cannot
// produce, and says whether it is derivable at all.
type capacityEnumGap struct {
	MissingMember string `json:"missing_member"`
	Derivable     bool   `json:"derivable"`
	Reason        string `json:"reason"`
}

// capacityReport is everything the command emits, minus the fetch provenance.
type capacityReport struct {
	By string `json:"-"`
	// UndeterminedPlants names every plant whose status could not be
	// derived, for the assertion message. Never used as data.
	UndeterminedPlants []string            `json:"-"`
	UnmodelledText     []string            `json:"-"`
	StatusEnum         []string            `json:"-"`
	StatusEnumComplete bool                `json:"-"`
	StatusEnumGap      *capacityEnumGap    `json:"-"`
	Buckets            []capacityBucket    `json:"buckets"`
	Totals             capacityTotals      `json:"totals"`
	Reconciliation     []capacityLedgerRow `json:"reconciliation"`
}

// bucket returns the bucket with the given key.
// bucket finds a status member anywhere in the report, INCLUDING nested inside
// a parent bucket's ByStatus split.
//
// It used to scan r.Buckets only. That is correct for --by status, where the
// status members ARE the top-level buckets, and wrong for --by technology and
// --by system, where each parent carries its own four-way ByStatus nesting and
// no status key appears at the top level at all. The caller that matters is the
// NoUnmodelledStatusSentinels assertion, which looks up
// capStatusUndetermined to quote how much capacity it covers: with the
// top-level-only scan the lookup missed under those two groupings and the
// failure sentence read "no capacity number" for rows whose capacity IS
// published and measured — asserting an absence over real data, which is the
// one thing this command exists to refuse.
func (r capacityReport) bucket(key capacityStatus) (capacityBucket, bool) {
	var find func([]capacityBucket) (capacityBucket, bool)
	find = func(bs []capacityBucket) (capacityBucket, bool) {
		for _, b := range bs {
			if b.Key == string(key) {
				return b, true
			}
			if nested, ok := find(b.ByStatus); ok {
				return nested, true
			}
		}
		return capacityBucket{}, false
	}
	return find(r.Buckets)
}

func capacityBoolPtr(b bool) *bool { return &b }
func capacityIntPtr(n int) *int    { return &n }

// ---------------------------------------------------------------------------
// --as-of validation
// ---------------------------------------------------------------------------

// capacityResolveAsOf turns a --as-of value into a catalogue row.
//
// The refusal grammar has two distinct exit codes and they mean different
// things: exit 2 is "that is not a question this source can be asked", exit 3
// is "that is a fair question and NEPRA published no document to answer it".
func capacityResolveAsOf(asOf string) (capacityAsOfSurface, error) {
	trimmed := strings.TrimSpace(asOf)
	if trimmed == "" {
		return capacityAsOfSurface{}, usageErr(fmt.Errorf(
			"--as-of is required to read a capacity stock; run `capacity` with no flags to list the %d dated surfaces",
			len(capacityAsOfCatalogue)))
	}
	d, err := time.Parse("2006-01-02", trimmed)
	if err != nil {
		return capacityAsOfSurface{}, usageErr(fmt.Errorf(
			"--as-of %q is not a calendar date in YYYY-MM-DD form", asOf))
	}
	if d.Month() != time.June || d.Day() != 30 {
		return capacityAsOfSurface{}, usageErr(fmt.Errorf(
			"--as-of %q is a mid-year date, and this source has NO in-year capacity series: NEPRA publishes "+
				"installed and dependable capacity once per fiscal year as two integer columns per plant. "+
				"Answering a mid-year date would date a number the source does not date, and silently rounding "+
				"it to 30 June would be worse. Pass a YYYY-06-30 date", asOf))
	}
	surface, ok := capacityAsOfByDate(trimmed)
	if !ok {
		return capacityAsOfSurface{}, usageErr(fmt.Errorf(
			"--as-of %q is outside the catalogued range %s..%s; run `capacity` with no flags to list every "+
				"dated surface and its state", asOf,
			capacityAsOfCatalogue[0].AsOf, capacityAsOfCatalogue[len(capacityAsOfCatalogue)-1].AsOf))
	}
	if surface.State != "reachable" {
		return capacityAsOfSurface{}, notFoundErr(fmt.Errorf(
			"no capacity workbook is published for --as-of %s (FY%s): %s", surface.AsOf, surface.FiscalYear, surface.Reason))
	}
	return surface, nil
}

// capacityNormaliseBy validates --by.
func capacityNormaliseBy(by string) (string, error) {
	got := strings.ToLower(strings.TrimSpace(by))
	if got == "" {
		got = "status"
	}
	for _, g := range capacityGroupings {
		if got == g {
			return got, nil
		}
	}
	return "", fmt.Errorf("unknown --by %q; accepted values are %s (fuel rollups belong to `gen --rollup`)",
		by, strings.Join(capacityGroupings, ", "))
}

// capacityFYPath builds the sheet URL path for a fiscal year. The upstream
// typo "Genenration" is load-bearing.
func capacityFYPath(fy string) string {
	return "/publications/State%20of%20Industry%20Reports/Detail%20of%20Generation/" +
		"List%20of%20Companies%20Genenration%20wise%20" + fy + "_files/sheet001.htm"
}

// ---------------------------------------------------------------------------
// the status derivation — the whole novelty
// ---------------------------------------------------------------------------

// capacityStatusOf derives the four-member status of one plant row.
//
// The fourth member CANNOT be read off Plant.Status. nepraparse's
// blockStatus() returns StateNumeric whenever the first monthly cell is not a
// status sentinel, so a WHOLLY BLANK monthly block comes back as "numeric" —
// which is exactly how 181.00 MW of no-data capacity (Reshma Power 97.00 +
// Gulf Powergen 84.00) gets booked as generating. It has to be derived from
// the cell states, and it is, here.
//
// This works for BOTH source representations — FY2023-24's single empty
// <td colspan=26> and FY2017-18/FY2020-21's 26 individual blanks — because
// BuildGrid expands spans before ParseValue runs.
func capacityStatusOf(p nepraparse.Plant) capacityStatus {
	switch p.Status {
	case nepraparse.StateDelicensed:
		return capStatusDelicensed
	case nepraparse.StateDecommissioned:
		return capStatusDecommissioned
	}
	numeric, blank, cells := 0, 0, 0
	for _, obs := range append(p.Months[:], p.Total) {
		for _, v := range [2]nepraparse.Value{obs.Utilisation, obs.Generation} {
			cells++
			switch v.State() {
			case nepraparse.StateNumeric:
				numeric++
			case nepraparse.StateNotReported:
				blank++
			}
		}
	}
	// `active` REQUIRES at least one published measurement. Anything else is
	// a block that measures nothing, and the two reasons for that are
	// materially different facts.
	if numeric > 0 {
		return capStatusActive
	}
	// THE CAPACITY CELL IS READ BEFORE THE BLANK TEST, and the order is the
	// correctness requirement. These plants have a blank monthly block AND a
	// sentinel where their megawatts should be; testing blankness first books
	// them as listed_no_data, which asserts that a capacity number was
	// published. Mirrors genRowClass's step 2.
	if p.InstalledCapacity.State() == nepraparse.StateExportToKElectric {
		return capStatusExportToKElectric
	}
	if blank == cells {
		return capStatusListedNoData
	}
	return capStatusUndetermined
}

// capacityBasisExportToKElectric explains the one member whose megawatts are
// unmeasured because NEPRA published a DESTINATION where the number goes.
const capacityBasisExportToKElectric = "the Installed Capacity cell publishes the sentinel " +
	"\"Export to K.Electric\" instead of a number, and the monthly block is blank. So NO capacity " +
	"figure is published for these plants and they carry no megawatts into any total — but unlike " +
	"listed_no_data, the source DID say something about them, and what it said is where their output " +
	"goes rather than how much of it there is. Operating status is not stated either way."

// capacityUnmodelledText returns the distinct verbatim strings in a plant's
// monthly block that this build could not classify, sorted. It reads the raw
// cell text and interprets none of it.
func capacityUnmodelledText(p nepraparse.Plant) []string {
	seen := map[string]struct{}{}
	for _, obs := range append(p.Months[:], p.Total) {
		for _, v := range [2]nepraparse.Value{obs.Utilisation, obs.Generation} {
			if v.State() == nepraparse.StateUnknownText || (v.State() == nepraparse.StateUnset && v.Raw() != "") {
				seen[v.Raw()] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for t := range seen {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// capacityStatusFacts describes one status member for output.
func capacityStatusFacts(s capacityStatus) (operating *bool, operatingKnown bool, basis string) {
	switch s {
	case capStatusActive:
		return capacityBoolPtr(true), true, capacityBasisActive
	case capStatusDelicensed:
		return capacityBoolPtr(false), true, capacityBasisDelicensed
	case capStatusDecommissioned:
		return capacityBoolPtr(false), true, capacityBasisDecommissioned
	case capStatusListedNoData:
		return nil, false, capacityBasisListedNoData
	case capStatusExportToKElectric:
		return nil, false, capacityBasisExportToKElectric
	case capStatusUndetermined:
		return nil, false, capacityBasisUndetermined
	case capStatusReportingOrSilent:
		return nil, false, capacityBasisReportingOrSilent
	default:
		return nil, false, "no status was recorded for these rows; that is a data regression, not a published state"
	}
}

// ---------------------------------------------------------------------------
// the live grouping
// ---------------------------------------------------------------------------

// capacityAgg accumulates one bucket.
type capacityAgg struct {
	plants     int
	installed  capacityMW
	dependable capacityMW
	cells      capacityCellCounts
}

func (a *capacityAgg) addPlant(installed, dependable nepraparse.Value) {
	a.plants++
	a.installed.add(installed)
	a.dependable.add(dependable)
	a.cells.observe(installed, dependable)
}

func (a capacityAgg) toBucket(key string, operating *bool, operatingKnown bool, basis string) capacityBucket {
	inst, dep, cells := a.installed, a.dependable, a.cells
	return capacityBucket{
		Key:                key,
		Plants:             capacityIntPtr(a.plants),
		Operating:          operating,
		OperatingKnown:     capacityBoolPtr(operatingKnown),
		Installed:          &inst,
		Dependable:         &dep,
		capacityCellCounts: &cells,
		Basis:              basis,
	}
}

// capacityGroupLive partitions a parsed workbook.
func capacityGroupLive(w *nepraparse.Workbook, by string) (capacityReport, error) {
	if w == nil {
		return capacityReport{}, fmt.Errorf("capacity: no workbook to group")
	}
	rep := capacityReport{By: by, StatusEnumComplete: true}
	for _, s := range capacityLiveStatusEnum {
		rep.StatusEnum = append(rep.StatusEnum, string(s))
	}

	var total capacityAgg
	byStatus := map[capacityStatus]*capacityAgg{}
	for _, s := range capacityLiveStatusEnum {
		byStatus[s] = &capacityAgg{}
	}
	type techAgg struct {
		agg      capacityAgg
		perState map[capacityStatus]*capacityAgg
		known    bool
	}
	techs := map[string]*techAgg{}
	var techOrder []string
	var exports capacityAgg
	unmodelled := map[string]struct{}{}
	var undeterminedPlants []string

	for _, p := range w.Plants {
		st := capacityStatusOf(p)
		if st == capStatusUndetermined {
			undeterminedPlants = append(undeterminedPlants, p.Name)
			for _, t := range capacityUnmodelledText(p) {
				unmodelled[t] = struct{}{}
			}
		}
		if byStatus[st] == nil {
			byStatus[st] = &capacityAgg{}
		}
		byStatus[st].addPlant(p.InstalledCapacity, p.DependableCapacity)
		total.addPlant(p.InstalledCapacity, p.DependableCapacity)

		t := techs[p.Technology]
		if t == nil {
			t = &techAgg{perState: map[capacityStatus]*capacityAgg{}, known: p.TechnologyKnown}
			for _, s := range capacityLiveStatusEnum {
				t.perState[s] = &capacityAgg{}
			}
			techs[p.Technology] = t
			techOrder = append(techOrder, p.Technology)
		}
		t.agg.addPlant(p.InstalledCapacity, p.DependableCapacity)
		if t.perState[st] == nil {
			t.perState[st] = &capacityAgg{}
		}
		t.perState[st].addPlant(p.InstalledCapacity, p.DependableCapacity)

		// A row whose INSTALLED cell holds the export sentinel is a
		// regulatory disposition, not a member of a capacity partition. It
		// keeps its place in cppag and in its technology.
		if p.InstalledCapacity.State() == nepraparse.StateExportToKElectric {
			exports.addPlant(p.InstalledCapacity, p.DependableCapacity)
		}
	}

	// A member outside the four-member enum is a fact about the source, not
	// a bug to fold away. The enum bookkeeping happens HERE, before the
	// grouping switch, so `--by technology` and `--by system` report the
	// same incompleteness that `--by status` does rather than claiming a
	// complete four-member enum while a fifth member sits in a nested split.
	var extras []capacityStatus
	for _, st := range capacityExtraStatusEnum {
		if agg := byStatus[st]; agg != nil && agg.plants > 0 {
			extras = append(extras, st)
			rep.StatusEnum = append(rep.StatusEnum, string(st))
			rep.StatusEnumComplete = false
		}
	}

	switch by {
	case "status":
		for _, s := range capacityLiveStatusEnum {
			op, known, basis := capacityStatusFacts(s)
			rep.Buckets = append(rep.Buckets, byStatus[s].toBucket(string(s), op, known, basis))
		}
		for _, st := range extras {
			op, known, basis := capacityStatusFacts(st)
			b := byStatus[st].toBucket(string(st), op, known, basis)
			if st == capStatusUndetermined {
				b.UnmodelledCellText = capacitySortedKeys(unmodelled)
			}
			rep.Buckets = append(rep.Buckets, b)
		}
	case "technology":
		// Keyed on the VERBATIM published string. "Coal" and "THERMAL- COAL"
		// are two distinct published technologies carrying materially
		// different facts and are never merged.
		sort.SliceStable(techOrder, func(i, j int) bool {
			li, _ := techs[techOrder[i]].agg.installed.Float64()
			lj, _ := techs[techOrder[j]].agg.installed.Float64()
			if li != lj {
				return li > lj
			}
			return techOrder[i] < techOrder[j]
		})
		for _, name := range techOrder {
			t := techs[name]
			b := t.agg.toBucket(name, nil, false, "")
			b.Operating, b.OperatingKnown = nil, nil
			b.Known = capacityBoolPtr(t.known)
			for _, s := range append(append([]capacityStatus{}, capacityLiveStatusEnum...), extras...) {
				agg := t.perState[s]
				if agg == nil {
					continue
				}
				op, known, _ := capacityStatusFacts(s)
				nested := agg.toBucket(string(s), op, known, "")
				if s == capStatusUndetermined {
					nested.UnmodelledCellText = capacitySortedKeys(unmodelled)
				}
				b.ByStatus = append(b.ByStatus, nested)
			}
			rep.Buckets = append(rep.Buckets, b)
		}
	case "system":
		rep.Buckets = capacitySystemBuckets(total, exports)
	default:
		return capacityReport{}, fmt.Errorf("capacity: unhandled --by %q", by)
	}

	rep.Totals = capacityTotals{
		PlantRows:          len(w.Plants),
		Installed:          total.installed,
		capacityCellCounts: total.cells,
	}
	dep := total.dependable
	rep.Totals.Dependable = &dep

	rep.UndeterminedPlants = undeterminedPlants
	rep.UnmodelledText = capacitySortedKeys(unmodelled)

	active := byStatus[capStatusActive]
	rep.Reconciliation = capacityLedger(w.FiscalYear.Label(),
		capacityLedgerRow{
			Label: capacityLedgerWorkbookActive, State: "derived",
			InstalledMW: capacityMWPtr(active.installed), DependableMW: capacityMWPtr(active.dependable),
			Plants: capacityIntPtr(active.plants),
			Source: "meta.artifact",
		},
		capacityLedgerRow{
			Label: capacityLedgerWorkbookAllPublished, State: "derived",
			InstalledMW: capacityMWPtr(total.installed), DependableMW: capacityMWPtr(total.dependable),
			Plants: capacityIntPtr(total.plants),
			Source: "meta.artifact",
		},
	)
	return rep, nil
}

// capacitySortedKeys returns a set's members in a stable order.
func capacitySortedKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// capacityMWPtr returns a pointer to the sum, or nil when nothing measured
// it. A ledger row must not carry a numeric field it did not measure.
func capacityMWPtr(c capacityMW) *float64 {
	n, ok := c.Float64()
	if !ok {
		return nil
	}
	return &n
}

// capacitySystemBuckets builds the three --by system entries.
//
// These do NOT partition the plant rows and are not claimed to: cppag is a
// whole-file attribution over every row, export_to_k_electric is a per-row
// disposition that is a SUBSET of cppag and carries no megawatts, and
// k_electric_own_fleet is a declared absence with no numeric key at all.
func capacitySystemBuckets(total, exports capacityAgg) []capacityBucket {
	cppag := total.toBucket("cppag", nil, false, capacitySystemBasisCPPAG)
	cppag.Operating, cppag.OperatingKnown = nil, nil

	exp := exports.toBucket("export_to_k_electric", nil, false, capacitySystemBasisExport)
	exp.Operating, exp.OperatingKnown = nil, nil

	kel := capacityBucket{
		Key:   "k_electric_own_fleet",
		State: string(nepraxwalk.StatusAbsentFromDataset),
		Basis: "K-Electric's own generating fleet is outside these workbooks' scope. This entry carries NO " +
			"installed or dependable key: absent from the dataset is not zero megawatts.",
	}
	for _, ap := range nepraxwalk.AbsentParents() {
		if ap.Status != nepraxwalk.StatusAbsentFromDataset {
			continue
		}
		kel.AbsenceNote = ap.AbsenceNote
		kel.ProbeTokens = ap.ProbeTokens
		kel.Caveat = ap.Caveat
		break
	}
	return []capacityBucket{cppag, exp, kel}
}

// capacityLedger prepends the derived rows to the unavailable ones for the
// fiscal year in question.
func capacityLedger(fy string, derived ...capacityLedgerRow) []capacityLedgerRow {
	unavailable := capacityUnavailableLedgerFor(fy)
	out := make([]capacityLedgerRow, 0, len(derived)+len(unavailable))
	for _, d := range derived {
		if d.Definition == "" {
			d.Definition = capacityLedgerDefinitions[d.Label]
		}
		out = append(out, d)
	}
	return append(out, unavailable...)
}

// ---------------------------------------------------------------------------
// the offline grouping
// ---------------------------------------------------------------------------

// capacityGroupLocal groups the embedded crosswalk, which is the only
// //go:embed'd data in the module. It is a DEGRADED answer and says so: the
// crosswalk stores no per-month cell states, so listed_no_data cannot be
// separated from active, and it stores no dependable capacity at all.
func capacityGroupLocal(fy, by string) (capacityReport, error) {
	observed := nepraxwalk.ObservedFYs()
	found := false
	for _, o := range observed {
		if o == fy {
			found = true
			break
		}
	}
	if !found {
		return capacityReport{}, fmt.Errorf(
			"the embedded crosswalk carries no observations for FY%s; it observes only %s (FY%s is deliberately "+
				"HELD OUT). A zero-megawatt report for an unobserved year would be a fabrication, so this is an "+
				"error. Pass --data-source live",
			fy, strings.Join(observed, ", "), strings.Join(nepraxwalk.HeldOutFYs(), ", "))
	}

	rep := capacityReport{By: by}
	for _, s := range capacityLocalStatusEnum {
		rep.StatusEnum = append(rep.StatusEnum, string(s))
	}
	rep.StatusEnumGap = &capacityEnumGap{
		MissingMember: string(capStatusListedNoData),
		Derivable:     false,
		Reason:        capacityLocalEnumGapReason,
	}

	var total capacityAgg
	byStatus := map[capacityStatus]*capacityAgg{}
	for _, s := range capacityLocalStatusEnum {
		byStatus[s] = &capacityAgg{}
	}
	type techAgg struct {
		agg      capacityAgg
		perState map[capacityStatus]*capacityAgg
		known    bool
	}
	techs := map[string]*techAgg{}
	var techOrder []string
	var exports capacityAgg
	var unrecorded capacityAgg

	for _, row := range nepraxwalk.Rows() {
		obs, ok := row.ObservedIn(fy)
		if !ok {
			continue
		}
		st := capStatusReportingOrSilent
		if bs, known := obs.BlockStatus(); !known {
			st = capStatusUnrecorded
		} else {
			switch bs {
			case nepraparse.StateDelicensed:
				st = capStatusDelicensed
			case nepraparse.StateDecommissioned:
				st = capStatusDecommissioned
			}
		}
		capv := obs.Capacity()
		agg := byStatus[st]
		if agg == nil {
			agg = &unrecorded
		}
		// There is NO dependable column in the crosswalk, so nothing is
		// added to a dependable sum and no dependable key is emitted.
		agg.plants++
		agg.installed.add(capv)
		agg.cells.observe(capv)
		total.plants++
		total.installed.add(capv)
		total.cells.observe(capv)

		t := techs[obs.Technology]
		if t == nil {
			t = &techAgg{perState: map[capacityStatus]*capacityAgg{}, known: nepraparse.KnownTechnology(obs.Technology)}
			for _, s := range capacityLocalStatusEnum {
				t.perState[s] = &capacityAgg{}
			}
			techs[obs.Technology] = t
			techOrder = append(techOrder, obs.Technology)
		}
		t.agg.plants++
		t.agg.installed.add(capv)
		t.agg.cells.observe(capv)
		if t.perState[st] == nil {
			t.perState[st] = &capacityAgg{}
		}
		t.perState[st].plants++
		t.perState[st].installed.add(capv)
		t.perState[st].cells.observe(capv)

		if capv.State() == nepraparse.StateExportToKElectric {
			exports.plants++
			exports.cells.observe(capv)
		}
	}

	stripDependable := func(b capacityBucket) capacityBucket {
		b.Dependable = nil
		b.DependableGap = capacityLocalDependableGap
		return b
	}

	switch by {
	case "status":
		for _, s := range capacityLocalStatusEnum {
			op, known, basis := capacityStatusFacts(s)
			rep.Buckets = append(rep.Buckets, stripDependable(byStatus[s].toBucket(string(s), op, known, basis)))
		}
		if unrecorded.plants > 0 {
			op, known, basis := capacityStatusFacts(capStatusUnrecorded)
			rep.Buckets = append(rep.Buckets, stripDependable(unrecorded.toBucket(string(capStatusUnrecorded), op, known, basis)))
			rep.StatusEnum = append(rep.StatusEnum, string(capStatusUnrecorded))
		}
	case "technology":
		sort.SliceStable(techOrder, func(i, j int) bool {
			li, _ := techs[techOrder[i]].agg.installed.Float64()
			lj, _ := techs[techOrder[j]].agg.installed.Float64()
			if li != lj {
				return li > lj
			}
			return techOrder[i] < techOrder[j]
		})
		for _, name := range techOrder {
			t := techs[name]
			b := stripDependable(t.agg.toBucket(name, nil, false, ""))
			b.Operating, b.OperatingKnown = nil, nil
			b.Known = capacityBoolPtr(t.known)
			members := append([]capacityStatus{}, capacityLocalStatusEnum...)
			if unrecorded.plants > 0 {
				members = append(members, capStatusUnrecorded)
			}
			for _, s := range members {
				agg := t.perState[s]
				if agg == nil {
					continue
				}
				op, known, _ := capacityStatusFacts(s)
				b.ByStatus = append(b.ByStatus, stripDependable(agg.toBucket(string(s), op, known, "")))
			}
			rep.Buckets = append(rep.Buckets, b)
		}
	case "system":
		for _, b := range capacitySystemBuckets(total, exports) {
			if b.Installed != nil {
				b = stripDependable(b)
			}
			rep.Buckets = append(rep.Buckets, b)
		}
	default:
		return capacityReport{}, fmt.Errorf("capacity: unhandled --by %q", by)
	}

	rep.Totals = capacityTotals{
		PlantRows:          total.plants,
		Installed:          total.installed,
		DependableGap:      capacityLocalDependableGap,
		capacityCellCounts: total.cells,
	}
	rep.Reconciliation = capacityLedger(fy,
		capacityLedgerRow{
			Label: capacityLedgerWorkbookActive, State: "unavailable",
			// NO published_as: this is not a figure NEPRA published under
			// another name, it is one the offline path cannot compute.
			Reason: "the offline path cannot produce the `active` member at all: " + capacityLocalEnumGapReason +
				" There is therefore no active-capacity figure to derive, and none is emitted.",
			Source:  "internal/nepraxwalk/crosswalk.json",
			Instead: "--data-source live, which derives active capacity from the workbook's own monthly cell states",
		},
		capacityLedgerRow{
			Label: capacityLedgerWorkbookAllPublished, State: "derived",
			InstalledMW: capacityMWPtr(total.installed),
			Plants:      capacityIntPtr(total.plants),
			Source:      "internal/nepraxwalk/crosswalk.json (embedded)",
			Caveat:      capacityLocalDependableGap,
		},
	)
	return rep, nil
}
