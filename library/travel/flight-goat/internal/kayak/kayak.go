// Package kayak fetches Kayak's /direct/<airport> page and extracts the
// embedded nonstop routes array. Kayak server-renders the entire destinations
// table into the HTML as a "routes":[...] JSON blob before client-side
// hydration replaces it with React components. We skip the hydration and
// parse the server payload directly.
//
// This is flight-goat's own Kayak integration (not copied from any existing
// library). Discovered by curl-fetching /direct/SEA with a browser User-Agent
// and grepping the response for "duration":N,"flightList":[].
package kayak

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://www.kayak.com/direct"
	browserUA      = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"
)

// Route is one nonstop destination from an origin airport.
// Field names match Kayak's own internal shape so we can unmarshal directly.
type Route struct {
	Code             string   `json:"code"`
	LocalizedDisplay string   `json:"localizedDisplay"`
	DisplayLocation  string   `json:"displayLocation"`
	FullName         string   `json:"fullName"`
	CountryCode      string   `json:"countryCode"`
	AirlineCodes     []string `json:"airlineCodes"`
	DistanceMiles    int      `json:"distance"`
	TravelTime       string   `json:"localizedTravelTime"`
	Duration         int      `json:"duration"` // minutes
	FlightsCount     int      `json:"flightsCount"`
	RouteSeoLink     string   `json:"routeSeoLink"`
}

// Client fetches Kayak /direct pages.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	UserAgent  string
}

// New returns a Client with sensible defaults.
func New() *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  browserUA,
	}
}

// Direct fetches nonstop routes from an origin IATA code.
// Returns the parsed Route slice along with the raw HTML length for debugging.
func (c *Client) Direct(origin string) ([]Route, error) {
	origin = strings.ToUpper(strings.TrimSpace(origin))
	if len(origin) < 3 || len(origin) > 4 {
		return nil, fmt.Errorf("invalid airport code %q (expected 3-4 letter IATA)", origin)
	}
	url := fmt.Sprintf("%s/%s", c.BaseURL, origin)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("kayak returned %s for %s", resp.Status, url)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return extractRoutes(body)
}

// extractRoutes finds the "routes":[{...}] array in a Kayak /direct HTML
// response and unmarshals it. The array is a sibling of other fields in a
// larger JSON object inside a <script> block, so we locate the array's
// opening bracket via string search and then do a balanced-bracket walk.
func extractRoutes(html []byte) ([]Route, error) {
	key := []byte(`"routes":[{`)
	idx := -1
	// Find the first routes array that actually contains route objects
	// (there are other "routes":[ occurrences with empty arrays earlier in
	// the HTML — those are navigation menus or unrelated sections).
	for start := 0; start < len(html); {
		next := bytesIndex(html[start:], key)
		if next < 0 {
			break
		}
		idx = start + next + len(`"routes":`)
		break
	}
	if idx < 0 {
		return nil, fmt.Errorf("no routes array found in Kayak response (page may have changed structure)")
	}
	end, err := balancedBracketEnd(html, idx)
	if err != nil {
		return nil, err
	}
	raw := html[idx:end]
	var routes []Route
	if err := json.Unmarshal(raw, &routes); err != nil {
		return nil, fmt.Errorf("parsing routes array: %w", err)
	}
	return routes, nil
}

// bytesIndex is strings.Index for []byte without a dependency loop.
func bytesIndex(b, sub []byte) int {
outer:
	for i := 0; i <= len(b)-len(sub); i++ {
		for j := range sub {
			if b[i+j] != sub[j] {
				continue outer
			}
		}
		return i
	}
	return -1
}

// balancedBracketEnd walks from an opening '[' and returns the index just
// past the matching ']'. Respects quoted strings and escaped characters so
// it doesn't get fooled by brackets inside route display strings.
func balancedBracketEnd(b []byte, start int) (int, error) {
	if start >= len(b) || b[start] != '[' {
		return 0, fmt.Errorf("expected [ at offset %d", start)
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(b); i++ {
		ch := b[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch ch {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return i + 1, nil
			}
		}
	}
	return 0, fmt.Errorf("unbalanced brackets starting at offset %d", start)
}
