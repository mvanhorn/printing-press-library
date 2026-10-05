// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.

package uberjobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

const (
	// SourceSite labels data read from the Uber careers site.
	SourceSite = "jobs.uber.com"
	// SourceOracle labels data read from the Oracle candidate-experience fallback.
	SourceOracle = "oracle-ce"
	// SourceLocal labels data read from the local store.
	SourceLocal = "local"

	searchPath   = "/api/jobs/search/"
	lookupPath   = "/api/jobs/recently-viewed/"
	facetsPath   = "/en/jobs/"
	maxPageSize  = 5000
	pageHeadroom = 50
)

// Query is a server-side search over /api/jobs/search/. Every key here was
// proven on 2026-10-05 (discovery/api-contract-notes.md): a nonsense value
// returns 0 and every returned row matches. Countries repeat as OR.
type Query struct {
	Search       string
	Countries    []string // site country names, not ISO codes
	Team         string
	SubTeam      string
	ContractType string
	Location     string
	Lat          *float64
	Lng          *float64
	Radius       int
}

// Values encodes the query with the site's JOB_QUERY_KEYS. The countries key
// repeats; the bracket form countries[] is silently ignored by the site.
func (q Query) Values() url.Values {
	v := url.Values{}
	if s := strings.TrimSpace(q.Search); s != "" {
		v.Set("search", s)
	}
	for _, c := range q.Countries {
		if c = strings.TrimSpace(c); c != "" {
			v.Add("countries", c)
		}
	}
	if s := strings.TrimSpace(q.Team); s != "" {
		v.Set("team", s)
	}
	if s := strings.TrimSpace(q.SubTeam); s != "" {
		v.Set("subTeam", s)
	}
	if s := strings.TrimSpace(q.ContractType); s != "" {
		v.Set("contractTypes", s)
	}
	if s := strings.TrimSpace(q.Location); s != "" {
		v.Set("location", s)
	}
	if q.Lat != nil && q.Lng != nil {
		v.Set("lat", strconv.FormatFloat(*q.Lat, 'f', -1, 64))
		v.Set("lng", strconv.FormatFloat(*q.Lng, 'f', -1, 64))
		if q.Radius > 0 {
			v.Set("radius", strconv.Itoa(q.Radius))
		}
	}
	return v
}

// IsUnfiltered reports whether the query covers the whole corpus.
func (q Query) IsUnfiltered() bool { return len(q.Values()) == 0 }

type searchEnvelope struct {
	Jobs       []json.RawMessage `json:"jobs"`
	TotalPages *int              `json:"totalPages"`
	TotalJobs  *int              `json:"totalJobs"`
	Page       *int              `json:"page"`
	PageSize   *int              `json:"pageSize"`
	Error      string            `json:"error"`
}

// parseSearchEnvelope is the positive content check for a search reply: a
// JSON object with a jobs array and a numeric totalJobs, and no error field.
func parseSearchEnvelope(rawURL string, body []byte) (*searchEnvelope, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, &ContentError{URL: rawURL, Reason: "response is not a JSON object"}
	}
	var env searchEnvelope
	if err := json.Unmarshal(trimmed, &env); err != nil {
		return nil, &ContentError{URL: rawURL, Reason: "invalid JSON: " + err.Error()}
	}
	if env.Error != "" {
		return nil, &ContentError{URL: rawURL, Reason: "site reported error: " + env.Error}
	}
	if env.Jobs == nil || env.TotalJobs == nil {
		return nil, &ContentError{URL: rawURL, Reason: "missing jobs array or totalJobs"}
	}
	return &env, nil
}

// SearchResult is one complete read of a query.
type SearchResult struct {
	Rows       []RawPosting
	Total      int
	Complete   bool
	ScanCapHit bool
	Requests   int
	CacheHit   bool
	Note       string
	Cache      map[string]string
}

// ProbeTotal asks for one row to learn the current total (the size probe).
func (c *Client) ProbeTotal(ctx context.Context, q Query) (int, *response, error) {
	v := q.Values()
	v.Set("page", "1")
	v.Set("pagesize", "1")
	u := joinURL(c.BaseURL, searchPath, v)
	resp, err := c.get(ctx, u, "application/json", UserAgent)
	if err != nil {
		return 0, nil, err
	}
	env, err := parseSearchEnvelope(u, resp.Body)
	if err != nil {
		return 0, nil, err
	}
	// The whole careers site never lists nothing; an empty corpus is a bad
	// read, and a complete empty read would let sync close every stored
	// posting. Rejected before caching, so a retry asks the site again.
	if *env.TotalJobs == 0 && q.IsUnfiltered() {
		return 0, nil, &ContentError{URL: u, Reason: "the unfiltered search reported 0 postings; refusing to treat an empty careers site as a complete read"}
	}
	c.remember("application/json", resp)
	return *env.TotalJobs, resp, nil
}

// SearchAll reads every row of a query with a size probe plus ONE page sized
// to the total. Multi-page walks reorder between page sizes (measured), and
// explicit non-default page sizes bypass the hours-stale edge cache that the
// default pagesize=10 URLs serve.
func (c *Client) SearchAll(ctx context.Context, q Query) (*SearchResult, error) {
	total, probe, err := c.ProbeTotal(ctx, q)
	if err != nil {
		return nil, err
	}
	res := &SearchResult{Requests: 1, CacheHit: probe.CacheHit, Cache: cacheHeaders(probe)}
	if !probe.CacheHit {
		res.Requests = 1
	} else {
		res.Requests = 0
	}
	if total == 0 {
		res.Complete = true
		res.Rows = []RawPosting{}
		return res, nil
	}
	size := total + pageHeadroom
	if size > maxPageSize {
		size = maxPageSize
	}
	v := q.Values()
	v.Set("page", "1")
	v.Set("pagesize", strconv.Itoa(size))
	u := joinURL(c.BaseURL, searchPath, v)
	resp, err := c.get(ctx, u, "application/json", UserAgent)
	if err != nil {
		return nil, err
	}
	if !resp.CacheHit {
		res.Requests++
	}
	res.CacheHit = res.CacheHit && resp.CacheHit
	res.Cache = cacheHeaders(resp)
	env, err := parseSearchEnvelope(u, resp.Body)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, len(env.Jobs))
	rows := make([]RawPosting, 0, len(env.Jobs))
	dups := 0
	for i, rawRow := range env.Jobs {
		var row RawPosting
		if err := json.Unmarshal(rawRow, &row); err != nil {
			return nil, &ContentError{URL: u, Reason: fmt.Sprintf("row %d does not decode: %v", i, err)}
		}
		id := strings.TrimSpace(string(row.ID))
		if id == "" {
			return nil, &ContentError{URL: u, Reason: fmt.Sprintf("row %d has no Id", i)}
		}
		if seen[id] {
			dups++
			continue
		}
		seen[id] = true
		rows = append(rows, row)
	}
	c.remember("application/json", resp)
	res.Rows = rows
	res.Total = *env.TotalJobs
	res.ScanCapHit = res.Total > size
	// The page must agree with the size probe: an empty or shrunken page
	// behind a larger probe total is a bad read, and treating it as
	// complete would let a full sync close every stored posting.
	res.Complete = !res.ScanCapHit && len(rows) == res.Total && res.Total == total
	switch {
	case res.ScanCapHit:
		res.Note = fmt.Sprintf("the site reports %d postings, more than the %d-row page cap; results are partial", res.Total, size)
	case res.Total != total:
		res.Note = fmt.Sprintf("the size probe reported %d postings but the full page reported %d; treat this read as incomplete", total, res.Total)
	case !res.Complete:
		res.Note = fmt.Sprintf("the site reported %d postings but returned %d unique rows (%d duplicates); treat this read as incomplete", res.Total, len(rows), dups)
	}
	return res, nil
}

func cacheHeaders(r *response) map[string]string {
	out := map[string]string{}
	if r == nil || r.Header == nil {
		return out
	}
	for _, k := range []string{"Cache-Control", "X-Vercel-Cache", "Age", "Cf-Cache-Status"} {
		if v := r.Header.Get(k); v != "" {
			out[strings.ToLower(k)] = v
		}
	}
	if r.CacheHit {
		out["local-cache"] = "hit"
	}
	return out
}

// LookupRow is one row of the site's batch lookup (recently-viewed) reply.
type LookupRow struct {
	ID        LooseString   `json:"id"`
	Title     LooseString   `json:"title"`
	URL       LooseString   `json:"url"`
	Locations []RawLocation `json:"locations"`
	Teams     []string      `json:"teams"`
}

// Lookup asks the site's batch lookup which ids are currently listed. Unknown
// ids are silently dropped by the site, so absence means not listed.
func (c *Client) Lookup(ctx context.Context, ids []string) ([]LookupRow, error) {
	u := c.BaseURL + lookupPath
	resp, err := c.postJSON(ctx, u, map[string]any{"jobIds": ids, "locale": "en"}, UserAgent)
	if err != nil {
		return nil, err
	}
	var env struct {
		Jobs []LookupRow `json:"jobs"`
	}
	trimmed := bytes.TrimSpace(resp.Body)
	if len(trimmed) == 0 || trimmed[0] != '{' || json.Unmarshal(trimmed, &env) != nil || env.Jobs == nil {
		return nil, &ContentError{URL: u, Reason: "lookup reply is not {jobs:[...]}"}
	}
	return env.Jobs, nil
}

// FetchFacets reads the job list page and parses the facet lists it embeds.
func (c *Client) FetchFacets(ctx context.Context) (*Facets, error) {
	u := c.BaseURL + facetsPath
	resp, err := c.get(ctx, u, "text/html", UserAgent)
	if err != nil {
		return nil, err
	}
	f, err := ParseFacets(resp.Body)
	if err != nil {
		return nil, &ContentError{URL: u, Reason: err.Error()}
	}
	c.remember("text/html", resp)
	return f, nil
}
