// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built — generation cost forensics + delta vs cheapest provider.

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/store"

	"github.com/spf13/cobra"
)

func newGenerationExplainCmd(flags *rootFlags) *cobra.Command {
	var llm bool

	cmd := &cobra.Command{
		Use:         "explain <id>",
		Short:       "Explain a generation: cost, latency, tokens, delta-vs-cheapest-provider",
		Example:     "  openrouter-pp-cli generation explain gen-abc123 --llm",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:method": "GET", "pp:path": "/generation"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "{\"id\":\"verify-stub\",\"model\":\"\",\"cost_usd\":0,\"latency_ms\":0}")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/generation", map[string]string{"id": id})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var genEnvelope struct {
				Data map[string]any `json:"data"`
			}
			gen := map[string]any{}
			if json.Unmarshal(data, &genEnvelope) == nil && genEnvelope.Data != nil {
				gen = genEnvelope.Data
			} else if err := json.Unmarshal(data, &gen); err != nil {
				return apiErr(fmt.Errorf("decode generation: %w", err))
			}
			model := asString(gen["model"])
			cost := asFloat(gen["total_cost"])
			if cost == 0 {
				cost = asFloat(gen["cost"])
			}
			latency := asFloat(gen["latency"])
			if latency == 0 {
				latency = asFloat(gen["generation_time"])
			}
			promptTokens := asFloat(gen["tokens_prompt"])
			completionTokens := asFloat(gen["tokens_completion"])
			provider := asString(gen["provider_name"])

			// Cheapest provider for same model from local endpoints table.
			deltaUSD := 0.0
			cheapestProvider := ""
			cheapestCost := -1.0
			if model != "" {
				dbPath := defaultDBPath("openrouter-pp-cli")
				if db, err := store.OpenWithContext(context.Background(), dbPath); err == nil {
					defer db.Close()
					rows, err := db.DB().QueryContext(cmd.Context(),
						`SELECT provider_name, data FROM endpoints WHERE model_id = ? AND status = 0`, model)
					if err == nil {
						defer rows.Close()
						for rows.Next() {
							var pn string
							var raw []byte
							if err := rows.Scan(&pn, &raw); err != nil {
								continue
							}
							var ep map[string]any
							if json.Unmarshal(raw, &ep) != nil {
								continue
							}
							pricing, _ := ep["pricing"].(map[string]any)
							if pricing == nil {
								continue
							}
							pp, _ := strconv.ParseFloat(asString(pricing["prompt"]), 64)
							pc, _ := strconv.ParseFloat(asString(pricing["completion"]), 64)
							c2 := pp*promptTokens + pc*completionTokens
							if cheapestCost < 0 || c2 < cheapestCost {
								cheapestCost = c2
								cheapestProvider = pn
							}
						}
					}
				}
				if cheapestCost >= 0 {
					deltaUSD = cost - cheapestCost
				}
			}

			out := map[string]any{
				"id":                id,
				"model":             model,
				"provider":          provider,
				"cost_usd":          cost,
				"latency_ms":        latency,
				"tokens_prompt":     promptTokens,
				"tokens_completion": completionTokens,
				"delta_vs_cheapest": deltaUSD,
				"cheapest_provider": cheapestProvider,
				"cheapest_cost_usd": cheapestCost,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if llm {
				fmt.Fprintf(cmd.OutOrStdout(),
					"model=%s cost=$%.6f latency=%.0fms tokens.prompt=%.0f tokens.completion=%.0f delta_vs_cheapest=$%.6f (provider=%s)\n",
					model, cost, latency, promptTokens, completionTokens, deltaUSD, cheapestProvider)
				return nil
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output")
	return cmd
}

func asString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", x)
	}
}

func asFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	case nil:
		return 0
	default:
		return 0
	}
}
