package parks

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type Check struct {
	Field     string   `json:"field"`
	Status    string   `json:"status"`
	Actual    any      `json:"actual"`
	Published any      `json:"published"`
	Evidence  []string `json:"evidence"`
}
type Vehicle struct {
	LengthM    float64
	WidthM     float64
	HeightM    float64
	Category   string
	Membership string
}
type FitResult struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	Decision      string  `json:"decision"`
	Checks        []Check `json:"checks"`
	ObservedAt    string  `json:"observed_at"`
	SourceUpdated string  `json:"source_updated_date_jst"`
	Note          string  `json:"note"`
}

func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func validMember(v string) bool {
	switch v {
	case "", "unknown", "nonmember", "member", "standard", "premium":
		return true
	}
	return false
}
func memberCheck(p Park, member string) Check {
	c := Check{Field: "membership", Status: "unknown", Actual: member, Published: p.Membership.Status, Evidence: []string{p.Membership.Evidence}}
	switch p.Membership.Status {
	case "not_required_stated":
		c.Status = "yes"
	case "required":
		if member == "nonmember" {
			c.Status = "no"
		} else if member != "" && member != "unknown" {
			if premiumMembershipRequired(p.Membership.Evidence) {
				c.Published = "premium_required"
				switch member {
				case "premium":
					c.Status = "yes"
				case "standard":
					c.Status = "no"
					// Generic membership does not declare a qualifying tier.
				}
			} else if strings.Contains(p.Membership.Evidence, "プレミアム会員") {
				// Qualified or ambiguous tier text cannot prove eligibility.
				c.Status = "unknown"
			} else {
				c.Status = "yes"
			}
		}
	}
	return c
}
func decision(checks []Check, yes, no, unknown string) string {
	hasUnknown := false
	for _, c := range checks {
		if c.Status == "no" {
			return no
		}
		if c.Status != "yes" {
			hasUnknown = true
		}
	}
	if hasUnknown {
		return unknown
	}
	return yes
}
func Fit(p Park, v Vehicle) (FitResult, error) {
	if !finite(v.LengthM) || !finite(v.WidthM) || !finite(v.HeightM) || v.LengthM <= 0 || v.WidthM <= 0 || v.HeightM <= 0 {
		return FitResult{}, fmt.Errorf("--length-m, --width-m and --height-m must be positive finite measurements in metres")
	}
	if v.LengthM > 100 || v.WidthM > 20 || v.HeightM > 20 {
		return FitResult{}, fmt.Errorf("vehicle measurements exceed supported planning bounds (100m length, 20m width/height)")
	}
	label, e := VehicleLabel(v.Category)
	if e != nil {
		return FitResult{}, e
	}
	if label == "" {
		return FitResult{}, fmt.Errorf("--vehicle is required, for example van or cab")
	}
	if !validMember(v.Membership) {
		return FitResult{}, fmt.Errorf("--membership must be nonmember, member, standard, premium or unknown")
	}
	out := FitResult{ID: p.ID, Name: p.Name, URL: p.URL, Checks: []Check{}, ObservedAt: p.ObservedAt, SourceUpdated: p.SourceUpdated, Note: "Published evidence only. Confirm access, actual pitch allocation, current operation and booking with the host; this is not road clearance or campsite permission."}
	flexible := strings.Contains(p.Sections["駐車場"], "はみ出し可能") || strings.Contains(p.Sections["駐車場"], "複数区画利用 可")
	for _, x := range []struct {
		field     string
		actual    float64
		max       *float64
		unlimited bool
	}{{"length_m", v.LengthM, p.Dimensions.LengthM, false}, {"width_m", v.WidthM, p.Dimensions.WidthM, false}, {"height_m", v.HeightM, p.Dimensions.HeightM, p.Dimensions.HeightUnrestricted}} {
		c := Check{Field: x.field, Status: "unknown", Actual: x.actual, Published: x.max, Evidence: []string{p.Dimensions.Raw}}
		if x.unlimited {
			c.Status = "yes"
			c.Published = "explicitly unrestricted"
		} else if x.max != nil {
			if x.actual <= *x.max {
				c.Status = "yes"
			} else if flexible && x.field != "height_m" {
				c.Evidence = append(c.Evidence, p.Sections["駐車場"])
			} else {
				c.Status = "no"
			}
		}
		out.Checks = append(out.Checks, c)
	}
	vc := Check{Field: "vehicle_category", Status: "unknown", Actual: label, Published: p.Vehicles, Evidence: append([]string{}, p.Vehicles...)}
	if len(p.Vehicles) > 0 {
		vc.Status = "no"
		for _, v := range p.Vehicles {
			if v == label {
				vc.Status = "yes"
			}
		}
	}
	out.Checks = append(out.Checks, vc, memberCheck(p, v.Membership))
	out.Decision = decision(out.Checks, "compatible_with_published_limits", "exceeds_or_conflicts_with_published_limits", "needs_confirmation")
	return out, nil
}

type Requirement struct {
	Facility string
	Fee      string
}

func ParseRequirements(raw string) ([]Requirement, error) {
	out := []Requirement{}
	if strings.TrimSpace(raw) == "" {
		return out, fmt.Errorf("--require must list facilities, for example electricity,water,pets")
	}
	if len(strings.Split(raw, ",")) > 16 {
		return nil, fmt.Errorf("at most 16 requirements are supported")
	}
	for _, part := range strings.Split(raw, ",") {
		bits := strings.Split(strings.TrimSpace(part), "=")
		if len(bits) > 2 {
			return nil, fmt.Errorf("use facility or facility=free/paid/included")
		}
		key := bits[0]
		known := false
		for _, k := range FacilityKeys {
			if key == k {
				known = true
			}
		}
		if !known {
			return nil, fmt.Errorf("unknown required facility %q; supported: %s", key, strings.Join(FacilityKeys, ","))
		}
		r := Requirement{Facility: key}
		if len(bits) == 2 {
			r.Fee = bits[1]
			if r.Fee != "free" && r.Fee != "paid" && r.Fee != "included" {
				return nil, fmt.Errorf("fee requirement must be free, paid or included")
			}
		}
		out = append(out, r)
	}
	return out, nil
}

type MatchResult struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	URL           string  `json:"url"`
	Decision      string  `json:"decision"`
	Checks        []Check `json:"checks"`
	ObservedAt    string  `json:"observed_at"`
	SourceUpdated string  `json:"source_updated_date_jst"`
}

func Match(p Park, requirements []Requirement, member string) (MatchResult, error) {
	if len(requirements) == 0 {
		return MatchResult{}, fmt.Errorf("at least one required service is needed")
	}
	if !validMember(member) {
		return MatchResult{}, fmt.Errorf("invalid membership value")
	}
	out := MatchResult{ID: p.ID, Name: p.Name, URL: p.URL, Checks: []Check{}, ObservedAt: p.ObservedAt, SourceUpdated: p.SourceUpdated}
	for _, r := range requirements {
		f, ok := p.Facilities[r.Facility]
		if !ok {
			f = Facility{Status: "unknown", Fee: "unknown", Evidence: []string{}}
		}
		c := Check{Field: r.Facility, Status: f.Status, Actual: "required", Published: f.Status, Evidence: append([]string{}, f.Evidence...)}
		if c.Status == "" {
			c.Status = "unknown"
		}
		out.Checks = append(out.Checks, c)
		if r.Fee != "" {
			c = Check{Field: r.Facility + ".fee", Status: "unknown", Actual: r.Fee, Published: f.Fee, Evidence: append([]string{}, f.Evidence...)}
			if f.Fee != "unknown" && f.Fee != "" {
				c.Status = "no"
				if f.Fee == r.Fee {
					c.Status = "yes"
				}
			}
			out.Checks = append(out.Checks, c)
		}
	}
	if member != "" && member != "unknown" {
		out.Checks = append(out.Checks, memberCheck(p, member))
	}
	if p.SourceLevel != "detail" {
		out.Checks = append(out.Checks, Check{Field: "detail_observation", Status: "unknown", Actual: "required", Published: p.SourceLevel, Evidence: []string{"Search cards do not provide all per-record conditions."}})
	}
	out.Decision = decision(out.Checks, "proven_match", "ruled_out", "needs_confirmation")
	return out, nil
}

type NearResult struct {
	ID            string       `json:"id"`
	Name          string       `json:"name"`
	URL           string       `json:"url"`
	DistanceKM    float64      `json:"distance_km"`
	Coordinates   *Coordinates `json:"coordinates"`
	ObservedAt    string       `json:"observed_at"`
	DistanceBasis string       `json:"distance_basis"`
}

func Near(ps []Park, lat, lon, radius float64, limit int) ([]NearResult, int, error) {
	out := []NearResult{}
	if !finite(lat) || !finite(lon) || lat < -90 || lat > 90 || lon < -180 || lon > 180 {
		return out, 0, fmt.Errorf("--latitude must be -90..90 and --longitude -180..180, both finite")
	}
	if !finite(radius) || radius <= 0 || radius > 20000 || limit < 1 || limit > 100 {
		return out, 0, fmt.Errorf("--radius-km must be positive and <=20000; --limit must be 1..100")
	}
	missing := 0
	for _, p := range ps {
		if p.Coordinates == nil {
			missing++
			continue
		}
		a, b := p.Coordinates.Latitude, p.Coordinates.Longitude
		if !finite(a) || !finite(b) || a < -90 || a > 90 || b < -180 || b > 180 {
			missing++
			continue
		}
		r := math.Pi / 180
		dlat, dlon := (a-lat)*r, (b-lon)*r
		h := math.Sin(dlat/2)*math.Sin(dlat/2) + math.Cos(lat*r)*math.Cos(a*r)*math.Sin(dlon/2)*math.Sin(dlon/2)
		if h > 1 {
			h = 1
		}
		if h < 0 {
			h = 0
		}
		dist := 6371.0088 * 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
		if dist <= radius {
			out = append(out, NearResult{p.ID, p.Name, p.URL, math.Round(dist*1000) / 1000, p.Coordinates, p.ObservedAt, "straight_line_haversine_not_driving_distance"})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].DistanceKM == out[j].DistanceKM {
			return out[i].ID < out[j].ID
		}
		return out[i].DistanceKM < out[j].DistanceKM
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, missing, nil
}

type Issue struct {
	Kind     string   `json:"kind"`
	Severity string   `json:"severity"`
	Evidence []string `json:"evidence"`
	Question string   `json:"question"`
	URL      string   `json:"source_url"`
}

func Audit(p Park, now time.Time) []Issue {
	out := []Issue{}
	add := func(k, severity, question string, ev ...string) {
		out = append(out, Issue{k, severity, append([]string{}, ev...), question, p.URL})
	}
	for _, w := range p.Warnings {
		if strings.HasPrefix(w, "detail_conflict_overrides_icon:") {
			add("facility_evidence_conflict", "warning", "Detailed restrictions override the icon. Confirm the current service with the facility.", w)
		} else {
			add("disabled_icon_positive_alt", "warning", "The disabled icon state was used. Confirm the actual service if needed.", w)
		}
	}
	if p.Dimensions.LengthM == nil || p.Dimensions.WidthM == nil || p.Dimensions.HeightM == nil && !p.Dimensions.HeightUnrestricted {
		add("dimension_evidence_missing", "confirmation", "Ask for missing vehicle/pitch dimension limits in metres.", p.Dimensions.Raw)
	}
	opening := p.Sections["利用可能期間"]
	if strings.Contains(opening, "通年") && (strings.Contains(opening, "休業") || strings.Contains(opening, "休館") || strings.Contains(opening, "降雪")) {
		add("opening_qualified", "warning", "Confirm the actual operating date despite the all-year label.", opening)
	}
	if p.Membership.Status == "unknown" {
		add("membership_unknown", "confirmation", "Ask whether this specific facility accepts nonmembers.", p.Membership.Evidence)
	}
	if p.Facilities["toilet_24h"].Status == "no" {
		add("toilet_hours_restricted", "confirmation", "Check toilet access during the intended overnight hours.", p.Sections["トイレ"])
	}
	if p.ObservedAt != "" {
		if observed, e := time.Parse(time.RFC3339, p.ObservedAt); e == nil && now.Sub(observed) > 7*24*time.Hour {
			add("observation_old", "confirmation", "Refresh this observation before relying on material conditions.", p.ObservedAt)
		}
	}
	if p.SourceUpdated != "" {
		t, e := time.ParseInLocation("2006-01-02", p.SourceUpdated, JST)
		if e == nil && now.Sub(t) > 180*24*time.Hour {
			add("source_update_old", "confirmation", "Confirm that published prices and rules still apply.", p.SourceUpdated)
		}
	}
	if v := p.Sections["ご利用に関する注意事項"]; strings.Contains(v, "焚き火") || strings.Contains(v, "調理") {
		add("outdoor_activity_scope", "confirmation", "Verify the exact requested activity and pitch rules; general camping permission is not inferred.", v)
	}
	return out
}

type CompareValue struct {
	ID    string `json:"id"`
	Value any    `json:"value"`
}
type CompareField struct {
	Field  string         `json:"field"`
	Values []CompareValue `json:"values"`
}

func Compare(ps []Park) []CompareField {
	out := []CompareField{}
	if len(ps) == 0 {
		return out
	}
	fields := []string{"url", "name", "dimensions", "vehicle_categories", "membership", "tariffs", "electricity", "water", "dump_station", "black_water", "grey_water", "toilet_24h", "bath", "shower", "pets", "garbage", "laundry", "generator", "opening", "booking", "source_updated_date_jst", "observed_at"}
	for _, field := range fields {
		row := CompareField{Field: field, Values: []CompareValue{}}
		for _, p := range ps {
			var v any
			switch field {
			case "url":
				v = p.URL
			case "name":
				v = p.Name
			case "dimensions":
				v = p.Dimensions
			case "vehicle_categories":
				v = p.Vehicles
			case "membership":
				v = p.Membership
			case "tariffs":
				v = p.Tariffs
			case "opening":
				v = p.Sections["利用可能期間"]
			case "booking":
				v = p.Booking
			case "observed_at":
				v = p.ObservedAt
			case "source_updated_date_jst":
				v = p.SourceUpdated
			default:
				v = p.Facilities[field]
			}
			row.Values = append(row.Values, CompareValue{p.ID, v})
		}
		out = append(out, row)
	}
	return out
}
