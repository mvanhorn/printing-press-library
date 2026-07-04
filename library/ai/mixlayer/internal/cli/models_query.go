// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/pricing"
	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source auto
func newModelsQueryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var refresh bool
	cmd := &cobra.Command{
		Use:         "query <dsl>",
		Short:       "Query the model catalog cache with a small DSL",
		Example:     `  mixlayer-pp-cli models query "ctx>=128k tools reasoning" --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			s, err := openMixStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			if refresh {
				if err := refreshModelCache(cmd.Context(), flags, s); err != nil {
					return err
				}
			} else if err := seedModelCache(cmd.Context(), s); err != nil {
				return err
			}
			rows, err := s.QueryModels(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return outputJSON(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Refresh the cache from Mixlayer /models before querying")
	return cmd
}

func seedModelCache(ctx context.Context, s *store.Store) error {
	existing, err := s.AllModels(ctx)
	if err == nil && len(existing) > 0 {
		return nil
	}
	for _, model := range pricing.KnownModels {
		price := pricing.Mixlayer[model.ID]
		raw, _ := json.Marshal(map[string]any{
			"id": model.ID, "context_window": model.ContextWindow, "tools": model.SupportsTools, "reasoning": model.SupportsReasoning,
			"input_price_per_million": price.InputPerMillion, "output_price_per_million": price.OutputPerMillion,
		})
		if err := s.SaveModelCache(ctx, store.ModelCacheRecord{
			ID: model.ID, ContextWindow: model.ContextWindow, SupportsTools: model.SupportsTools, SupportsReasoning: model.SupportsReasoning,
			InputPricePerMillion: price.InputPerMillion, OutputPricePerMillion: price.OutputPerMillion, RawJSON: raw,
		}); err != nil {
			return err
		}
	}
	return nil
}

func refreshModelCache(ctx context.Context, flags *rootFlags, s *store.Store) error {
	c, err := flags.newClient()
	if err != nil {
		return err
	}
	raw, err := c.GetNoCache(ctx, "/models", nil)
	if err != nil {
		return err
	}
	records, err := parseModelCatalog(raw)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return fmt.Errorf("models refresh returned no model IDs")
	}
	for _, record := range records {
		if err := s.SaveModelCache(ctx, record); err != nil {
			return err
		}
	}
	return nil
}

func parseModelCatalog(raw json.RawMessage) ([]store.ModelCacheRecord, error) {
	var decoded any
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	items := catalogItems(decoded)
	records := make([]store.ModelCacheRecord, 0, len(items))
	for _, item := range items {
		id := stringField(item, "id", "name", "model")
		if id == "" {
			continue
		}
		encoded, _ := json.Marshal(item)
		info := knownModelInfo(id)
		price := pricing.Mixlayer[id]
		record := store.ModelCacheRecord{
			ID:                    id,
			ContextWindow:         firstPositive(intField(item, "context_window", "context_length", "context_size", "max_context_tokens"), info.ContextWindow),
			SupportsTools:         boolField(item, info.SupportsTools, "supports_tools", "tools", "tool_use", "function_calling"),
			SupportsReasoning:     boolField(item, info.SupportsReasoning, "supports_reasoning", "reasoning", "thinking"),
			InputPricePerMillion:  firstPositiveFloat(floatField(item, "input_price_per_million", "prompt_price_per_million"), price.InputPerMillion),
			OutputPricePerMillion: firstPositiveFloat(floatField(item, "output_price_per_million", "completion_price_per_million"), price.OutputPerMillion),
			RawJSON:               encoded,
		}
		records = append(records, record)
	}
	return records, nil
}

func catalogItems(v any) []map[string]any {
	switch typed := v.(type) {
	case []any:
		return mapItems(typed)
	case map[string]any:
		for _, key := range []string{"data", "models", "results"} {
			if nested, ok := typed[key]; ok {
				return catalogItems(nested)
			}
		}
		return []map[string]any{typed}
	default:
		return nil
	}
}

func mapItems(items []any) []map[string]any {
	out := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if mapped, ok := item.(map[string]any); ok {
			out = append(out, mapped)
		}
	}
	return out
}

func knownModelInfo(id string) pricing.ModelInfo {
	for _, info := range pricing.KnownModels {
		if info.ID == id {
			return info
		}
	}
	return pricing.ModelInfo{}
}

func stringField(m map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := m[key].(string); ok {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func intField(m map[string]any, keys ...string) int {
	for _, key := range keys {
		switch value := m[key].(type) {
		case json.Number:
			n, _ := value.Int64()
			return int(n)
		case float64:
			return int(value)
		case int:
			return value
		}
	}
	return 0
}

func floatField(m map[string]any, keys ...string) float64 {
	for _, key := range keys {
		switch value := m[key].(type) {
		case json.Number:
			n, _ := value.Float64()
			return n
		case float64:
			return value
		case int:
			return float64(value)
		}
	}
	return 0
}

func boolField(m map[string]any, fallback bool, keys ...string) bool {
	for _, key := range keys {
		switch value := m[key].(type) {
		case bool:
			return value
		case string:
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "true", "yes", "supported":
				return true
			case "false", "no", "unsupported":
				return false
			}
		case []any:
			if capabilitiesInclude(value, key) {
				return true
			}
		}
	}
	if caps, ok := m["capabilities"].([]any); ok {
		for _, key := range keys {
			if capabilitiesInclude(caps, key) {
				return true
			}
		}
	}
	return fallback
}

func capabilitiesInclude(items []any, key string) bool {
	needle := strings.ToLower(key)
	for _, item := range items {
		value, ok := item.(string)
		if !ok {
			continue
		}
		value = strings.ToLower(value)
		if strings.Contains(value, needle) || strings.Contains(value, strings.TrimPrefix(needle, "supports_")) {
			return true
		}
	}
	return false
}

func firstPositive(a, b int) int {
	if a > 0 {
		return a
	}
	return b
}

func firstPositiveFloat(a, b float64) float64 {
	if a > 0 {
		return a
	}
	return b
}
