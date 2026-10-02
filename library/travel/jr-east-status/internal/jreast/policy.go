package jreast

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ReportingCoverage classifies the observation's reporting window, including midnight rollover.
func ReportingCoverage(t time.Time) Coverage {
	t = t.In(JST)
	state := "open"
	if t.Hour() >= 2 && t.Hour() < 4 {
		state = "outside_reporting_hours"
	}
	day := t
	if t.Hour() < 4 {
		day = t.AddDate(0, 0, -1)
	}
	return Coverage{Timezone: "Asia/Tokyo (UTC+09:00)", ReportingHours: "04:00–02:00 following day JST", ReportingState: state, ServiceDay: day.Format("2006-01-02"), EnglishThreshold: "anticipated or actual delay in excess of 30 minutes", JapaneseThreshold: "anticipated or actual delay of 30 minutes or more", ThresholdConflict: true, NormalMeaning: "General source label; does not establish zero delay or an individual train's punctuality.", BRT: "Long suspensions only; other BRT lateness is outside this reporting coverage.", SourceURL: Origin + "/train_info/e/"}
}

// RegionByID resolves the small source-backed service-area catalogue.
func RegionByID(id string) (Region, error) {
	if id == "chyokyori" {
		id = "express"
	}
	for _, r := range Regions() {
		if r.ID == id {
			return r, nil
		}
	}
	return Region{}, fmt.Errorf("--region %q is unsupported; use kanto, tohoku, shinetsu, express or shinkansen", id)
}

// Normalize folds width and accents for bilingual discovery without merging native identifiers.
func Normalize(s string) string {
	s = norm.NFD.String(norm.NFKC.String(strings.ToLower(strings.TrimSpace(s))))
	latinBase := false
	s = strings.Map(func(r rune) rune {
		if unicode.Is(unicode.Mn, r) && latinBase {
			return -1
		}
		if !unicode.Is(unicode.Mn, r) {
			latinBase = unicode.Is(unicode.Latin, r)
		}
		return r
	}, s)
	return norm.NFC.String(s)
}

// FindLine resolves exact native IDs/names; ambiguous text is never silently picked.
func FindLine(lines []Line, query string) (Line, error) {
	q := Normalize(query)
	matches := make([]Line, 0)
	for _, l := range lines {
		for _, s := range []string{l.ID, l.SourceID, l.NameJA, l.NameEN} {
			if q != "" && Normalize(s) == q {
				matches = append(matches, l)
				break
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return Line{}, fmt.Errorf("--line %q is ambiguous; choose a native id using lines --query %q", query, query)
	}
	return Line{}, fmt.Errorf("--line %q was not found in this source region; run lines --region <region> --query %q", query, query)
}

// Assessment keeps source availability separate from general labels and planned notices.
func Assessment(l Line, states []SourceState, now time.Time) string {
	if ReportingCoverage(now).ReportingState != "open" {
		return "outside_reporting_hours"
	}
	for _, s := range states {
		if s.ReportingState != "open" {
			return "outside_reporting_hours"
		}
		if s.Freshness != "fresh" {
			return "source_" + s.Freshness
		}
	}
	for _, s := range []string{"suspended", "cancelled", "partial_cancellation", "through_service_stopped", "delayed"} {
		for _, v := range l.Statuses {
			if v == s {
				return "reported_" + s
			}
		}
	}
	if l.LanguageConflict {
		return "language_conflict"
	}
	for _, n := range l.Notices {
		if n.Planned {
			return "planned_notice"
		}
		if n.Status == "notice" {
			return "notice"
		}
	}
	if len(l.Statuses) == 1 && l.Statuses[0] == "normal_label" {
		return "normal_label_only"
	}
	return "unknown"
}

// ReferenceLines returns the bundled source identity catalogue, with no operational labels.
func ReferenceLines(r Region) []Line {
	out := make([]Line, 0)
	for id, v := range catalogue[r.ID] {
		out = append(out, Line{IdentitySource: "bundled_source_catalogue", ID: r.ID + ":" + id, SourceID: id, Region: r.ID, NameJA: v.JA, NameEN: v.EN, Groups: make([]Group, 0), Statuses: make([]string, 0), SourceJA: r.SourceJA, SourceEN: r.SourceEN, Assessment: "outside_reporting_hours"})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
