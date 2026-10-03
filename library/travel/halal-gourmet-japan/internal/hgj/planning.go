// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package hgj

import (
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"time"
)

func ParseSelections(restaurants, prayer []string) ([]Selection, error) {
	out := []Selection{}
	seen := map[string]bool{}
	for _, group := range []struct {
		kind string
		ids  []string
	}{{Restaurant, restaurants}, {Prayer, prayer}} {
		if len(group.ids) > 20 {
			return nil, fmt.Errorf("select at most 20 IDs per source kind")
		}
		for _, id := range group.ids {
			id = strings.TrimSpace(id)
			if _, err := CanonicalURL(group.kind, id); err != nil {
				return nil, fmt.Errorf("invalid %s ID %q: %w", group.kind, id, err)
			}
			key := group.kind + ":" + id
			if !seen[key] {
				seen[key] = true
				out = append(out, Selection{group.kind, id})
			}
		}
	}
	return out, nil
}

type PlaceRef struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	SourceURL  string `json:"source_url"`
	ObservedAt string `json:"observed_at"`
}

func ref(p Place) PlaceRef { return PlaceRef{p.ID, p.Kind, p.Name, p.SourceURL, p.ObservedAt} }

type Requirement struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	State     string `json:"state"`
	SourceURL string `json:"source_url"`
}
type MatchRow struct {
	PlaceRef
	Requirements []Requirement `json:"requirements"`
	Status       string        `json:"status"`
}

// Match evaluates every requested label from full detail; inapplicable is not a supported match.
func Match(places []Place, requested []string) ([]MatchRow, error) {
	if len(requested) == 0 {
		return nil, fmt.Errorf("provide --require with one or more source condition keys")
	}
	if len(requested) > 14 {
		return nil, fmt.Errorf("--require accepts at most 14 conditions")
	}
	keys := []string{}
	seen := map[string]bool{}
	for _, k := range requested {
		k, err := NormalizeCondition(k)
		if err != nil {
			return nil, err
		}
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	out := []MatchRow{}
	for _, p := range places {
		if p.EvidenceScope != "detail" {
			return nil, fmt.Errorf("%s:%s needs a full-detail inspection before matching", p.Kind, p.ID)
		}
		r := MatchRow{PlaceRef: ref(p), Requirements: []Requirement{}, Status: "all_requested_labels_reported"}
		applicable := 0
		for _, k := range keys {
			c, ok := p.Conditions[k]
			if !ok {
				state := Inapplicable
				if contains(ConditionKeys(p.Kind), k) {
					state = NotReported
				}
				c = Condition{Label: labels[k], State: state, SourceURL: p.SourceURL}
			}
			if c.State != Inapplicable {
				applicable++
			}
			if c.State != Reported {
				r.Status = "needs_confirmation"
			}
			r.Requirements = append(r.Requirements, Requirement{k, c.Label, c.State, p.SourceURL})
		}
		if applicable == 0 {
			r.Status = "requirements_inapplicable"
		}
		out = append(out, r)
	}
	return out, nil
}

type Gap struct {
	Field    string `json:"field"`
	Question string `json:"question"`
}
type GapRow struct {
	PlaceRef
	Gaps                []Gap   `json:"gaps"`
	ObservationAgeHours float64 `json:"observation_age_hours"`
}

// Gaps lists fixed clarification questions; it never scores halal or access confidence.
func Gaps(places []Place, now time.Time) ([]GapRow, error) {
	out := []GapRow{}
	for _, p := range places {
		if p.EvidenceScope != "detail" {
			return nil, fmt.Errorf("%s:%s needs a full-detail inspection before gap triage", p.Kind, p.ID)
		}
		at, err := time.Parse(time.RFC3339Nano, p.ObservedAt)
		if err != nil {
			return nil, fmt.Errorf("invalid saved observation time for %s:%s", p.Kind, p.ID)
		}
		age := now.Sub(at).Hours()
		if age < 0 {
			age = 0
		}
		r := GapRow{PlaceRef: ref(p), Gaps: []Gap{}, ObservationAgeHours: math.Round(age*100) / 100}
		if p.Kind == Restaurant {
			if p.Certification.LabelState != Reported {
				r.Gaps = append(r.Gaps, Gap{factPath("certification", "label"), "Does the venue hold halal certification, and what is its scope?"})
			}
			if p.Certification.Certifier.State != Reported {
				r.Gaps = append(r.Gaps, Gap{factPath("certification", "certifier"), "Which body issued the certification, and can its source be checked?"})
			}
			if p.Certification.ValidUntil.State != Reported {
				r.Gaps = append(r.Gaps, Gap{factPath("certification", "valid_until"), "What validity or expiry evidence is available for the certification?"})
			}
			if p.Verification.State != Reported {
				r.Gaps = append(r.Gaps, Gap{"verification", "Is there a dated HGJ verification record for this listing?"})
			}
		}
		if p.HoursState != Reported {
			r.Gaps = append(r.Gaps, Gap{"hours", "What hours and any day-specific exceptions apply to this place?"})
		}
		if p.Kind == Prayer && p.AccessState != Reported {
			r.Gaps = append(r.Gaps, Gap{"access", "Who may use the prayer place, and are there booking, customer-only, security or equipment restrictions?"})
		}
		if !validCoordinates(p.Coordinates) {
			r.Gaps = append(r.Gaps, Gap{"coordinates", "What exact location coordinates are provided for this place?"})
		}
		out = append(out, r)
	}
	return out, nil
}
func validCoordinates(g *Coordinates) bool {
	return g != nil && !math.IsNaN(g.Latitude) && !math.IsNaN(g.Longitude) && !math.IsInf(g.Latitude, 0) && !math.IsInf(g.Longitude, 0) && g.Latitude >= -90 && g.Latitude <= 90 && g.Longitude >= -180 && g.Longitude <= 180
}
func distance(a, b *Coordinates) float64 {
	r := math.Pi / 180
	lat1 := a.Latitude * r
	lat2 := b.Latitude * r
	dLat := (b.Latitude - a.Latitude) * r
	dLon := (b.Longitude - a.Longitude) * r
	v := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	if v > 1 {
		v = 1
	}
	return 6371.0088 * 2 * math.Atan2(math.Sqrt(v), math.Sqrt(1-v))
}

type PairRow struct {
	Restaurant             Place   `json:"restaurant"`
	Prayer                 Place   `json:"prayer"`
	StraightLineDistanceKM float64 `json:"straight_line_distance_km"`
}
type PairResult struct {
	Results                []PairRow `json:"results"`
	CandidatePairs         int       `json:"candidate_pairs"`
	SkippedCoordinatePairs int       `json:"skipped_coordinate_pairs"`
	MatchedPairs           int       `json:"matched_pairs"`
	Truncated              bool      `json:"truncated"`
	Note                   string    `json:"note"`
}

// Pair joins bounded full-detail selections using great-circle distance, not route distance or visitability.
func Pair(places []Place, maxKM float64, limit int) (PairResult, error) {
	out := PairResult{Results: []PairRow{}, Note: "Straight-line proximity only; no walking route, travel time, current opening or access guarantee. Source access notes and observation dates remain independent."}
	if math.IsNaN(maxKM) || math.IsInf(maxKM, 0) || maxKM <= 0 || maxKM > 100 {
		return out, fmt.Errorf("--max-km must be finite and greater than 0 up to 100")
	}
	if limit < 1 || limit > 50 {
		return out, fmt.Errorf("--limit must be between 1 and 50")
	}
	rest := []Place{}
	prayer := []Place{}
	for _, p := range places {
		if p.EvidenceScope != "detail" {
			return out, fmt.Errorf("%s:%s needs a full-detail inspection before pairing", p.Kind, p.ID)
		}
		if p.Kind == Restaurant {
			rest = append(rest, p)
		} else if p.Kind == Prayer {
			prayer = append(prayer, p)
		}
	}
	if len(rest) > 20 || len(prayer) > 20 {
		return out, fmt.Errorf("pair at most 20 IDs per source kind")
	}
	for _, r := range rest {
		for _, p := range prayer {
			out.CandidatePairs++
			if !validCoordinates(r.Coordinates) || !validCoordinates(p.Coordinates) {
				out.SkippedCoordinatePairs++
				continue
			}
			km := distance(r.Coordinates, p.Coordinates)
			if km <= maxKM {
				out.Results = append(out.Results, PairRow{r, p, math.Round(km*1000) / 1000})
			}
		}
	}
	sort.Slice(out.Results, func(i, j int) bool {
		a, b := out.Results[i], out.Results[j]
		if a.StraightLineDistanceKM != b.StraightLineDistanceKM {
			return a.StraightLineDistanceKM < b.StraightLineDistanceKM
		}
		return a.Restaurant.ID+":"+a.Prayer.ID < b.Restaurant.ID+":"+b.Prayer.ID
	})
	out.MatchedPairs = len(out.Results)
	if len(out.Results) > limit {
		out.Results = out.Results[:limit]
		out.Truncated = true
	}
	if len(out.Results) == 0 {
		out.Note += " No coordinate pairs matched the selected records and --max-km threshold; inspect missing records/coordinates or widen --max-km."
	}
	return out, nil
}

type FieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
	Note   string `json:"note,omitempty"`
}
type ChangeRow struct {
	PlaceRef
	BeforeObservedAt string        `json:"before_observed_at,omitempty"`
	AfterObservedAt  string        `json:"after_observed_at"`
	State            string        `json:"state"`
	Changes          []FieldChange `json:"changes"`
}

// Changes ignores observation timestamps and compares factual fields only.
func Changes(before *Place, after Place) (ChangeRow, error) {
	out := ChangeRow{PlaceRef: ref(after), AfterObservedAt: after.ObservedAt, State: "baseline_missing", Changes: []FieldChange{}}
	if after.EvidenceScope != "detail" {
		return out, fmt.Errorf("latest observation must be full detail")
	}
	if before == nil {
		return out, nil
	}
	if before.EvidenceScope != "detail" || before.Kind != after.Kind || before.ID != after.ID {
		return out, fmt.Errorf("changes need two full-detail observations of the same source kind/ID")
	}
	out.BeforeObservedAt = before.ObservedAt
	out.State = "compared"
	b, a := facts(*before), facts(after)
	keys := []string{}
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !reflect.DeepEqual(b[k], a[k]) {
			c := FieldChange{Field: k, Before: b[k], After: a[k]}
			if strings.HasPrefix(k, "conditions.") && b[k] == Reported && a[k] == NotReported {
				c.Note = "The label is no longer reported in this observation; this is not an explicit negative."
			}
			out.Changes = append(out.Changes, c)
		}
	}
	return out, nil
}
func facts(p Place) map[string]any {
	out := map[string]any{"name": p.Name, "name_japanese": p.NameJapanese, "prefecture": p.Prefecture, "category": p.Category, "prayer_type": p.PrayerType, "address": p.Address, "coordinates": p.Coordinates, "weekly_hours": p.WeeklyHours, "hours_state": p.HoursState, "hours_notes": p.HoursNotes, "access_notes": p.AccessNotes, "access_state": p.AccessState, "access_notes_truncated": p.AccessNotesTruncated, "price_range": p.PriceRange, factPath("verification", "state"): p.Verification.State, factPath("verification", "month"): p.Verification.Month, factPath("certification", "certifier"): p.Certification.Certifier, factPath("certification", "valid_until"): p.Certification.ValidUntil}
	for k := range labels {
		c, ok := p.Conditions[k]
		if !ok {
			c.State = Inapplicable
			if contains(ConditionKeys(p.Kind), k) {
				c.State = NotReported
			}
		}
		out["conditions."+k] = c.State
	}
	return out
}

func factPath(parts ...string) string { return strings.Join(parts, ".") }
