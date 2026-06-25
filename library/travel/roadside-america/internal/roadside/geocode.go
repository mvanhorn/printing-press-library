package roadside

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/roadside-america/internal/cliutil"
)

// GeoResult is a resolved place -> coordinates record, cached in the store.
type GeoResult struct {
	Query    string  `json:"query"`
	Lat      float64 `json:"lat"`
	Lng      float64 `json:"lng"`
	Display  string  `json:"display,omitempty"`
	CachedAt string  `json:"cached_at,omitempty"`
	Geocoder string  `json:"geocoder,omitempty"`
}

// NominatimBaseURL is the keyless OpenStreetMap geocoder. No API key required;
// its usage policy asks for <=1 req/s and a descriptive User-Agent, both of
// which this CLI honors (results are cached so live calls are rare).
const NominatimBaseURL = "https://nominatim.openstreetmap.org/search"

// BuildNominatimURL builds a geocoding request URL scoped to the US and Canada
// (the only regions RoadsideAmerica.com covers).
func BuildNominatimURL(place string) string {
	q := url.Values{}
	q.Set("q", strings.TrimSpace(place))
	q.Set("format", "jsonv2")
	q.Set("limit", "1")
	q.Set("countrycodes", "us,ca")
	q.Set("addressdetails", "0")
	return NominatimBaseURL + "?" + q.Encode()
}

// nominatimItem is one entry in the Nominatim response array. lat/lon are
// JSON-encoded strings, so they are extracted via cliutil-style tolerant
// parsing rather than typed float fields.
type nominatimItem struct {
	Lat         string `json:"lat"`
	Lon         string `json:"lon"`
	DisplayName string `json:"display_name"`
}

// ParseNominatimResponse parses a Nominatim jsonv2 array, returning the first
// result. It returns a not-found error when the array is empty.
func ParseNominatimResponse(body []byte) (lat, lng float64, display string, err error) {
	var items []nominatimItem
	if err := json.Unmarshal(body, &items); err != nil {
		return 0, 0, "", fmt.Errorf("decoding geocoder response: %w", err)
	}
	if len(items) == 0 {
		return 0, 0, "", ErrPlaceNotFound
	}
	it := items[0]
	lat, err = strconv.ParseFloat(strings.TrimSpace(it.Lat), 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("parsing latitude %q: %w", it.Lat, err)
	}
	lng, err = strconv.ParseFloat(strings.TrimSpace(it.Lon), 64)
	if err != nil {
		return 0, 0, "", fmt.Errorf("parsing longitude %q: %w", it.Lon, err)
	}
	return lat, lng, strings.TrimSpace(it.DisplayName), nil
}

// ErrPlaceNotFound signals a geocode query returned no match.
var ErrPlaceNotFound = fmt.Errorf("place not found")

// Geocoder performs keyless geocoding against Nominatim with its own polite
// rate limiter and User-Agent. Results should be cached by the caller.
type Geocoder struct {
	HTTPClient *http.Client
	UserAgent  string
	limiter    *cliutil.AdaptiveLimiter
}

// NewGeocoder returns a geocoder limited to ratePerSec requests/second
// (Nominatim policy is <=1/s; pass 1.0). timeout bounds each request.
func NewGeocoder(userAgent string, ratePerSec float64, timeout time.Duration) *Geocoder {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Geocoder{
		HTTPClient: &http.Client{Timeout: timeout},
		UserAgent:  userAgent,
		limiter:    cliutil.NewAdaptiveLimiter(ratePerSec),
	}
}

// Geocode resolves a place name to coordinates. It surfaces a clear error on
// HTTP 429 rather than returning empty coordinates.
func (g *Geocoder) Geocode(ctx context.Context, place string) (GeoResult, error) {
	place = strings.TrimSpace(place)
	if place == "" {
		return GeoResult{}, fmt.Errorf("empty place query")
	}
	g.limiter.Wait()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, BuildNominatimURL(place), nil)
	if err != nil {
		return GeoResult{}, fmt.Errorf("building geocode request: %w", err)
	}
	ua := g.UserAgent
	if ua == "" {
		ua = "roadside-america-pp-cli (geocoding via Nominatim)"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/json")
	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		return GeoResult{}, fmt.Errorf("geocode request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return GeoResult{}, fmt.Errorf("reading geocode response: %w", err)
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return GeoResult{}, &cliutil.RateLimitError{URL: NominatimBaseURL, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode >= 400 {
		return GeoResult{}, fmt.Errorf("geocoder returned HTTP %d", resp.StatusCode)
	}
	lat, lng, display, err := ParseNominatimResponse(body)
	if err != nil {
		return GeoResult{}, err
	}
	return GeoResult{Query: place, Lat: lat, Lng: lng, Display: display, Geocoder: "nominatim"}, nil
}
