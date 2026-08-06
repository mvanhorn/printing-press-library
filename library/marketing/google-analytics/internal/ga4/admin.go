// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package ga4

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// AdminListKeys enumerates the repeated list fields AdminList knows how to merge
// across pages, mirroring the live Python reference used to validate this surface.
var AdminListKeys = []string{"accountSummaries", "accounts", "properties", "keyEvents", "customDimensions", "customMetrics", "dataStreams", "audiences", "conversionEvents", "firebaseLinks", "googleAdsLinks", "accessBindings"}

// ValidAdminAPI reports whether api names an Admin API version this client can
// address. Callers validate up front so an unknown selector cannot silently
// fall back to v1beta.
func ValidAdminAPI(api string) bool { return api == "v1beta" || api == "v1alpha" }

// adminBase resolves the Admin API root for a version selector ("v1beta" or
// "v1alpha"). v1alpha is the Admin alpha host, NOT the Data API alpha host.
func (c *Client) adminBase(api string) string {
	if api == "v1alpha" {
		return c.AdminAlphaBase
	}
	return c.AdminBase
}

// adminURL joins an API-relative path onto the selected Admin root and appends
// updateMask as a query parameter, respecting an existing "?" in the path.
func (c *Client) adminURL(api, path, updateMask string) string {
	base := strings.TrimRight(c.adminBase(api), "/")
	target := base + "/" + strings.TrimLeft(path, "/")
	if updateMask != "" {
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + "updateMask=" + url.QueryEscape(updateMask)
	}
	return target
}

// adminDo performs an arbitrary Admin HTTP method, optionally with a JSON body,
// reusing the shared do() auth header / error handling.
func (c *Client) adminDo(ctx context.Context, method, target string, body any, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req, out)
}

// AdminCall performs a single Admin API call. method is an uppercase HTTP verb;
// path is relative to the Admin API root (leading slash tolerated).
func (c *Client) AdminCall(ctx context.Context, api, method, path string, body any, updateMask string) (map[string]any, int, error) {
	out := map[string]any{}
	st, err := c.adminDo(ctx, method, c.adminURL(api, path, updateMask), body, &out)
	return out, st, err
}

// AdminList fetches and merges a paginated Admin list endpoint. It follows
// nextPageToken with pageSize=200 and merges the single repeated list key it
// finds from adminListKeys. If no known list key is present, the raw (last)
// page is returned unchanged rather than guessing.
func (c *Client) AdminList(ctx context.Context, api, path string) (map[string]any, int, error) {
	listKey := ""
	merged := []any{}
	lastStatus := 0
	var lastRaw map[string]any
	pageToken := ""
	for {
		target := c.adminURL(api, path, "")
		sep := "?"
		if strings.Contains(target, "?") {
			sep = "&"
		}
		target += sep + "pageSize=200"
		if pageToken != "" {
			target += "&pageToken=" + url.QueryEscape(pageToken)
		}
		page := map[string]any{}
		st, err := c.get(ctx, target, &page)
		lastStatus = st
		lastRaw = page
		if err != nil {
			if lastRaw == nil {
				lastRaw = map[string]any{}
			}
			return lastRaw, st, err
		}
		if listKey == "" {
			for _, k := range AdminListKeys {
				if _, ok := page[k]; ok {
					listKey = k
					break
				}
			}
		}
		if listKey != "" {
			if arr, ok := page[listKey].([]any); ok {
				merged = append(merged, arr...)
			}
		}
		next, _ := page["nextPageToken"].(string)
		if next == "" {
			break
		}
		pageToken = next
	}
	if lastStatus == 0 {
		lastStatus = 200
	}
	if listKey != "" {
		return map[string]any{listKey: merged}, lastStatus, nil
	}
	if lastRaw == nil {
		lastRaw = map[string]any{}
	}
	return lastRaw, lastStatus, nil
}

func (c *Client) KeyEvents(ctx context.Context, property string) (map[string]any, int, error) {
	return c.AdminList(ctx, "v1beta", fmt.Sprintf("properties/%s/keyEvents", url.PathEscape(property)))
}
func (c *Client) CreateKeyEvent(ctx context.Context, property string, body map[string]any) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPost, fmt.Sprintf("properties/%s/keyEvents", url.PathEscape(property)), body, "")
}
func (c *Client) PatchKeyEvent(ctx context.Context, name string, body map[string]any, updateMask string) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPatch, name, body, updateMask)
}
func (c *Client) DeleteKeyEvent(ctx context.Context, name string) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodDelete, name, nil, "")
}

func (c *Client) CustomDimensions(ctx context.Context, property string) (map[string]any, int, error) {
	return c.AdminList(ctx, "v1beta", fmt.Sprintf("properties/%s/customDimensions", url.PathEscape(property)))
}
func (c *Client) CreateCustomDimension(ctx context.Context, property string, body map[string]any) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPost, fmt.Sprintf("properties/%s/customDimensions", url.PathEscape(property)), body, "")
}
func (c *Client) ArchiveCustomDimension(ctx context.Context, name string) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPost, name+":archive", nil, "")
}

func (c *Client) CustomMetrics(ctx context.Context, property string) (map[string]any, int, error) {
	return c.AdminList(ctx, "v1beta", fmt.Sprintf("properties/%s/customMetrics", url.PathEscape(property)))
}
func (c *Client) CreateCustomMetric(ctx context.Context, property string, body map[string]any) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPost, fmt.Sprintf("properties/%s/customMetrics", url.PathEscape(property)), body, "")
}
func (c *Client) ArchiveCustomMetric(ctx context.Context, name string) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPost, name+":archive", nil, "")
}

func (c *Client) CreateDataStream(ctx context.Context, property string, body map[string]any) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPost, fmt.Sprintf("properties/%s/dataStreams", url.PathEscape(property)), body, "")
}
func (c *Client) PatchDataStream(ctx context.Context, name string, body map[string]any, updateMask string) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodPatch, name, body, updateMask)
}
func (c *Client) DeleteDataStream(ctx context.Context, name string) (map[string]any, int, error) {
	return c.AdminCall(ctx, "v1beta", http.MethodDelete, name, nil, "")
}
