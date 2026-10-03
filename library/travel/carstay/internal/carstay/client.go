// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package carstay

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/cliutil"
)

type Client struct {
	HTTP    *http.Client
	BaseURL string
	limiter *cliutil.AdaptiveLimiter
}

func New(base string, rate float64) *Client {
	if base == "" {
		base = "https://carstay.jp"
	}
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0 || rate > 2 {
		rate = 2
	}
	return &Client{HTTP: &http.Client{}, BaseURL: strings.TrimRight(base, "/"), limiter: cliutil.NewAdaptiveLimiter(rate)}
}
func (c *Client) get(ctx context.Context, path string, q url.Values) ([]byte, error) {
	u := c.BaseURL + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "carstay-pp-cli/0.1 public-read-only")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Carstay public GET: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(resp)}
	}
	if resp.StatusCode != http.StatusOK {
		return nil, &client.APIError{Method: "GET", Path: path, StatusCode: resp.StatusCode}
	}
	if !strings.Contains(resp.Header.Get("Content-Type"), "json") {
		return nil, fmt.Errorf("Carstay returned HTML instead of JSON for %s", path)
	}
	const maxBytes = 8 << 20
	b, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxBytes {
		return nil, fmt.Errorf("Carstay response exceeds the 8 MiB read cap")
	}
	c.limiter.OnSuccess()
	return b, nil
}
func (c *Client) Directory(ctx context.Context) ([]Spot, error) {
	b, err := c.get(ctx, "/ja/api/data/no-route/stations", nil)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(strings.TrimSpace(string(b)), "[") {
		return nil, fmt.Errorf("invalid Carstay directory array")
	}
	var raw []json.RawMessage
	if err = json.Unmarshal(b, &raw); err != nil {
		return nil, fmt.Errorf("invalid Carstay directory: %w", err)
	}
	if len(raw) > 5000 {
		return nil, fmt.Errorf("directory exceeds 5000-record safety cap")
	}
	out := make([]Spot, 0, len(raw))
	at := time.Now().UTC().Format(time.RFC3339)
	for _, r := range raw {
		s, e := Normalize(r, at)
		if e != nil {
			return nil, e
		}
		out = append(out, s)
	}
	return out, nil
}
func (c *Client) Detail(ctx context.Context, s Spot) (Spot, error) {
	if !ValidID(s.ID) || !Overnight(s) {
		return Spot{}, fmt.Errorf("detail requires a known overnight station")
	}
	path := fmt.Sprintf("/ja/api/data/stations/%s/station/%s", AreaFor(s.Prefecture), s.ID)
	b, err := c.get(ctx, path, nil)
	if err != nil {
		return Spot{}, err
	}
	var obj struct {
		Station json.RawMessage `json:"station"`
	}
	if err = json.Unmarshal(b, &obj); err != nil {
		return Spot{}, fmt.Errorf("invalid Carstay detail: %w", err)
	}
	d, err := Normalize(obj.Station, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return Spot{}, err
	}
	if d.ID != s.ID {
		return Spot{}, fmt.Errorf("detail ID mismatch")
	}
	if !Overnight(d) {
		return Spot{}, fmt.Errorf("detail is not an overnight station")
	}
	return d, nil
}

type DateSearch struct {
	Spots   []Spot
	Total   int
	Pages   int
	Scanned int
	Scope   string
}

func (c *Client) DateCandidates(ctx context.Context, language, in, out string, maxPages int) (DateSearch, error) {
	v := DateSearch{Spots: []Spot{}, Scope: "provider_date_filtered_candidates_availability_unknown"}
	if err := Dates(in, out); err != nil || in == "" {
		if err == nil {
			err = fmt.Errorf("date candidates require both JST dates")
		}
		return v, err
	}
	if language != "ja" && language != "en" {
		return v, fmt.Errorf("language must be ja or en")
	}
	if maxPages < 1 || maxPages > 10 {
		return v, fmt.Errorf("max scan pages must be 1 to 10")
	}
	seen := map[string]bool{}
	for page := 1; page <= maxPages; page++ {
		q := url.Values{"checkIn": {in}, "checkOut": {out}, "page": {strconv.Itoa(page)}}
		b, err := c.get(ctx, "/"+language+"/api/data/stations", q)
		if err != nil {
			return v, err
		}
		var raw struct {
			Total    *int              `json:"total"`
			Stations []json.RawMessage `json:"stations"`
		}
		if err = json.Unmarshal(b, &raw); err != nil || raw.Total == nil || *raw.Total < 0 || raw.Stations == nil {
			return v, fmt.Errorf("invalid dated candidate envelope")
		}
		if len(raw.Stations) > 100 {
			return v, fmt.Errorf("dated page exceeds 100-record safety cap")
		}
		if page == 1 {
			v.Total = *raw.Total
		}
		v.Pages++
		v.Scanned += len(raw.Stations)
		at := time.Now().UTC().Format(time.RFC3339)
		for _, r := range raw.Stations {
			s, e := Normalize(r, at)
			if e != nil {
				return v, e
			}
			if seen[s.ID] {
				return v, fmt.Errorf("source repeated station across dated pages; refuse incomplete pagination")
			}
			seen[s.ID] = true
			// The date route language is provenance, independent of preserved Japanese names.
			s.SourceLanguage = language
			if Overnight(s) {
				v.Spots = append(v.Spots, s)
			}
		}
		if len(raw.Stations) == 0 || v.Scanned >= v.Total {
			break
		}
	}
	return v, nil
}
