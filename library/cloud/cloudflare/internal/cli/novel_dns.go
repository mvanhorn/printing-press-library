// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

// PATCH transcendence-commands: hand-built idempotent DNS apply (no-op-if-identical) + zone-resolve helper. Phase 3 transcendence layer.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// unwrapCFArray pulls the result array out of a Cloudflare API response.
// The raw client returns the standard CF envelope {success, errors, result}.
// Some upstream wrappers add an extra layer ({results: {result: [...]}}) so
// we try both shapes plus a top-level array as fallback.
func unwrapCFArray(resp json.RawMessage) []json.RawMessage {
	// Shape 1: top-level array
	var arr []json.RawMessage
	if err := json.Unmarshal(resp, &arr); err == nil && len(arr) > 0 {
		return arr
	}
	// Shape 2: {result: [...]} (CF standard envelope)
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(resp, &env); err == nil && len(env.Result) > 0 {
		var inner []json.RawMessage
		if json.Unmarshal(env.Result, &inner) == nil && len(inner) > 0 {
			return inner
		}
	}
	// Shape 3: {results: {result: [...]}} (printing-press resolveRead wrapping)
	var env2 struct {
		Results struct {
			Result json.RawMessage `json:"result"`
		} `json:"results"`
	}
	if err := json.Unmarshal(resp, &env2); err == nil && len(env2.Results.Result) > 0 {
		var inner []json.RawMessage
		if json.Unmarshal(env2.Results.Result, &inner) == nil && len(inner) > 0 {
			return inner
		}
	}
	return nil
}

// resolveZoneID resolves a zone name (e.g. "example.com") to its zone ID.
// If `nameOrID` already looks like a 32-hex zone ID, returns it unchanged.
func resolveZoneID(c interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}, nameOrID string) (string, error) {
	if len(nameOrID) == 32 && !strings.ContainsAny(nameOrID, ".") {
		return nameOrID, nil
	}
	resp, err := c.Get("/zones", map[string]string{"name": nameOrID})
	if err != nil {
		return "", fmt.Errorf("looking up zone %q: %w", nameOrID, err)
	}
	items := unwrapCFArray(resp)
	for _, raw := range items {
		var z struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &z) == nil && z.ID != "" {
			return z.ID, nil
		}
	}
	return "", fmt.Errorf("zone %q not found", nameOrID)
}

func newDnsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dns",
		Short: "DNS-record convenience commands (idempotent apply)",
		Long:  "DNS-record convenience commands. Use `apply` for create-or-update with no-op-if-identical semantics.",
	}
	cmd.AddCommand(newDnsApplyCmd(flags))
	return cmd
}

func newDnsApplyCmd(flags *rootFlags) *cobra.Command {
	var (
		zone     string
		recType  string
		name     string
		content  string
		ttl      int
		proxied  bool
		priority int
		comment  string
	)

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Create or update a DNS record idempotently",
		Long: `Create a DNS record if it doesn't exist, update it if content differs, or no-op if identical.

Safe to run repeatedly with the same arguments — useful from scripts or agents.`,
		Example: `  cloudflare-pp-cli dns apply --zone example.com --type A --name @ --content 203.0.113.10 --proxied=false`,
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2,3,4,5,7",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if zone == "" || recType == "" || name == "" || content == "" {
				return cmd.Help()
			}
			// Short-circuit dry-run / verify env BEFORE any API call so the
			// command shape is testable without credentials or live zones.
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "would_apply",
					"zone":    zone,
					"type":    recType,
					"name":    name,
					"content": content,
					"ttl":     ttl,
					"proxied": proxied,
					"dry_run": true,
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

			// Look up an existing record matching name + type
			lookupParams := map[string]string{"type": recType, "name": resolveDNSName(name, zone)}
			existing, err := c.Get(fmt.Sprintf("/zones/%s/dns_records", zoneID), lookupParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var existingArr []map[string]any
			for _, raw := range unwrapCFArray(existing) {
				var r map[string]any
				if json.Unmarshal(raw, &r) == nil {
					existingArr = append(existingArr, r)
				}
			}

			result := map[string]any{
				"zone":   zone,
				"type":   recType,
				"name":   name,
				"action": "",
			}

			body := map[string]any{
				"type":    recType,
				"name":    name,
				"content": content,
				"ttl":     ttl,
				"proxied": proxied,
			}
			if recType == "MX" {
				body["priority"] = priority
			}
			if comment != "" {
				body["comment"] = comment
			}

			if len(existingArr) > 0 {
				cur := existingArr[0]
				if curContent, _ := cur["content"].(string); curContent == content {
					curTTL, _ := cur["ttl"].(float64)
					curProxied, _ := cur["proxied"].(bool)
					if int(curTTL) == ttl && curProxied == proxied {
						result["action"] = "noop"
						result["id"] = cur["id"]
						return printJSONFiltered(cmd.OutOrStdout(), result, flags)
					}
				}
				if dryRunOK(flags) {
					result["action"] = "would_update"
					result["id"] = cur["id"]
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				curID, _ := cur["id"].(string)
				resp, _, err := c.Patch(fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, curID), body)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				result["action"] = "updated"
				result["id"] = curID
				result["response"] = json.RawMessage(resp)
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}

			if dryRunOK(flags) {
				result["action"] = "would_create"
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			resp, _, err := c.Post(fmt.Sprintf("/zones/%s/dns_records", zoneID), body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			result["action"] = "created"
			result["response"] = json.RawMessage(resp)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&zone, "zone", "", "Zone name or ID (e.g. example.com)")
	cmd.Flags().StringVar(&recType, "type", "", "DNS record type (A, AAAA, CNAME, MX, TXT, SRV, NS, CAA, PTR)")
	cmd.Flags().StringVar(&name, "name", "", `Record name (use "@" for the apex)`)
	cmd.Flags().StringVar(&content, "content", "", "Record content (IP for A/AAAA, target for CNAME, etc.)")
	cmd.Flags().IntVar(&ttl, "ttl", 1, "TTL in seconds (1 = automatic)")
	cmd.Flags().BoolVar(&proxied, "proxied", false, "Whether the record is proxied through Cloudflare (orange cloud)")
	cmd.Flags().IntVar(&priority, "priority", 10, "MX priority (only for MX records)")
	cmd.Flags().StringVar(&comment, "comment", "", "Optional comment attached to the record")
	return cmd
}

// resolveDNSName expands "@" to the bare zone name, otherwise leaves the name alone.
// Cloudflare's API expects fully-qualified names in lookups.
func resolveDNSName(name, zone string) string {
	if name == "@" {
		return zone
	}
	if !strings.Contains(name, ".") && zone != "" {
		return name + "." + zone
	}
	return name
}

