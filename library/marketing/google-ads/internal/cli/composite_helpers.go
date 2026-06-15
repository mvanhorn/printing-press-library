// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ads/internal/client"
)

// flexInt decodes a JSON value that the Google Ads REST API may serialize as
// either a number or a string. int64 fields (cost_micros, clicks) come back as
// quoted strings per the protobuf-JSON mapping, while some come as bare
// numbers; flexInt accepts both so the composites never panic on either shape.
type flexInt int64

func (f *flexInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		*f = flexInt(i)
		return nil
	}
	fl, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("flexInt: cannot parse %q", s)
	}
	*f = flexInt(int64(fl))
	return nil
}

// fetchGAQLRows pages through googleAds:search for the given GAQL query and
// returns every results[] entry decoded into T. The caller supplies the row
// type matching the SELECT fields. Read-only; reuses the existing client POST
// and adds no new HTTP code.
func fetchGAQLRows[T any](c *client.Client, customerID, query string) ([]T, error) {
	path := replacePathParam(
		"/v22/customers/{customerId}/googleAds:search",
		"customerId", strings.ReplaceAll(customerID, "-", ""),
	)
	const maxPages = 50
	var all []T
	pageToken := ""
	for page := 0; page < maxPages; page++ {
		body := map[string]any{"query": query}
		if pageToken != "" {
			body["pageToken"] = pageToken
		}
		data, _, err := c.Post(path, body)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Results       []T    `json:"results"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("decoding googleAds:search response: %w", err)
		}
		all = append(all, resp.Results...)
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
	}
	return all, nil
}

// normalizeQueryKey canonicalizes a query/keyword for cross-source joins:
// lowercase, trimmed, internal whitespace collapsed to single spaces.
func normalizeQueryKey(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}
