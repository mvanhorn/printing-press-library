// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newWorkerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Worker convenience commands (single-pane bindings view)",
	}
	cmd.AddCommand(newWorkerBindingsCmd(flags))
	return cmd
}

func newWorkerBindingsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bindings",
		Short: "Worker bindings introspection",
	}
	cmd.AddCommand(newWorkerBindingsShowCmd(flags))
	return cmd
}

func newWorkerBindingsShowCmd(flags *rootFlags) *cobra.Command {
	var account string

	cmd := &cobra.Command{
		Use:   "show <script>",
		Short: "Show every binding (KV, R2, D1, queues, secrets, routes, custom domains, crons) for one Worker",
		Long: `Fetch the Worker's metadata and resolve binding names → namespace IDs / bucket names /
DB names. Lists routes, secrets (names only), queue consumers, cron triggers, and custom
domains in one table — no more flipping between dashboard tabs.`,
		Example:     `  cloudflare-pp-cli worker bindings show my-worker --account <account_id> --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || account == "" {
				return cmd.Help()
			}
			script := args[0]

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"script":  script,
					"account": account,
					"dry_run": true,
					"action":  "would_show",
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Fetch script settings (includes bindings).
			settings, err := c.Get(fmt.Sprintf("/accounts/%s/workers/scripts/%s/settings", account, script), nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var settingsObj map[string]any
			_ = json.Unmarshal(settings, &settingsObj)

			// Bindings live in result.bindings (after envelope unwrap, may be top-level).
			var bindings []map[string]any
			if bs, ok := settingsObj["bindings"].([]any); ok {
				for _, b := range bs {
					if bm, ok := b.(map[string]any); ok {
						bindings = append(bindings, bm)
					}
				}
			}

			// Fetch deployments to find the active deployment metadata
			deployments, _ := c.Get(fmt.Sprintf("/accounts/%s/workers/scripts/%s/deployments", account, script), nil)
			var deploymentsArr []map[string]any
			_ = json.Unmarshal(deployments, &deploymentsArr)

			// Subdomain (workers.dev): account-level setting
			subResp, _ := c.Get(fmt.Sprintf("/accounts/%s/workers/subdomain", account), nil)

			// Custom domains for this script: filter from /accounts/{id}/workers/domains
			domainsResp, _ := c.Get(fmt.Sprintf("/accounts/%s/workers/domains", account), nil)
			var allDomains []map[string]any
			_ = json.Unmarshal(domainsResp, &allDomains)
			scriptDomains := []map[string]any{}
			for _, d := range allDomains {
				if s, _ := d["service"].(string); strings.EqualFold(s, script) {
					scriptDomains = append(scriptDomains, d)
				}
			}

			result := map[string]any{
				"script":          script,
				"account":         account,
				"bindings":        bindings,
				"binding_count":   len(bindings),
				"deployments":     deploymentsArr,
				"subdomain":       json.RawMessage(subResp),
				"custom_domains":  scriptDomains,
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&account, "account", "", "Cloudflare account ID (required)")
	return cmd
}
