// Copyright 2026 zjsng. Licensed under Apache-2.0.
package cycling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const feedLimit = 16 << 20
const pageLimit = 1 << 20

type Client struct {
	HTTP     *http.Client
	indexURL string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return fmt.Errorf("unexpected source redirect; consult %s", req.URL.Host)
	}}, indexURL: IndexURL}
}

type HTTPError struct {
	Status int
	Host   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("%s returned HTTP %d; retry later or inspect the official source", e.Host, e.Status)
}

// StatusFeedError retains discovery data while signalling that the observation
// cannot safely replace a saved availability snapshot.
type StatusFeedError struct{ Cause error }

func (e *StatusFeedError) Error() string { return "station_status unavailable: " + e.Cause.Error() }
func (e *StatusFeedError) Unwrap() error { return e.Cause }
func (c *Client) get(ctx context.Context, u string, limit int64) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Accept", "application/json, text/html;q=0.8")
	req.Header.Set("User-Agent", "hello-cycling-pp-cli/0.1 read-only station planner")
	r, e := c.HTTP.Do(req)
	if e != nil {
		return nil, fmt.Errorf("source request failed for %s: %w", req.URL.Host, e)
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		if r.StatusCode == 429 {
			return nil, &cliutil.RateLimitError{URL: u, Cause: &HTTPError{r.StatusCode, req.URL.Host}}
		}
		return nil, &HTTPError{r.StatusCode, req.URL.Host}
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("source response exceeds %d bytes; no partial snapshot is used", limit)
	}
	return b, nil
}

type envelope struct {
	TTL         int             `json:"ttl"`
	LastUpdated int64           `json:"last_updated"`
	Version     string          `json:"version"`
	Data        json.RawMessage `json:"data"`
}

func (c *Client) feed(ctx context.Context, u string, dst any) (FeedMeta, int64, error) {
	b, e := c.get(ctx, u, feedLimit)
	if e != nil {
		return FeedMeta{}, 0, e
	}
	var env envelope
	if e = json.Unmarshal(b, &env); e != nil {
		return FeedMeta{}, int64(len(b)), fmt.Errorf("invalid GBFS response from %s: %w", u, e)
	}
	if env.LastUpdated <= 0 || env.Version != "2.3" || len(env.Data) == 0 {
		return FeedMeta{}, int64(len(b)), fmt.Errorf("unsupported or incomplete GBFS metadata at %s", u)
	}
	if e = json.Unmarshal(env.Data, dst); e != nil {
		return FeedMeta{}, int64(len(b)), fmt.Errorf("invalid GBFS data at %s: %w", u, e)
	}
	return FeedMeta{u, env.LastUpdated, env.TTL, env.Version}, int64(len(b)), nil
}
func (c *Client) allowedFeed(raw, name string) bool {
	u, e := url.Parse(raw)
	base, e2 := url.Parse(c.indexURL)
	if e != nil || e2 != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	return u.Scheme == base.Scheme && u.Host == base.Host && u.Path == strings.TrimSuffix(base.Path, "gbfs.json")+name+".json"
}

// Fetch makes one index request and three advertised feed requests, with no retries.
func (c *Client) Fetch(ctx context.Context) (Snapshot, error) {
	s := Snapshot{ObservedAt: time.Now().UTC(), Feeds: map[string]FeedMeta{}, Warnings: make([]string, 0), Vehicles: make([]Vehicle, 0)}
	var idx map[string]struct {
		Feeds []struct {
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"feeds"`
	}
	m, n, e := c.feed(ctx, c.indexURL, &idx)
	s.Requests++
	s.Bytes += n
	if e != nil {
		return s, e
	}
	s.Feeds["gbfs"] = m
	urls := map[string]string{}
	for _, f := range idx["ja"].Feeds {
		if f.Name == "station_information" || f.Name == "station_status" || f.Name == "vehicle_types" {
			if !c.allowedFeed(f.URL, f.Name) {
				return s, fmt.Errorf("GBFS advertises an unexpected %s URL; no request sent", f.Name)
			}
			urls[f.Name] = f.URL
		}
	}
	if urls["station_information"] == "" || urls["station_status"] == "" || urls["vehicle_types"] == "" {
		return s, fmt.Errorf("GBFS index is missing required Japanese station/type feeds")
	}
	var info struct {
		Stations []Info `json:"stations"`
	}
	var status struct {
		Stations []Status `json:"stations"`
	}
	var types struct {
		Types []Vehicle `json:"vehicle_types"`
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	errs := map[string]error{}
	for name, dst := range map[string]any{"station_information": &info, "station_status": &status, "vehicle_types": &types} {
		wg.Add(1)
		go func(name string, dst any) {
			defer wg.Done()
			meta, size, err := c.feed(ctx, urls[name], dst)
			mu.Lock()
			defer mu.Unlock()
			s.Requests++
			s.Bytes += size
			if err != nil {
				errs[name] = err
			} else {
				s.Feeds[name] = meta
			}
		}(name, dst)
	}
	wg.Wait()
	if e := errs["station_information"]; e != nil {
		return s, e
	}
	if len(info.Stations) == 0 {
		return s, fmt.Errorf("station_information has no stations; source schema may have changed")
	}
	for _, name := range []string{"station_status", "vehicle_types"} {
		if err := errs[name]; err != nil {
			var h *HTTPError
			if errors.As(err, &h) && (h.Status == 429 || h.Status == 401 || h.Status == 403) {
				return s, err
			}
			s.Warnings = append(s.Warnings, name+" unavailable: "+err.Error())
		}
	}
	seen := map[string]bool{}
	for _, r := range info.Stations {
		if r.ID == "" || r.Name == "" || seen[r.ID] {
			return s, fmt.Errorf("station_information contains missing/duplicate identity")
		}
		seen[r.ID] = true
	}
	seen = map[string]bool{}
	for _, r := range status.Stations {
		if r.ID == "" || seen[r.ID] {
			return s, fmt.Errorf("station_status contains missing/duplicate identity")
		}
		seen[r.ID] = true
	}
	s.Information = info.Stations
	if errs["station_status"] == nil {
		s.Statuses = status.Stations
	}
	if errs["vehicle_types"] == nil {
		s.Vehicles = types.Types
	}
	s.ObservedAt = time.Now().UTC()
	if err := errs["station_status"]; err != nil {
		return s, &StatusFeedError{Cause: err}
	}
	if len(s.Statuses) == 0 {
		err := fmt.Errorf("station_status has no rows; source schema may have changed")
		s.Warnings = append(s.Warnings, err.Error())
		return s, &StatusFeedError{Cause: err}
	}
	return s, nil
}

// VehicleRules combines current provider classes with explicitly sourced model/eligibility boundaries.
func (c *Client) VehicleRules(ctx context.Context) (map[string]any, error) {
	var data struct {
		Types []Vehicle `json:"vehicle_types"`
	}
	u := strings.TrimSuffix(c.indexURL, "gbfs.json") + "vehicle_types.json"
	m, n, e := c.feed(ctx, u, &data)
	if e != nil {
		return nil, e
	}
	rulesBody, e := c.get(ctx, RulesURL, pageLimit)
	if e != nil {
		return nil, e
	}
	for _, fragment := range []string{"交通ルールテストへの合格", "16歳以上", "年齢確認書類の提出", "HELLO CYCLINGアプリからしか"} {
		if !strings.Contains(string(rulesBody), fragment) {
			return nil, fmt.Errorf("vehicle eligibility page changed; consult %s", RulesURL)
		}
	}
	n += int64(len(rulesBody))
	return map[string]any{"meta": map[string]any{"source": "live", "observed_at": time.Now().UTC(), "request_count": 2, "response_bytes": n, "feed": m, "attribution": "HELLO CYCLING / OpenStreet Co., Ltd. via ODPT", "license": "CC BY 4.0", "license_url": LicenseURL}, "results": data.Types, "rules_source_url": RulesURL, "scope": "Published GBFS classes only. Type 2 is a generic electric-assist bicycle; it does not identify city/sports/e-Bike models, batteries or model-specific rates.", "rules": []string{"Some vehicle models must be returned at dedicated or compatible stations; confirm the actual vehicle in the app.", "Electric cycles are currently app-only. The provider requires passing its traffic-rules test and submitting proof of age 16 or older.", "Electric-cycle stations must be selected using the electric-cycle filter in the official app; generic GBFS class compatibility does not prove electric-cycle eligibility."}}, nil
}
