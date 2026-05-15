// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel command — not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// demandEntry is the public projection returned by GET /demand. The server
// strips operational fields (endpoint, clickUrl, creative) and commercial
// terms (bid), so this struct mirrors only what the public projection ships.
type demandEntry struct {
	ID          string   `json:"id"`
	Service     string   `json:"service"`
	ServiceType string   `json:"serviceType"`
	Category    string   `json:"category"`
	Description string   `json:"description"`
	Domain      string   `json:"domain"`
	Assets      []string `json:"assets"`
	Type        string   `json:"type"`
}

func newDemandSimilarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "similar <advertiser-id>",
		Short: "Find advertisers with overlapping category, assets, or serviceType.",
		Long: `Find advertisers in the active demand index that overlap with <advertiser-id>
on category, serviceType, or assets.

Match rule: an entry is "similar" if it shares any of:
  - same category
  - same serviceType
  - at least one overlapping asset

The target advertiser is excluded from the result set.`,
		Example: strings.Trim(`
  operon-pp-cli demand similar adv_changenow
  operon-pp-cli demand similar adv_jupiter --json
  operon-pp-cli demand similar adv_simpleswap --compact
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			targetID := strings.TrimSpace(args[0])
			if targetID == "" {
				return cmd.Help()
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Attach X-Operon-Client UUID header. Production /demand is gated on
			// a well-formed UUID; sandbox is permissive. Prefer cfg.Headers, fall
			// back to OPERON_CLIENT_UUID env, else use a stable fixture UUID so
			// dry-run + verify pass without requiring user setup.
			headers := map[string]string{}
			if c.Config != nil {
				if v, ok := c.Config.Headers["X-Operon-Client"]; ok && v != "" {
					headers["X-Operon-Client"] = v
				}
			}
			if headers["X-Operon-Client"] == "" {
				if v := os.Getenv("OPERON_CLIENT_UUID"); v != "" {
					headers["X-Operon-Client"] = v
				}
			}
			if headers["X-Operon-Client"] == "" {
				headers["X-Operon-Client"] = "00000000-0000-4000-8000-000000000001"
			}

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s/demand\n", c.BaseURL)
				fmt.Fprintf(cmd.OutOrStdout(), "  X-Operon-Client: %s\n", headers["X-Operon-Client"])
				fmt.Fprintf(cmd.OutOrStdout(), "(dry run - target advertiser: %s)\n", targetID)
				return nil
			}

			data, err := c.GetWithHeaders("/demand", nil, headers)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var entries []demandEntry
			if err := json.Unmarshal(data, &entries); err != nil {
				return apiErr(fmt.Errorf("parsing /demand response: %w", err))
			}

			var target *demandEntry
			rest := make([]demandEntry, 0, len(entries))
			for i := range entries {
				if entries[i].ID == targetID {
					t := entries[i]
					target = &t
					continue
				}
				rest = append(rest, entries[i])
			}
			if target == nil {
				examples := make([]string, 0, 3)
				for i := 0; i < len(entries) && len(examples) < 3; i++ {
					examples = append(examples, entries[i].ID)
				}
				exampleStr := "no other entries available"
				if len(examples) > 0 {
					exampleStr = strings.Join(examples, ", ")
				}
				return notFoundErr(fmt.Errorf(
					"advertiser %q not found in /demand index\nhint: known ids include: %s\n      run 'operon-pp-cli demand' to list all active advertisers",
					targetID, exampleStr,
				))
			}

			matches := make([]demandEntry, 0, len(rest))
			targetAssets := assetSet(target.Assets)
			for _, e := range rest {
				if overlapsDemand(target, &e, targetAssets) {
					matches = append(matches, e)
				}
			}

			// --compact: keep only id, service, category, serviceType (per spec).
			// Bypass the generic compactFields helper so the projection matches
			// the documented shape exactly rather than the canonical allow-list.
			if flags.compact && flags.selectFields == "" {
				slim := make([]map[string]any, 0, len(matches))
				for _, m := range matches {
					slim = append(slim, map[string]any{
						"id":          m.ID,
						"service":     m.Service,
						"category":    m.Category,
						"serviceType": m.ServiceType,
					})
				}
				return printJSONFiltered(cmd.OutOrStdout(), slim, &rootFlags{
					asJSON:       flags.asJSON,
					csv:          flags.csv,
					quiet:        flags.quiet,
					selectFields: flags.selectFields,
				})
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headersRow := []string{"id", "service", "category", "serviceType", "assets"}
				rows := make([][]string, 0, len(matches))
				for _, m := range matches {
					rows = append(rows, []string{
						m.ID,
						m.Service,
						m.Category,
						m.ServiceType,
						strings.Join(m.Assets, ","),
					})
				}
				if len(matches) == 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "no advertisers overlap with %s on category, serviceType, or assets\n", targetID)
					return nil
				}
				return flags.printTable(cmd, headersRow, rows)
			}

			return printJSONFiltered(cmd.OutOrStdout(), matches, flags)
		},
	}

	return cmd
}

func assetSet(assets []string) map[string]bool {
	m := make(map[string]bool, len(assets))
	for _, a := range assets {
		a = strings.TrimSpace(a)
		if a == "" {
			continue
		}
		m[strings.ToLower(a)] = true
	}
	return m
}

func overlapsDemand(target, candidate *demandEntry, targetAssets map[string]bool) bool {
	if target.Category != "" && target.Category == candidate.Category {
		return true
	}
	if target.ServiceType != "" && target.ServiceType == candidate.ServiceType {
		return true
	}
	for _, a := range candidate.Assets {
		if targetAssets[strings.ToLower(strings.TrimSpace(a))] {
			return true
		}
	}
	return false
}
