// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package kayak

// PATCH(library): Kayak multi-city flight search.
//
// Discovered by recording Kayak's network traffic in a real browser session:
//
//   GET  /flights/SFO-NRT/2026-08-15/NRT-ICN/2026-08-28/ICN-SFO/2026-09-05?sort=bestflight_a
//     -> sets session cookies, embeds CSRF formToken in HTML
//   POST /i/api/search/dynamic/flights/poll  (repeat until status:"complete")
//     headers: X-CSRF: <formToken>, X-Requested-With: XMLHttpRequest, Cookie: <session>
//     body:    JSON with userSearchParams.legs[] + searchMetaData.pageNumber
//
// Unlike Google Flights' multi-city flow, Kayak's POST does NOT require
// authenticated user sessions — only the anonymous session cookies that
// the initial shell GET sets. Plain net/http + cookiejar works end-to-end.
//
// Response shape (per the captured poll JSON):
//   results[] {
//     bookingOptions[] { displayPrice {price,currency}, bookingUrl }
//     legs[] { segments[] {departure,arrival,duration,carrier,equipment} }
//   }

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	multiCitySearchBase = "https://www.kayak.com/flights"
	pollEndpoint        = "https://www.kayak.com/i/api/search/dynamic/flights/poll"
	maxPolls            = 6
)

// pollSpacing is the delay between poll attempts. A var (not a const) so tests
// can drive the exhausted-without-completion path without real-time sleeps.
var pollSpacing = 1500 * time.Millisecond

// Segment is one leg of a multi-city query.
type Segment struct {
	Origin        string // IATA
	Destination   string // IATA
	DepartureDate string // YYYY-MM-DD
}

// MultiCityOptions tunes a multi-city search.
type MultiCityOptions struct {
	Segments   []Segment
	Passengers int    // default 1
	Cabin      string // "" (economy), "premium", "business", "first"
	Nonstop    bool
	Currency   string // "" (USD)
}

// MultiCityResult is the user-facing summary of a multi-city poll response.
type MultiCityResult struct {
	Success    bool   `json:"success"`
	Source     string `json:"source"`      // "kayak"
	DataSource string `json:"data_source"` // "kayak_web"
	SearchType string `json:"search_type"` // "flights_multicity"
	// Complete reports whether Kayak's incremental search finished (status
	// reached "complete") before maxPolls was exhausted. When false, the
	// Itineraries below are whatever partial set the last poll returned —
	// often far fewer than TotalCount — and the caller should warn rather
	// than present them as the full result set.
	Complete    bool              `json:"complete"`
	Query       MultiCityQuery    `json:"query"`
	Count       int               `json:"count"`
	TotalCount  int               `json:"total_count,omitempty"`
	Itineraries []MultiCityFlight `json:"itineraries"`
	SearchURL   string            `json:"search_url,omitempty"`
}

// MultiCityQuery echoes the user's request back so the JSON consumer can
// re-key the response by input.
type MultiCityQuery struct {
	Segments   []Segment `json:"segments"`
	Passengers int       `json:"passengers"`
	Cabin      string    `json:"cabin,omitempty"`
	Nonstop    bool      `json:"nonstop,omitempty"`
	Currency   string    `json:"currency,omitempty"`
}

// MultiCityFlight is one itinerary covering all N legs of the multi-city
// query. Price is in the response currency (USD unless overridden).
type MultiCityFlight struct {
	Price          float64        `json:"price"`
	Currency       string         `json:"currency"`
	LocalizedPrice string         `json:"localized_price,omitempty"`
	BookingURL     string         `json:"booking_url,omitempty"`
	TotalDuration  int            `json:"total_duration_minutes,omitempty"`
	Legs           []MultiCityLeg `json:"legs"`
}

// MultiCityLeg is one leg (covering one segment of the requested itinerary,
// which itself may have intermediate stops).
type MultiCityLeg struct {
	Origin      string             `json:"origin"`
	Destination string             `json:"destination"`
	DepartTime  string             `json:"depart_time,omitempty"`
	ArriveTime  string             `json:"arrive_time,omitempty"`
	Duration    int                `json:"duration_minutes,omitempty"`
	Stops       int                `json:"stops"`
	Segments    []MultiCitySegment `json:"segments,omitempty"`
}

// MultiCitySegment is one hop within a leg (a leg with stops>0 has multiple).
type MultiCitySegment struct {
	Carrier     string `json:"carrier,omitempty"`
	FlightNum   string `json:"flight_number,omitempty"`
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	DepartTime  string `json:"depart_time,omitempty"`
	ArriveTime  string `json:"arrive_time,omitempty"`
	Duration    int    `json:"duration_minutes,omitempty"`
	Equipment   string `json:"equipment,omitempty"`
}

// formTokenRE matches the inlined formToken assignment in Kayak's shell HTML.
// Format observed live: `formToken = '<87-ish-char-token>';`
var formTokenRE = regexp.MustCompile(`formToken\s*=\s*'([^']+)'`)

// MultiCityClient holds the cookie jar and HTTP client used for one
// multi-city query. One client per call — cookies are not reusable across
// queries (CSRF token is bound to the session that produced it).
type MultiCityClient struct {
	HTTPClient *http.Client
	UserAgent  string
	cookieJar  *cookiejar.Jar
}

// NewMultiCityClient returns a fresh client with a cookie jar.
func NewMultiCityClient() (*MultiCityClient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &MultiCityClient{
		HTTPClient: &http.Client{
			Timeout: 45 * time.Second,
			Jar:     jar,
		},
		UserAgent: browserUA,
		cookieJar: jar,
	}, nil
}

// SearchMultiCity runs a Kayak multi-city query and returns parsed results.
// The two-phase flow: GET the shell URL to warm cookies + extract formToken,
// then POST /poll repeatedly until status:"complete".
func (c *MultiCityClient) SearchMultiCity(ctx context.Context, opts MultiCityOptions) (*MultiCityResult, error) {
	if len(opts.Segments) < 2 {
		return nil, fmt.Errorf("multi-city requires >= 2 segments; got %d", len(opts.Segments))
	}
	if opts.Passengers <= 0 {
		opts.Passengers = 1
	}

	shellURL := buildShellURL(opts)
	formToken, err := c.fetchShellAndExtractToken(ctx, shellURL)
	if err != nil {
		return nil, fmt.Errorf("kayak: fetch shell: %w", err)
	}
	if formToken == "" {
		return nil, fmt.Errorf("kayak: formToken not found in shell HTML (Kayak may have changed shape)")
	}

	body := buildPollBody(opts, "")
	var pollResp pollResponse
	var searchID string
	complete := false
	for attempt := 1; attempt <= maxPolls; attempt++ {
		respBody := body
		if searchID != "" {
			respBody = buildPollBody(opts, searchID)
		}
		raw, err := c.post(ctx, pollEndpoint, formToken, shellURL, respBody)
		if err != nil {
			return nil, fmt.Errorf("kayak: poll #%d: %w", attempt, err)
		}
		pollResp = pollResponse{}
		if jerr := json.Unmarshal(raw, &pollResp); jerr != nil {
			return nil, fmt.Errorf("kayak: poll #%d decode: %w", attempt, jerr)
		}
		if pollResp.SearchID != "" {
			searchID = pollResp.SearchID
		}
		if pollResp.Status == "complete" || pollResp.Status == "completed" {
			complete = true
			break
		}
		if attempt < maxPolls {
			select {
			case <-time.After(pollSpacing):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	out := &MultiCityResult{
		Success:    true,
		Source:     "kayak",
		DataSource: "kayak_web",
		SearchType: "flights_multicity",
		// Complete=false means the loop ran out of poll attempts before Kayak
		// reported the search finished; Itineraries below are a partial set.
		Complete: complete,
		Query: MultiCityQuery{
			Segments: opts.Segments, Passengers: opts.Passengers,
			Cabin: opts.Cabin, Nonstop: opts.Nonstop, Currency: opts.Currency,
		},
		TotalCount: pollResp.TotalCount,
		SearchURL:  shellURL,
	}
	out.Itineraries = parseItineraries(pollResp.Results, pollResp.Legs, pollResp.Segments)
	out.Count = len(out.Itineraries)
	return out, nil
}

// buildShellURL constructs the user-facing /flights URL with segments
// inlined into the path. Mirrors the live shape Kayak's React app generates.
func buildShellURL(opts MultiCityOptions) string {
	parts := []string{multiCitySearchBase}
	for _, s := range opts.Segments {
		parts = append(parts, fmt.Sprintf("%s-%s/%s",
			strings.ToUpper(s.Origin), strings.ToUpper(s.Destination), s.DepartureDate))
	}
	switch strings.ToLower(opts.Cabin) {
	case "business":
		parts = append(parts, "business")
	case "premium":
		parts = append(parts, "premium")
	case "first":
		parts = append(parts, "first")
	}
	if opts.Passengers > 1 {
		parts = append(parts, fmt.Sprintf("%dadults", opts.Passengers))
	}
	if opts.Nonstop {
		parts = append(parts, "nonstop")
	}
	return strings.Join(parts, "/") + "?sort=bestflight_a"
}

// fetchShellAndExtractToken does the initial cookied GET and pulls the
// CSRF formToken out of the inline JS in the response body.
func (c *MultiCityClient) fetchShellAndExtractToken(ctx context.Context, shellURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, shellURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("shell GET returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	m := formTokenRE.FindSubmatch(body)
	if len(m) < 2 {
		return "", nil
	}
	return string(m[1]), nil
}

// post sends the JSON poll body with the headers Kayak's web client uses.
func (c *MultiCityClient) post(ctx context.Context, urlStr, csrf, referer string, body []byte) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, urlStr, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF", csrf)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", referer)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		snippet := string(out)
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("poll returned HTTP %d: %s", resp.StatusCode, snippet)
	}
	return out, nil
}

// buildPollBody produces the JSON body Kayak expects on the poll endpoint.
// searchID is empty on the first call and present on subsequent polls.
func buildPollBody(opts MultiCityOptions, searchID string) []byte {
	legs := make([]map[string]any, 0, len(opts.Segments))
	for _, s := range opts.Segments {
		legs = append(legs, map[string]any{
			"origin": map[string]any{
				"airports":     []string{strings.ToUpper(s.Origin)},
				"locationType": "airports",
			},
			"destination": map[string]any{
				"airports":     []string{strings.ToUpper(s.Destination)},
				"locationType": "airports",
			},
			"date": s.DepartureDate,
			"flex": "exact",
		})
	}
	pax := make([]string, opts.Passengers)
	paxDetails := make([]map[string]any, opts.Passengers)
	for i := range pax {
		pax[i] = "ADT"
		paxDetails[i] = map[string]any{"ptc": "ADT"}
	}
	user := map[string]any{
		"legs":             legs,
		"passengers":       pax,
		"passengerDetails": paxDetails,
		"sortMode":         "bestflight_a",
	}
	if searchID != "" {
		user["searchId"] = searchID
	}
	body := map[string]any{
		"filterParams":     map[string]any{},
		"userSearchParams": user,
		"searchMetaData": map[string]any{
			"pageNumber":  1,
			"searchTypes": []any{},
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		// Inputs are all basic types (string, int, map[string]any with the same).
		// json.Marshal cannot fail on this shape; if it ever does, a future caller
		// added an unserializable value — surface immediately rather than silently
		// returning an empty body (which would land an opaque downstream HTTP error).
		panic(fmt.Sprintf("buildPollBody: json.Marshal failed on basic-type payload: %v", err))
	}
	return b
}

// pollResponse mirrors only the subset of /poll fields we surface. Kayak's
// real response has dozens of additional fields (filters, ads, etc.) that
// we deliberately ignore. legs and segments are top-level lookup MAPS
// keyed by ID; result entries reference IDs and we cross-look them up to
// hydrate timing/carrier detail.
type pollResponse struct {
	Status     string                 `json:"status"`
	SearchID   string                 `json:"searchId"`
	TotalCount int                    `json:"totalCount"`
	Results    []json.RawMessage      `json:"results"`
	Legs       map[string]pollLegRef  `json:"legs"`
	Segments   map[string]pollSegment `json:"segments"`
}

type pollLegRef struct {
	Arrival   string `json:"arrival"`
	Departure string `json:"departure"`
	Duration  int    `json:"duration"`
	Segments  []struct {
		ID string `json:"id"`
	} `json:"segments"`
}

type pollSegment struct {
	Airline            string `json:"airline"`
	Arrival            string `json:"arrival"`
	Departure          string `json:"departure"`
	Origin             string `json:"origin"`
	Destination        string `json:"destination"`
	Duration           int    `json:"duration"`
	FlightNumber       string `json:"flightNumber"`
	Equipment          string `json:"equipmentTypeName"`
	OperationalDisplay string `json:"operationalDisplay"`
}

// parseItineraries walks Kayak's results[] and cross-references the
// top-level legs + segments lookup maps to hydrate timing / carrier /
// route detail. Defensive against missing fields — any malformed row is
// skipped silently rather than failing the whole search.
func parseItineraries(raw []json.RawMessage, legLookup map[string]pollLegRef, segLookup map[string]pollSegment) []MultiCityFlight {
	out := make([]MultiCityFlight, 0, len(raw))
	for _, r := range raw {
		var row struct {
			Legs []struct {
				ID string `json:"id"`
			} `json:"legs"`
			BookingOptions []struct {
				DisplayPrice struct {
					Price          float64 `json:"price"`
					Currency       string  `json:"currency"`
					LocalizedPrice string  `json:"localizedPrice"`
				} `json:"displayPrice"`
				BookingURL struct {
					URL     string `json:"url"`
					URLType string `json:"urlType"`
				} `json:"bookingUrl"`
			} `json:"bookingOptions"`
			TotalDuration int `json:"totalDuration"`
		}
		if err := json.Unmarshal(r, &row); err != nil {
			continue
		}
		if len(row.BookingOptions) == 0 || len(row.Legs) == 0 {
			continue
		}
		best := row.BookingOptions[0]
		flight := MultiCityFlight{
			Price:          best.DisplayPrice.Price,
			Currency:       best.DisplayPrice.Currency,
			LocalizedPrice: best.DisplayPrice.LocalizedPrice,
			BookingURL:     resolveKayakBookingURL(best.BookingURL.URL, best.BookingURL.URLType),
			TotalDuration:  row.TotalDuration,
		}
		// Hydrate each leg from the top-level lookup tables.
		for _, lref := range row.Legs {
			ld, ok := legLookup[lref.ID]
			if !ok {
				continue
			}
			leg := MultiCityLeg{
				DepartTime: ld.Departure,
				ArriveTime: ld.Arrival,
				Duration:   ld.Duration,
				Stops:      len(ld.Segments) - 1,
			}
			if leg.Stops < 0 {
				leg.Stops = 0
			}
			for i, sref := range ld.Segments {
				sd, ok := segLookup[sref.ID]
				if !ok {
					continue
				}
				if i == 0 {
					leg.Origin = sd.Origin
				}
				if i == len(ld.Segments)-1 {
					leg.Destination = sd.Destination
				}
				leg.Segments = append(leg.Segments, MultiCitySegment{
					Carrier:     sd.Airline,
					FlightNum:   sd.FlightNumber,
					Origin:      sd.Origin,
					Destination: sd.Destination,
					DepartTime:  sd.Departure,
					ArriveTime:  sd.Arrival,
					Duration:    sd.Duration,
					Equipment:   sd.Equipment,
				})
			}
			flight.Legs = append(flight.Legs, leg)
		}
		// Skip if every leg's lookup missed.
		if len(flight.Legs) == 0 {
			continue
		}
		out = append(out, flight)
	}
	return out
}

// resolveKayakBookingURL turns Kayak's `{url, urlType}` shape into a clickable
// absolute URL. Relative paths get prefixed with the kayak.com origin.
func resolveKayakBookingURL(raw, urlType string) string {
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://") {
		return raw
	}
	// "relative" type (or absent) gets the origin prefix.
	if strings.HasPrefix(raw, "/") {
		return "https://www.kayak.com" + raw
	}
	if u, err := url.Parse(raw); err == nil && u.Host == "" {
		return "https://www.kayak.com/" + strings.TrimLeft(raw, "/")
	}
	return raw
}
