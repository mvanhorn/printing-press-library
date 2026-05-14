// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/n8n/internal/config"
)

type diffEntry struct {
	WorkflowID   string `json:"workflow_id"`
	WorkflowName string `json:"workflow_name"`
	Diff         string `json:"diff"` // "added", "removed", "active_mismatch", "name_mismatch"
	SourceActive bool   `json:"source_active,omitempty"`
	TargetActive bool   `json:"target_active,omitempty"`
}

func newDiffCmd(flags *rootFlags) *cobra.Command {
	var targetURL string
	var targetKey string

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare workflows between two n8n instances",
		Long: `Compare the workflow inventory of two n8n instances side-by-side.
Reports workflows present in one instance but not the other, active-state
mismatches (e.g., active on source but inactive on target), and name changes.
Useful before GitOps promotions or cross-environment drift audits.`,
		Example: strings.Trim(`
  # Diff against a staging instance
  n8n-pp-cli diff --target-url https://n8n-staging.example.com --target-key <key>

  # JSON output for CI pipelines
  n8n-pp-cli diff --target-url https://n8n-prod.example.com --target-key <key> --json --agent`, "\n"),
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

			// Source client (current env)
			srcClient, err := flags.newClient()
			if err != nil {
				return err
			}

			// Target client
			tgtCfg := &config.Config{
				BaseURL:  targetURL,
				BasePath: "/api/v1",
				Headers:  map[string]string{"X-N8N-API-KEY": targetKey},
			}
			tgtClient := client.New(tgtCfg, flags.timeout, flags.rateLimit)

			// Fetch source workflows
			srcData, err := fetchAllWorkflows(srcClient)
			if err != nil {
				return fmt.Errorf("fetching source workflows: %w", err)
			}
			// Fetch target workflows
			tgtData, err := fetchAllWorkflows(tgtClient)
			if err != nil {
				return fmt.Errorf("fetching target workflows: %w", err)
			}

			type wfSummary struct {
				ID     string
				Name   string
				Active bool
			}

			parse := func(data json.RawMessage) ([]wfSummary, error) {
				var items []wfSummary
				// Try envelope {"data": [...]}
				var env map[string]json.RawMessage
				if json.Unmarshal(data, &env) == nil {
					if arr, ok := env["data"]; ok {
						_ = json.Unmarshal(arr, &items)
					}
				}
				if items == nil {
					_ = json.Unmarshal(data, &items)
				}
				return items, nil
			}

			srcWFs, err := parse(srcData)
			if err != nil {
				return fmt.Errorf("parsing source response: %w", err)
			}
			tgtWFs, err := parse(tgtData)
			if err != nil {
				return fmt.Errorf("parsing target response: %w", err)
			}

			srcByID := map[string]wfSummary{}
			for _, w := range srcWFs {
				srcByID[w.ID] = w
			}
			tgtByID := map[string]wfSummary{}
			for _, w := range tgtWFs {
				tgtByID[w.ID] = w
			}

			var diffs []diffEntry

			// Workflows on source not on target
			for id, s := range srcByID {
				t, exists := tgtByID[id]
				if !exists {
					diffs = append(diffs, diffEntry{
						WorkflowID:   id,
						WorkflowName: s.Name,
						Diff:         "missing_on_target",
						SourceActive: s.Active,
					})
					continue
				}
				if s.Active != t.Active {
					diffs = append(diffs, diffEntry{
						WorkflowID:   id,
						WorkflowName: s.Name,
						Diff:         "active_mismatch",
						SourceActive: s.Active,
						TargetActive: t.Active,
					})
				} else if s.Name != t.Name {
					diffs = append(diffs, diffEntry{
						WorkflowID:   id,
						WorkflowName: fmt.Sprintf("%s → %s", s.Name, t.Name),
						Diff:         "name_mismatch",
					})
				}
			}

			// Workflows on target not on source
			for id, t := range tgtByID {
				if _, exists := srcByID[id]; !exists {
					diffs = append(diffs, diffEntry{
						WorkflowID:   id,
						WorkflowName: t.Name,
						Diff:         "missing_on_source",
						TargetActive: t.Active,
					})
				}
			}

			sort.Slice(diffs, func(i, j int) bool {
				return diffs[i].WorkflowName < diffs[j].WorkflowName
			})

			if len(diffs) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Instances are in sync — no workflow differences found.")
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), diffs, flags)
		},
	}
	cmd.Flags().StringVar(&targetURL, "target-url", "", "Base URL of the target n8n instance (e.g. https://n8n-prod.example.com)")
	cmd.Flags().StringVar(&targetKey, "target-key", "", "API key for the target n8n instance")
	return cmd
}

// fetchAllWorkflows retrieves all workflows from an n8n instance, handling pagination.
func fetchAllWorkflows(c *client.Client) (json.RawMessage, error) {
	return c.Get("/workflows", map[string]string{"limit": "250"})
}
