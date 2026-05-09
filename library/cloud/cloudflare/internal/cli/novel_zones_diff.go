// Copyright 2026 alex-osti. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func newZonesDiffCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "diff <zone-a> <zone-b>",
		Short: "Diff two zones across settings, page rules, and DNS records",
		Long: `Fetch zone settings, page rules, and DNS records for both zones and emit the
deltas. Useful before promoting staging to prod, during incident review, or when
onboarding a tenant zone from a template.

DNS records match semantically on (name, type) — order-independent.`,
		Example:     `  cloudflare-pp-cli zones diff staging.makertoo.win prod.makertoo.win --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			a, b := args[0], args[1]
			if dryRunOK(flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"action":  "would_diff",
					"zone_a":  a,
					"zone_b":  b,
					"dry_run": true,
				}, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			aID, err := resolveZoneID(c, a)
			if err != nil {
				return notFoundErr(fmt.Errorf("zone-a: %w", err))
			}
			bID, err := resolveZoneID(c, b)
			if err != nil {
				return notFoundErr(fmt.Errorf("zone-b: %w", err))
			}

			fetch := func(zid string) (settings map[string]any, dnsRecords []map[string]any, pageRules []map[string]any) {
				sresp, _ := c.Get(fmt.Sprintf("/zones/%s/settings", zid), nil)
				settings = map[string]any{}
				for _, raw := range unwrapCFArray(sresp) {
					var s map[string]any
					if json.Unmarshal(raw, &s) != nil {
						continue
					}
					if id, ok := s["id"].(string); ok {
						settings[id] = s["value"]
					}
				}
				dresp, _ := c.Get(fmt.Sprintf("/zones/%s/dns_records", zid), map[string]string{"per_page": "200"})
				for _, raw := range unwrapCFArray(dresp) {
					var r map[string]any
					if json.Unmarshal(raw, &r) == nil {
						dnsRecords = append(dnsRecords, r)
					}
				}
				presp, _ := c.Get(fmt.Sprintf("/zones/%s/pagerules", zid), nil)
				for _, raw := range unwrapCFArray(presp) {
					var pr map[string]any
					if json.Unmarshal(raw, &pr) == nil {
						pageRules = append(pageRules, pr)
					}
				}
				return
			}

			aSettings, aDNS, aPR := fetch(aID)
			bSettings, bDNS, bPR := fetch(bID)

			result := map[string]any{
				"zone_a":      a,
				"zone_b":      b,
				"settings":    diffSettings(aSettings, bSettings),
				"dns_records": diffDNS(aDNS, bDNS),
				"page_rules":  diffPageRules(aPR, bPR),
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

func diffSettings(a, b map[string]any) map[string]any {
	delta := map[string]any{}
	keys := map[string]struct{}{}
	for k := range a {
		keys[k] = struct{}{}
	}
	for k := range b {
		keys[k] = struct{}{}
	}
	keyList := make([]string, 0, len(keys))
	for k := range keys {
		keyList = append(keyList, k)
	}
	sort.Strings(keyList)
	for _, k := range keyList {
		av, ah := a[k]
		bv, bh := b[k]
		if !ah {
			delta[k] = map[string]any{"only_in": "b", "value": bv}
		} else if !bh {
			delta[k] = map[string]any{"only_in": "a", "value": av}
		} else if !jsonEqual(av, bv) {
			delta[k] = map[string]any{"a": av, "b": bv}
		}
	}
	return delta
}

func diffDNS(aDNS, bDNS []map[string]any) map[string]any {
	keyOf := func(r map[string]any) string {
		t, _ := r["type"].(string)
		n, _ := r["name"].(string)
		return t + " " + n
	}
	aMap := map[string]map[string]any{}
	for _, r := range aDNS {
		aMap[keyOf(r)] = r
	}
	bMap := map[string]map[string]any{}
	for _, r := range bDNS {
		bMap[keyOf(r)] = r
	}
	onlyA := []map[string]any{}
	onlyB := []map[string]any{}
	differ := []map[string]any{}
	for k, r := range aMap {
		br, ok := bMap[k]
		if !ok {
			onlyA = append(onlyA, r)
			continue
		}
		if !jsonEqual(r["content"], br["content"]) || !jsonEqual(r["proxied"], br["proxied"]) || !jsonEqual(r["ttl"], br["ttl"]) {
			differ = append(differ, map[string]any{"key": k, "a": r, "b": br})
		}
	}
	for k, r := range bMap {
		if _, ok := aMap[k]; !ok {
			onlyB = append(onlyB, r)
		}
	}
	return map[string]any{"only_in_a": onlyA, "only_in_b": onlyB, "differ": differ}
}

func diffPageRules(aPR, bPR []map[string]any) map[string]any {
	keyOf := func(p map[string]any) string {
		targets, _ := p["targets"].([]any)
		if len(targets) == 0 {
			return ""
		}
		t, _ := targets[0].(map[string]any)
		c, _ := t["constraint"].(map[string]any)
		v, _ := c["value"].(string)
		return v
	}
	aMap := map[string]map[string]any{}
	for _, p := range aPR {
		k := keyOf(p)
		if k != "" {
			aMap[k] = p
		}
	}
	bMap := map[string]map[string]any{}
	for _, p := range bPR {
		k := keyOf(p)
		if k != "" {
			bMap[k] = p
		}
	}
	onlyA := []map[string]any{}
	onlyB := []map[string]any{}
	for k, p := range aMap {
		if _, ok := bMap[k]; !ok {
			onlyA = append(onlyA, p)
		}
	}
	for k, p := range bMap {
		if _, ok := aMap[k]; !ok {
			onlyB = append(onlyB, p)
		}
	}
	return map[string]any{"only_in_a": onlyA, "only_in_b": onlyB}
}

func jsonEqual(a, b any) bool {
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	return string(ab) == string(bb)
}
