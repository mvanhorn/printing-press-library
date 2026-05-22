// Package supa is the Supabase PostgREST client used by the prospeo CLI for
// cache lookups, audit logging, and offline FTS-style queries.
//
// Schema: every table lives under the `outreach` Postgres schema. PostgREST
// exposes them as <schema>/<table> via Accept-Profile / Content-Profile
// headers.
//
// Config:
//   - SUPABASE_URL: base URL of the PostgREST endpoint, e.g.
//     https://supabase.example.com
//   - SUPABASE_SERVICE_KEY: service-role JWT used for both Authorization
//     and the apikey header (PostgREST convention).
//
// All methods are read/write helpers shaped for the prospeo cache shape; the
// CLI uses them through the commands in internal/cli/.
package supa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Config is the resolved Supabase connection.
type Config struct {
	URL        string
	ServiceKey string
	Schema     string
	Timeout    time.Duration
}

// Client is a thin PostgREST wrapper. Zero-value is unusable; construct via
// New().
type Client struct {
	cfg  Config
	http *http.Client
}

// LoadConfig reads SUPABASE_URL + SUPABASE_SERVICE_KEY from the environment.
// Schema defaults to "outreach"; override via SUPABASE_SCHEMA.
func LoadConfig() (Config, error) {
	cfg := Config{
		URL:        strings.TrimRight(os.Getenv("SUPABASE_URL"), "/"),
		ServiceKey: os.Getenv("SUPABASE_SERVICE_KEY"),
		Schema:     os.Getenv("SUPABASE_SCHEMA"),
		Timeout:    20 * time.Second,
	}
	if cfg.Schema == "" {
		cfg.Schema = "outreach"
	}
	if cfg.URL == "" {
		return cfg, fmt.Errorf("SUPABASE_URL not set")
	}
	if cfg.ServiceKey == "" {
		return cfg, fmt.Errorf("SUPABASE_SERVICE_KEY not set")
	}
	return cfg, nil
}

// New builds a Client from cfg.
func New(cfg Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.Timeout},
	}
}

// IsConfigured returns true if both SUPABASE_URL and SUPABASE_SERVICE_KEY
// are set in the environment. Commands that need Supabase but tolerate its
// absence (with a warning) check this before constructing a Client.
func IsConfigured() bool {
	return os.Getenv("SUPABASE_URL") != "" && os.Getenv("SUPABASE_SERVICE_KEY") != ""
}

// doRequest is the inner HTTP wrapper. PostgREST returns 200/201/204 on
// success and a JSON {message, code, hint} body on error.
func (c *Client) doRequest(ctx context.Context, method, path string, params url.Values, body any, write bool) ([]byte, int, error) {
	u := c.cfg.URL + "/rest/v1" + path
	if params != nil {
		u = u + "?" + params.Encode()
	}
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, reqBody)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("apikey", c.cfg.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.ServiceKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if write {
		req.Header.Set("Content-Profile", c.cfg.Schema)
		req.Header.Set("Prefer", "return=representation,resolution=merge-duplicates")
	} else {
		req.Header.Set("Accept-Profile", c.cfg.Schema)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return respBody, resp.StatusCode, fmt.Errorf("supabase %s %s -> %d: %s", method, path, resp.StatusCode, truncate(string(respBody), 300))
	}
	return respBody, resp.StatusCode, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// Ping issues HEAD against the people table to verify connectivity. Used by
// doctor.
func (c *Client) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "HEAD", c.cfg.URL+"/rest/v1/people?limit=1", nil)
	if err != nil {
		return err
	}
	req.Header.Set("apikey", c.cfg.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.ServiceKey)
	req.Header.Set("Accept-Profile", c.cfg.Schema)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ping returned %d", resp.StatusCode)
	}
	return nil
}

// Schema returns the configured schema name (default "outreach").
func (c *Client) Schema() string { return c.cfg.Schema }

// Select runs a GET against the given table with query params.
func (c *Client) Select(ctx context.Context, table string, params url.Values) ([]byte, error) {
	body, _, err := c.doRequest(ctx, "GET", "/"+table, params, nil, false)
	return body, err
}

// Upsert posts rows to the given table using resolution=merge-duplicates.
func (c *Client) Upsert(ctx context.Context, table string, rows any) ([]byte, error) {
	body, _, err := c.doRequest(ctx, "POST", "/"+table, nil, rows, true)
	return body, err
}

// CountResponse parses a Content-Range count from a PostgREST response. Use
// Prefer: count=exact in conjunction.
type CountResponse struct {
	Total int
	Rows  json.RawMessage
}

// SelectCount runs a GET with Prefer: count=exact and returns rows + total.
func (c *Client) SelectCount(ctx context.Context, table string, params url.Values) (*CountResponse, error) {
	u := c.cfg.URL + "/rest/v1/" + table
	if params != nil {
		u = u + "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("apikey", c.cfg.ServiceKey)
	req.Header.Set("Authorization", "Bearer "+c.cfg.ServiceKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Profile", c.cfg.Schema)
	req.Header.Set("Prefer", "count=exact")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	rows, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("supabase select-count -> %d: %s", resp.StatusCode, truncate(string(rows), 300))
	}
	cr := &CountResponse{Rows: rows}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		// Format: "0-24/45254" or "*/0"
		if idx := strings.Index(cr, "/"); idx >= 0 && idx < len(cr)-1 {
			fmt.Sscanf(cr[idx+1:], "%d", &resp)
		}
	}
	// Re-parse Content-Range cleanly:
	cr.Total = parseContentRangeTotal(resp.Header.Get("Content-Range"))
	return cr, nil
}

func parseContentRangeTotal(cr string) int {
	if cr == "" {
		return 0
	}
	idx := strings.Index(cr, "/")
	if idx < 0 || idx == len(cr)-1 {
		return 0
	}
	var n int
	if _, err := fmt.Sscanf(cr[idx+1:], "%d", &n); err != nil {
		return 0
	}
	return n
}
