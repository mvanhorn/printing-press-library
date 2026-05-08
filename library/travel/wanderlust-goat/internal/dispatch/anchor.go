package dispatch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// AnchorResolution describes a geocoded anchor location.
type AnchorResolution struct {
	Query   string  `json:"query"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Country string  `json:"country"`
	Display string  `json:"display"`
	City    string  `json:"city,omitempty"` // Resolved from Nominatim address fields, used for Stage-2 name+city queries.
}

// nominatimBase is the public Nominatim instance. Override with the
// WANDERLUST_GOAT_NOMINATIM env var (used by tests pointing to httptest).
const nominatimBase = "https://nominatim.openstreetmap.org"

// userAgentDefault is the contact-bearing UA Nominatim's policy requires.
const userAgentDefault = "wanderlust-goat-pp-cli/0.2 (+https://github.com/joeheitzeberg/wanderlust-goat)"

// ResolveAnchor parses lat,lng or geocodes via Nominatim. Returns the
// resolved coordinates and country code (ISO alpha-2, uppercase) or "*"
// when the address has no country (atypical).
func ResolveAnchor(ctx context.Context, anchor string) (AnchorResolution, error) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return AnchorResolution{}, fmt.Errorf("anchor cannot be empty")
	}
	if lat, lng, ok := parseLatLng(anchor); ok {
		return AnchorResolution{
			Query: anchor, Lat: lat, Lng: lng, Country: "*", Display: anchor,
		}, nil
	}

	base := strings.TrimRight(getenv("WANDERLUST_GOAT_NOMINATIM", nominatimBase), "/")
	q := strings.ReplaceAll(anchor, " ", "+")
	u := fmt.Sprintf("%s/search?q=%s&format=json&limit=1&addressdetails=1&accept-language=en", base, q)

	body, err := httpGetJSON(ctx, u, getenv("WANDERLUST_GOAT_UA", userAgentDefault), 10*time.Second)
	if err != nil {
		return AnchorResolution{}, fmt.Errorf("geocode %q: %w", anchor, err)
	}
	var results []struct {
		Lat         string `json:"lat"`
		Lon         string `json:"lon"`
		DisplayName string `json:"display_name"`
		Address     struct {
			CountryCode string `json:"country_code"`
			City        string `json:"city"`
			Town        string `json:"town"`
			Village     string `json:"village"`
			Municipality string `json:"municipality"`
			Suburb      string `json:"suburb"`
			CityDistrict string `json:"city_district"`
		} `json:"address"`
	}
	if err := json.Unmarshal(body, &results); err != nil {
		return AnchorResolution{}, fmt.Errorf("parse geocode response: %w", err)
	}
	if len(results) == 0 {
		return AnchorResolution{}, fmt.Errorf("geocode returned no results for %q", anchor)
	}
	r := results[0]
	lat, _ := strconv.ParseFloat(r.Lat, 64)
	lng, _ := strconv.ParseFloat(r.Lon, 64)
	cc := strings.ToUpper(r.Address.CountryCode)
	if cc == "" {
		cc = "*"
	}
	// City: prefer city, fall back through town/village/municipality/suburb.
	// Most country reverse-geocodes populate one of these for inhabited areas.
	city := firstNonEmpty(r.Address.City, r.Address.Town, r.Address.Municipality, r.Address.Village, r.Address.CityDistrict, r.Address.Suburb)
	return AnchorResolution{
		Query: anchor, Lat: lat, Lng: lng, Country: cc, Display: r.DisplayName, City: city,
	}, nil
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if s := strings.TrimSpace(x); s != "" {
			return s
		}
	}
	return ""
}

// parseLatLng accepts "<lat>,<lng>" with optional whitespace; returns
// (0,0,false) when the input is not a coordinate pair.
func parseLatLng(s string) (float64, float64, bool) {
	parts := strings.SplitN(s, ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	lat, err1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	lng, err2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		return 0, 0, false
	}
	return lat, lng, true
}

func httpGetJSON(ctx context.Context, u, ua string, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return body, nil
}

func getenv(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

// ResolveURLForAnchor is the public URL used by `research-plan` to embed
// the anchor's source URL. Cheap helper.
func ResolveURLForAnchor(query string) string {
	return nominatimBase + "/search?q=" + url.QueryEscape(query)
}
