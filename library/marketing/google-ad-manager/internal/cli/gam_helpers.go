// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for Google Ad Manager novel commands.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/client"
	"github.com/mvanhorn/printing-press-library/library/marketing/google-ad-manager/internal/store"
)

// resolveNetworkCode returns the GAM network code from (in order) the command's
// --network flag, the GOOGLE_AD_MANAGER_NETWORK_CODE env var. Every Ad Manager
// resource path is scoped to a network (networks/{code}/...), so novel commands
// that hit the live API call this before building a request path.
func resolveNetworkCode(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if v := os.Getenv("GOOGLE_AD_MANAGER_NETWORK_CODE"); v != "" {
		return v, nil
	}
	return "", usageErr(fmt.Errorf("network code required: pass --network <code> or set GOOGLE_AD_MANAGER_NETWORK_CODE"))
}

// networkParent builds the resource parent prefix for a network code, e.g.
// "networks/123456". Strip any leading "networks/" the caller may have already
// supplied so both "123456" and "networks/123456" resolve the same way.
func networkParent(code string) string {
	if len(code) > len("networks/") && code[:len("networks/")] == "networks/" {
		return code
	}
	return "networks/" + code
}

// pollReportOperation polls a long-running report operation until it reports
// done, then returns its raw JSON. opName is the operation resource name
// returned by reports.run (e.g. "networks/123/operations/reports/runs/456").
// It GETs /v1/{opName} every interval until "done": true or ctx/deadline
// expires. The returned bytes are the full Operation object; callers read
// .response (or .metadata) for the report result name used by fetchRows.
func pollReportOperation(ctx context.Context, c *client.Client, opName string, interval, timeout time.Duration) (json.RawMessage, error) {
	if interval <= 0 {
		interval = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	path := "/v1/" + opName
	for {
		data, err := c.GetNoCache(ctx, path, nil)
		if err != nil {
			return nil, fmt.Errorf("polling report operation %q: %w", opName, err)
		}
		var op struct {
			Done  bool            `json:"done"`
			Error json.RawMessage `json:"error"`
		}
		if jerr := json.Unmarshal(data, &op); jerr == nil && op.Done {
			if len(op.Error) > 0 && string(op.Error) != "null" {
				return data, apiErr(fmt.Errorf("report operation failed: %s", string(op.Error)))
			}
			return data, nil
		}
		if timeout > 0 && time.Now().After(deadline) {
			return data, apiErr(fmt.Errorf("report operation %q did not complete within %s; re-run without --wait to poll later", opName, timeout))
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// gamIDFromName extracts the trailing id segment from a GAM resource name like
// "networks/123/adUnits/456" -> "456". Falls back to the whole string when there
// is no slash.
func gamIDFromName(name string) string {
	if name == "" {
		return ""
	}
	if i := strings.LastIndex(name, "/"); i >= 0 && i+1 < len(name) {
		return name[i+1:]
	}
	return name
}

// gamFetchList fetches every item of a network-scoped list resource live from
// the API, following nextPageToken. camelResource is both the path segment and
// the response wrapper key (e.g. "adUnits", "lineItems", "placements",
// "customTargetingKeys", "customTargetingValues", "orders"). maxPages bounds the
// scan; pass a small value under dogfood. Returns the raw item objects.
func gamFetchList(ctx context.Context, c *client.Client, networkCode, camelResource string, maxPages int) ([]json.RawMessage, error) {
	if maxPages <= 0 {
		maxPages = 25
	}
	path := "/v1/" + networkParent(networkCode) + "/" + camelResource
	out := make([]json.RawMessage, 0)
	pageToken := ""
	for page := 0; page < maxPages; page++ {
		params := map[string]string{"pageSize": "200"}
		if pageToken != "" {
			params["pageToken"] = pageToken
		}
		data, err := c.Get(ctx, path, params)
		if err != nil {
			return out, err
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(data, &env); err != nil {
			return out, fmt.Errorf("parsing %s response: %w", camelResource, err)
		}
		if arr, ok := env[camelResource]; ok {
			var items []json.RawMessage
			if json.Unmarshal(arr, &items) == nil {
				out = append(out, items...)
			}
		}
		nextToken := ""
		if raw, ok := env["nextPageToken"]; ok {
			_ = json.Unmarshal(raw, &nextToken)
		}
		if nextToken == "" {
			break
		}
		pageToken = nextToken
	}
	return out, nil
}

// gamCacheItems upserts fetched items into the local mirror under
// resourceTypeKebab, keyed by each item's trailing id segment. Best-effort: a
// cache write failure is non-fatal (the command still has the live data).
func gamCacheItems(st *store.Store, resourceTypeKebab string, items []json.RawMessage) {
	if st == nil {
		return
	}
	for _, it := range items {
		var obj struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		}
		_ = json.Unmarshal(it, &obj)
		id := gamIDFromName(obj.Name)
		if id == "" {
			id = obj.ID
		}
		if id == "" {
			continue
		}
		_ = st.Upsert(resourceTypeKebab, id, it)
	}
}

// gamItemsForNetwork keeps only the cached items whose GAM resource name is
// scoped to networkCode (name begins "networks/{code}/"). The local store keys
// resources by type alone, not by network, so a mirror populated for one network
// must be filtered before it can answer a query for another — otherwise a second
// --network value silently reads back the first network's cached rows. When
// networkCode is empty the items are returned unchanged (mirror-only,
// network-agnostic read).
func gamItemsForNetwork(items []json.RawMessage, networkCode string) []json.RawMessage {
	if strings.TrimSpace(networkCode) == "" {
		return items
	}
	prefix := networkParent(networkCode) + "/"
	out := make([]json.RawMessage, 0, len(items))
	for _, it := range items {
		var obj struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(it, &obj) == nil && strings.HasPrefix(obj.Name, prefix) {
			out = append(out, it)
		}
	}
	return out
}

// gamLoadResource returns the items for a GAM resource, preferring the local
// mirror and falling back to a live fetch (cached into the mirror) when the
// mirror holds nothing for this network. This makes the offline novel commands
// work on a cold cache: the first run hits the API and populates the mirror;
// later runs read locally. Returns (items, fromLive, error).
//
//   - st may be nil (no mirror): always live-fetches when networkCode is set.
//   - Mirror reads are filtered to networkCode by resource-name prefix, so a
//     store populated for another network is not served across --network values.
//   - networkCode == "": mirror-only; returns whatever the mirror holds (maybe none).
//   - resourceTypeKebab is the store key (e.g. "ad-units"); camelResource is the
//     API path segment / wrapper key (e.g. "adUnits").
func gamLoadResource(ctx context.Context, flags *rootFlags, st *store.Store, networkCode, resourceTypeKebab, camelResource string, maxPages int) ([]json.RawMessage, bool, error) {
	if st != nil {
		if items, err := st.List(resourceTypeKebab, 0); err == nil && len(items) > 0 {
			// The store keys resources by type only, so a mirror populated for a
			// different network must be filtered by resource-name prefix. If
			// nothing remains for this network, fall through to a live fetch.
			if scoped := gamItemsForNetwork(items, networkCode); len(scoped) > 0 {
				return scoped, false, nil
			}
		}
	}
	if networkCode == "" {
		return make([]json.RawMessage, 0), false, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, false, err
	}
	items, err := gamFetchList(ctx, c, networkCode, camelResource, maxPages)
	if err != nil {
		return items, true, err
	}
	gamCacheItems(st, resourceTypeKebab, items)
	return items, true, nil
}

// gamCamelResource maps a kebab store resource_type to its camelCase GAM API
// path segment / response wrapper key. Falls back to a kebab->lowerCamel
// transform for resources not in the explicit table.
func gamCamelResource(kebab string) string {
	switch kebab {
	case "ad-units":
		return "adUnits"
	case "ad-spots":
		return "adSpots"
	case "line-items":
		return "lineItems"
	case "custom-targeting-keys":
		return "customTargetingKeys"
	case "custom-targeting-values":
		return "customTargetingValues"
	case "private-auctions":
		return "privateAuctions"
	case "private-auction-deals":
		return "privateAuctionDeals"
	case "programmatic-buyers":
		return "programmaticBuyers"
	}
	parts := strings.Split(kebab, "-")
	if len(parts) == 1 {
		return kebab
	}
	out := parts[0]
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		out += strings.ToUpper(p[:1]) + p[1:]
	}
	return out
}
