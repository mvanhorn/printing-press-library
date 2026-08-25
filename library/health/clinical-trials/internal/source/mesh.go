// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// MeSHBase is the NLM MeSH RDF term-lookup endpoint (no key required).
const MeSHBase = "https://id.nlm.nih.gov/mesh/lookup/term"

// MeSHTerm is one Medical Subject Headings descriptor match used to normalize a
// free-text disease/condition into canonical vocabulary.
type MeSHTerm struct {
	Label    string `json:"label"`
	Resource string `json:"resource,omitempty"`
}

// LookupMeSH returns canonical MeSH terms matching a free-text label. Used to
// suggest a normalized condition vocabulary; degrades to an empty list on miss.
func LookupMeSH(ctx context.Context, c *Client, label string, limit int) ([]MeSHTerm, error) {
	label = strings.TrimSpace(label)
	if label == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	u := fmt.Sprintf("%s?label=%s&match=contains&limit=%d", MeSHBase, url.QueryEscape(label), limit)
	var raw []struct {
		Resource string `json:"resource"`
		Label    string `json:"label"`
	}
	if err := c.GetJSON(ctx, u, nil, &raw); err != nil {
		return nil, err
	}
	out := make([]MeSHTerm, 0, len(raw))
	for _, r := range raw {
		out = append(out, MeSHTerm{Label: r.Label, Resource: r.Resource})
	}
	return out, nil
}
