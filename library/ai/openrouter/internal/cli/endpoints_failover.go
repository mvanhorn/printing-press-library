// Copyright 2026 rick-van-de-laar. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH transcendence-commands: hand-built — rank a model's providers by status + cost + latency.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/openrouter/internal/cliutil"

	"github.com/spf13/cobra"
)

func newEndpointsFailoverCmd(flags *rootFlags) *cobra.Command {
	var llm bool

	cmd := &cobra.Command{
		Use:         "failover <author>/<slug>",
		Short:       "Rank a model's endpoints by status, pricing, and p50 latency",
		Example:     "  openrouter-pp-cli endpoints failover anthropic/claude-opus-4-7 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:method": "GET"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref := args[0]
			if !strings.Contains(ref, "/") {
				return usageErr(fmt.Errorf("expected author/slug, got %q", ref))
			}
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/models/"+ref+"/endpoints", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var envelope struct {
				Data map[string]any `json:"data"`
			}
			modelObj := map[string]any{}
			if json.Unmarshal(data, &envelope) == nil && envelope.Data != nil {
				modelObj = envelope.Data
			} else if err := json.Unmarshal(data, &modelObj); err != nil {
				return apiErr(err)
			}
			rawEps, _ := modelObj["endpoints"].([]any)
			type ep struct {
				Provider string  `json:"provider"`
				Status   int     `json:"status"`
				CostC    float64 `json:"cost_completion"`
				CostP    float64 `json:"cost_prompt"`
				P50      float64 `json:"latency_p50_ms"`
				Uptime30 float64 `json:"uptime_last_30m"`
				Tag      string  `json:"tag"`
			}
			parsed := make([]ep, 0, len(rawEps))
			for _, raw := range rawEps {
				e, _ := raw.(map[string]any)
				if e == nil {
					continue
				}
				var p ep
				p.Provider = asString(e["provider_name"])
				p.Status = int(asFloat(e["status"]))
				p.Uptime30 = asFloat(e["uptime_last_30m"])
				p.Tag = asString(e["tag"])
				if pricing, ok := e["pricing"].(map[string]any); ok {
					p.CostP, _ = strconv.ParseFloat(asString(pricing["prompt"]), 64)
					p.CostC, _ = strconv.ParseFloat(asString(pricing["completion"]), 64)
				}
				if lat, ok := e["latency_last_30m"].(map[string]any); ok {
					p.P50 = asFloat(lat["p50"])
				}
				parsed = append(parsed, p)
			}
			sort.SliceStable(parsed, func(i, j int) bool {
				if (parsed[i].Status == 0) != (parsed[j].Status == 0) {
					return parsed[i].Status == 0
				}
				if parsed[i].CostC != parsed[j].CostC {
					return parsed[i].CostC < parsed[j].CostC
				}
				return parsed[i].P50 < parsed[j].P50
			})

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), parsed, flags)
			}
			if llm {
				if len(parsed) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "no endpoints")
					return nil
				}
				for _, p := range parsed {
					fmt.Fprintf(cmd.OutOrStdout(),
						"provider=%s status=%d cost.completion=%g latency=%.0fms\n",
						p.Provider, p.Status, p.CostC, p.P50)
				}
				return nil
			}
			if len(parsed) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no endpoints")
				return nil
			}
			rows := make([][]string, 0, len(parsed))
			for _, p := range parsed {
				rows = append(rows, []string{p.Provider, fmt.Sprintf("%d", p.Status),
					fmt.Sprintf("%g", p.CostC), fmt.Sprintf("%.0f", p.P50)})
			}
			return flags.printTable(cmd, []string{"PROVIDER", "STATUS", "COST_C", "LAT_P50"}, rows)
		},
	}
	cmd.Flags().BoolVar(&llm, "llm", false, "Terse k:v output")
	return cmd
}
