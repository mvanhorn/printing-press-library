// Package googleplaces is the Stage-1 seed source for the two-stage funnel.
//
// Uses the Google Places API (New) v1 — "places:searchNearby" and
// "places:searchText". Returns OPERATIONAL places only (CLOSED_PERMANENTLY
// is filtered out at seed; CLOSED_TEMPORARILY is preserved with the
// business_status field so the dispatcher can surface it as a warning).
//
// Auth: GOOGLE_PLACES_API_KEY env var. Free $200/mo credit per Google's
// pricing. The CLI's `doctor` command checks for the key and exits code 4
// (auth error) when a Stage-1 command is invoked without it.
//
// Wire format:
//
//	POST https://places.googleapis.com/v1/places:searchNearby
//	  X-Goog-Api-Key: <key>
//	  X-Goog-FieldMask: places.id,places.displayName,places.location,...
//	  Body: { "locationRestriction": { "circle": { "center":..., "radius":... } } }
package googleplaces

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wanderlust-goat/internal/httperr"
)

// EnvVar is the name of the env var carrying the API key.
const EnvVar = "GOOGLE_PLACES_API_KEY"

// ErrMissingAPIKey is returned by NewClient when the env var is unset.
var ErrMissingAPIKey = errors.New("GOOGLE_PLACES_API_KEY is not set; get a key at https://developers.google.com/maps/documentation/places/web-service/get-api-key")

// FieldMask is the default X-Goog-FieldMask sent on every request. Keep
// this list narrow — Google bills per requested field. Adding a field
// here grows every Stage-1 invocation's cost.
const FieldMask = "places.id," +
	"places.displayName," +
	"places.formattedAddress," +
	"places.location," +
	"places.types," +
	"places.primaryType," +
	"places.rating," +
	"places.userRatingCount," +
	"places.businessStatus," +
	"places.googleMapsUri"

// Client is the Stage-1 seed client.
type Client struct {
	apiKey     string
	httpClient *http.Client
	base       string
}

// Place is a normalized seed candidate. JSON tags match the dispatcher's
// Candidate shape so passing through is cheap.
type Place struct {
	ID              string   `json:"id"`
	DisplayName     string   `json:"display_name"`
	Address         string   `json:"address"`
	Lat             float64  `json:"lat"`
	Lng             float64  `json:"lng"`
	Types           []string `json:"types"`
	PrimaryType     string   `json:"primary_type,omitempty"`
	Rating          float64  `json:"rating,omitempty"`
	UserRatingCount int      `json:"user_rating_count,omitempty"`
	BusinessStatus  string   `json:"business_status,omitempty"`
	GoogleMapsURI   string   `json:"google_maps_uri,omitempty"`
}

// NewClient returns a Client using GOOGLE_PLACES_API_KEY. Returns
// ErrMissingAPIKey when the env var is unset.
func NewClient() (*Client, error) {
	key := strings.TrimSpace(os.Getenv(EnvVar))
	if key == "" {
		return nil, ErrMissingAPIKey
	}
	return &Client{
		apiKey:     key,
		httpClient: &http.Client{Timeout: 12 * time.Second},
		base:       "https://places.googleapis.com",
	}, nil
}

// NewClientWithBase is for tests; allows pointing at httptest.Server.
func NewClientWithBase(apiKey, baseURL string) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: 12 * time.Second},
		base:       strings.TrimRight(baseURL, "/"),
	}
}

// SetHTTPClient lets callers swap the HTTP client (rate-limit wrappers,
// instrumented transports). Optional.
func (c *Client) SetHTTPClient(hc *http.Client) {
	if hc != nil {
		c.httpClient = hc
	}
}

// nearbyRequest matches the Places API (New) v1 searchNearby body.
type nearbyRequest struct {
	IncludedTypes       []string       `json:"includedTypes,omitempty"`
	MaxResultCount      int            `json:"maxResultCount,omitempty"`
	LanguageCode        string         `json:"languageCode,omitempty"`
	LocationRestriction locationCircle `json:"locationRestriction"`
}

type locationCircle struct {
	Circle circle `json:"circle"`
}
type circle struct {
	Center latLng  `json:"center"`
	Radius float64 `json:"radius"` // meters
}
type latLng struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// textRequest matches the Places API (New) v1 searchText body.
type textRequest struct {
	TextQuery      string          `json:"textQuery"`
	MaxResultCount int             `json:"maxResultCount,omitempty"`
	LanguageCode   string          `json:"languageCode,omitempty"`
	LocationBias   *locationCircle `json:"locationBias,omitempty"`
}

// nearbyResponse matches the Places API v1 response envelope.
type apiResponse struct {
	Places []rawPlace `json:"places"`
}

type rawPlace struct {
	ID               string      `json:"id"`
	DisplayName      displayName `json:"displayName"`
	FormattedAddress string      `json:"formattedAddress"`
	Location         latLng      `json:"location"`
	Types            []string    `json:"types"`
	PrimaryType      string      `json:"primaryType"`
	Rating           float64     `json:"rating"`
	UserRatingCount  int         `json:"userRatingCount"`
	BusinessStatus   string      `json:"businessStatus"`
	GoogleMapsURI    string      `json:"googleMapsUri"`
}

type displayName struct {
	Text         string `json:"text"`
	LanguageCode string `json:"languageCode"`
}

// NearbySearch performs places:searchNearby. center is the seed point;
// radiusMeters is the circle radius in crow-flies meters; includedTypes
// (optional) restricts to e.g. ["cafe","restaurant"]; maxResults clamps
// candidate count (Google caps at 20). languageCode defaults to "en".
//
// CLOSED_PERMANENTLY entries are dropped at seed; CLOSED_TEMPORARILY
// pass through with BusinessStatus populated so the caller can surface
// the warning.
func (c *Client) NearbySearch(ctx context.Context, lat, lng, radiusMeters float64, includedTypes []string, maxResults int, languageCode string) ([]Place, error) {
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 20
	}
	if languageCode == "" {
		languageCode = "en"
	}
	body := nearbyRequest{
		IncludedTypes:  includedTypes,
		MaxResultCount: maxResults,
		LanguageCode:   languageCode,
		LocationRestriction: locationCircle{
			Circle: circle{
				Center: latLng{Latitude: lat, Longitude: lng},
				Radius: radiusMeters,
			},
		},
	}
	return c.post(ctx, "/v1/places:searchNearby", body)
}

// SearchText performs places:searchText. Free-text query; optional location
// bias (lat, lng, radius) when known. Returns the same Place shape as
// NearbySearch, with the same OPERATIONAL filter applied.
func (c *Client) SearchText(ctx context.Context, query string, biasLat, biasLng, biasRadius float64, maxResults int, languageCode string) ([]Place, error) {
	if strings.TrimSpace(query) == "" {
		return nil, errors.New("googleplaces.SearchText: query is empty")
	}
	if maxResults <= 0 || maxResults > 20 {
		maxResults = 20
	}
	if languageCode == "" {
		languageCode = "en"
	}
	req := textRequest{
		TextQuery:      query,
		MaxResultCount: maxResults,
		LanguageCode:   languageCode,
	}
	if biasRadius > 0 {
		req.LocationBias = &locationCircle{
			Circle: circle{
				Center: latLng{Latitude: biasLat, Longitude: biasLng},
				Radius: biasRadius,
			},
		}
	}
	return c.post(ctx, "/v1/places:searchText", req)
}

func (c *Client) post(ctx context.Context, path string, body any) ([]Place, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Goog-Api-Key", c.apiKey)
	req.Header.Set("X-Goog-FieldMask", FieldMask)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("googleplaces %s: %w", path, err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("googleplaces %s: read body: %w", path, err)
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("googleplaces %s: auth failed (HTTP %d): %s", path, resp.StatusCode, httperr.Snippet(respBody))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("googleplaces %s: HTTP %d: %s", path, resp.StatusCode, httperr.Snippet(respBody))
	}
	var out apiResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("googleplaces %s: parse response: %w", path, err)
	}
	results := make([]Place, 0, len(out.Places))
	for _, p := range out.Places {
		// HARD FILTER: drop CLOSED_PERMANENTLY at seed.
		if strings.EqualFold(p.BusinessStatus, "CLOSED_PERMANENTLY") {
			continue
		}
		results = append(results, Place{
			ID:              p.ID,
			DisplayName:     p.DisplayName.Text,
			Address:         p.FormattedAddress,
			Lat:             p.Location.Latitude,
			Lng:             p.Location.Longitude,
			Types:           p.Types,
			PrimaryType:     p.PrimaryType,
			Rating:          p.Rating,
			UserRatingCount: p.UserRatingCount,
			BusinessStatus:  p.BusinessStatus,
			GoogleMapsURI:   p.GoogleMapsURI,
		})
	}
	return results, nil
}

// HasAPIKey returns true when GOOGLE_PLACES_API_KEY is set. Cheap helper
// for `doctor` and command preflight.
func HasAPIKey() bool {
	return strings.TrimSpace(os.Getenv(EnvVar)) != ""
}
