// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// CTISSearchURL is the EU CTIS public search endpoint (the EUCTR successor).
// It is a portal-fed JSON API with no documented stable schema, so this client
// parses defensively and degrades gracefully rather than assuming field shapes.
const CTISSearchURL = "https://euclinicaltrials.eu/ctis-public-api/search"

// CTISTrial is the normalized subset extracted from a CTIS record. Field
// presence varies by record, so every field is best-effort.
type CTISTrial struct {
	CTNumber  string   `json:"ct_number,omitempty"`
	Title     string   `json:"title,omitempty"`
	Phase     string   `json:"phase,omitempty"`
	Sponsor   string   `json:"sponsor,omitempty"`
	Countries []string `json:"countries,omitempty"`
	Source    string   `json:"source"`
}

// CTISResult is a CTIS search outcome.
type CTISResult struct {
	Query string      `json:"query"`
	Count int         `json:"count"`
	Items []CTISTrial `json:"items,omitempty"`
}

// ctisRequest is the search payload. The CTIS portal API accepts a paginated
// search with a free-text criterion; exact field names are undocumented, so we
// send the widely-observed shape and tolerate schema drift on the response.
type ctisRequest struct {
	Pagination     ctisPagination `json:"pagination"`
	SearchCriteria ctisCriteria   `json:"searchCriteria"`
}

type ctisPagination struct {
	Page int `json:"page"`
	Size int `json:"size"`
}

type ctisCriteria struct {
	ContainAll string `json:"containAll,omitempty"`
}

// SearchCTIS queries the EU CTIS public API for trials matching term. It returns
// a typed *cliutil.RateLimitError on throttling; other transport failures also
// surface so callers can mark the source degraded rather than empty.
func SearchCTIS(ctx context.Context, c *Client, term string, limit int) (*CTISResult, error) {
	term = strings.TrimSpace(term)
	res := &CTISResult{Query: term}
	if limit <= 0 {
		limit = 20
	}
	req := ctisRequest{
		Pagination:     ctisPagination{Page: 1, Size: limit},
		SearchCriteria: ctisCriteria{ContainAll: term},
	}
	// Parse into a loose map so undocumented response shape drift does not
	// break the whole command; pull the fields we recognize.
	var raw map[string]json.RawMessage
	if err := c.PostJSON(ctx, CTISSearchURL, nil, req, &raw); err != nil {
		return res, err
	}
	res.Count = ctisExtractCount(raw)
	res.Items = ctisExtractItems(raw, limit)
	return res, nil
}

// ProbeCTIS does a minimal reachability check used by `health`. It returns the
// total record count and any error, without parsing individual records.
func ProbeCTIS(ctx context.Context, c *Client) (int, error) {
	req := ctisRequest{Pagination: ctisPagination{Page: 1, Size: 1}}
	var raw map[string]json.RawMessage
	if err := c.PostJSON(ctx, CTISSearchURL, nil, req, &raw); err != nil {
		return 0, err
	}
	return ctisExtractCount(raw), nil
}

func ctisExtractCount(raw map[string]json.RawMessage) int {
	// Look for a totalRecords/totalSize/total field at top level or under pagination.
	for _, key := range []string{"totalSize", "totalRecords", "total", "totalElements"} {
		if v, ok := raw[key]; ok {
			var n int
			if json.Unmarshal(v, &n) == nil {
				return n
			}
		}
	}
	if pag, ok := raw["pagination"]; ok {
		var pm map[string]json.RawMessage
		if json.Unmarshal(pag, &pm) == nil {
			for _, key := range []string{"totalRecords", "totalSize", "total", "totalElements"} {
				if v, ok := pm[key]; ok {
					var n int
					if json.Unmarshal(v, &n) == nil {
						return n
					}
				}
			}
		}
	}
	return 0
}

func ctisExtractItems(raw map[string]json.RawMessage, limit int) []CTISTrial {
	// The record array is usually under "data" or "results".
	var arr []json.RawMessage
	for _, key := range []string{"data", "results", "content", "trials"} {
		if v, ok := raw[key]; ok {
			if json.Unmarshal(v, &arr) == nil && len(arr) > 0 {
				break
			}
		}
	}
	out := make([]CTISTrial, 0, len(arr))
	for i, rec := range arr {
		if i >= limit {
			break
		}
		var m map[string]json.RawMessage
		if json.Unmarshal(rec, &m) != nil {
			continue
		}
		out = append(out, CTISTrial{
			CTNumber:  ctisString(m, "ctNumber", "ctisId", "euCtNumber", "id"),
			Title:     cliutil.CleanText(ctisString(m, "ctTitle", "shortTitle", "title", "publicTitle")),
			Phase:     cliutil.CleanText(ctisString(m, "trialPhase", "phase")),
			Sponsor:   cliutil.CleanText(ctisString(m, "sponsor", "sponsorName", "primarySponsor")),
			Countries: ctisCountries(m, "trialCountries"),
			Source:    "eu-ctis",
		})
	}
	return out
}

// ctisCountries extracts the country list, stripping the ":count" suffix CTIS
// appends to each entry (e.g. "France:5" -> "France").
func ctisCountries(m map[string]json.RawMessage, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	var raw []string
	if json.Unmarshal(v, &raw) != nil {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, entry := range raw {
		name := entry
		if i := strings.IndexByte(entry, ':'); i >= 0 {
			name = entry[:i]
		}
		name = strings.TrimSpace(name)
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}

// ctisString returns the first present string-valued key (handles both string
// and nested {value:...} shapes defensively).
func ctisString(m map[string]json.RawMessage, keys ...string) string {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if json.Unmarshal(v, &s) == nil && s != "" {
			return s
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(v, &nested) == nil {
			if inner, ok := nested["value"]; ok {
				var is string
				if json.Unmarshal(inner, &is) == nil && is != "" {
					return is
				}
			}
		}
	}
	return ""
}
