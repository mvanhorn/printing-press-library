// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package service

import (
	"fmt"
	"time"
)

type Constraints struct {
	On          string `json:"on"`
	AsOf        string `json:"as_of"`
	Party       int    `json:"party"`
	RequireFree bool   `json:"require_free"`
}
type Check struct {
	State    string  `json:"state"`
	Reason   string  `json:"reason"`
	Deadline *string `json:"request_deadline"`
}
type Comparison struct {
	Service       Service `json:"service"`
	Notice        Check   `json:"notice"`
	Party         Check   `json:"party"`
	Free          Check   `json:"free"`
	PublishedSpan Check   `json:"published_date_span"`
	Compatibility string  `json:"compatibility"`
	Availability  string  `json:"availability"`
}

func ValidateConstraints(c Constraints) error {
	for k, s := range map[string]string{"on": c.On, "as-of": c.AsOf} {
		if s != "" {
			if _, e := time.Parse("2006-01-02", s); e != nil {
				return fmt.Errorf("--%s must be exact YYYY-MM-DD", k)
			}
		}
	}
	if c.Party < 0 || c.Party > 10000 {
		return fmt.Errorf("--party must be 1..10000 when provided")
	}
	if c.On != "" && c.AsOf > c.On {
		return fmt.Errorf("--as-of cannot be after --on")
	}
	return nil
}
func subtract(t time.Time, l LeadTime) time.Time {
	if l.Unit == "months" {
		y, m, d := t.Date()
		base := time.Date(y, m-time.Month(l.Value), 1, 0, 0, 0, 0, time.UTC)
		last := time.Date(base.Year(), base.Month()+1, 0, 0, 0, 0, 0, time.UTC).Day()
		if d > last {
			d = last
		}
		return time.Date(base.Year(), base.Month(), d, 0, 0, 0, 0, time.UTC)
	}
	n := l.Value
	if l.Unit == "weeks" {
		n *= 7
	}
	return t.AddDate(0, 0, -n)
}
func Compare(s Service, c Constraints) Comparison {
	unknown := func(reason string) Check { return Check{State: "unknown", Reason: reason} }
	notAsked := Check{State: "not_requested", Reason: "No corresponding constraint supplied."}
	x := Comparison{Service: s, Notice: notAsked, Party: notAsked, Free: notAsked, PublishedSpan: unknown("Published dates do not establish individual sessions or daily operation."), Compatibility: "unknown", Availability: "unknown"}
	checks := []Check{}
	if c.On != "" {
		x.Notice = unknown("No single unambiguous published request rule.")
		on, _ := time.Parse("2006-01-02", c.On)
		if s.Request.Status == "known" {
			if len(s.Request.LeadTimes) == 1 && c.AsOf != "" {
				d := subtract(on, s.Request.LeadTimes[0]).Format("2006-01-02")
				state := "supported"
				reason := "As-of date meets the published calendar notice rule; cutoff time and guide availability remain unknown."
				if c.AsOf > d {
					state = "excluded"
					reason = "As-of date is after the published request deadline."
				}
				x.Notice = Check{state, reason, &d}
			} else if len(s.Request.Options) == 1 && s.Request.Options[0] == "予約不要" {
				x.Notice = Check{State: "supported", Reason: "Source says no advance reservation; availability remains unknown."}
			} else if len(s.Request.Options) == 1 && s.Request.Options[0] == "当日" {
				x.Notice = Check{State: "supported", Reason: "Source lists same-day request; availability remains unknown."}
			}
		}
		checks = append(checks, x.Notice)
		if s.Schedule.StartDate != nil && s.Schedule.EndDate != nil {
			x.PublishedSpan = Check{State: "within_published_span", Reason: "Visit date is inside the published date envelope; operation remains unknown."}
			if c.On < *s.Schedule.StartDate || c.On > *s.Schedule.EndDate {
				x.PublishedSpan = Check{State: "outside_published_span", Reason: "Visit date is outside the published date envelope."}
				checks = append(checks, Check{State: "excluded", Reason: x.PublishedSpan.Reason})
			}
		}
	}
	if c.Party > 0 {
		x.Party = unknown("No explicit single minimum participant count.")
		if s.MinimumParty != nil {
			state := "supported"
			reason := "Party meets the explicitly published minimum; other conditions remain unknown."
			if c.Party < *s.MinimumParty {
				state = "excluded"
				reason = "Party is smaller than the published minimum."
			}
			x.Party = Check{State: state, Reason: reason}
		}
		checks = append(checks, x.Party)
	}
	if c.RequireFree {
		x.Free = unknown("No unambiguous zero-cost evidence.")
		if s.Price.Status == "free" {
			x.Free = Check{State: "supported", Reason: "Source explicitly states free; confirm current terms."}
		} else if s.Price.Status == "paid" || s.Price.Status == "expenses" {
			x.Free = Check{State: "excluded", Reason: "Published guide fee or expense obligations do not meet zero-cost requirement."}
		}
		checks = append(checks, x.Free)
	}
	if len(checks) > 0 {
		x.Compatibility = "supported_by_published_rules"
		for _, v := range checks {
			if v.State == "unknown" {
				x.Compatibility = "unknown"
			}
		}
		for _, v := range checks {
			if v.State == "excluded" {
				x.Compatibility = "excluded"
			}
		}
	}
	if s.SourceClosed != nil && *s.SourceClosed {
		x.Compatibility = "excluded"
	}
	return x
}
