// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// FEMA_NSS/0 enrichment: the OpenShelters feed (the authoritative spine) carries
// the open/full status and the core fields, but FEMA's richer
// FEMA_NSS/FeatureServer/0 layer adds county, the driving incident, generator /
// floodplain / surge attributes, a population breakdown, and org contact lines.
// This file fetches that layer best-effort and merges the extra fields onto the
// spine, joined on the STABLE shelter_id (verified 1:1 and same-physical-shelter
// across both feeds). It NEVER fails a command: any fetch/parse error degrades to
// null extra fields plus an honest note.

package cli

import (
	"context"
	"net/url"
	"strings"
	"time"
)

// enrichState records what happened during enrichment so a consumer can tell a
// genuine null (the feed did not report a field) from a missed fetch (we never
// got the data). It rides alongside each command's payload.
type enrichState struct {
	Attempted bool   `json:"attempted"`
	OK        bool   `json:"ok"`
	Note      string `json:"note,omitempty"`
}

// humanNote returns a one-line note worth showing a human ONLY when enrichment
// did not fully succeed (skipped or failed); a clean success stays quiet.
func (e enrichState) humanNote() string {
	if e.OK || e.Note == "" {
		return ""
	}
	return e.Note
}

// joinNotes joins the non-empty notes with newlines (used where a single note
// string is rendered, e.g. the shelter detail view).
func joinNotes(notes ...string) string {
	out := make([]string, 0, len(notes))
	for _, n := range notes {
		if n != "" {
			out = append(out, n)
		}
	}
	return strings.Join(out, "\n")
}

// fetchEnrichment fetches and parses the FEMA_NSS/0 enrichment layer, returning
// the extra fields keyed by shelter_id. It is a package var so tests can stub it
// without touching the network.
var fetchEnrichment = femaNSSEnrich

// femaNSSEnrich fetches FEMA's richer FEMA_NSS/FeatureServer/0 layer.
//
// This is deliberately a plain best-effort GET, OUTSIDE resolveReadWithStrategy
// and the response cache, for three reasons: (1) it is optional augmentation of
// the authoritative OpenShelters spine and must degrade silently to null, so the
// caller turns any error here into a note rather than a command failure;
// (2) caching a side feed risks serving enrichment that disagrees with a freshly
// fetched spine; (3) it is skipped entirely under --data-source local /
// --no-enrich / --fixture (see applyEnrichment), so it never runs when the user
// asked to stay offline. The stub-able package var above keeps it unit-testable.
func femaNSSEnrich(ctx context.Context) (map[int]Shelter, error) {
	v := url.Values{}
	v.Set("where", "1=1")
	// Request only the fields we actually merge, not "*". This bounds the
	// response size (the full 72-column layer can be large during a big event),
	// and as defense-in-depth keeps the layer's org_address / org_email /
	// mailing / POC personal-contact columns off the wire entirely (we never
	// parse them; not fetching them means they never enter the process). If a
	// future schema rename drops a field, that field simply comes back absent
	// and enrichment degrades field-by-field rather than failing.
	v.Set("outFields", strings.Join(enrichOutFields, ","))
	v.Set("returnGeometry", "false")
	v.Set("f", "json")
	u := openSheltersBase + femaNSSQuery + "?" + v.Encode()
	body, err := httpGetJSON(ctx, u)
	if err != nil {
		return nil, err
	}
	return parseEnrichment(body)
}

// enrichOutFields is the exact set of FEMA_NSS/0 columns femaNSSEnrich requests:
// the shelter_id join key plus every field parseEnrichment reads. Personal /
// postal-contact columns are deliberately excluded.
var enrichOutFields = []string{
	"shelter_id", "county_parish", "facility_type", "shelter_status_code",
	"incident_name", "incident_number", "shelter_open_date", "shelter_closed_date",
	"generator_onsite", "self_sufficient_electricity", "in_100_yr_floodplain",
	"in_500_yr_floodplain", "in_surge_slosh_area", "pre_landfall_shelter",
	"population_code", "general_population", "medical_needs_population",
	"other_population", "pet_population", "pet_accommodations_desc",
	"org_main_phone", "org_hotline_phone",
}

// parseEnrichment decodes a FEMA_NSS/0 query response into per-shelter enrichment
// records keyed by shelter_id. Only the extra fields (and shelter_id) are read;
// the overlapping core fields keep FEMA_NSS's own column names (address_1,
// shelter_status_code, org_organization_name, ...) which differ from the spine,
// so they are intentionally NOT mapped here. A record without a shelter_id has no
// join key and is dropped. Shares decodeFeatures, so a wrong-shape body fails
// loudly exactly like the spine parser.
func parseEnrichment(raw []byte) (map[int]Shelter, error) {
	attrs, err := decodeFeatures(raw)
	if err != nil {
		return nil, err
	}
	out := make(map[int]Shelter, len(attrs))
	dup := map[int]bool{}
	for _, a := range attrs {
		id := attrIntPtr(a, "shelter_id")
		if id == nil || *id == 0 {
			continue // no stable join key
		}
		if dup[*id] {
			continue // already known-ambiguous; never re-add
		}
		if _, exists := out[*id]; exists {
			// The join assumes shelter_id is 1:1 in this feed (verified against
			// the live data). If it ever is not (e.g. a closed and an open
			// record share an id during an event), we cannot tell which is
			// authoritative, so we drop the id entirely: the spine shelter gets
			// honest-null enrichment rather than one record's incident/dates
			// silently misattributed onto it.
			delete(out, *id)
			dup[*id] = true
			continue
		}
		s := Shelter{ShelterID: *id}
		// Proper-noun / free-text fields: trim only, preserve case.
		s.CountyParish = attrStr(a, "county_parish")
		s.IncidentName = attrStr(a, "incident_name")
		s.IncidentNumber = attrStr(a, "incident_number")
		s.PetAccommodationsDesc = attrStr(a, "pet_accommodations_desc")
		s.OrgMainPhone = attrStr(a, "org_main_phone")
		s.OrgHotlinePhone = attrStr(a, "org_hotline_phone")
		// Coded vocabularies: normalize (trim + upper) so filters/labels match
		// the feed's UNK / YES / NO / SHELTER / OPEN regardless of casing.
		s.FacilityType = normCode(attrStr(a, "facility_type"))
		s.ShelterStatusCode = normCode(attrStr(a, "shelter_status_code"))
		s.GeneratorOnsite = normCode(attrStr(a, "generator_onsite"))
		s.SelfSufficientElectricity = normCode(attrStr(a, "self_sufficient_electricity"))
		s.In100YrFloodplain = normCode(attrStr(a, "in_100_yr_floodplain"))
		s.In500YrFloodplain = normCode(attrStr(a, "in_500_yr_floodplain"))
		s.InSurgeSloshArea = normCode(attrStr(a, "in_surge_slosh_area"))
		s.PreLandfallShelter = normCode(attrStr(a, "pre_landfall_shelter"))
		s.PopulationCode = normCode(attrStr(a, "population_code"))
		// Epoch-millis dates -> UTC calendar date.
		s.ShelterOpenDate = attrEpochDateStr(a, "shelter_open_date")
		s.ShelterClosedDate = attrEpochDateStr(a, "shelter_closed_date")
		// Nullable population breakdown.
		s.GeneralPopulation = attrIntPtr(a, "general_population")
		s.MedicalNeedsPopulation = attrIntPtr(a, "medical_needs_population")
		s.OtherPopulation = attrIntPtr(a, "other_population")
		s.PetPopulation = attrIntPtr(a, "pet_population")
		out[*id] = s
	}
	return out, nil
}

// mergeEnrichment copies the FEMA_NSS extra fields from em onto each spine
// shelter, matched by shelter_id. The spine's authoritative core fields (status,
// address, org, coords, capacity) are left untouched. Spine rows with no match
// keep their empty/nil enrichment fields. A spine row with shelter_id 0 (the
// feed omitted the key) is skipped so it can never accidentally match an em[0]
// entry. Mutates spine in place.
func mergeEnrichment(spine []Shelter, em map[int]Shelter) {
	for i := range spine {
		if spine[i].ShelterID == 0 {
			continue
		}
		e, ok := em[spine[i].ShelterID]
		if !ok {
			continue
		}
		spine[i].CountyParish = e.CountyParish
		spine[i].FacilityType = e.FacilityType
		spine[i].ShelterStatusCode = e.ShelterStatusCode
		spine[i].IncidentName = e.IncidentName
		spine[i].IncidentNumber = e.IncidentNumber
		spine[i].ShelterOpenDate = e.ShelterOpenDate
		spine[i].ShelterClosedDate = e.ShelterClosedDate
		spine[i].GeneratorOnsite = e.GeneratorOnsite
		spine[i].SelfSufficientElectricity = e.SelfSufficientElectricity
		spine[i].In100YrFloodplain = e.In100YrFloodplain
		spine[i].In500YrFloodplain = e.In500YrFloodplain
		spine[i].InSurgeSloshArea = e.InSurgeSloshArea
		spine[i].PreLandfallShelter = e.PreLandfallShelter
		spine[i].PopulationCode = e.PopulationCode
		spine[i].GeneralPopulation = e.GeneralPopulation
		spine[i].MedicalNeedsPopulation = e.MedicalNeedsPopulation
		spine[i].OtherPopulation = e.OtherPopulation
		spine[i].PetPopulation = e.PetPopulation
		spine[i].PetAccommodationsDesc = e.PetAccommodationsDesc
		spine[i].OrgMainPhone = e.OrgMainPhone
		spine[i].OrgHotlinePhone = e.OrgHotlinePhone
	}
}

// applyEnrichment best-effort augments the spine in place and reports what
// happened. Enrichment is skipped (no network) when the user opted out
// (--no-enrich) or asked to stay offline (--data-source local, or the spine
// itself came from the local store); in those cases the extra fields are simply
// omitted and the note says so. A fetch/parse failure also degrades to a note,
// never an error, so the core command still succeeds.
func applyEnrichment(ctx context.Context, flags *rootFlags, shelters []Shelter, spineSource string) enrichState {
	if flags.noEnrich {
		return enrichState{Note: "Enrichment disabled (--no-enrich); FEMA_NSS/0 fields are omitted."}
	}
	if flags.dataSource == "local" || spineSource == "local" {
		return enrichState{Note: "Enrichment skipped (offline / --data-source local); FEMA_NSS/0 fields are omitted."}
	}
	em, err := fetchEnrichment(ctx)
	if err != nil {
		return enrichState{Attempted: true, Note: "Enrichment (FEMA_NSS/0) fetch failed; the extended fields are null. Spine data is unaffected."}
	}
	mergeEnrichment(shelters, em)
	return enrichState{Attempted: true, OK: true}
}

// attrEpochDateStr reads an epoch-milliseconds attribute and renders the UTC
// calendar date (YYYY-MM-DD). FEMA stamps shelter_open_date / shelter_closed_date
// as epoch millis; "" is returned when the field is absent. UTC is explicit so
// the rendered date never shifts by the machine's local timezone.
func attrEpochDateStr(m map[string]any, k string) string {
	if v, ok := m[k].(float64); ok {
		return time.UnixMilli(int64(v)).UTC().Format("2006-01-02")
	}
	return ""
}
