// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package nepraxwalk

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/nepraparse"
)

// MWSum is a summed installed capacity that knows whether it is a measurement.
//
// It exists because a float64 cannot express "nothing was measured", and a
// zero-valued float is the single most dangerous answer this package can give:
// asked for FY2020-21 — a year deliberately held out of the crosswalk, so no
// row carries an observation for it — the earlier report returned
// CapacityReportedMW = 0 for a year in which the workbook publishes
// 36,902 MW, and simultaneously reported that nothing had gone unreported.
// Two fabrications from one missing branch.
//
// The zero value is correctly unmeasured, so an MWSum nobody populated cannot
// be read as 0 MW.
type MWSum struct {
	mw       float64
	measured bool
	plants   int
}

// Float64 returns the summed megawatts and whether they are a measurement at
// all. A false second return means the sum has no population behind it — the
// fiscal year was never observed — and the first return carries no
// information. There is deliberately no accessor that returns a bare float64.
func (s MWSum) Float64() (float64, bool) { return s.mw, s.measured }

// Measured reports whether this sum has a population behind it.
func (s MWSum) Measured() bool { return s.measured }

// Plants is how many distinct plants contributed to the sum. It can be 0 on a
// measured sum, which is a real finding ("this year had no non-operating
// capacity"), quite unlike an unmeasured sum ("this year was never observed").
func (s MWSum) Plants() int { return s.plants }

// String renders the sum without printing a number that is not a measurement.
func (s MWSum) String() string {
	if !s.measured {
		return "<not measured>"
	}
	return fmt.Sprintf("%.2f MW over %d plants", s.mw, s.plants)
}

// mwSumJSON is the wire shape of an [MWSum]. `mw` is OMITTED on an unmeasured
// sum so a JSON consumer cannot read a zero that was never measured.
type mwSumJSON struct {
	Measured bool     `json:"measured"`
	MW       *float64 `json:"mw,omitempty"`
	Plants   int      `json:"plants"`
}

// MarshalJSON emits the measured flag alongside the number.
//
// MWSum has only unexported fields, so without this the default encoder would
// emit `{}` for every sum — the identical defect that made json.Marshal of a
// nepraparse.Value report Tarbela's 3,478 MW as nothing.
func (s MWSum) MarshalJSON() ([]byte, error) {
	out := mwSumJSON{Measured: s.measured, Plants: s.plants}
	if s.measured {
		mw := s.mw
		out.MW = &mw
	}
	return json.Marshal(out)
}

// UnmarshalJSON restores an [MWSum], refusing a measured sum with no number
// and an unmeasured sum that carries one.
func (s *MWSum) UnmarshalJSON(data []byte) error {
	var in mwSumJSON
	if err := json.Unmarshal(data, &in); err != nil {
		return err
	}
	switch {
	case in.Measured && in.MW == nil:
		return fmt.Errorf("nepraxwalk: measured MWSum carries no mw")
	case !in.Measured && in.MW != nil:
		return fmt.Errorf("nepraxwalk: unmeasured MWSum carries mw %v", *in.MW)
	}
	s.measured, s.plants, s.mw = in.Measured, in.Plants, 0
	if in.MW != nil {
		s.mw = *in.MW
	}
	return nil
}

// CoverageReport is what a batch of published names actually resolved to. It
// exists so coverage is a measured number a caller can print, not a claim.
type CoverageReport struct {
	// FY is the fiscal year the names came from, echoed back.
	FY string `json:"fy"`
	// FYObserved reports whether FY is one of the fiscal years whose workbooks
	// were fetched to build the crosswalk. When it is false, NO row carries an
	// observation for FY and every capacity sum below is unmeasured by
	// construction. Read this before reading any capacity.
	FYObserved bool `json:"fy_observed"`
	// CapacityBasis states in prose what the capacity sums are and are not,
	// so a report that cannot measure capacity says so in its own output.
	CapacityBasis string `json:"capacity_basis"`

	// Total is len(publishedNames), duplicates included.
	Total int `json:"total"`
	// Resolved and Declined partition Total.
	Resolved int `json:"resolved"`
	Declined int `json:"declined"`
	// DeclinedNames are the normalised inputs that got no match, sorted, so a
	// human can extend the crosswalk instead of guessing at runtime.
	DeclinedNames []string `json:"declined_names,omitempty"`
	// ByKind counts resolutions by how they matched. A large
	// MatchParenStripped count means the crosswalk's alias table is behind the
	// publication and should be topped up.
	ByKind map[MatchKind]int `json:"by_kind"`
	// WithParent counts resolved names whose plant has any parent asserted.
	WithParent int `json:"with_parent"`
	// WithTicker counts resolved names whose plant has a PSX symbol asserted.
	WithTicker int `json:"with_ticker"`
	// ByConfidence counts resolved names by their row's attribution confidence.
	ByConfidence map[Confidence]int `json:"by_confidence"`
	// DistinctPlants is how many distinct canonical plants the batch reached.
	DistinctPlants int `json:"distinct_plants"`
	// ObservedInFY is how many of those distinct plants actually appear in FY's
	// workbook. It is the population every capacity sum is drawn from, and it
	// is 0 for a fiscal year the crosswalk never observed.
	ObservedInFY int `json:"observed_in_fy"`

	// CapacityReported is every numeric installed capacity published in FY,
	// operating or not. It is the figure that ties to the workbook total.
	CapacityReported MWSum `json:"capacity_reported"`
	// CapacityOperating is the part of CapacityReported whose monthly
	// generation block is ordinary data. THIS is current generating capacity;
	// CapacityReported is not.
	CapacityOperating MWSum `json:"capacity_operating"`
	// CapacityNonOperating is the part whose monthly block reads DELICENSED or
	// DECOMMISSIONED. Measured at 3,880.00 MW over 13 plants in FY2023-24 —
	// capacity that is published, real, and NOT generating.
	CapacityNonOperating MWSum `json:"capacity_non_operating"`
	// CapacityStatusUnknown is the part whose operating status was not
	// recorded. It belongs to neither of the two above and must not be folded
	// into either.
	CapacityStatusUnknown MWSum `json:"capacity_status_unknown"`
	// CapacityTickered is the part of CapacityReported carrying a PSX symbol.
	CapacityTickered MWSum `json:"capacity_tickered"`
	// CapacityTickeredOperating is the part of CapacityTickered that is
	// actually generating. The gap between the two is what an equity analyst
	// would otherwise book as live capacity: in FY2023-24 it is the Hub Power
	// Company's delicensed 1,292 MW.
	CapacityTickeredOperating MWSum `json:"capacity_tickered_operating"`

	// NonOperatingPlants counts plants observed in FY that were not generating.
	NonOperatingPlants int `json:"non_operating_plants"`
	// CapacityNotReported counts plants observed in FY whose capacity cell was
	// blank or NBSP-only. Excluded from every sum rather than treated as zero.
	CapacityNotReported int `json:"capacity_not_reported"`
	// CapacityStatusCell counts plants whose capacity CELL itself holds a
	// status sentinel rather than a number — "Export to K.Electric", 3 rows in
	// FY2020-21. Those plants have a real disposition and no published number,
	// which is neither a zero nor a blank.
	CapacityStatusCell int `json:"capacity_status_cell"`
	// CapacityUnknownText counts capacity cells holding unmodelled text. Any
	// non-zero value here means the workbook has a sentinel this build does
	// not model, and it is surfaced rather than swallowed.
	CapacityUnknownText int `json:"capacity_unknown_text"`
	// NoObservationInFY counts distinct resolved plants absent from FY's
	// workbook entirely. When FYObserved is false this equals DistinctPlants,
	// which is the shape of a query against a year that was never fetched.
	NoObservationInFY int `json:"no_observation_in_fy"`
}

// Coverage resolves every published name and reports what happened, including
// what it declined. Pass the fiscal year the names came from; capacity figures
// are read from that year's observation only, so a plant re-rated between
// years is never counted at the wrong year's capacity.
//
// Capacity is reported ONLY for a fiscal year the crosswalk actually observed.
// For any other year — FY2020-21 is held out on purpose — every capacity sum
// comes back unmeasured and NoObservationInFY accounts for every plant, rather
// than a 0 MW that would read as a measurement.
func Coverage(fy string, publishedNames []string) CoverageReport {
	rep := CoverageReport{
		FY:           fy,
		FYObserved:   fyObserved(fy),
		Total:        len(publishedNames),
		ByKind:       map[MatchKind]int{},
		ByConfidence: map[Confidence]int{},
	}
	seen := map[string]bool{}
	declined := map[string]bool{}

	var reported, operating, nonOperating, statusUnknown, tickered, tickeredOperating float64
	var nReported, nOperating, nNonOperating, nStatusUnknown, nTickered, nTickeredOperating int

	for _, n := range publishedNames {
		m, ok := Resolve(n)
		if !ok {
			rep.Declined++
			declined[Normalize(n)] = true
			continue
		}
		rep.Resolved++
		rep.ByKind[m.Kind]++
		rep.ByConfidence[m.Row.Confidence]++
		if m.Row.HasParent() {
			rep.WithParent++
		}
		if m.Row.HasTicker() {
			rep.WithTicker++
		}
		if seen[m.Row.CanonicalName] {
			continue
		}
		seen[m.Row.CanonicalName] = true
		rep.DistinctPlants++

		o, found := m.Row.ObservedIn(fy)
		if !found {
			// The plant is not in this year's workbook. That is absence from
			// the publication and contributes to no sum, in either direction.
			rep.NoObservationInFY++
			continue
		}
		rep.ObservedInFY++

		isOperating, statusKnown := o.Operating()
		if statusKnown && !isOperating {
			rep.NonOperatingPlants++
		}

		cap := o.Capacity()
		mw, isNumber := cap.Float64()
		if !isNumber {
			// No published number. Record WHICH of the three reasons, because
			// they are different facts about the plant.
			switch cap.State() {
			case nepraparse.StateNotReported:
				rep.CapacityNotReported++
			case nepraparse.StateUnknownText:
				rep.CapacityUnknownText++
			default:
				rep.CapacityStatusCell++
			}
			continue
		}

		reported += mw
		nReported++
		switch {
		case !statusKnown:
			statusUnknown += mw
			nStatusUnknown++
		case isOperating:
			operating += mw
			nOperating++
		default:
			nonOperating += mw
			nNonOperating++
		}
		if m.Row.HasTicker() {
			tickered += mw
			nTickered++
			if statusKnown && isOperating {
				tickeredOperating += mw
				nTickeredOperating++
			}
		}
	}

	// Every sum is a measurement only if there was a population to measure.
	// This single flag is the fix for the fabricated zero: with no observation
	// in FY there is no population, so no sum claims a number.
	measurable := rep.ObservedInFY > 0
	rep.CapacityReported = MWSum{mw: reported, measured: measurable, plants: nReported}
	rep.CapacityOperating = MWSum{mw: operating, measured: measurable, plants: nOperating}
	rep.CapacityNonOperating = MWSum{mw: nonOperating, measured: measurable, plants: nNonOperating}
	rep.CapacityStatusUnknown = MWSum{mw: statusUnknown, measured: measurable, plants: nStatusUnknown}
	rep.CapacityTickered = MWSum{mw: tickered, measured: measurable, plants: nTickered}
	rep.CapacityTickeredOperating = MWSum{mw: tickeredOperating, measured: measurable, plants: nTickeredOperating}
	rep.CapacityBasis = capacityBasis(fy, rep.FYObserved, measurable, rep.NoObservationInFY)

	for n := range declined {
		rep.DeclinedNames = append(rep.DeclinedNames, n)
	}
	sort.Strings(rep.DeclinedNames)
	return rep
}

// fyObserved reports whether fy is one of the fiscal years the crosswalk was
// built from.
func fyObserved(fy string) bool {
	if fy == "" {
		return false
	}
	for _, o := range ObservedFYs() {
		if o == fy {
			return true
		}
	}
	return false
}

// capacityBasis explains, in the report itself, what the capacity sums mean —
// including the case where they mean nothing.
func capacityBasis(fy string, observed, measurable bool, absent int) string {
	switch {
	case fy == "":
		return "no fiscal year was supplied, so no capacity could be read; every capacity sum is unmeasured"
	case !observed:
		return fmt.Sprintf("FY%s is not one of the crosswalk's observed years %v, so no row carries an observation for it: "+
			"all %d resolved plants are unobserved in FY%s and every capacity sum is unmeasured. This is NOT a finding of zero capacity — "+
			"the FY%s workbook publishes capacity for plants this crosswalk simply did not index that year",
			fy, ObservedFYs(), absent, fy, fy)
	case !measurable:
		return fmt.Sprintf("no resolved plant was observed in FY%s, so every capacity sum is unmeasured", fy)
	default:
		return fmt.Sprintf("capacity is the installed-capacity column of the FY%s workbook, summed over distinct resolved plants observed that year; "+
			"non-operating plants (DELICENSED/DECOMMISSIONED monthly blocks) keep their published capacity and are reported separately from operating capacity", fy)
	}
}
