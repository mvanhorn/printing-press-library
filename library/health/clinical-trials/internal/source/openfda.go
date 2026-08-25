// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/cliutil"
)

// OpenFDABase is the public openFDA drug endpoint (optional key for higher
// rate limits only).
const OpenFDABase = "https://api.fda.gov/drug"

// FAERSResult is the adverse-event signal for a drug from the FDA Adverse
// Event Reporting System.
type FAERSResult struct {
	Drug         string           `json:"drug"`
	TotalReports int              `json:"total_reports"`
	TopReactions []rankedReaction `json:"top_reactions,omitempty"`
	Note         string           `json:"note,omitempty"`
}

type rankedReaction struct {
	Reaction string `json:"reaction"`
	Count    int    `json:"count"`
}

// AdverseEvents queries FAERS for the most-reported reactions for a drug. A 404
// from openFDA means "no matching reports", which is returned as an empty
// result (not an error) so safety can report an honest zero.
func AdverseEvents(ctx context.Context, c *Client, drug string, limit int, apiKey string) (*FAERSResult, error) {
	drug = strings.TrimSpace(drug)
	res := &FAERSResult{Drug: drug}
	if drug == "" {
		return res, nil
	}
	if limit <= 0 {
		limit = 10
	}
	search := fmt.Sprintf(`patient.drug.medicinalproduct:"%s"`, drug)
	keyParam := ""
	if apiKey != "" {
		keyParam = "&api_key=" + url.QueryEscape(apiKey)
	}

	// Total report count: meta.results.total from a 1-row search.
	var meta struct {
		Meta struct {
			Results struct {
				Total int `json:"total"`
			} `json:"results"`
		} `json:"meta"`
	}
	totalURL := fmt.Sprintf("%s/event.json?search=%s&limit=1%s", OpenFDABase, url.QueryEscape(search), keyParam)
	if err := c.GetJSON(ctx, totalURL, nil, &meta); err != nil {
		if isNotFound(err) {
			res.Note = "no adverse-event reports found in FAERS for this drug name"
			return res, nil
		}
		return res, redactAPIKey(err, apiKey)
	}
	res.TotalReports = meta.Meta.Results.Total

	// Top reactions via the count aggregation.
	var counts struct {
		Results []struct {
			Term  string `json:"term"`
			Count int    `json:"count"`
		} `json:"results"`
	}
	countURL := fmt.Sprintf("%s/event.json?search=%s&count=patient.reaction.reactionmeddrapt.exact&limit=%d%s",
		OpenFDABase, url.QueryEscape(search), limit, keyParam)
	if err := c.GetJSON(ctx, countURL, nil, &counts); err != nil {
		if isNotFound(err) {
			return res, nil
		}
		return res, redactAPIKey(err, apiKey)
	}
	for _, r := range counts.Results {
		res.TopReactions = append(res.TopReactions, rankedReaction{Reaction: strings.ToLower(r.Term), Count: r.Count})
	}
	return res, nil
}

// redactAPIKey returns err with any occurrence of the raw API key (and its
// URL-escaped form) replaced by a redaction marker, so a transport error that
// echoes the failing URL never leaks the key into logs or terminal output.
// A *cliutil.RateLimitError is rebuilt as the same type with redacted fields —
// flattening it to a plain string would break the errors.As detection that
// callers (e.g. isRateLimit in evidence.go) rely on to surface throttles as
// hard errors instead of silently-empty results.
func redactAPIKey(err error, apiKey string) error {
	if err == nil || apiKey == "" {
		return err
	}
	redact := func(s string) string {
		s = strings.ReplaceAll(s, apiKey, "REDACTED")
		return strings.ReplaceAll(s, url.QueryEscape(apiKey), "REDACTED")
	}
	var rl *cliutil.RateLimitError
	if errors.As(err, &rl) {
		return &cliutil.RateLimitError{URL: redact(rl.URL), RetryAfter: rl.RetryAfter, Body: redact(rl.Body)}
	}
	return errors.New(redact(err.Error()))
}

// isNotFound reports whether err is an openFDA 404 (no matching records), which
// is a valid empty result rather than a transport failure.
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "HTTP 404")
}