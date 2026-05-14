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

type variableDiffEntry struct {
	Key         string `json:"key"`
	Diff        string `json:"diff"` // "missing_on_target", "missing_on_source", "value_mismatch"
	SourceValue string `json:"source_value,omitempty"`
	TargetValue string `json:"target_value,omitempty"`
}

func newVariablesDiffCmd(flags *rootFlags) *cobra.Command {
	var targetURL string
	var targetKey string
	var hideValues bool

	cmd := &cobra.Command{
		Use:   "diff",
		Short: "Detect variable drift between two n8n instances",
		Long: `Compare environment variables between two n8n instances and report
keys that are missing on one side or have different values. Useful for
pre-deployment validation to ensure all required variables are set.`,
		Example: strings.Trim(`
  # Compare variables between current and staging
  n8n-pp-cli variables diff --target-url https://n8n-staging.example.com --target-key <key>

  # Suppress values in output (show keys only)
  n8n-pp-cli variables diff --target-url https://n8n-prod.example.com --target-key <key> --hide-values

  # JSON for CI gate
  n8n-pp-cli variables diff --target-url https://n8n-prod.example.com --target-key <key> --json --agent`, "\n"),
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

			srcData, err := srcClient.Get("/variables", map[string]string{"limit": "200"})
			if err != nil {
				return fmt.Errorf("fetching source variables: %w", err)
			}
			tgtData, err := tgtClient.Get("/variables", map[string]string{"limit": "200"})
			if err != nil {
				return fmt.Errorf("fetching target variables: %w", err)
			}

			type varItem struct {
				Key   string `json:"key"`
				Value string `json:"value"`
			}

			parseVars := func(data json.RawMessage) (map[string]string, error) {
				var items []varItem
				var env map[string]json.RawMessage
				if json.Unmarshal(data, &env) == nil {
					if arr, ok := env["data"]; ok {
						_ = json.Unmarshal(arr, &items)
					}
				}
				if items == nil {
					_ = json.Unmarshal(data, &items)
				}
				m := map[string]string{}
				for _, v := range items {
					m[v.Key] = v.Value
				}
				return m, nil
			}

			srcVars, err := parseVars(srcData)
			if err != nil {
				return fmt.Errorf("parsing source variables: %w", err)
			}
			tgtVars, err := parseVars(tgtData)
			if err != nil {
				return fmt.Errorf("parsing target variables: %w", err)
			}

			var diffs []variableDiffEntry

			for k, sv := range srcVars {
				tv, exists := tgtVars[k]
				if !exists {
					e := variableDiffEntry{Key: k, Diff: "missing_on_target"}
					if !hideValues {
						e.SourceValue = sv
					}
					diffs = append(diffs, e)
				} else if sv != tv {
					e := variableDiffEntry{Key: k, Diff: "value_mismatch"}
					if !hideValues {
						e.SourceValue = sv
						e.TargetValue = tv
					}
					diffs = append(diffs, e)
				}
			}
			for k, tv := range tgtVars {
				if _, exists := srcVars[k]; !exists {
					e := variableDiffEntry{Key: k, Diff: "missing_on_source"}
					if !hideValues {
						e.TargetValue = tv
					}
					diffs = append(diffs, e)
				}
			}

			sort.Slice(diffs, func(i, j int) bool {
				return diffs[i].Key < diffs[j].Key
			})

			if len(diffs) == 0 {
				if flags.asJSON {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
					return nil
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Variables are in sync — no differences found.")
				return nil
			}

			return printJSONFiltered(cmd.OutOrStdout(), diffs, flags)
		},
	}
	cmd.Flags().StringVar(&targetURL, "target-url", "", "Base URL of the target n8n instance")
	cmd.Flags().StringVar(&targetKey, "target-key", "", "API key for the target n8n instance")
	cmd.Flags().BoolVar(&hideValues, "hide-values", false, "Suppress variable values in output (show keys only)")
	return cmd
}
