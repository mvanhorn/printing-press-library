// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Live occupancy: the FEMA OpenShelters spine and the Red Cross EA view both
// carry a null total_population on their public mirrors, so neither answers
// "how many people are in this shelter right now". The American Red Cross
// Open_Shelters operational layer (same ArcGIS org as the EA view) does: it
// reports total_population, the general/medical/other/pet breakdown, the
// evacuation/post-impact capacity, and the driving incident. This file fetches
// that layer best-effort and overlays it onto the public union, matched on
// physical identity (normalized name + state + compatible ZIP) because its
// SHELTER_ID is an ARC-internal id, NOT FEMA's stable shelter_id.
//
// IMPORTANT (visibility): Open_Shelters is the Red Cross OPERATIONAL roster and
// includes sites the Red Cross deliberately keeps off the public map (privacy,
// transitional, non-walk-in), and it carries NO public/hide flag of its own. So
// this is a FILL-ONLY overlay: it adds population/capacity onto a shelter that is
// ALREADY in the public union (FEMA's public OpenShelters feed or the public Red
// Cross EA view, which is itself filtered to hide_from_public='No'), and NEVER
// introduces a new shelter. A row with no match in either public feed is withheld
// so the CLI surfaces only publicly listed shelters.
//
// Like the other overlays it NEVER fails a command: any fetch/parse error
// degrades to null occupancy plus an honest note. Personal-contact columns (org
// POC name/phone/email, mailing address) are deliberately never requested, so
// they never touch the wire.

package cli

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// occOutFields is the exact, non-PII set of Open_Shelters columns parseOccupancy
// reads. Requesting an explicit list (not "*") bounds the response and, as
// defense-in-depth, keeps the layer's ORG_POC_* / ORG_EMAIL / ORG_*_PHONE /
// MAILING_* personal-contact columns off the wire entirely (we never parse them,
// so not fetching them means they never enter the process). UPPERCASE because
// that is how the Open_Shelters schema names its fields.
var occOutFields = []string{
	"SHELTER_NAME", "ADDRESS_1", "CITY", "STATE", "ZIP", "COUNTY_PARISH",
	"TOTAL_POPULATION", "GENERAL_POPULATION", "MEDICAL_NEEDS_POPULATION",
	"OTHER_POPULATION", "PET_POPULATION", "EVACUATION_CAPACITY", "POST_IMPACT_CAPACITY",
	"SHELTER_STATUS_CODE", "INCIDENT_NAME", "INCIDENT_NUMBER", "SHELTER_OPEN_DATE",
	"ADA_COMPLIANT", "WHEELCHAIR_ACCESSIBLE", "PET_ACCOMMODATIONS_CODE",
	"PET_ACCOMMODATIONS_DESC", "GENERATOR_ONSITE", "LATITUDE", "LONGITUDE", "FACILITY_TYPE",
}

// fetchOccupancy fetches and parses the Open_Shelters occupancy layer. Package
// var so tests stub it without the network.
var fetchOccupancy = occupancyFetch

func occupancyFetch(ctx context.Context) ([]Shelter, error) {
	v := url.Values{}
	// where=1=1 mirrors the spine query: the layer is the operational OPEN set
	// (live data is 100% SHELTER_STATUS_CODE=OPEN). Fetching everything and
	// surfacing the literal status avoids a server-side case-sensitivity trap and
	// keeps parity with the OpenShelters spine; a stray non-open row would simply
	// carry its real status rather than be silently filtered.
	v.Set("where", "1=1")
	v.Set("outFields", strings.Join(occOutFields, ","))
	v.Set("returnGeometry", "false")
	v.Set("f", "json")
	body, err := httpGet(ctx, redCrossBase+openSheltersOccQuery+"?"+v.Encode(), "")
	if err != nil {
		return nil, err
	}
	return parseOccupancy(body)
}

// parseOccupancy decodes an Open_Shelters query response into Shelter records
// carrying live occupancy (Source "occupancy"). It shares decodeFeatures, so a
// valid-JSON-but-wrong-shape body fails loud exactly like the spine parser: a
// broken or blocked feed is never read as "0 shelters with population". The
// attribute keys are UPPERCASE.
func parseOccupancy(raw []byte) ([]Shelter, error) {
	attrs, err := decodeFeatures(raw)
	if err != nil {
		return nil, err
	}
	out := make([]Shelter, 0, len(attrs))
	for _, a := range attrs {
		s := Shelter{Source: "occupancy"}
		s.Name = attrStr(a, "SHELTER_NAME")
		s.Address = attrStr(a, "ADDRESS_1")
		s.City = cleanCity(attrStr(a, "CITY"))
		s.State = normCode(attrStr(a, "STATE"))
		s.Zip = strings.TrimSpace(attrStr(a, "ZIP"))
		s.CountyParish = attrStr(a, "COUNTY_PARISH")
		// Open_Shelters codes status as the FEMA vocabulary (OPEN/CLOSED/...), so it
		// is safe to surface under both the spine Status and the coded enrichment
		// field; downstream filters read them as FEMA codes.
		s.Status = normCode(attrStr(a, "SHELTER_STATUS_CODE"))
		s.ShelterStatusCode = normCode(attrStr(a, "SHELTER_STATUS_CODE"))
		s.FacilityType = normCode(attrStr(a, "FACILITY_TYPE"))
		s.ADACompliant = normCode(attrStr(a, "ADA_COMPLIANT"))
		s.WheelchairAccessible = normCode(attrStr(a, "WHEELCHAIR_ACCESSIBLE"))
		// rcPetCode maps the ARC pet-shelter code onto the FEMA pet vocabulary
		// conservatively (only clear co-located / on-site arrangements count as
		// pet-friendly), the same mapping the Red Cross parser uses.
		s.PetAccommodations = rcPetCode(attrStr(a, "PET_ACCOMMODATIONS_CODE"))
		s.PetAccommodationsDesc = attrStr(a, "PET_ACCOMMODATIONS_DESC")
		s.GeneratorOnsite = normCode(attrStr(a, "GENERATOR_ONSITE"))
		s.IncidentName = attrStr(a, "INCIDENT_NAME")
		s.IncidentNumber = attrStr(a, "INCIDENT_NUMBER")
		s.ShelterOpenDate = attrEpochDateStr(a, "SHELTER_OPEN_DATE")
		// The whole point of this feed: live population + capacity. Null stays nil.
		s.TotalPopulation = attrIntPtr(a, "TOTAL_POPULATION")
		s.GeneralPopulation = attrIntPtr(a, "GENERAL_POPULATION")
		s.MedicalNeedsPopulation = attrIntPtr(a, "MEDICAL_NEEDS_POPULATION")
		s.OtherPopulation = attrIntPtr(a, "OTHER_POPULATION")
		s.PetPopulation = attrIntPtr(a, "PET_POPULATION")
		s.EvacuationCapacity = attrIntPtr(a, "EVACUATION_CAPACITY")
		s.PostImpactCapacity = attrIntPtr(a, "POST_IMPACT_CAPACITY")
		s.Latitude = attrFloatPtr(a, "LATITUDE")
		s.Longitude = attrFloatPtr(a, "LONGITUDE")
		out = append(out, s)
	}
	return out, nil
}

// cleanCity strips the trailing comma/space the Open_Shelters CITY column
// sometimes carries ("Highland," -> "Highland").
func cleanCity(s string) string {
	return strings.TrimRight(strings.TrimSpace(s), ", ")
}

// overlayOccupancy overlays the Open_Shelters occupancy feed onto the (already
// FEMA-union-Red-Cross) PUBLIC feed as a FILL-ONLY pass. An occupancy row is the
// same physical shelter as a unioned row when their alphanumeric-normalized names
// and states match AND their 5-digit ZIPs are compatible (equal, or at least one
// missing) -- the same identity test the Red Cross union uses, because
// Open_Shelters has no FEMA shelter_id to join on. On a match the live occupancy
// (and any still-empty descriptive gaps) are filled in via fillOccupancy and
// "+occupancy" is appended to the row's provenance.
//
// A row with NO match is withheld, never appended. Open_Shelters is the Red Cross
// operational roster and includes sites kept off the public map; with no public
// corroboration (it is absent from both FEMA's public feed and the public Red
// Cross EA view) and no visibility flag to consult, the CLI does not surface it.
// The base slice is copied so the caller's slice is not mutated underneath it.
// Returns the filled slice plus the counts of shelters filled and operational-only
// rows withheld.
func overlayOccupancy(base, occ []Shelter) (out []Shelter, filled, withheld int) {
	out = make([]Shelter, len(base))
	copy(out, base)
	idx := map[string][]int{}
	for i := range out {
		k := normName(out[i].Name) + "|" + out[i].State
		idx[k] = append(idx[k], i)
	}
	merged := map[int]bool{} // base indices already filled by an occupancy row
	for _, r := range occ {
		if normName(r.Name) == "" {
			continue // no usable name -> cannot match; not counted as withheld
		}
		k := normName(r.Name) + "|" + r.State
		matched := -1
		for _, ci := range idx[k] {
			// Only fill an un-consumed base row; a second occupancy row that keys the
			// same does not get its own row (fill-only), it is withheld like any other
			// row without a distinct public match.
			if !merged[ci] && zipCompatible(out[ci], r) {
				matched = ci
				break
			}
		}
		if matched >= 0 {
			fillOccupancy(&out[matched], r)
			out[matched].Source = addOccupancySource(out[matched].Source)
			merged[matched] = true
			filled++
		} else {
			withheld++ // operational-only: not in any public feed, so not shown
		}
	}
	return out, filled, withheld
}

// fillOccupancy folds an Open_Shelters record onto a unioned shelter. The
// population breakdown and capacity are filled ONLY where the destination has no
// value: the FEMA spine and its FEMA_NSS/0 enrichment stay authoritative where
// they reported a number, and Open_Shelters supplies the count where they left a
// null (which, on the public mirrors, is everywhere). A null in Open_Shelters
// never clears an existing value. Descriptive fields cross over only to fill a
// gap, and only fields that share a coded vocabulary with FEMA are crossed (the
// status/facility/ADA/pet codes are FEMA-coded in this feed; pet uses rcPetCode).
func fillOccupancy(dst *Shelter, src Shelter) {
	if dst.TotalPopulation == nil {
		dst.TotalPopulation = src.TotalPopulation
	}
	if dst.GeneralPopulation == nil {
		dst.GeneralPopulation = src.GeneralPopulation
	}
	if dst.MedicalNeedsPopulation == nil {
		dst.MedicalNeedsPopulation = src.MedicalNeedsPopulation
	}
	if dst.OtherPopulation == nil {
		dst.OtherPopulation = src.OtherPopulation
	}
	if dst.PetPopulation == nil {
		dst.PetPopulation = src.PetPopulation
	}
	if dst.EvacuationCapacity == nil {
		dst.EvacuationCapacity = src.EvacuationCapacity
	}
	if dst.PostImpactCapacity == nil {
		dst.PostImpactCapacity = src.PostImpactCapacity
	}
	if dst.Address == "" {
		dst.Address = src.Address
	}
	if dst.City == "" {
		dst.City = src.City
	}
	if dst.Zip == "" {
		dst.Zip = src.Zip
	}
	if dst.CountyParish == "" {
		dst.CountyParish = src.CountyParish
	}
	if dst.IncidentName == "" {
		dst.IncidentName = src.IncidentName
	}
	if dst.IncidentNumber == "" {
		dst.IncidentNumber = src.IncidentNumber
	}
	if dst.ShelterOpenDate == "" {
		dst.ShelterOpenDate = src.ShelterOpenDate
	}
	if dst.FacilityType == "" {
		dst.FacilityType = src.FacilityType
	}
	if dst.ShelterStatusCode == "" {
		dst.ShelterStatusCode = src.ShelterStatusCode
	}
	if dst.ADACompliant == "" {
		dst.ADACompliant = src.ADACompliant
	}
	if dst.WheelchairAccessible == "" {
		dst.WheelchairAccessible = src.WheelchairAccessible
	}
	if dst.GeneratorOnsite == "" {
		dst.GeneratorOnsite = src.GeneratorOnsite
	}
	if dst.PetAccommodations == "" {
		dst.PetAccommodations = src.PetAccommodations
	}
	if dst.PetAccommodationsDesc == "" {
		dst.PetAccommodationsDesc = src.PetAccommodationsDesc
	}
	if dst.Latitude == nil && dst.Longitude == nil && src.Latitude != nil && src.Longitude != nil {
		dst.Latitude, dst.Longitude = src.Latitude, src.Longitude
	}
}

// addOccupancySource appends the occupancy provenance to an existing source tag
// ("fema" -> "fema+occupancy", "fema+redcross" -> "fema+redcross+occupancy"),
// idempotently. An empty source becomes "occupancy".
func addOccupancySource(src string) string {
	if src == "" {
		return "occupancy"
	}
	if strings.Contains(src, "occupancy") {
		return src
	}
	return src + "+occupancy"
}

// applyOccupancyOverlay best-effort overlays the Open_Shelters occupancy feed onto
// the public union in place and reports what happened. Skipped (no network) under
// --no-enrich or offline data sources; a fetch failure degrades to null occupancy
// with an honest note, never an error. Run AFTER applyRedCrossUnion so occupancy
// fills population onto FEMA and Red Cross rows alike; it never adds a shelter, so
// operational-only Open_Shelters rows (kept off the public map) are withheld.
func applyOccupancyOverlay(ctx context.Context, flags *rootFlags, feed *shelterFeed, spineSource string) enrichState {
	if flags.noEnrich {
		return enrichState{Note: "Live occupancy skipped (--no-enrich); population and capacity are omitted."}
	}
	if flags.dataSource == "local" || spineSource == "local" {
		return enrichState{Note: "Live occupancy skipped (offline / --data-source local); population and capacity are omitted."}
	}
	occ, err := fetchOccupancy(ctx)
	if err != nil {
		return enrichState{Attempted: true, Note: "Live occupancy (Red Cross Open_Shelters) unavailable; population and capacity are null where the other feeds did not report them. Spine data is unaffected."}
	}
	var filled, withheld int
	feed.Shelters, filled, withheld = overlayOccupancy(feed.Shelters, occ)
	note := fmt.Sprintf("Merged live occupancy onto %d publicly listed shelter(s).", filled)
	if withheld > 0 {
		note += fmt.Sprintf(" Withheld %d shelter(s) listed only in the Red Cross operational roster and not in either public feed; the CLI shows only publicly listed shelters.", withheld)
	}
	return enrichState{Attempted: true, OK: true, Note: note}
}
