// Copyright 2026 Ryan Kelley and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(private-import-review-hardening): exhaust offset-paginated API/report reads.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-search-ads/internal/client"
)

const maxAppleOffsetPages = 1000

type offsetPageFetcher func(offset, limit int) (json.RawMessage, error)
type offsetPageExtractor func(json.RawMessage) ([]json.RawMessage, error)

func collectOffsetPages(pageSize int, fetch offsetPageFetcher, extract offsetPageExtractor) ([]json.RawMessage, error) {
	if pageSize <= 0 {
		return nil, fmt.Errorf("page size must be positive")
	}
	var all []json.RawMessage
	var previous [32]byte
	for page := 0; page < maxAppleOffsetPages; page++ {
		offset := page * pageSize
		data, err := fetch(offset, pageSize)
		if err != nil {
			return nil, err
		}
		items, err := extract(data)
		if err != nil {
			return nil, err
		}
		signatureBytes, _ := json.Marshal(items)
		signature := sha256.Sum256(signatureBytes)
		if page > 0 && len(items) > 0 && signature == previous {
			return nil, fmt.Errorf("pagination did not advance at offset %d", offset)
		}
		previous = signature
		all = append(all, items...)
		if len(items) < pageSize {
			return all, nil
		}
	}
	return nil, fmt.Errorf("pagination exceeded %d-page safety limit", maxAppleOffsetPages)
}

func fetchAllReportingPayload(ctx context.Context, c *client.Client, path string, baseBody map[string]any, pageSize int) (json.RawMessage, error) {
	rows, err := collectOffsetPages(pageSize, func(offset, limit int) (json.RawMessage, error) {
		body, cloneErr := cloneReportBody(baseBody, offset, limit)
		if cloneErr != nil {
			return nil, cloneErr
		}
		data, _, postErr := c.Post(ctx, path, body)
		return data, postErr
	}, func(data json.RawMessage) ([]json.RawMessage, error) {
		return extractOffsetPage(data, c.DryRun, extractReportingRowsRaw)
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"data": map[string]any{
			"reportingDataResponse": map[string]any{"row": rows},
		},
	})
}

func fetchAllOffsetItems(ctx context.Context, c *client.Client, path string, pageSize int) (json.RawMessage, error) {
	items, err := collectOffsetPages(pageSize, func(offset, limit int) (json.RawMessage, error) {
		return c.Get(ctx, path, map[string]string{
			"offset": fmt.Sprintf("%d", offset),
			"limit":  fmt.Sprintf("%d", limit),
		})
	}, func(data json.RawMessage) ([]json.RawMessage, error) {
		return extractOffsetPage(data, c.DryRun, extractGenericItemsRaw)
	})
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"data": items})
}

func extractOffsetPage(data json.RawMessage, dryRun bool, extract offsetPageExtractor) ([]json.RawMessage, error) {
	if dryRun && isDryRunResponse(data) {
		return nil, nil
	}
	return extract(data)
}

func cloneReportBody(base map[string]any, offset, limit int) (map[string]any, error) {
	raw, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("copying reporting request: %w", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("copying reporting request: %w", err)
	}
	selector, ok := body["selector"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("reporting request is missing selector")
	}
	selector["pagination"] = map[string]any{"offset": offset, "limit": limit}
	return body, nil
}

func extractReportingRowsRaw(data json.RawMessage) ([]json.RawMessage, error) {
	payload := data
	for _, key := range []string{"data", "reportingDataResponse"} {
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(payload, &nested); err != nil {
			return nil, fmt.Errorf("parsing reporting envelope: %w", err)
		}
		value, ok := nested[key]
		if !ok {
			return nil, fmt.Errorf("reporting response is missing %q", key)
		}
		payload = value
	}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(payload, &wrapper); err != nil {
		return nil, fmt.Errorf("parsing reporting response: %w", err)
	}
	rowsRaw, ok := wrapper["row"]
	if !ok {
		return nil, fmt.Errorf("reporting response is missing row array")
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(rowsRaw, &rows); err != nil {
		return nil, fmt.Errorf("parsing reporting rows: %w", err)
	}
	return rows, nil
}

func extractGenericItemsRaw(data json.RawMessage) ([]json.RawMessage, error) {
	var direct []json.RawMessage
	if err := json.Unmarshal(data, &direct); err == nil {
		return direct, nil
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, fmt.Errorf("parsing paginated response: %w", err)
	}
	for _, key := range []string{"data", "items", "campaigns", "adGroups", "keywords", "searchTerms"} {
		if raw, ok := top[key]; ok {
			var items []json.RawMessage
			if err := json.Unmarshal(raw, &items); err == nil {
				return items, nil
			}
		}
	}
	return nil, fmt.Errorf("paginated response does not contain an item array")
}
