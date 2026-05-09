// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRedirectCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "redirect",
		Short: "Page-rule redirect convenience commands",
		Long:  "Compose page-rule forwarding URLs without hand-rolling the JSON body.",
	}
	cmd.AddCommand(newRedirectSetCmd(flags))
	return cmd
}

func newRedirectSetCmd(flags *rootFlags) *cobra.Command {
	var (
		zone   string
		status int
		priority int
	)

	cmd := &cobra.Command{
		Use:   "set <from-pattern> <to-template>",
		Short: "Create a 301/302 redirect Page Rule",
		Long: `Create a Page Rule with the forwarding_url action.

The from-pattern is a URL pattern (e.g. "legacy.example.com/*"). The to-template is a URL with
$1 / $2 backreferences for the wildcards (e.g. "https://example.com/$1").`,
		Example: `  cloudflare-pp-cli redirect set "legacy.example.com/*" "https://example.com/$1" --zone legacy.example.com --status 301`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 || zone == "" {
				return cmd.Help()
			}
			from := args[0]
			to := args[1]

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":   "would_create",
					"zone":     zone,
					"from":     from,
					"to":       to,
					"status":   status,
					"priority": priority,
					"dry_run":  true,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			zoneID, err := resolveZoneID(c, zone)
			if err != nil {
				return notFoundErr(err)
			}

			body := map[string]any{
				"targets": []map[string]any{
					{
						"target": "url",
						"constraint": map[string]any{
							"operator": "matches",
							"value":    from,
						},
					},
				},
				"actions": []map[string]any{
					{
						"id": "forwarding_url",
						"value": map[string]any{
							"url":         to,
							"status_code": status,
						},
					},
				},
				"priority": priority,
				"status":   "active",
			}

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":   "would_create",
					"zone":     zone,
					"from":     from,
					"to":       to,
					"status":   status,
					"priority": priority,
				}, flags)
			}

			resp, _, err := c.Post(fmt.Sprintf("/zones/%s/pagerules", zoneID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"action":   "created",
				"zone":     zone,
				"from":     from,
				"to":       to,
				"status":   status,
				"response": resp,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&zone, "zone", "", "Zone name or ID that owns the page rule")
	cmd.Flags().IntVar(&status, "status", 301, "HTTP redirect status code (301 or 302)")
	cmd.Flags().IntVar(&priority, "priority", 1, "Page Rule priority (1 = highest)")
	return cmd
}
