// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/yeswehack/internal/client"
)

func readHunterAccessPrograms(cmd *cobra.Command, c *client.Client, flags *rootFlags) (json.RawMessage, DataProvenance, error) {
	path := "/v2/hunter/access/programs"

	switch flags.dataSource {
	case "local":
		data, prov, err := resolveLocal(cmd.Context(), "hunter-access-programs", true, path, map[string]string{}, "user_requested")
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return data, attachFreshness(prov, flags), nil

	case "live":
		data, err := c.GetWithHeaders(path, map[string]string{}, nil)
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return annotateHunterAccessPrograms(data), attachFreshness(DataProvenance{Source: "live"}, flags), nil

	default:
		data, err := c.GetWithHeaders(path, map[string]string{}, nil)
		if err == nil {
			annotated := annotateHunterAccessPrograms(data)
			writeThroughCache(cmd.Context(), "hunter-access-programs", annotated)
			return annotated, attachFreshness(DataProvenance{Source: "live"}, flags), nil
		}
		if !isNetworkError(err) {
			return nil, DataProvenance{}, err
		}
		fallbackData, fallbackProv, fallbackErr := resolveLocal(cmd.Context(), "hunter-access-programs", true, path, map[string]string{}, "api_unreachable")
		if fallbackErr != nil {
			return nil, DataProvenance{}, fmt.Errorf("API unreachable and no local data. Run 'yeswehack-pp-cli sync' to enable offline access.\n\nOriginal error: %w", err)
		}
		return fallbackData, attachFreshness(fallbackProv, flags), nil
	}
}

func annotateHunterAccessPrograms(data json.RawMessage) json.RawMessage {
	var rows []map[string]any
	if json.Unmarshal(data, &rows) == nil {
		addHunterAccessProgramIDs(rows)
		if encoded, err := json.Marshal(rows); err == nil {
			return encoded
		}
	}

	var envelope map[string]any
	if err := json.Unmarshal(data, &envelope); err != nil {
		return data
	}
	if rawItems, ok := envelope["items"].([]any); ok {
		rows = rows[:0]
		for _, item := range rawItems {
			if row, ok := item.(map[string]any); ok {
				rows = append(rows, row)
			}
		}
		addHunterAccessProgramIDs(rows)
		if encoded, err := json.Marshal(envelope); err == nil {
			return encoded
		}
	}
	return data
}

func addHunterAccessProgramIDs(rows []map[string]any) {
	for _, row := range rows {
		if row == nil {
			continue
		}
		slug := stringField(row, "slug")
		if slug == "" {
			continue
		}
		row["id"] = "hunter-access-programs|" + slug
		row["program_slug"] = slug
	}
}
