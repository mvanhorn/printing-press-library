// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/config"
)

type instanceHealth struct {
	Label          string `json:"label"`
	URL            string `json:"url"`
	WorkflowTotal  int    `json:"workflow_total"`
	WorkflowActive int    `json:"workflow_active"`
	Reachable      bool   `json:"reachable"`
	Error          string `json:"error,omitempty"`
}

type healthCompareResult struct {
	Source instanceHealth `json:"source"`
	Target instanceHealth `json:"target"`
	Deltas struct {
		WorkflowTotalDelta  int `json:"workflow_total_delta"`
		WorkflowActiveDelta int `json:"workflow_active_delta"`
	} `json:"deltas"`
}

func newHealthCompareCmd(flags *rootFlags) *cobra.Command {
	var targetURL string
	var targetKey string

	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare health and workflow counts between two n8n instances",
		Long: `Fetch basic health metrics from two n8n instances and show them side-by-side.
Reports total/active workflow counts and any reachability errors. Useful for
verifying that a failover target is in a similar state to the primary, or for
monitoring multi-tenant deployments.`,
		Example: strings.Trim(`
  # Compare current instance with production
  n8n-pp-cli health compare --target-url https://n8n-prod.example.com --target-key <key>

  # JSON output for dashboards
  n8n-pp-cli health compare --target-url https://n8n-prod.example.com --target-key <key> --json --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if targetURL == "" {
				return usageErr(fmt.Errorf("--target-url is required"))
			}
			if targetKey == "" {
				return usageErr(fmt.Errorf("--target-key is required"))
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), `{"dry_run":true,"target_url":%q}`+"\n", targetURL)
				return nil
			}

			srcClient, err := flags.newClient()
			if err != nil {
				return err
			}

			tgtCfg := &config.Config{
				BaseURL:  targetURL,
				BasePath: "/api/v1",
				Headers:  map[string]string{"X-N8N-API-KEY": targetKey},
			}
			tgtClient := client.New(tgtCfg, flags.timeout, flags.rateLimit)

			probe := func(c *client.Client, label, url string) instanceHealth {
				h := instanceHealth{Label: label, URL: url}
				data, err := c.Get("/workflows", map[string]string{"limit": "1"})
				if err != nil {
					h.Error = err.Error()
					return h
				}
				h.Reachable = true
				var env map[string]json.RawMessage
				if json.Unmarshal(data, &env) == nil {
					// n8n returns {"data":[...],"nextCursor":null}
					if countRaw, ok := env["count"]; ok {
						_ = json.Unmarshal(countRaw, &h.WorkflowTotal)
					}
					if arr, ok := env["data"]; ok {
						var items []json.RawMessage
						_ = json.Unmarshal(arr, &items)
					}
				}
				// Get active count
				activeData, aErr := c.Get("/workflows", map[string]string{"active": "true", "limit": "1"})
				if aErr == nil {
					var activeEnv map[string]json.RawMessage
					if json.Unmarshal(activeData, &activeEnv) == nil {
						if countRaw, ok := activeEnv["count"]; ok {
							_ = json.Unmarshal(countRaw, &h.WorkflowActive)
						}
					}
				}
				return h
			}

			srcLabel := "source"
			tgtLabel := "target"

			src := probe(srcClient, srcLabel, "(current N8N_BASE_URL)")
			tgt := probe(tgtClient, tgtLabel, targetURL)

			result := healthCompareResult{
				Source: src,
				Target: tgt,
			}
			result.Deltas.WorkflowTotalDelta = tgt.WorkflowTotal - src.WorkflowTotal
			result.Deltas.WorkflowActiveDelta = tgt.WorkflowActive - src.WorkflowActive

			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&targetURL, "target-url", "", "Base URL of the target n8n instance")
	cmd.Flags().StringVar(&targetKey, "target-key", "", "API key for the target n8n instance")
	return cmd
}
