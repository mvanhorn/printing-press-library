// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newSetupZoneCmd(flags *rootFlags) *cobra.Command {
	var (
		origin       string
		redirectFrom string
		account      string
		proxied      bool
		alwaysHTTPS  bool
		sslMode      string
	)

	cmd := &cobra.Command{
		Use:   "setup_zone <zone>",
		Short: "End-to-end zone setup: A record, optional redirect, SSL strict, Always-Use-HTTPS",
		Long: `Compose the most common new-deploy workflow into one command:
  1. Resolve or create the zone (zone must already exist on the account)
  2. Apply the A record idempotently (--origin)
  3. Optionally create a 301 redirect Page Rule (--redirect-from)
  4. Set SSL mode to strict
  5. Enable Always-Use-HTTPS

Returns a structured report of every step taken.`,
		Example: `  cloudflare-pp-cli setup_zone example.com --origin 203.0.113.10 --redirect-from "legacy.example.com/*"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || origin == "" {
				return cmd.Help()
			}
			zone := args[0]
			if dryRunOK(flags) {
				steps := []map[string]any{
					{"step": "dns_a_record", "name": zone, "content": origin, "action": "would_apply"},
				}
				if redirectFrom != "" {
					steps = append(steps, map[string]any{
						"step":   "redirect_page_rule",
						"from":   redirectFrom,
						"to":     fmt.Sprintf("https://%s/$1", zone),
						"action": "would_create",
					})
				}
				steps = append(steps, map[string]any{"step": "ssl_mode", "mode": sslMode, "action": "would_set"})
				if alwaysHTTPS {
					steps = append(steps, map[string]any{"step": "always_use_https", "action": "would_enable"})
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"zone":    zone,
					"dry_run": true,
					"steps":   steps,
					"verify_command": fmt.Sprintf("cloudflare-pp-cli propagate watch %s A --expect %s --watch", zone, origin),
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

			steps := []map[string]any{}

			// Step 1: A record (idempotent)
			dnsName := zone
			dnsBody := map[string]any{
				"type":    "A",
				"name":    dnsName,
				"content": origin,
				"ttl":     1,
				"proxied": proxied,
			}
			lookupParams := map[string]string{"type": "A", "name": dnsName}
			existing, _ := c.Get(fmt.Sprintf("/zones/%s/dns_records", zoneID), lookupParams)
			var existingArr []map[string]any
			for _, raw := range unwrapCFArray(existing) {
				var r map[string]any
				if json.Unmarshal(raw, &r) == nil {
					existingArr = append(existingArr, r)
				}
			}
			dnsStep := map[string]any{"step": "dns_a_record", "name": dnsName, "content": origin}
			if dryRunOK(flags) {
				if len(existingArr) > 0 {
					dnsStep["action"] = "would_update"
				} else {
					dnsStep["action"] = "would_create"
				}
			} else if len(existingArr) > 0 {
				cur := existingArr[0]
				if curContent, _ := cur["content"].(string); curContent == origin {
					dnsStep["action"] = "noop"
					dnsStep["id"] = cur["id"]
				} else {
					curID, _ := cur["id"].(string)
					_, _, err := c.Patch(fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, curID), dnsBody)
					if err != nil {
						dnsStep["error"] = err.Error()
					} else {
						dnsStep["action"] = "updated"
						dnsStep["id"] = curID
					}
				}
			} else {
				_, _, err := c.Post(fmt.Sprintf("/zones/%s/dns_records", zoneID), dnsBody)
				if err != nil {
					dnsStep["error"] = err.Error()
				} else {
					dnsStep["action"] = "created"
				}
			}
			steps = append(steps, dnsStep)

			// Step 2: Optional redirect Page Rule
			if redirectFrom != "" {
				to := fmt.Sprintf("https://%s/$1", zone)
				prBody := map[string]any{
					"targets": []map[string]any{
						{"target": "url", "constraint": map[string]any{"operator": "matches", "value": redirectFrom}},
					},
					"actions": []map[string]any{
						{"id": "forwarding_url", "value": map[string]any{"url": to, "status_code": 301}},
					},
					"priority": 1,
					"status":   "active",
				}
				prStep := map[string]any{"step": "redirect_page_rule", "from": redirectFrom, "to": to}
				if dryRunOK(flags) {
					prStep["action"] = "would_create"
				} else {
					_, _, err := c.Post(fmt.Sprintf("/zones/%s/pagerules", zoneID), prBody)
					if err != nil {
						prStep["error"] = err.Error()
					} else {
						prStep["action"] = "created"
					}
				}
				steps = append(steps, prStep)
			}

			// Step 3: SSL mode
			sslStep := map[string]any{"step": "ssl_mode", "mode": sslMode}
			if dryRunOK(flags) {
				sslStep["action"] = "would_set"
			} else {
				_, _, err := c.Patch(fmt.Sprintf("/zones/%s/settings/ssl", zoneID), map[string]any{"value": sslMode})
				if err != nil {
					sslStep["error"] = err.Error()
				} else {
					sslStep["action"] = "set"
				}
			}
			steps = append(steps, sslStep)

			// Step 4: Always-Use-HTTPS
			if alwaysHTTPS {
				ahStep := map[string]any{"step": "always_use_https"}
				if dryRunOK(flags) {
					ahStep["action"] = "would_enable"
				} else {
					_, _, err := c.Patch(fmt.Sprintf("/zones/%s/settings/always_use_https", zoneID), map[string]any{"value": "on"})
					if err != nil {
						ahStep["error"] = err.Error()
					} else {
						ahStep["action"] = "enabled"
					}
				}
				steps = append(steps, ahStep)
			}

			watchHint := fmt.Sprintf("cloudflare-pp-cli propagate watch %s A --expect %s --watch", zone, origin)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"zone":          zone,
				"steps":         steps,
				"verify_command": watchHint,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&origin, "origin", "", "Origin IP for the apex A record (required)")
	cmd.Flags().StringVar(&redirectFrom, "redirect-from", "", `Optional URL pattern to redirect to this zone (e.g. "alias.example.com/*")`)
	cmd.Flags().StringVar(&account, "account", "", "Cloudflare account ID (used only when creating a zone; not required for existing zones)")
	cmd.Flags().BoolVar(&proxied, "proxied", false, "Whether the A record is proxied (orange cloud)")
	cmd.Flags().BoolVar(&alwaysHTTPS, "always-https", true, "Enable Always-Use-HTTPS zone setting")
	cmd.Flags().StringVar(&sslMode, "ssl", "strict", "SSL mode: off, flexible, full, strict")
	return cmd
}
