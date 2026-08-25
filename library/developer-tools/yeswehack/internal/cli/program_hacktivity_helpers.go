// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/yeswehack/internal/client"
)

// PATCH(program-hacktivity): project hacktivity is exposed by the YesWeHack
// web app at /programs/{slug}/hacktivity, not through the global
// /v2/hacktivity feed with a programs query parameter.
func readProgramHacktivity(cmd *cobra.Command, c *client.Client, flags *rootFlags, slug, page string, resultsPerPage int, fetchAll bool) (json.RawMessage, DataProvenance, error) {
	path := "/programs/{slug}/hacktivity"
	path = replacePathParam(path, "slug", slug)
	params := map[string]string{
		"page":           page,
		"resultsPerPage": fmt.Sprintf("%v", resultsPerPage),
	}

	switch flags.dataSource {
	case "local":
		data, prov, err := resolveLocal(cmd.Context(), "hacktivity", true, path, map[string]string{}, "user_requested")
		if err != nil {
			return nil, DataProvenance{}, err
		}
		filtered := filterHacktivityProgramRows(data, slug)
		return filtered, attachFreshness(prov, flags), nil

	case "live":
		data, err := paginatedGet(c, path, params, nil, fetchAll, "", "", "")
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return annotateHacktivityProgramRows(data, slug), attachFreshness(DataProvenance{Source: "live"}, flags), nil

	default:
		data, err := paginatedGet(c, path, params, nil, fetchAll, "", "", "")
		if err == nil {
			annotated := annotateHacktivityProgramRows(data, slug)
			writeThroughCache(cmd.Context(), "hacktivity", annotated)
			return annotated, attachFreshness(DataProvenance{Source: "live"}, flags), nil
		}
		if !isNetworkError(err) {
			return nil, DataProvenance{}, err
		}
		fallbackData, fallbackProv, fallbackErr := resolveLocal(cmd.Context(), "hacktivity", true, path, map[string]string{}, "api_unreachable")
		if fallbackErr != nil {
			return nil, DataProvenance{}, fmt.Errorf("API unreachable and no local data. Run 'yeswehack-pp-cli sync' to enable offline access.\n\nOriginal error: %w", err)
		}
		return filterHacktivityProgramRows(fallbackData, slug), attachFreshness(fallbackProv, flags), nil
	}
}

func annotateHacktivityProgramRows(data json.RawMessage, slug string) json.RawMessage {
	if slug == "" {
		return data
	}
	if rows, ok := hacktivityRowsFromRaw(data); ok {
		for _, row := range rows {
			addHacktivityProgram(row, slug)
		}
		if encoded, err := json.Marshal(rows); err == nil {
			return encoded
		}
	}

	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		return data
	}
	if rawItems, ok := envelope["items"].([]any); ok {
		rows := make([]map[string]any, 0, len(rawItems))
		for _, item := range rawItems {
			if row, ok := item.(map[string]any); ok {
				addHacktivityProgram(row, slug)
				rows = append(rows, row)
			}
		}
		if encoded, err := json.Marshal(rows); err == nil {
			return encoded
		}
	}
	return data
}

func filterHacktivityProgramRows(data json.RawMessage, slug string) json.RawMessage {
	rows, ok := hacktivityRowsFromRaw(data)
	if !ok {
		return data
	}
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if hacktivityProgramSlug(row) == slug {
			filtered = append(filtered, row)
		}
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return data
	}
	return encoded
}

func hacktivityRowsFromRaw(data json.RawMessage) ([]map[string]any, bool) {
	var rows []map[string]any
	if err := json.Unmarshal(data, &rows); err == nil {
		return rows, true
	}
	return nil, false
}

func addHacktivityProgram(row map[string]any, slug string) {
	if row == nil || slug == "" {
		return
	}
	if _, ok := row["program"]; !ok {
		row["program"] = map[string]any{"slug": slug}
	}
	if _, ok := row["program_slug"]; !ok {
		row["program_slug"] = slug
	}
}
