// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

package source

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

// RxNormBase is the public RxNav REST endpoint (no key required).
const RxNormBase = "https://rxnav.nlm.nih.gov/REST"

// Drug is a RxNorm-normalized drug: the resolved ingredient name plus brand /
// generic synonyms used to broaden trial and adverse-event searches so
// "Keytruda" and "pembrolizumab" resolve to the same query set.
type Drug struct {
	Query    string   `json:"query"`
	RxCUI    string   `json:"rxcui,omitempty"`
	Name     string   `json:"normalized_name"`
	Synonyms []string `json:"synonyms,omitempty"`
	Resolved bool     `json:"resolved"`
}

// SearchTerms returns the distinct names to query downstream sources with:
// the normalized name plus synonyms, falling back to the raw query when RxNorm
// could not resolve the drug.
func (d *Drug) SearchTerms() []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, s)
	}
	add(d.Name)
	for _, s := range d.Synonyms {
		add(s)
	}
	if len(out) == 0 {
		add(d.Query)
	}
	return out
}

// Normalize resolves a free-text drug name to its RxNorm concept. It degrades
// gracefully: if RxNorm cannot resolve the name, the returned Drug carries the
// raw query as its name and Resolved=false rather than erroring, so compare /
// safety still run against the literal term.
func Normalize(ctx context.Context, c *Client, name string) (*Drug, error) {
	name = strings.TrimSpace(name)
	d := &Drug{Query: name, Name: name}
	if name == "" {
		return d, nil
	}

	// approximateTerm tolerates misspellings and brand/generic variation.
	var approx struct {
		ApproximateGroup struct {
			Candidate []struct {
				RxCUI string `json:"rxcui"`
				Name  string `json:"name"`
			} `json:"candidate"`
		} `json:"approximateGroup"`
	}
	u := fmt.Sprintf("%s/approximateTerm.json?term=%s&maxEntries=4", RxNormBase, url.QueryEscape(name))
	if err := c.GetJSON(ctx, u, nil, &approx); err != nil {
		// Network/throttle errors must surface (RateLimitError especially);
		// only an unresolved name is a soft miss.
		return d, err
	}
	var rxcui string
	for _, cand := range approx.ApproximateGroup.Candidate {
		if cand.RxCUI != "" {
			rxcui = cand.RxCUI
			if cand.Name != "" {
				d.Name = cand.Name
			}
			break
		}
	}
	if rxcui == "" {
		return d, nil
	}
	d.RxCUI = rxcui
	d.Resolved = true

	// Pull ingredient + brand-name relatives as synonyms.
	var related struct {
		RelatedGroup struct {
			ConceptGroup []struct {
				TTY               string `json:"tty"`
				ConceptProperties []struct {
					Name string `json:"name"`
				} `json:"conceptProperties"`
			} `json:"conceptGroup"`
		} `json:"relatedGroup"`
	}
	ru := fmt.Sprintf("%s/rxcui/%s/related.json?tty=IN+BN", RxNormBase, url.PathEscape(rxcui))
	if err := c.GetJSON(ctx, ru, nil, &related); err == nil {
		for _, g := range related.RelatedGroup.ConceptGroup {
			for _, cp := range g.ConceptProperties {
				if cp.Name != "" && !strings.EqualFold(cp.Name, d.Name) {
					d.Synonyms = append(d.Synonyms, cp.Name)
				}
				// Prefer the ingredient (IN) as the canonical normalized name.
				if g.TTY == "IN" && cp.Name != "" {
					d.Name = cp.Name
				}
			}
		}
	}
	return d, nil
}
