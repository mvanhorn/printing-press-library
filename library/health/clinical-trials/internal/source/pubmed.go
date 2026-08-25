// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// EutilsBase is the NCBI E-utilities endpoint (optional key for higher rate
// limits only).
const EutilsBase = "https://eutils.ncbi.nlm.nih.gov/entrez/eutils"

// PubMedResult lists publications referencing a trial.
type PubMedResult struct {
	Query string        `json:"query"`
	Count int           `json:"count"`
	Items []Publication `json:"items,omitempty"`
}

// Publication is one PubMed record.
type Publication struct {
	PMID  string `json:"pmid"`
	Title string `json:"title,omitempty"`
	Year  string `json:"year,omitempty"`
	URL   string `json:"url,omitempty"`
}

// SearchPubMed runs an esearch + esummary for term (typically an NCT id) and
// returns up to limit publications.
func SearchPubMed(ctx context.Context, c *Client, term string, limit int, apiKey string) (*PubMedResult, error) {
	term = strings.TrimSpace(term)
	res := &PubMedResult{Query: term}
	if term == "" {
		return res, nil
	}
	if limit <= 0 {
		limit = 10
	}
	keyParam := ""
	if apiKey != "" {
		keyParam = "&api_key=" + url.QueryEscape(apiKey)
	}

	var search struct {
		ESearchResult struct {
			Count  string   `json:"count"`
			IDList []string `json:"idlist"`
		} `json:"esearchresult"`
	}
	searchURL := fmt.Sprintf("%s/esearch.fcgi?db=pubmed&retmode=json&retmax=%d&term=%s%s",
		EutilsBase, limit, url.QueryEscape(term), keyParam)
	if err := c.GetJSON(ctx, searchURL, nil, &search); err != nil {
		return res, redactAPIKey(err, apiKey)
	}
	if n := strings.TrimSpace(search.ESearchResult.Count); n != "" {
		_, _ = fmt.Sscanf(n, "%d", &res.Count)
	}
	if len(search.ESearchResult.IDList) == 0 {
		return res, nil
	}

	// esummary returns a map keyed by uid plus a "uids" ordering array.
	var summary struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	sumURL := fmt.Sprintf("%s/esummary.fcgi?db=pubmed&retmode=json&id=%s%s",
		EutilsBase, url.QueryEscape(strings.Join(search.ESearchResult.IDList, ",")), keyParam)
	if err := c.GetJSON(ctx, sumURL, nil, &summary); err != nil {
		// Fall back to bare PMIDs if esummary fails — the IDs are still useful.
		for _, id := range search.ESearchResult.IDList {
			res.Items = append(res.Items, Publication{PMID: id, URL: "https://pubmed.ncbi.nlm.nih.gov/" + id + "/"})
		}
		return res, nil
	}
	for _, id := range search.ESearchResult.IDList {
		raw, ok := summary.Result[id]
		if !ok {
			continue
		}
		var rec struct {
			Title   string `json:"title"`
			PubDate string `json:"pubdate"`
		}
		_ = json.Unmarshal(raw, &rec)
		year := rec.PubDate
		if len(year) >= 4 {
			year = year[:4]
		}
		res.Items = append(res.Items, Publication{
			PMID:  id,
			Title: cliutil.CleanText(rec.Title),
			Year:  year,
			URL:   "https://pubmed.ncbi.nlm.nih.gov/" + id + "/",
		})
	}
	return res, nil
}
