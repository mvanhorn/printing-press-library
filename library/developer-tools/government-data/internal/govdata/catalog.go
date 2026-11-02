// Package govdata normalizes public catalog metadata without conflating it with data files.
package govdata

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Resource struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Format string `json:"format"`
	Name   string `json:"name"`
}
type Dataset struct {
	Source      string          `json:"source"`
	ID          string          `json:"id"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Publisher   string          `json:"publisher"`
	License     string          `json:"license"`
	Modified    string          `json:"modified"`
	Resources   []Resource      `json:"resources"`
	Raw         json.RawMessage `json:"raw,omitempty"`
}
type Failure struct {
	Source string `json:"source"`
	Error  string `json:"error"`
}
type Snapshot struct {
	Scope      map[string]any `json:"scope"`
	ID         string         `json:"id"`
	Query      string         `json:"query"`
	Source     string         `json:"source"`
	CapturedAt string         `json:"captured_at"`
	Datasets   []Dataset      `json:"datasets"`
	Complete   bool           `json:"complete"`
	Failures   []Failure      `json:"failures"`
}
type Result struct {
	SnapshotID    string            `json:"snapshot_id,omitempty"`
	Note          string            `json:"note,omitempty"`
	Results       []Dataset         `json:"results"`
	Source        string            `json:"source"`
	Query         string            `json:"query"`
	ScannedPages  int               `json:"scanned_pages"`
	Complete      bool              `json:"complete"`
	Next          map[string]string `json:"next"`
	FetchFailures []Failure         `json:"fetch_failures"`
}
type Client struct {
	HTTP          *http.Client
	Key           string
	CaliforniaURL string
	FederalURL    string
	limiter       *cliutil.AdaptiveLimiter
}

func New(key string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "https" || req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("cross-origin redirect refused; use exact resource destination URL")
		}
		return nil
	}}, Key: key, CaliforniaURL: "https://publish.data.ca.gov/api/3/action", FederalURL: "https://api.gsa.gov/technology/datagov/v4", limiter: cliutil.NewAdaptiveLimiter(1)}
}
func (c *Client) Fetch(ctx context.Context, source, path string, q url.Values) (map[string]any, error) {
	base := c.CaliforniaURL
	if source == "federal" {
		base = c.FederalURL
		if c.Key == "" {
			return nil, fmt.Errorf("federal access requires DATA_GOV_API_KEY or --demo for limited exploration")
		}
	}
	u := strings.TrimRight(base, "/") + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	if source == "federal" {
		req.Header.Set("X-Api-Key", c.Key)
	}
	r, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer r.Body.Close()
	if r.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return nil, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(r)}
	}
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return nil, fmt.Errorf("%s catalog HTTP %d", source, r.StatusCode)
	}
	c.limiter.OnSuccess()
	b, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	var d map[string]any
	decoder := json.NewDecoder(bytes.NewReader(b))
	decoder.UseNumber()
	if err = decoder.Decode(&d); err != nil {
		return nil, fmt.Errorf("%s catalog JSON: %w", source, err)
	}
	if source == "california" {
		if ok, _ := d["success"].(bool); !ok {
			return nil, fmt.Errorf("California catalog action rejected request")
		}
	}
	return d, nil
}
func String(m map[string]any, k string) string { s, _ := m[k].(string); return s }
func Map(v any) map[string]any {
	m, _ := v.(map[string]any)
	if m == nil {
		return map[string]any{}
	}
	return m
}
func Array(v any) []any { a, _ := v.([]any); return a }
func Normalize(source string, m map[string]any) Dataset {
	raw, _ := json.Marshal(m)
	d := Dataset{Source: source, ID: String(m, "id"), Title: String(m, "title"), Description: String(m, "notes"), License: String(m, "license_title"), Modified: String(m, "metadata_modified"), Resources: []Resource{}, Raw: raw}
	if source == "california" {
		if d.ID == "" {
			d.ID = String(m, "name")
		}
		d.Publisher = String(Map(m["organization"]), "title")
		for _, v := range Array(m["resources"]) {
			r := Map(v)
			d.Resources = append(d.Resources, Resource{ID: String(r, "id"), URL: String(r, "url"), Format: String(r, "format"), Name: String(r, "name")})
		}
		return d
	}
	if d.ID == "" {
		d.ID = String(m, "slug")
	}
	if d.ID == "" {
		d.ID = String(m, "identifier")
	}
	dc := Map(m["dcat"])
	if len(dc) == 0 {
		dc = m
	}
	if d.Title == "" {
		d.Title = String(dc, "title")
	}
	d.Description = String(dc, "description")
	d.License = String(dc, "license")
	d.Modified = String(dc, "modified")
	d.Publisher = String(Map(dc["publisher"]), "name")
	if d.Publisher == "" {
		d.Publisher = String(m, "publisher")
	}
	for i, v := range Array(dc["distribution"]) {
		r := Map(v)
		u := String(r, "downloadURL")
		if u == "" {
			u = String(r, "accessURL")
		}
		format := String(r, "format")
		if format == "" {
			format = String(r, "mediaType")
		}
		d.Resources = append(d.Resources, Resource{ID: fmt.Sprint(i), URL: u, Format: format, Name: String(r, "title")})
	}
	return d
}
func (c *Client) Show(ctx context.Context, source, id string) (Dataset, error) {
	path := "/package_show"
	q := url.Values{"id": {id}}
	if source == "federal" {
		path = "/dataset/" + url.PathEscape(id)
		q = nil
	}
	m, err := c.Fetch(ctx, source, path, q)
	if err != nil {
		return Dataset{}, err
	}
	if source == "california" {
		m = Map(m["result"])
	}
	if v, ok := m["dataset"]; ok {
		m = Map(v)
	}
	return Normalize(source, m), nil
}
func Sources(source string) ([]string, error) {
	switch source {
	case "both":
		return []string{"california", "federal"}, nil
	case "california", "federal":
		return []string{source}, nil
	}
	return nil, fmt.Errorf("--source must be both, california, or federal")
}
func (c *Client) Find(ctx context.Context, query, source, format, org string, limit, pages int) (Result, error) {
	out := Result{Results: []Dataset{}, Source: source, Query: query, Complete: true, Next: map[string]string{}, FetchFailures: []Failure{}}
	sources, err := Sources(source)
	if err != nil {
		return out, err
	}
	if limit < 1 || limit > 1000 || pages < 1 || pages > 100 {
		return out, fmt.Errorf("--limit must be 1..1000 and --max-pages 1..100")
	}
	var sourceErrors []error
	for _, src := range sources {
		kept := 0
		cursor := ""
		offset := 0
		for page := 0; page < pages; page++ {
			q := url.Values{}
			path := "/package_search"
			if src == "california" {
				q.Set("q", query)
				q.Set("rows", fmt.Sprint(limit))
				q.Set("start", fmt.Sprint(offset))
				fq := []string{}
				if format != "" {
					fq = append(fq, `res_format:"`+solrQuote(format)+`"`)
				}
				if org != "" {
					fq = append(fq, `organization:"`+solrQuote(org)+`"`)
				}
				if len(fq) > 0 {
					q.Set("fq", strings.Join(fq, " AND "))
				}
			} else {
				path = "/search"
				q.Set("q", query)
				q.Set("per_page", fmt.Sprint(limit))
				if cursor != "" {
					q.Set("after", cursor)
				}
				if org != "" {
					q.Set("org_slug", org)
				}
			}
			m, e := c.Fetch(ctx, src, path, q)
			if e != nil {
				out.Complete = false
				out.FetchFailures = append(out.FetchFailures, Failure{src, e.Error()})
				sourceErrors = append(sourceErrors, fmt.Errorf("%s: %w", src, e))
				break
			}
			out.ScannedPages++
			if src == "california" {
				m = Map(m["result"])
			}
			rows := Array(m["results"])
			for _, v := range rows {
				d := Normalize(src, Map(v))
				if format != "" && src == "federal" {
					match := false
					for _, r := range d.Resources {
						if strings.EqualFold(r.Format, format) || strings.Contains(strings.ToLower(r.Format), strings.ToLower(format)) {
							match = true
						}
					}
					if !match {
						continue
					}
				}
				if kept < limit {
					out.Results = append(out.Results, d)
					kept++
				}
			}
			if src == "california" {
				offset += len(rows)
				var count int64
				if n, ok := m["count"].(json.Number); ok {
					count, _ = n.Int64()
				}
				if len(rows) == 0 || offset >= int(count) {
					delete(out.Next, src)
					break
				}
				out.Next[src] = fmt.Sprint(offset)
			} else {
				cursor = String(m, "after")
				if len(rows) == 0 || cursor == "" {
					delete(out.Next, src)
					break
				}
				out.Next[src] = cursor
			}
			if kept >= limit {
				break
			}
		}
	}
	out.Complete = len(out.Next) == 0 && len(out.FetchFailures) == 0
	if len(out.Results) == 0 {
		out.Note = "No matches in this bounded scan; increase --max-pages or refine the query. This does not prove the complete catalog has no matches."
	}
	if len(out.FetchFailures) == len(sources) {
		return out, fmt.Errorf("all catalog sources failed: %w", errors.Join(sourceErrors...))
	}
	return out, nil
}

// Download streams only a user-selected HTTPS resource and enforces an explicit byte cap.
func (c *Client) Download(ctx context.Context, u string, w io.Writer, max int64) (int64, error) {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return 0, fmt.Errorf("--url must be an absolute HTTPS resource URL")
	}
	if err = c.limiter.Wait(ctx); err != nil {
		return 0, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return 0, err
	}
	r, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer r.Body.Close()
	if r.StatusCode == 429 {
		c.limiter.OnRateLimit()
		return 0, &cliutil.RateLimitError{URL: u, RetryAfter: cliutil.RetryAfter(r)}
	}
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return 0, fmt.Errorf("resource HTTP %d", r.StatusCode)
	}
	c.limiter.OnSuccess()
	n, err := io.Copy(w, io.LimitReader(r.Body, max+1))
	if err == nil && n > max {
		return n, fmt.Errorf("resource exceeds --max-bytes; partial output removed")
	}
	return n, err
}

func solrQuote(s string) string { return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) }
