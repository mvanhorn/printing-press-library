// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Package seatsaero is a thin native client for the Seats.aero Partner API
// (https://seats.aero/partnerapi). Seats.aero caches award/mileage redemption
// availability across airline alliance programs, then exposes it read-only via
// a Partner API. This integrates award (miles) availability into flight-goat
// alongside the cash-fare sources (Google Flights, Kayak, FlySoar).
//
// Auth: the Partner API requires an API key sent as the Partner-Authorization
// header. flight-goat reads it from SEATS_AERO_API_KEY (the same env var the
// standalone seats-aero skill uses) or the config file. A key is required for
// live searches; cached search (the endpoint this client uses) is available to
// Pro users. Live search is commercial-only and intentionally not exposed.
//
// Response shapes are from the current seats.aero OpenAPI reference (v1.0);
// see https://developers.seats.aero/reference/cached-search.
package seatsaero

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// DefaultBaseURL is the Seats.aero Partner API root. All paths below are
// joined onto it.
const DefaultBaseURL = "https://seats.aero/partnerapi"

func defaultAPIKey() string {
	if v := strings.TrimSpace(os.Getenv("SEATS_AERO_API_KEY")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("SEATS_AERO_PARTNER_PARTNER_AUTHORIZATION")); v != "" {
		return v
	}
	// config.toml fallback, mirroring the standalone seats-aero skill's layout.
	// The config file stores the key under `aero_partner_partner_authorization`.
	p := os.Getenv("SEATS_AERO_CONFIG")
	if p == "" {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, ".config", "seats-aero-pp-cli", "config.toml")
		}
	}
	if p == "" {
		return ""
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	var cfg struct {
		SeatsAeroPartnerPartnerAuthorization string `toml:"aero_partner_partner_authorization"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return ""
	}
	return strings.TrimSpace(cfg.SeatsAeroPartnerPartnerAuthorization)
}

// Client talks to the Seats.aero Partner API.
type Client struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
}

// NewClient builds a Client with defaults. APIKey is resolved from
// SEATS_AERO_API_KEY (or the config file) when left empty.
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		apiKey = defaultAPIKey()
	}
	return &Client{
		BaseURL: strings.TrimRight(DefaultBaseURL, "/"),
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 45 * time.Second},
	}
}

// ErrNoAPIKey is returned when a live search is attempted without a configured
// Partner API key.
type ErrNoAPIKey struct{}

func (ErrNoAPIKey) Error() string {
	return "seats.aero requires a Partner API key: set SEATS_AERO_API_KEY (Pro users can generate one in seats.aero settings)"
}

// SearchParams mirror the cached-search query parameters.
type SearchParams struct {
	// OriginAirport / DestinationAirport are IATA codes; comma-delimited lists
	// (e.g. "SFO,LAX") are allowed.
	OriginAirport      string
	DestinationAirport string
	// StartDate / EndDate bound the departure window in YYYY-MM-DD (inclusive).
	StartDate string
	EndDate   string
	// Cabin filters availability by cabin: economy, premium, business, first.
	Cabin string
	// OrderBy: empty (default: by date, premium-first) or "lowest_mileage".
	OrderBy string
	// OnlyDirectFlights restricts to non-stop availability when true.
	OnlyDirectFlights bool
	// Take caps results (10..1000; default 500).
	Take int
}

// AvailabilityEntry is one cached itinerary award availability row.
type AvailabilityEntry struct {
	ID      string `json:"ID"`
	RouteID string `json:"RouteID"`
	Route   struct {
		OriginAirport      string `json:"OriginAirport"`
		DestinationAirport string `json:"DestinationAirport"`
		Source             string `json:"Source"`
		NumDaysOut         int    `json:"NumDaysOut"`
		Distance           int    `json:"Distance"`
	} `json:"Route"`
	Date string `json:"Date"`
	// Cabin availability + mileage costs. Seats.aero emits mileage costs as
	// strings ("12500") because some programs quote non-integer/absent values;
	// the cli layer parses them.
	YAvailable bool   `json:"YAvailable"`
	WAvailable bool   `json:"WAvailable"`
	JAvailable bool   `json:"JAvailable"`
	FAvailable bool   `json:"FAvailable"`
	YMileage   string `json:"YMileageCost"`
	WMileage   string `json:"WMileageCost"`
	JMileage   string `json:"JMileageCost"`
	FMileage   string `json:"FMileageCost"`
	// Direct-flight availability flags (nonstop per cabin).
	YDirect bool   `json:"YDirect"`
	WDirect bool   `json:"WDirect"`
	JDirect bool   `json:"JDirect"`
	FDirect bool   `json:"FDirect"`
	Source  string `json:"Source"`
}

// AnyDirect reports whether any cabin has a nonstop award option. The
// human-facing table uses this for its NONSTOP column so a business-only
// direct itinerary isn't shown as non-direct just because economy isn't.
func (e *AvailabilityEntry) AnyDirect() bool {
	return e.YDirect || e.WDirect || e.JDirect || e.FDirect
}

// SearchResult is the parsed cached-search response.
type SearchResult struct {
	Data    []AvailabilityEntry `json:"data"`
	Count   int                 `json:"count"`
	HasMore bool                `json:"hasMore"`
	Cursor  int                 `json:"cursor"`
	// APIKeyUsed / Cached describe provenance. This is the cached-search
	// endpoint, so Cached is always true on success — it is NOT a live
	// redemption search (those are commercial-only) and callers must not
	// present the data as freshly computed.
	APIKeyUsed bool `json:"api_key_used,omitempty"`
	Cached     bool `json:"cached"`
}

// Search runs a cached award-availability search. Returns ErrNoAPIKey when no
// API key is configured (callers may pre-check via HasAPIKey to emit a clearer
// message; a call without a key is an error, never a silent empty result).
func (c *Client) Search(ctx context.Context, p SearchParams) (*SearchResult, error) {
	if c.APIKey == "" {
		return nil, ErrNoAPIKey{}
	}
	u, err := url.Parse(c.BaseURL + "/search")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("origin_airport", p.OriginAirport)
	q.Set("destination_airport", p.DestinationAirport)
	if p.StartDate != "" {
		q.Set("start_date", p.StartDate)
	}
	if p.EndDate != "" {
		q.Set("end_date", p.EndDate)
	}
	if p.Cabin != "" {
		q.Set("cabin", p.Cabin)
	}
	if p.OrderBy != "" {
		q.Set("order_by", p.OrderBy)
	}
	if p.OnlyDirectFlights {
		q.Set("only_direct_flights", "true")
	}
	if p.Take > 0 {
		q.Set("take", fmt.Sprintf("%d", p.Take))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Partner-Authorization", c.APIKey)
	req.Header.Set("User-Agent", "flight-goat-pp-cli/seatsaero")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("seats.aero search: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("seats.aero search: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("seats.aero search: HTTP %d: %s", resp.StatusCode, truncate(body))
	}
	var result SearchResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("seats.aero search: decode: %w (body=%s)", err, truncate(body))
	}
	result.Cached = true
	return &result, nil
}

// HasAPIKey reports whether a Partner API key is present so callers can prompt
// before a request.
func (c *Client) HasAPIKey() bool { return c.APIKey != "" }

func truncate(b []byte) string {
	const max = 512
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
