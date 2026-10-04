// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package trip

import (
	"fmt"
	"strings"
	"time"
)

func ValidDate(s string) bool {
	t, e := time.Parse("2006-01-02", s)
	return e == nil && t.Format("2006-01-02") == s
}
func ValidateQuery(q Query) error {
	if q.Kind != "" && q.Kind != "all" && q.Kind != "spots" && q.Kind != "events" {
		return fmt.Errorf("--kind must be spots, events or all")
	}
	if q.From != "" && !ValidDate(q.From) || q.To != "" && !ValidDate(q.To) {
		return fmt.Errorf("dates must use valid YYYY-MM-DD values")
	}
	if q.From != "" && q.To != "" && q.From > q.To {
		return fmt.Errorf("--from must be on or before --to")
	}
	if q.Limit < 1 || q.Limit > 50 {
		return fmt.Errorf("--limit must be from 1 to 50")
	}
	if q.AgeMonths < -1 || q.AgeMonths > 216 {
		return fmt.Errorf("--age-months must be from 0 to 216")
	}
	for _, a := range q.Amenities {
		switch a {
		case "indoor", "nursing", "changing", "stroller":
		default:
			return fmt.Errorf("unknown --amenities value %q; use indoor,nursing,changing,stroller", a)
		}
	}
	return nil
}
func window(s Schedule, from, to string) string {
	if from == "" && to == "" {
		return "not_requested"
	}
	if s.Start == "" || s.End == "" {
		return "unknown"
	}
	if from == "" {
		from = to
	}
	if to == "" {
		to = from
	}
	if s.End < from || s.Start > to {
		return "outside"
	}
	if s.Precision == "single_date" {
		return "published_date_overlap"
	}
	return "published_span_overlap"
}
func Evaluate(r Record, on, asOf string, ageMonths int, amenities []string) Assessment {
	out := Assessment{Record: r, Age: Check{Status: "not_requested"}, Schedule: Check{Status: "not_requested"}, Application: Check{Status: "unknown", Reason: "No explicit application interval was parsed; seat availability remains unknown."}, Amenities: map[string]Check{}, Overall: "not_requested"}
	if on != "" || ageMonths >= 0 || len(amenities) > 0 {
		out.Overall = "supported"
	}
	if ageMonths >= 0 {
		out.Age = Check{Status: "unknown", Reason: "No explicit numeric source age range is available."}
		if r.Age.Status == "published_range" && r.Age.MinMonths != nil && r.Age.MaxExclusiveMonths != nil {
			out.Age = Check{Status: "supported", Reason: "Within the published age description; this is not an admission or safety guarantee."}
			if ageMonths < *r.Age.MinMonths || ageMonths >= *r.Age.MaxExclusiveMonths {
				out.Age.Status = "excluded"
				out.Age.Reason = "Outside the explicitly published age description."
			}
		}
	}
	for _, a := range amenities {
		f := r.Amenities[a]
		c := Check{Status: "unknown", Reason: "The source did not make an unambiguous facility-level statement."}
		if f.Status == "reported_present" {
			c = Check{Status: "supported", Reason: "Explicit source statement; suitability and current availability remain unverified."}
		}
		if f.Status == "reported_absent" {
			c = Check{Status: "excluded", Reason: "The source explicitly reports this amenity absent."}
		}
		out.Amenities[a] = c
	}
	if on != "" {
		switch window(r.Schedule, on, on) {
		case "outside":
			out.Schedule = Check{Status: "excluded", Reason: "Date is outside the published event date/span."}
		case "published_date_overlap":
			out.Schedule = Check{Status: "supported", Reason: "Matches the published event date; actual operation and availability are unverified."}
		case "published_span_overlap":
			out.Schedule = Check{Status: "unknown", Reason: "Published span overlaps this date, but individual event days or daily operation are not established."}
		default:
			out.Schedule = Check{Status: "unknown", Reason: "No unambiguous event date applies; spot hours are not dated opening evidence."}
		}
	}
	if asOf != "" && r.Booking.ApplicationStart != "" && r.Booking.ApplicationEnd != "" {
		out.Application = Check{Status: "within_published_window", Reason: "Published application interval only; capacity/lottery is not seat inventory."}
		if asOf < r.Booking.ApplicationStart {
			out.Application.Status = "before_published_window"
		}
		if asOf > r.Booking.ApplicationEnd {
			out.Application.Status = "after_published_window"
		}
	}
	checks := []Check{out.Age, out.Schedule}
	for _, c := range out.Amenities {
		checks = append(checks, c)
	}
	for _, c := range checks {
		if c.Status == "excluded" {
			out.Overall = "excluded"
			break
		}
		if c.Status == "unknown" {
			out.Overall = "unknown"
		}
	}
	return out
}
func searchText(r Record) string {
	parts := []string{r.Name, r.Reading, r.Location, r.Address, r.Fees.PublishedText, r.Hours}
	parts = append(parts, r.Tags...)
	for _, e := range r.Evidence {
		parts = append(parts, e.Text)
	}
	return strings.ToLower(strings.Join(parts, " "))
}
func Filter(records []Record, q Query) Discovery {
	out := Discovery{Records: make([]Candidate, 0), Coverage: make([]Coverage, 0), Scope: ScopeNote}
	seen := map[string]bool{}
	for _, r := range records {
		if seen[r.Ref] {
			continue
		}
		seen[r.Ref] = true
		out.ScannedRecords++
		if q.Kind != "" && q.Kind != "all" && q.Kind != r.Kind {
			out.ExcludedRecords++
			continue
		}
		if q.Keyword != "" && !strings.Contains(searchText(r), strings.ToLower(q.Keyword)) {
			out.ExcludedRecords++
			continue
		}
		state := window(r.Schedule, q.From, q.To)
		if state == "outside" {
			out.ExcludedRecords++
			continue
		}
		if state == "unknown" {
			out.UnknownDateRecords++
			continue
		}
		assessment := Evaluate(r, "", "", q.AgeMonths, q.Amenities)
		if assessment.Overall == "excluded" {
			out.ExcludedRecords++
			continue
		}
		if q.AgeMonths >= 0 || len(q.Amenities) > 0 {
			if assessment.Overall != "supported" {
				out.UnknownRequirementRecords++
				continue
			}
		}
		out.MatchedRecords++
		if len(out.Records) < q.Limit {
			out.Records = append(out.Records, Candidate{Record: r, WindowMatch: state})
		} else {
			out.OmittedMatches++
		}
	}
	if len(out.Records) == 0 {
		out.Note = "No matching records in this bounded saved/fetched window. Unknown dates or requirements are not matches. Inspect coverage and unknown counts; increase --max-pages (up to 5) for live discovery or choose another source area. This does not establish source-wide absence."
	} else {
		out.Note = "Keyword/date filtering is local to the reported window. Date-span overlap is a candidate signal; daily operation, admission and availability remain unknown. Missing amenities remain unknown."
	}
	return out
}

// RefreshStatus evaluates archived/upcoming status for an explicit date while
// preserving the original observation timestamp of saved evidence.
func RefreshStatus(r Record, asOf string) Record {
	if r.Schedule.Start == "" || r.Schedule.End == "" {
		return r
	}
	switch {
	case r.Schedule.End < asOf:
		r.Schedule.Status = "ended"
	case r.Schedule.Start > asOf:
		r.Schedule.Status = "upcoming"
	default:
		r.Schedule.Status = "published_span_current"
	}
	return r
}
