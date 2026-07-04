// Copyright 2026 richardadonnell and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored (NOT generated). Shared helpers for the novel Luma commands
// (agenda, near, ics, watch). Parsing, geo math, windowing, fan-out fetch, and
// ICS serialization live here so each command file stays thin.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/luma/internal/cliutil"
)

const eventsPath = "/discover/get-paginated-events"

// lumaCoord is the {latitude, longitude} shape Luma uses on event.coordinate.
type lumaCoord struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type lumaEventInner struct {
	APIID         string         `json:"api_id"`
	Name          string         `json:"name"`
	StartAt       string         `json:"start_at"`
	EndAt         string         `json:"end_at"`
	Timezone      string         `json:"timezone"`
	URL           string         `json:"url"`
	Coordinate    *lumaCoord     `json:"coordinate"`
	CalendarAPIID string         `json:"calendar_api_id"`
	GeoAddress    map[string]any `json:"geo_address_info"`
}

// lumaEntry is one entry from get-paginated-events (or a synced events row).
type lumaEntry struct {
	APIID       string         `json:"api_id"`
	Event       lumaEventInner `json:"event"`
	GuestCount  int            `json:"guest_count"`
	TicketCount int            `json:"ticket_count"`
}

// lumaEventView is the normalized, agent-friendly output row.
type lumaEventView struct {
	APIID       string  `json:"api_id"`
	Name        string  `json:"name"`
	StartAt     string  `json:"start_at"`
	Timezone    string  `json:"timezone,omitempty"`
	URL         string  `json:"url,omitempty"`
	City        string  `json:"city,omitempty"`
	Address     string  `json:"address,omitempty"`
	GuestCount  int     `json:"guest_count"`
	TicketCount int     `json:"ticket_count"`
	Lat         float64 `json:"lat,omitempty"`
	Lng         float64 `json:"lng,omitempty"`
	DistanceKm  float64 `json:"distance_km,omitempty"`
}

func (e lumaEntry) id() string {
	if e.Event.APIID != "" {
		return e.Event.APIID
	}
	return e.APIID
}

func (e lumaEntry) startTime() (time.Time, bool) {
	if e.Event.StartAt == "" {
		return time.Time{}, false
	}
	if t, err := time.Parse(time.RFC3339, e.Event.StartAt); err == nil {
		return t, true
	}
	// Fallback for millisecond-precision variants (e.g. "2026-06-20T18:00:00.000Z")
	// in case a future toolchain's strict RFC3339 rejects fractional seconds.
	if t, err := time.Parse(time.RFC3339Nano, e.Event.StartAt); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func (e lumaEntry) geoField(key string) string {
	if e.Event.GeoAddress == nil {
		return ""
	}
	if v, ok := e.Event.GeoAddress[key]; ok {
		if s, ok := v.(string); ok {
			return cliutil.CleanText(s)
		}
	}
	return ""
}

// eventPermalink turns the API's bare event slug (e.g. "7dpi2p8h") into a
// clickable canonical permalink (https://luma.com/7dpi2p8h). The discovery API
// returns only the slug in event.url; the same luma.com/<slug> form is what
// buildICS emits, so JSON output stays consistent with the ICS export. If the
// API ever returns an already-absolute URL, it is passed through unchanged.
func eventPermalink(slug string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ""
	}
	if strings.HasPrefix(slug, "http://") || strings.HasPrefix(slug, "https://") {
		return slug
	}
	return "https://luma.com/" + strings.TrimPrefix(slug, "/")
}

func (e lumaEntry) view() lumaEventView {
	v := lumaEventView{
		APIID:       e.id(),
		Name:        cliutil.CleanText(e.Event.Name),
		StartAt:     e.Event.StartAt,
		Timezone:    e.Event.Timezone,
		URL:         eventPermalink(e.Event.URL),
		City:        e.geoField("city_state"),
		Address:     e.geoField("short_address"),
		GuestCount:  e.GuestCount,
		TicketCount: e.TicketCount,
	}
	if v.City == "" {
		v.City = e.geoField("city")
	}
	if e.Event.Coordinate != nil {
		v.Lat = e.Event.Coordinate.Latitude
		v.Lng = e.Event.Coordinate.Longitude
	}
	return v
}

// lumaPage is the {entries, has_more, next_cursor} envelope.
type lumaPage struct {
	Entries    []json.RawMessage `json:"entries"`
	HasMore    bool              `json:"has_more"`
	NextCursor string            `json:"next_cursor"`
}

// fetchEventEntries pages get-paginated-events for ONE filter (place or
// category), bounded by maxScanPages, returning parsed entries.
func fetchEventEntries(ctx context.Context, c *client.Client, baseParams map[string]string, pageLimit, maxScanPages int) ([]lumaEntry, error) {
	var out []lumaEntry
	cursor := ""
	for page := 0; page < maxScanPages; page++ {
		params := map[string]string{"pagination_limit": fmt.Sprintf("%d", pageLimit)}
		for k, v := range baseParams {
			if v != "" {
				params[k] = v
			}
		}
		if cursor != "" {
			params["pagination_cursor"] = cursor
		}
		raw, err := c.Get(ctx, eventsPath, params)
		if err != nil {
			return out, err
		}
		var pg lumaPage
		if err := json.Unmarshal(raw, &pg); err != nil {
			return out, fmt.Errorf("parsing events page: %w", err)
		}
		for _, e := range pg.Entries {
			var entry lumaEntry
			if json.Unmarshal(e, &entry) == nil {
				out = append(out, entry)
			}
		}
		if !pg.HasMore || pg.NextCursor == "" {
			break
		}
		cursor = pg.NextCursor
	}
	return out, nil
}

// dedupeByID keeps the first entry per event api_id, preserving order.
func dedupeByID(in []lumaEntry) []lumaEntry {
	seen := make(map[string]struct{}, len(in))
	out := in[:0:0]
	for _, e := range in {
		id := e.id()
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, e)
	}
	return out
}

// withinWindow reports whether t is in [now, now+window]. A zero window means
// "any upcoming time" (only excludes events already past).
func withinWindow(t, now time.Time, window time.Duration) bool {
	if t.Before(now.Add(-1 * time.Hour)) {
		return false
	}
	if window <= 0 {
		return true
	}
	return !t.After(now.Add(window))
}

// sortByStart orders entries ascending by start time; unparseable times sink.
func sortByStart(in []lumaEntry) {
	sort.SliceStable(in, func(i, j int) bool {
		ti, oki := in[i].startTime()
		tj, okj := in[j].startTime()
		if oki != okj {
			return oki
		}
		return ti.Before(tj)
	})
}

// haversineKm returns the great-circle distance in kilometers.
func haversineKm(lat1, lng1, lat2, lng2 float64) float64 {
	const earthKm = 6371.0
	rad := func(d float64) float64 { return d * math.Pi / 180 }
	dLat := rad(lat2 - lat1)
	dLng := rad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(rad(lat1))*math.Cos(rad(lat2))*math.Sin(dLng/2)*math.Sin(dLng/2)
	return earthKm * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

// parseWindow turns a duration flag (7d, 24h, "") into a Duration. Empty => 0.
func parseWindow(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	return cliutil.ParseDurationLoose(s)
}

// icsEscape escapes a value per RFC 5545 (commas, semicolons, backslashes, newlines).
func icsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func icsStamp(t time.Time) string { return t.UTC().Format("20060102T150405Z") }

// buildICS serializes entries to an RFC 5545 VCALENDAR string.
func buildICS(entries []lumaEntry, stamp time.Time) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//luma-pp-cli//Luma events//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	for _, e := range entries {
		start, okStart := e.startTime()
		if !okStart {
			continue
		}
		b.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&b, "UID:%s@luma.com\r\n", e.id())
		fmt.Fprintf(&b, "DTSTAMP:%s\r\n", icsStamp(stamp))
		fmt.Fprintf(&b, "DTSTART:%s\r\n", icsStamp(start))
		if e.Event.EndAt != "" {
			if end, err := time.Parse(time.RFC3339, e.Event.EndAt); err == nil {
				fmt.Fprintf(&b, "DTEND:%s\r\n", icsStamp(end))
			}
		}
		fmt.Fprintf(&b, "SUMMARY:%s\r\n", icsEscape(cliutil.CleanText(e.Event.Name)))
		if loc := e.geoField("full_address"); loc != "" {
			fmt.Fprintf(&b, "LOCATION:%s\r\n", icsEscape(loc))
		}
		if link := eventPermalink(e.Event.URL); link != "" {
			fmt.Fprintf(&b, "URL:%s\r\n", link)
		}
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// scanPagesForEnv lowers the scan cap under live-dogfood so the 30s per-command
// matrix timeout is respected.
func scanPagesForEnv(requested int) int {
	if cliutil.IsDogfoodEnv() && requested > 1 {
		return 1
	}
	return requested
}
