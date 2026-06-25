package gplay

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// DataSafety fetches the data-safety section for an app by parsing the
// datasafety HTML page (ds:3).
func (c *Client) DataSafety(ctx context.Context, appID string) (*DataSafety, error) {
	if appID == "" {
		return nil, fmt.Errorf("appId is required")
	}
	q := url.Values{}
	q.Set("id", appID)
	html, err := c.getHTML(ctx, "/store/apps/datasafety", q)
	if err != nil {
		return nil, err
	}
	ds, err := extractAFData(html)
	if err != nil {
		return nil, fmt.Errorf("parsing data-safety page for %s: %w", appID, err)
	}
	raw, ok := ds["ds:3"]
	if !ok {
		return nil, fmt.Errorf("data-safety block (ds:3) not found for %s", appID)
	}
	// The data-safety payload lives under an object key "138" at [1][2][1];
	// section [4][0] is shared, [4][1] is collected. Entry layout shifts, so
	// collect entries by shape within each section subtree.
	section := decode(raw).path(1, 2, 1).key("138").path(4)
	out := &DataSafety{
		DataShared:    section.at(0).walkEntries(40),
		DataCollected: section.at(1).walkEntries(40),
	}
	if pp := findPrivacyURL(string(raw)); pp != "" {
		out.PrivacyPolicyURL = pp
	}
	return out, nil
}

var privacyURLRe = regexp.MustCompile(`https?://[^"\\\s]+`)

func findPrivacyURL(raw string) string {
	for _, m := range privacyURLRe.FindAllString(raw, -1) {
		l := strings.ToLower(m)
		if strings.Contains(l, "privacy") || strings.Contains(l, "policy") {
			return m
		}
	}
	return ""
}
