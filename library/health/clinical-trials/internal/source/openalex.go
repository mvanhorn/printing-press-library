// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// OpenAlexBase is the public OpenAlex works endpoint (no key; a mailto puts the
// request in the faster "polite pool").
const OpenAlexBase = "https://api.openalex.org/works"

// OpenAlexResult is the publication/citation context for a trial.
type OpenAlexResult struct {
	Query string   `json:"query"`
	Count int      `json:"count"`
	Items []OAWork `json:"items,omitempty"`
}

// OAWork is one OpenAlex work.
type OAWork struct {
	ID      string `json:"id"`
	Title   string `json:"title,omitempty"`
	Year    int    `json:"year,omitempty"`
	CitedBy int    `json:"cited_by_count"`
	DOI     string `json:"doi,omitempty"`
}

// SearchOpenAlex finds works mentioning term (typically an NCT id). mailto is
// optional but recommended by OpenAlex for the polite pool.
func SearchOpenAlex(ctx context.Context, c *Client, term string, limit int, mailto string) (*OpenAlexResult, error) {
	term = strings.TrimSpace(term)
	res := &OpenAlexResult{Query: term}
	if term == "" {
		return res, nil
	}
	if limit <= 0 {
		limit = 10
	}
	u := fmt.Sprintf("%s?search=%s&per-page=%s", OpenAlexBase, url.QueryEscape(term), strconv.Itoa(limit))
	if mailto != "" {
		u += "&mailto=" + url.QueryEscape(mailto)
	}
	var resp struct {
		Meta struct {
			Count int `json:"count"`
		} `json:"meta"`
		Results []struct {
			ID              string `json:"id"`
			Title           string `json:"title"`
			PublicationYear int    `json:"publication_year"`
			CitedByCount    int    `json:"cited_by_count"`
			DOI             string `json:"doi"`
		} `json:"results"`
	}
	if err := c.GetJSON(ctx, u, nil, &resp); err != nil {
		return res, err
	}
	res.Count = resp.Meta.Count
	for _, w := range resp.Results {
		res.Items = append(res.Items, OAWork{
			ID:      w.ID,
			Title:   cliutil.CleanText(w.Title),
			Year:    w.PublicationYear,
			CitedBy: w.CitedByCount,
			DOI:     w.DOI,
		})
	}
	return res, nil
}
