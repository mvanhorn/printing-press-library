// Package openmeteo holds shared helpers used by hand-written novel
// commands: place-name to coordinate resolution, forecast-snapshot
// read/write, and other cross-command building blocks. Generated CLI
// code does not import this package.
package openmeteo

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/open-meteo/internal/client"
)

// Place is one geocoded location.
type Place struct {
	Name        string  `json:"name"`
	CountryCode string  `json:"country_code,omitempty"`
	Admin1      string  `json:"admin1,omitempty"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	Elevation   float64 `json:"elevation,omitempty"`
	Timezone    string  `json:"timezone,omitempty"`
	ID          int     `json:"id,omitempty"`
}

// ResolvePlace looks up a single place by name through the Open-Meteo geocoding
// API and returns the top match. The query is case-insensitive and may include
// disambiguating context (e.g., "Springfield, IL").
func ResolvePlace(c *client.Client, query string) (*Place, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("place query is empty")
	}
	// Strip everything after the first comma for the upstream search; the
	// upstream geocoder narrows matches by name, not free-form region. Keep
	// the rest for client-side ranking.
	primary := query
	if i := strings.Index(query, ","); i > 0 {
		primary = strings.TrimSpace(query[:i])
	}
	raw, err := c.Get("https://geocoding-api.open-meteo.com/v1/search", map[string]string{
		"name":  primary,
		"count": "10",
	})
	if err != nil {
		return nil, fmt.Errorf("geocoding %q: %w", query, err)
	}
	var resp struct {
		Results []struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
			Elevation   float64 `json:"elevation"`
			Timezone    string  `json:"timezone"`
			CountryCode string  `json:"country_code"`
			Admin1      string  `json:"admin1"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("parsing geocoding response: %w", err)
	}
	if len(resp.Results) == 0 {
		return nil, fmt.Errorf("no places found for %q", query)
	}
	// Apply disambiguation: when the query had region tokens after the comma,
	// prefer the result whose admin1/country_code matches.
	pick := 0
	if strings.Contains(query, ",") {
		want := strings.ToLower(strings.TrimSpace(query[strings.Index(query, ",")+1:]))
		for i, r := range resp.Results {
			combined := strings.ToLower(r.Admin1 + " " + r.CountryCode)
			if want != "" && strings.Contains(combined, want) {
				pick = i
				break
			}
		}
	}
	r := resp.Results[pick]
	return &Place{
		ID:          r.ID,
		Name:        r.Name,
		Admin1:      r.Admin1,
		CountryCode: r.CountryCode,
		Latitude:    r.Latitude,
		Longitude:   r.Longitude,
		Elevation:   r.Elevation,
		Timezone:    r.Timezone,
	}, nil
}

// CoordsFromFlags resolves a (place, latitude, longitude) flag triple into one
// or more Place values. When place is non-empty it is geocoded; comma-separated
// place values are resolved one-by-one (sequential — geocoding is fast). When
// place is empty, the latitude/longitude strings are interpreted as
// comma-separated CSV pairs.
func CoordsFromFlags(c *client.Client, place, latitude, longitude string) ([]Place, error) {
	place = strings.TrimSpace(place)
	if place != "" {
		var places []Place
		for _, name := range strings.Split(place, ",") {
			// Heuristic: tokens with len <=3 are likely country/admin codes
			// belonging to the previous place rather than a separate place.
			// Re-attach them.
			name = strings.TrimSpace(name)
			if name == "" {
				continue
			}
			if len(places) > 0 && len(name) <= 3 {
				places[len(places)-1].Name += ", " + name
				continue
			}
			p, err := ResolvePlace(c, name)
			if err != nil {
				return nil, err
			}
			places = append(places, *p)
		}
		if len(places) == 0 {
			return nil, fmt.Errorf("no valid places parsed from %q", place)
		}
		return places, nil
	}
	if latitude == "" || longitude == "" {
		return nil, fmt.Errorf("either --place or both --latitude and --longitude are required")
	}
	lats := strings.Split(latitude, ",")
	lons := strings.Split(longitude, ",")
	if len(lats) != len(lons) {
		return nil, fmt.Errorf("--latitude and --longitude have mismatched count (%d vs %d)", len(lats), len(lons))
	}
	out := make([]Place, 0, len(lats))
	for i := range lats {
		var lat, lon float64
		if _, err := fmt.Sscanf(strings.TrimSpace(lats[i]), "%f", &lat); err != nil {
			return nil, fmt.Errorf("invalid latitude %q: %w", lats[i], err)
		}
		if _, err := fmt.Sscanf(strings.TrimSpace(lons[i]), "%f", &lon); err != nil {
			return nil, fmt.Errorf("invalid longitude %q: %w", lons[i], err)
		}
		out = append(out, Place{
			Name:      fmt.Sprintf("%.4f,%.4f", lat, lon),
			Latitude:  lat,
			Longitude: lon,
		})
	}
	return out, nil
}
