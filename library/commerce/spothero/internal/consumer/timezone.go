package consumer

import (
	"fmt"
	"strings"
	"time"

	_ "time/tzdata"
)

// citySlugTimezones maps SpotHero city slugs to IANA zones for naive --starts/--ends.
// Non-binding: fallback when input lacks a numeric offset. See PLAN.md.
var citySlugTimezones = map[string]string{
	"san-francisco": "America/Los_Angeles",
	"los-angeles":   "America/Los_Angeles",
	"seattle":       "America/Los_Angeles",
	"portland":      "America/Los_Angeles",
	"chicago":       "America/Chicago",
	"new-york":      "America/New_York",
	"boston":        "America/New_York",
	"washington-dc": "America/New_York",
	"philadelphia":  "America/New_York",
	"denver":        "America/Denver",
	"phoenix":       "America/Phoenix",
	"dallas":        "America/Chicago",
	"houston":       "America/Chicago",
	"austin":        "America/Chicago",
	"miami":         "America/New_York",
	"atlanta":       "America/New_York",
}

// SearchPeriod is a UTC window sent to Craig bulk transient search.
type SearchPeriod struct {
	Starts string `json:"starts"`
	Ends   string `json:"ends"`
}

func locationForSearch(params SearchParams) (*time.Location, error) {
	if params.TimeZone != "" {
		loc, err := time.LoadLocation(params.TimeZone)
		if err != nil {
			return nil, fmt.Errorf("facility time zone %q: %w", params.TimeZone, err)
		}
		return loc, nil
	}
	if params.CitySlug != "" {
		if name, ok := citySlugTimezones[params.CitySlug]; ok {
			if loc, err := time.LoadLocation(name); err == nil {
				return loc, nil
			}
		}
	}
	return time.UTC, nil
}

func periodsToUTC(starts, ends string, params SearchParams) ([]SearchPeriod, *time.Location, error) {
	startUTC, loc, err := parseSearchInstantUTC(starts, params)
	if err != nil {
		return nil, nil, fmt.Errorf("starts: %w", err)
	}
	endUTC, _, err := parseSearchInstantUTC(ends, params)
	if err != nil {
		return nil, nil, fmt.Errorf("ends: %w", err)
	}
	return []SearchPeriod{
		{Starts: startUTC.Format(time.RFC3339), Ends: endUTC.Format(time.RFC3339)},
	}, loc, nil
}

func parseSearchInstantUTC(raw string, params SearchParams) (time.Time, *time.Location, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}, nil, fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.UTC(), t.Location(), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.UTC(), t.Location(), nil
	}
	loc, err := locationForSearch(params)
	if err != nil {
		return time.Time{}, nil, err
	}
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t.UTC(), loc, nil
		}
	}
	return time.Time{}, nil, fmt.Errorf("unrecognized time %q (use RFC3339 or local time with city from search-params)", raw)
}

// formatCheckoutInstant returns RFC3339 with numeric offset for checkout item_context.
func formatCheckoutInstant(raw string, params SearchParams) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("empty time")
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t.Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, raw); err == nil {
		return t.Format(time.RFC3339), nil
	}
	loc, err := locationForSearch(params)
	if err != nil {
		return "", err
	}
	layouts := []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, raw, loc); err == nil {
			return t.Format(time.RFC3339), nil
		}
	}
	return "", fmt.Errorf("unrecognized time %q", raw)
}
