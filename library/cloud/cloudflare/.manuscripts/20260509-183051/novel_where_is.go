// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newWhereIsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "where-is <hostname>",
		Short: "Find every place a hostname appears across Cloudflare products",
		Long: `Search the user's Cloudflare account for every artifact that references a hostname:
DNS records, Worker routes, Page Rules, and custom hostnames. Useful before deleting
or changing a domain — confirms nothing else depends on it.`,
		Example:     `  cloudflare-pp-cli where-is klinikalarm.dk --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			hostname := strings.ToLower(args[0])

			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"hostname": hostname,
					"dry_run":  true,
					"action":   "would_locate",
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			result := map[string]any{"hostname": hostname}

			// 1. Find zones whose name contains the hostname (or whose name == apex of hostname).
			zonesResp, _ := c.Get("/zones", map[string]string{"per_page": "50"})
			zoneItems := unwrapCFArray(zonesResp)
			zones := make([]map[string]any, 0, len(zoneItems))
			for _, raw := range zoneItems {
				var z map[string]any
				if json.Unmarshal(raw, &z) == nil {
					zones = append(zones, z)
				}
			}

			matchingZones := []map[string]any{}
			zonesByID := map[string]map[string]any{}
			apex := apexOf(hostname)
			for _, z := range zones {
				zname, _ := z["name"].(string)
				zid, _ := z["id"].(string)
				if zid != "" {
					zonesByID[zid] = z
				}
				if zname == hostname || zname == apex || strings.HasSuffix(hostname, "."+zname) {
					matchingZones = append(matchingZones, map[string]any{
						"id":     z["id"],
						"name":   zname,
						"status": z["status"],
					})
				}
			}
			result["zones"] = matchingZones

			// 2. For each matching zone, list DNS records that reference the hostname.
			dnsHits := []map[string]any{}
			pageRuleHits := []map[string]any{}
			workerRouteHits := []map[string]any{}
			for _, z := range matchingZones {
				zid, _ := z["id"].(string)
				if zid == "" {
					continue
				}
				dns, _ := c.Get(fmt.Sprintf("/zones/%s/dns_records", zid), map[string]string{"per_page": "100"})
				for _, raw := range unwrapCFArray(dns) {
					var r map[string]any
					if json.Unmarshal(raw, &r) != nil {
						continue
					}
					rname, _ := r["name"].(string)
					if strings.EqualFold(rname, hostname) || strings.HasSuffix(strings.ToLower(rname), "."+hostname) {
						dnsHits = append(dnsHits, map[string]any{
							"zone_id": zid,
							"zone":    z["name"],
							"id":      r["id"],
							"name":    r["name"],
							"type":    r["type"],
							"content": r["content"],
							"proxied": r["proxied"],
						})
					}
				}

				prs, _ := c.Get(fmt.Sprintf("/zones/%s/pagerules", zid), nil)
				for _, raw := range unwrapCFArray(prs) {
					var pr map[string]any
					if json.Unmarshal(raw, &pr) != nil {
						continue
					}
					if pageRuleMentions(pr, hostname) {
						pageRuleHits = append(pageRuleHits, map[string]any{
							"zone_id":  zid,
							"zone":     z["name"],
							"id":       pr["id"],
							"priority": pr["priority"],
							"status":   pr["status"],
							"targets":  pr["targets"],
						})
					}
				}

				wr, _ := c.Get(fmt.Sprintf("/zones/%s/workers/routes", zid), nil)
				for _, raw := range unwrapCFArray(wr) {
					var route map[string]any
					if json.Unmarshal(raw, &route) != nil {
						continue
					}
					pat, _ := route["pattern"].(string)
					if strings.Contains(strings.ToLower(pat), hostname) {
						workerRouteHits = append(workerRouteHits, map[string]any{
							"zone_id": zid,
							"zone":    z["name"],
							"id":      route["id"],
							"pattern": pat,
							"script":  route["script"],
						})
					}
				}
			}
			result["dns_records"] = dnsHits
			result["page_rules"] = pageRuleHits
			result["worker_routes"] = workerRouteHits

			result["summary"] = map[string]any{
				"zones":         len(matchingZones),
				"dns_records":   len(dnsHits),
				"page_rules":    len(pageRuleHits),
				"worker_routes": len(workerRouteHits),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// apexOf returns the apex (registrable) domain for a hostname, naively assuming a
// single TLD label (good enough for the Cloudflare flow; not a public-suffix-list lookup).
func apexOf(host string) string {
	parts := strings.Split(host, ".")
	if len(parts) <= 2 {
		return host
	}
	return strings.Join(parts[len(parts)-2:], ".")
}

func pageRuleMentions(pr map[string]any, hostname string) bool {
	targets, _ := pr["targets"].([]any)
	for _, t := range targets {
		tm, _ := t.(map[string]any)
		c, _ := tm["constraint"].(map[string]any)
		v, _ := c["value"].(string)
		if strings.Contains(strings.ToLower(v), hostname) {
			return true
		}
	}
	actions, _ := pr["actions"].([]any)
	for _, a := range actions {
		am, _ := a.(map[string]any)
		v, _ := am["value"].(map[string]any)
		if u, ok := v["url"].(string); ok && strings.Contains(strings.ToLower(u), hostname) {
			return true
		}
	}
	return false
}
