// Copyright 2026 riteshtiwari. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

func newLLMCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "llm",
		Short: "LLM observability and cost analysis commands",
		Long: `Compound commands for LLM cost attribution and observability.

For raw LLM trace access, use: posthog-pp-cli projects llm-observability`,
	}
	cmd.AddCommand(newLLMCostAttributionCmd(flags))
	return cmd
}

// newLLMCostAttributionCmd breaks down LLM spend by feature flag variant.
func newLLMCostAttributionCmd(flags *rootFlags) *cobra.Command {
	var projectID string
	var flagKey string
	var since string
	var limit int

	cmd := &cobra.Command{
		Use:         "cost-attribution",
		Short:       "Break down LLM spend by feature flag variant",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Group LLM trace cost and token usage by feature flag variant. Requires PostHog LLM Observability to be instrumented.`,
		Example: `  posthog-pp-cli llm cost-attribution --project 12345
  posthog-pp-cli llm cost-attribution --project 12345 --flag my-model-flag
  posthog-pp-cli llm cost-attribution --project 12345 --since 7d --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if projectID == "" {
				orgData, err2 := c.Get("/api/organizations/", nil)
				if err2 == nil {
					var orgs struct {
						Results []struct{ ID string `json:"id"` } `json:"results"`
					}
					if json.Unmarshal(orgData, &orgs) == nil && len(orgs.Results) > 0 {
						orgID := orgs.Results[0].ID
						projData, err3 := c.Get(fmt.Sprintf("/api/organizations/%s/projects/", orgID), nil)
						if err3 == nil {
							var projs struct {
								Results []struct{ ID int `json:"id"` } `json:"results"`
							}
							if json.Unmarshal(projData, &projs) == nil && len(projs.Results) > 0 {
								projectID = strconv.Itoa(projs.Results[0].ID)
							}
						}
					}
				}
			}
			if projectID == "" {
				return fmt.Errorf("--project is required")
			}

			params := map[string]string{
				"limit": strconv.Itoa(limit),
			}
			if since != "" {
				if t, err := parseSinceDuration(since); err == nil {
					params["after"] = t.UTC().Format(time.RFC3339)
				} else {
					params["after"] = since
				}
			}

			// Fetch LLM traces with pagination.
			type variant struct {
				Variant           string  `json:"variant"`
				TraceCount        int     `json:"trace_count"`
				TotalInputTokens  int     `json:"total_input_tokens"`
				TotalOutputTokens int     `json:"total_output_tokens"`
				TotalCostUSD      float64 `json:"total_cost_usd"`
				AvgCostPerTrace   float64 `json:"avg_cost_per_trace"`
			}

			variantMap := map[string]*variant{}
			unknownVariant := "unknown"

			var traces []map[string]any
			nextURL := fmt.Sprintf("/api/projects/%s/llm_analytics/trace_reviews/", projectID)
			pageParams := params
			for nextURL != "" && len(traces) < limit {
				data, fetchErr := c.Get(nextURL, pageParams)
				if fetchErr != nil {
					if len(traces) == 0 {
						return fmt.Errorf("fetching LLM traces: %w\n\nEnsure LLM Observability is enabled in your PostHog project.\nUse: posthog-pp-cli projects llm-observability trace-list <project_id>", fetchErr)
					}
					break
				}
				pageParams = nil
				var envelope struct {
					Next    string           `json:"next"`
					Results []map[string]any `json:"results"`
				}
				if json.Unmarshal(data, &envelope) == nil && envelope.Results != nil {
					traces = append(traces, envelope.Results...)
					nextURL = envelope.Next
				} else {
					var page []map[string]any
					if json.Unmarshal(data, &page) == nil {
						traces = append(traces, page...)
					}
					break
				}
			}

			for _, trace := range traces {
				variantKey := unknownVariant
				if flagKey != "" {
					// Look for the flag variant in trace properties or metadata.
					for _, field := range []string{"$feature/" + flagKey, "feature_flag_variant", "variant"} {
						if v, ok := trace[field]; ok {
							variantKey = fmt.Sprintf("%v", v)
							break
						}
					}
					if variantKey == unknownVariant {
						if props, ok := trace["properties"].(map[string]any); ok {
							for _, field := range []string{"$feature/" + flagKey, "feature_flag_variant"} {
								if v, ok := props[field]; ok {
									variantKey = fmt.Sprintf("%v", v)
									break
								}
							}
						}
					}
				}

				if _, ok := variantMap[variantKey]; !ok {
					variantMap[variantKey] = &variant{Variant: variantKey}
				}
				v := variantMap[variantKey]
				v.TraceCount++

				if inputTokens, ok := trace["input_tokens"]; ok {
					v.TotalInputTokens += int(toFloat(inputTokens))
				}
				if outputTokens, ok := trace["output_tokens"]; ok {
					v.TotalOutputTokens += int(toFloat(outputTokens))
				}
				if cost, ok := trace["total_cost"]; ok {
					v.TotalCostUSD += toFloat(cost)
				}
			}

			var variants []variant
			for _, v := range variantMap {
				if v.TraceCount > 0 {
					v.AvgCostPerTrace = v.TotalCostUSD / float64(v.TraceCount)
				}
				variants = append(variants, *v)
			}
			sort.Slice(variants, func(i, j int) bool {
				return variants[i].TotalCostUSD > variants[j].TotalCostUSD
			})

			type result struct {
				ProjectID     string    `json:"project_id"`
				FlagKey       string    `json:"flag_key,omitempty"`
				Since         string    `json:"since,omitempty"`
				TracesAnalyzed int      `json:"traces_analyzed"`
				Variants      []variant `json:"variants"`
				GeneratedAt   string    `json:"generated_at"`
			}
			out := result{
				ProjectID:      projectID,
				FlagKey:        flagKey,
				Since:          since,
				TracesAnalyzed: len(traces),
				Variants:       variants,
				GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
			}
			if out.Variants == nil {
				out.Variants = []variant{}
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, out)
			}

			w := cmd.OutOrStdout()
			if flagKey != "" {
				fmt.Fprintf(w, "LLM cost attribution by flag %q (project %s)\n\n", flagKey, projectID)
			} else {
				fmt.Fprintf(w, "LLM cost attribution (project %s)\n\n", projectID)
			}
			fmt.Fprintf(w, "Traces analyzed: %d\n\n", out.TracesAnalyzed)

			if len(variants) == 0 {
				fmt.Fprintln(w, "No LLM trace data found. Ensure LLM Observability is instrumented.")
				return nil
			}

			fmt.Fprintf(w, "  %-16s  %-8s  %-14s  %-14s  %-12s  %s\n",
				"VARIANT", "TRACES", "INPUT_TOKENS", "OUTPUT_TOKENS", "TOTAL_COST", "AVG_COST")
			for _, v := range variants {
				fmt.Fprintf(w, "  %-16s  %-8d  %-14d  %-14d  $%-11.4f  $%.4f\n",
					v.Variant, v.TraceCount, v.TotalInputTokens, v.TotalOutputTokens,
					v.TotalCostUSD, v.AvgCostPerTrace)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&projectID, "project", "", "PostHog project ID, auto-detected if omitted")
	cmd.Flags().StringVar(&flagKey, "flag", "", "Feature flag key to group variants by (optional, e.g. my-flag)")
	cmd.Flags().StringVar(&since, "since", "", "Only include traces after this date (e.g. 2025-01-01, YYYY-MM-DD format)")
	cmd.Flags().IntVar(&limit, "limit", 500, "Maximum number of LLM traces to include in the cost attribution analysis")
	return cmd
}

// toFloat safely converts any numeric-ish value to float64.
func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case string:
		f, _ := strconv.ParseFloat(x, 64)
		return f
	default:
		return 0
	}
}
