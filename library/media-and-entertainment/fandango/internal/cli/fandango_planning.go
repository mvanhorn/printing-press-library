// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type fandangoPlanRow struct {
	ID          string   `json:"id,omitempty"`
	MovieID     string   `json:"movie_id,omitempty"`
	Title       string   `json:"title,omitempty"`
	TheaterID   string   `json:"theater_id,omitempty"`
	Theater     string   `json:"theater,omitempty"`
	Start       string   `json:"start,omitempty"`
	DisplayDate string   `json:"display_date,omitempty"`
	Format      string   `json:"format,omitempty"`
	SeatingType string   `json:"seating_type,omitempty"`
	Distance    float64  `json:"distance_miles,omitempty"`
	Amenities   []string `json:"amenities,omitempty"`
	PurchaseURL string   `json:"purchase_url,omitempty"`
}

func fetchFandangoShowtimes(cmd *cobra.Command, flags *rootFlags, params map[string]string) ([]fandangoPlanRow, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	raw, err := c.GetWithHeadersNoCache(cmd.Context(), "/Fandango/Showtimes", params, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	return parseFandangoRows(raw)
}

func parseFandangoRows(raw json.RawMessage) ([]fandangoPlanRow, error) {
	var envelope struct {
		Data struct {
			Showtimes []map[string]any `json:"showtimes"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("decode Fandango showtimes: %w", err)
	}
	rows := make([]fandangoPlanRow, 0, len(envelope.Data.Showtimes))
	for _, item := range envelope.Data.Showtimes {
		row := fandangoPlanRow{
			ID:          fdString(item, "id"),
			MovieID:     fdString(item, "movieId"),
			Title:       fdString(item, "movieTitle"),
			TheaterID:   fdString(item, "theaterId"),
			Theater:     fdString(item, "theaterName"),
			DisplayDate: fdString(item, "displayDate"),
			Format:      fdString(item, "formatName"),
			SeatingType: fdString(item, "seatingType"),
			Distance:    fdFloat(item, "distance"),
		}
		if dateTime, ok := item["dateTime"].(map[string]any); ok {
			row.Start = fdString(dateTime, "local")
			if row.Start == "" {
				row.Start = fdString(dateTime, "utc")
			}
		}
		if amenities, ok := item["amenities"].([]any); ok {
			for _, value := range amenities {
				if amenity, ok := value.(map[string]any); ok {
					if name := fdString(amenity, "name"); name != "" {
						row.Amenities = append(row.Amenities, name)
					}
				}
			}
		}
		if links, ok := item["links"].([]any); ok {
			for _, value := range links {
				if link, ok := value.(map[string]any); ok {
					href := fdString(link, "href")
					rel := strings.ToLower(fdString(link, "rel"))
					if href != "" && (strings.Contains(rel, "ticket") || strings.Contains(rel, "purchase")) {
						row.PurchaseURL = href
						break
					}
				}
			}
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Start != rows[j].Start {
			return rows[i].Start < rows[j].Start
		}
		return rows[i].Distance < rows[j].Distance
	})
	return rows, nil
}

func fdString(item map[string]any, key string) string {
	switch value := item[key].(type) {
	case string:
		return value
	case float64:
		return strconv.FormatFloat(value, 'f', -1, 64)
	default:
		return ""
	}
}

func fdFloat(item map[string]any, key string) float64 {
	switch value := item[key].(type) {
	case float64:
		return value
	case string:
		parsed, _ := strconv.ParseFloat(value, 64)
		return parsed
	default:
		return 0
	}
}

func fandangoDateTime(date, clock string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02 15:04", date+" "+clock, time.Local)
}

func rowTime(row fandangoPlanRow) (time.Time, bool) {
	if parsed, err := time.Parse(time.RFC3339, row.Start); err == nil {
		return parsed, true
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, row.Start, time.Local); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func filterFandangoWindow(rows []fandangoPlanRow, start, end time.Time) []fandangoPlanRow {
	filtered := make([]fandangoPlanRow, 0, len(rows))
	for _, row := range rows {
		when, ok := rowTime(row)
		if !ok || when.Before(start) || when.After(end) {
			continue
		}
		filtered = append(filtered, row)
	}
	return filtered
}

func splitNonEmpty(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
