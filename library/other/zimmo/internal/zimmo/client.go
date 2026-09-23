// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.

package zimmo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/cliutil"
)

// Client talks to Zimmo's JSON micro-services with the anonymous token.
type Client struct {
	HTTP    *http.Client
	Limiter *cliutil.AdaptiveLimiter
	// Token, when set, is used instead of minting an anonymous one
	// (ZIMMO_TOKEN override).
	Token string
}

// ErrNotFound is returned for 404 responses.
var ErrNotFound = errors.New("not found on Zimmo")

// NewClient returns a client with a polite default pace (2 req/s ceiling).
func NewClient(timeout time.Duration, token string) *Client {
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Client{
		HTTP:    &http.Client{Timeout: timeout},
		Limiter: cliutil.NewAdaptiveLimiter(2),
		Token:   token,
	}
}

// APIError is a non-2xx response other than 404/429.
type APIError struct {
	Status int
	URL    string
	Body   string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Zimmo API %s: HTTP %d: %s", e.URL, e.Status, truncate(e.Body, 300))
}

func (c *Client) bearer(ctx context.Context) (string, error) {
	if c.Token != "" {
		return c.Token, nil
	}
	return AnonymousToken(ctx, c.HTTP)
}

// Do sends one request and returns the body. It re-mints the anonymous
// token once on 401 and retries 429/5xx with backoff.
func (c *Client) Do(ctx context.Context, method, rawURL string, params map[string]string, body any) ([]byte, error) {
	if len(params) > 0 {
		q := url.Values{}
		for k, v := range params {
			if v != "" {
				q.Set(k, v)
			}
		}
		if enc := q.Encode(); enc != "" {
			sep := "?"
			if strings.Contains(rawURL, "?") {
				sep = "&"
			}
			rawURL += sep + enc
		}
	}
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = b
	}
	reauthed := false
	for attempt := 0; ; attempt++ {
		if err := c.Limiter.Wait(ctx); err != nil {
			return nil, err
		}
		var rdr io.Reader
		if payload != nil {
			rdr = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, rawURL, rdr)
		if err != nil {
			return nil, err
		}
		setCommonHeaders(req)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		tok, err := c.bearer(ctx)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := c.HTTP.Do(req)
		if err != nil {
			if attempt < 2 && ctx.Err() == nil {
				if err := sleepCtx(ctx, cliutil.Backoff(attempt)); err != nil {
					return nil, err
				}
				continue
			}
			return nil, fmt.Errorf("calling %s: %w", rawURL, err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			c.Limiter.OnSuccess()
			return raw, nil
		case resp.StatusCode == http.StatusUnauthorized && !reauthed && c.Token == "":
			InvalidateToken()
			reauthed = true
			continue
		case resp.StatusCode == http.StatusNotFound:
			return nil, fmt.Errorf("%w: %s", ErrNotFound, rawURL)
		case resp.StatusCode == http.StatusTooManyRequests:
			c.Limiter.OnRateLimit()
			if attempt < 3 {
				wait := cliutil.RetryAfter(resp)
				if wait <= 0 {
					wait = cliutil.Backoff(attempt)
				}
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(wait):
				}
				continue
			}
			return nil, &cliutil.RateLimitError{URL: rawURL, RetryAfter: cliutil.RetryAfter(resp), Body: truncate(string(raw), 200)}
		case resp.StatusCode >= 500 && attempt < 2:
			if err := sleepCtx(ctx, cliutil.Backoff(attempt)); err != nil {
				return nil, err
			}
			continue
		default:
			return nil, &APIError{Status: resp.StatusCode, URL: rawURL, Body: string(raw)}
		}
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// SearchRequest is the body of POST listings/search.
type SearchRequest struct {
	Paging  Paging           `json:"paging"`
	Sorting []map[string]any `json:"sorting,omitempty"`
	Filter  map[string]any   `json:"filter"`
}

// Paging is Zimmo's offset paging.
type Paging struct {
	From int `json:"from"`
	Size int `json:"size"`
}

// SearchResult is one page of results.
type SearchResult struct {
	Total    int               `json:"total"`
	Listings []json.RawMessage `json:"listings"`
}

// MaxPageSize is the largest page the site itself requests comfortably.
const MaxPageSize = 100

// Search runs one listings/search page.
func (c *Client) Search(ctx context.Context, req SearchRequest) (SearchResult, error) {
	if req.Paging.Size <= 0 {
		req.Paging.Size = 50
	}
	var out SearchResult
	raw, err := c.Do(ctx, http.MethodPost, SearchHost+"/listings/search", nil, req)
	if err != nil {
		return out, err
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return out, fmt.Errorf("decoding search response: %w", err)
	}
	return out, nil
}

// Listing fetches one listing by zimmo code or UUID.
func (c *Client) Listing(ctx context.Context, ref string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodGet, SearchHost+"/listings/"+url.PathEscape(ref), nil, nil)
}

// Place is one geo-api place.
type Place struct {
	ID           int               `json:"id"`
	Translations map[string]string `json:"translations"`
	Area         struct {
		ID         string `json:"id"`
		Level      int    `json:"level"`
		Name       string `json:"name"`
		Slug       string `json:"slug"`
		PostalCode string `json:"postalCode"`
		ParentID   string `json:"parentId"`
	} `json:"administrativeArea"`
	Center struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"center"`
}

// Name returns the French name, falling back to Dutch or the area name.
func (p Place) Name() string {
	for _, k := range []string{"fr", "nl", "en"} {
		if v := p.Translations[k]; v != "" {
			return v
		}
	}
	return p.Area.Name
}

// Places resolves a postcode and/or name.
func (c *Client) Places(ctx context.Context, postcode, name string) ([]Place, error) {
	raw, err := c.Do(ctx, http.MethodGet, GeoHost+"/places/by-postal-code-and-or-name", map[string]string{"postalCode": postcode, "name": name}, nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		Places []Place `json:"places"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decoding places: %w", err)
	}
	return out.Places, nil
}

// Geocoded is the useful part of geo-api/geocode.
type Geocoded struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Places    []Place `json:"places"`
}

// Geocode resolves a Belgian address.
func (c *Client) Geocode(ctx context.Context, address string) (Geocoded, error) {
	var g Geocoded
	raw, err := c.Do(ctx, http.MethodGet, GeoHost+"/geocode", map[string]string{"address": address}, nil)
	if err != nil {
		return g, err
	}
	var out struct {
		Coordinates struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
		} `json:"coordinates"`
		Places []Place `json:"places"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return g, fmt.Errorf("decoding geocode: %w", err)
	}
	g.Latitude, g.Longitude, g.Places = out.Coordinates.Latitude, out.Coordinates.Longitude, out.Places
	if g.Latitude == 0 && g.Longitude == 0 {
		return g, fmt.Errorf("%w: address %q could not be geocoded", ErrNotFound, address)
	}
	return g, nil
}

// TypePrice is €/m² for one property type.
type TypePrice struct {
	Type  string  `json:"type"`
	Price float64 `json:"price"`
	Count int     `json:"count,omitempty"`
}

// PricePoint is one month of the €/m² series.
type PricePoint struct {
	Date  string      `json:"date"`
	Types []TypePrice `json:"types"`
}

// LocalityPrice is score-api's €/m² response.
type LocalityPrice struct {
	PlaceID  int          `json:"place_id"`
	Price    float64      `json:"price_per_m2"`
	Types    []TypePrice  `json:"types"`
	History  []PricePoint `json:"history"`
	Endpoint string       `json:"-"`
}

// ForType returns the €/m² for a category (HOUSE/APARTMENT), falling back
// to the all-types price.
func (lp LocalityPrice) ForType(category string) (float64, int, bool) {
	for _, t := range lp.Types {
		if strings.EqualFold(t.Type, category) && t.Price > 0 {
			return t.Price, t.Count, true
		}
	}
	if lp.Price > 0 {
		return lp.Price, 0, false
	}
	return 0, 0, false
}

// LocalityPriceFor fetches €/m² for a place; sub=true uses the
// sub-locality endpoint.
func (c *Client) LocalityPriceFor(ctx context.Context, placeID int, startDate string, sub bool) (LocalityPrice, error) {
	path := "/locality-price/"
	if sub {
		path = "/sub-locality-price/"
	}
	lp := LocalityPrice{PlaceID: placeID}
	raw, err := c.Do(ctx, http.MethodGet, ScoreHost+path+strconv.Itoa(placeID), map[string]string{"startDate": startDate}, nil)
	if err != nil {
		return lp, err
	}
	var out struct {
		PPSM struct {
			Price            float64      `json:"price"`
			Types            []TypePrice  `json:"types"`
			HistoricalPrices []PricePoint `json:"historicalPrices"`
		} `json:"pricePerSquareMeter"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return lp, fmt.Errorf("decoding locality price: %w", err)
	}
	lp.Price, lp.Types, lp.History = out.PPSM.Price, out.PPSM.Types, out.PPSM.HistoricalPrices
	return lp, nil
}

// Dealer fetches an agency.
func (c *Client) Dealer(ctx context.Context, id string) (json.RawMessage, error) {
	return c.Do(ctx, http.MethodGet, SearchHost+"/dealers/"+url.PathEscape(id), nil, nil)
}
