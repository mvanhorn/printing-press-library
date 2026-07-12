// Copyright 2026 Mayank Lavania and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/other/passive-indices/internal/indiapassivefunds"
)

// rawToStringLocal converts a json.RawMessage that may be a string or
// number into its string form, for decoding field-coded API rows.
func rawToStringLocal(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	return string(raw)
}

// trackerRow is one fund matched to an index via the underlyingIndex join,
// shared by `index funds`, `index tracking`, and `index cheapest-tracker`.
type trackerRow struct {
	SchemeID     string `json:"scheme_id"`
	SchemeName   string `json:"scheme_name"`
	CategoryName string `json:"category_name,omitempty"`
}

// resolveIndexTrackers resolves indexName to its underlyingIndex filter
// value and returns every fund the screener reports tracking it. This is
// the shared join step behind index funds/tracking/cheapest-tracker: the
// join key is indiapassivefunds' own enumerated "underlyingIndex" filter
// (e.g. "Nifty 50 TRI" -> 320), not name fuzzy-matching.
func resolveIndexTrackers(ctx context.Context, c *indiapassivefunds.Client, indexName string) ([]trackerRow, string, error) {
	filters, err := c.ScreenerFilters(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("fetching underlying-index taxonomy: %w", err)
	}
	val, matchedText, found := indiapassivefunds.FindUnderlyingIndexValue(filters, indexName)
	if !found {
		return nil, "", fmt.Errorf("no fund-tracking data for index %q — indiapassivefunds does not enumerate this index in its underlying-index taxonomy", indexName)
	}

	env, err := c.Screen(ctx, indiapassivefunds.ScreenParams{
		UnderlyingIndex: val,
		PageSize:        indiapassivefunds.ScreenerBulkPageSize,
	})
	if err != nil {
		return nil, matchedText, fmt.Errorf("screening funds for %q: %w", matchedText, err)
	}

	var nameField, catField, idField string
	for _, col := range env.Columns {
		switch {
		case nameField == "" && (col.DisplayName == "Fund Name" || col.DisplayName == "schemename"):
			nameField = col.Field
		case catField == "" && col.DisplayName == "CategoryName":
			catField = col.Field
		case idField == "" && (col.DisplayName == "scheme_id" || col.DisplayName == "Scheme_id"):
			idField = col.Field
		}
	}

	rows := make([]trackerRow, 0, len(env.Data))
	for _, raw := range env.Data {
		row := trackerRow{}
		if nameField != "" {
			if v, ok := raw[nameField]; ok {
				row.SchemeName = rawToStringLocal(v)
			}
		}
		if catField != "" {
			if v, ok := raw[catField]; ok {
				row.CategoryName = rawToStringLocal(v)
			}
		}
		if idField != "" {
			if v, ok := raw[idField]; ok {
				row.SchemeID = rawToStringLocal(v)
			}
		}
		if row.SchemeName != "" {
			rows = append(rows, row)
		}
	}
	return rows, matchedText, nil
}
