// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/yeswehack/internal/client"
)

func readProgramScopes(cmd *cobra.Command, c *client.Client, flags *rootFlags, slug string) (json.RawMessage, DataProvenance, error) {
	path := "/programs/{slug}/scopes"
	path = replacePathParam(path, "slug", slug)

	switch flags.dataSource {
	case "local":
		data, prov, err := resolveLocal(cmd.Context(), "program-scopes", true, path, map[string]string{}, "user_requested")
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return filterProgramScopes(data, slug), attachFreshness(prov, flags), nil

	case "live":
		data, err := c.GetWithHeaders(path, map[string]string{}, nil)
		if err != nil {
			return nil, DataProvenance{}, err
		}
		return annotateProgramScopes(data, slug), attachFreshness(DataProvenance{Source: "live"}, flags), nil

	default:
		data, err := c.GetWithHeaders(path, map[string]string{}, nil)
		if err == nil {
			annotated := annotateProgramScopes(data, slug)
			writeThroughCache(cmd.Context(), "program-scopes", annotated)
			return annotated, attachFreshness(DataProvenance{Source: "live"}, flags), nil
		}
		if !isNetworkError(err) {
			return nil, DataProvenance{}, err
		}
		fallbackData, fallbackProv, fallbackErr := resolveLocal(cmd.Context(), "program-scopes", true, path, map[string]string{}, "api_unreachable")
		if fallbackErr != nil {
			return nil, DataProvenance{}, fmt.Errorf("API unreachable and no local data. Run 'yeswehack-pp-cli sync' to enable offline access.\n\nOriginal error: %w", err)
		}
		return filterProgramScopes(fallbackData, slug), attachFreshness(fallbackProv, flags), nil
	}
}

func annotateProgramScopes(data json.RawMessage, slug string) json.RawMessage {
	if slug == "" {
		return data
	}
	var rows []map[string]any
	if json.Unmarshal(data, &rows) == nil {
		addProgramScopeFields(rows, slug)
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
		addProgramScopeFields(rows, slug)
		if encoded, err := json.Marshal(envelope); err == nil {
			return encoded
		}
	}
	return data
}

func addProgramScopeFields(rows []map[string]any, slug string) {
	for _, row := range rows {
		if row == nil {
			continue
		}
		row["program_slug"] = slug
		if _, ok := row["program"]; !ok {
			row["program"] = map[string]any{"slug": slug}
		}
		if _, ok := row["id"]; !ok {
			row["id"] = stableProgramScopeID(slug, row)
		}
	}
}

func stableProgramScopeID(slug string, row map[string]any) string {
	encoded, err := json.Marshal(row)
	if err != nil {
		return fmt.Sprintf("%s|%p", slug, row)
	}
	sum := sha256.Sum256(encoded)
	return fmt.Sprintf("%s|%x", slug, sum[:12])
}

func filterProgramScopes(data json.RawMessage, slug string) json.RawMessage {
	var rows []map[string]any
	if json.Unmarshal(data, &rows) != nil {
		return data
	}
	filtered := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		if stringField(row, "program_slug", "program.slug") == slug {
			filtered = append(filtered, row)
		}
	}
	encoded, err := json.Marshal(filtered)
	if err != nil {
		return data
	}
	return encoded
}
