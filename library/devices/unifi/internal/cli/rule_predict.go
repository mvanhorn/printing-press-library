// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: see .printing-press-patches/ for context. Hand-authored, not
// generator output — regen-merge preserves this file.
//
// Named "rule-predict", not "firewall explain" — the latter collides with
// the generator's own "sites firewall" resource-group naming check.

// pp:data-source computed

package cli

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"sort"

	"github.com/spf13/cobra"
)

type ruleAddressFilter struct {
	Type            string `json:"type"`
	IPAddressFilter *struct {
		Type          string `json:"type"`
		MatchOpposite bool   `json:"matchOpposite"`
		Items         []struct {
			Type  string `json:"type"`
			Value string `json:"value"`
		} `json:"items"`
	} `json:"ipAddressFilter"`
	ZoneID string `json:"zoneId"`
}

type firewallPolicyDoc struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Index   int    `json:"index"`
	Enabled bool   `json:"enabled"`
	Action  struct {
		Type string `json:"type"`
	} `json:"action"`
	Source      ruleAddressFilter `json:"source"`
	Destination ruleAddressFilter `json:"destination"`
}

// matchesSide reports whether ip matches a policy's source/destination
// filter side. A side with no trafficFilter is zone-wide ("any address in
// this zone") — this command has no live zone→network→subnet resolution, so
// it treats a zone-wide side as a possible match rather than excluding it,
// and says so via the returned note. Only the observed real-world shape
// (IP_ADDRESS / SUBNET or ADDRESS items) is evaluated precisely; any other
// traffic-matching-list-referencing filter type is treated the same way —
// not excluded, flagged as unresolved.
func matchesSide(side ruleAddressFilter, ip netip.Addr) (matched bool, certain bool) {
	if side.IPAddressFilter == nil {
		return true, false // zone-wide wildcard: possible match, not certain
	}
	if side.IPAddressFilter.Type != "IP_ADDRESSES" {
		return true, false // unrecognized filter shape: not excluded, not certain
	}
	found := false
	for _, item := range side.IPAddressFilter.Items {
		switch item.Type {
		case "SUBNET":
			prefix, err := netip.ParsePrefix(item.Value)
			if err != nil {
				continue
			}
			if prefix.Contains(ip) {
				found = true
			}
		case "ADDRESS":
			addr, err := netip.ParseAddr(item.Value)
			if err != nil {
				continue
			}
			if addr == ip {
				found = true
			}
		}
	}
	if side.IPAddressFilter.MatchOpposite {
		found = !found
	}
	return found, true
}

type rulePredictResult struct {
	Src        string `json:"src"`
	Dst        string `json:"dst"`
	Port       string `json:"port,omitempty"`
	Matched    bool   `json:"matched"`
	PolicyID   string `json:"policy_id,omitempty"`
	PolicyName string `json:"policy_name,omitempty"`
	Action     string `json:"action,omitempty"`
	Certain    bool   `json:"certain"`
	Note       string `json:"note,omitempty"`
}

func newNovelRulePredictCmd(flags *rootFlags) *cobra.Command {
	var flagSite string
	var flagSrc string
	var flagDst string
	var flagPort string

	cmd := &cobra.Command{
		Use:   "rule-predict",
		Short: "Predict which firewall policy would match a hypothetical packet before making a live change.",
		Long: "Walks the locally synced firewall policies in ascending index " +
			"order (first match wins, matching how the gateway evaluates them) " +
			"and reports the first enabled policy whose source/destination " +
			"IP-address filter matches --src/--dst. This is a local simulation " +
			"against the last synced ruleset, not a live trace on the gateway — " +
			"it does not guarantee real traffic will be handled the same way if " +
			"the config has changed since the last sync. Policy sides with no " +
			"explicit IP filter (zone-wide) or a traffic-matching-list reference " +
			"this command can't resolve are reported as an uncertain match " +
			"(certain=false) rather than silently assumed. --port is echoed in " +
			"output for reference only; protocol/port scope matching is out of " +
			"scope for this command. Run 'unifi-pp-cli sync' first.",
		Example:     "  unifi-pp-cli rule-predict --src 10.0.3.0/24 --dst 10.0.0.1 --port 443 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--src=192.0.2.1;--dst=192.0.2.2"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rule-predict")
			}
			if flagSrc == "" || flagDst == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--src and --dst are required"))
			}
			srcIP, err := parseHostOrFirstOfSubnet(flagSrc)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --src %q: %w", flagSrc, err))
			}
			dstIP, err := parseHostOrFirstOfSubnet(flagDst)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --dst %q: %w", flagDst, err))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath := defaultDBPath("unifi-pp-cli")
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return err
			}
			result := rulePredictResult{Src: flagSrc, Dst: flagDst, Port: flagPort}
			if db == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync\n", dbPath)
				result.Note = "no local mirror synced yet"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				return nil
			}
			defer db.Close()

			siteID, _, err := resolveSiteIDLocal(ctx, db.DB(), flagSite)
			if err != nil {
				if isNoLocalDataYet(err) {
					fmt.Fprintf(cmd.ErrOrStderr(), "%s\nrun: unifi-pp-cli sync\n", err)
					result.Note = err.Error()
					if !wantsHumanTable(cmd.OutOrStdout(), flags) {
						return printJSONFiltered(cmd.OutOrStdout(), result, flags)
					}
					return nil
				}
				return err
			}
			policyRows, err := genericResourceRows(ctx, db.DB(), "v1_sites_firewall_policies", siteID)
			if err != nil {
				return err
			}

			policies := make([]firewallPolicyDoc, 0, len(policyRows))
			for _, id := range sortedKeys(policyRows) {
				var p firewallPolicyDoc
				if json.Unmarshal(policyRows[id], &p) != nil {
					continue
				}
				policies = append(policies, p)
			}
			sort.Slice(policies, func(i, j int) bool { return policies[i].Index < policies[j].Index })

			for _, p := range policies {
				if !p.Enabled {
					continue
				}
				srcMatch, srcCertain := matchesSide(p.Source, srcIP)
				dstMatch, dstCertain := matchesSide(p.Destination, dstIP)
				if srcMatch && dstMatch {
					result.Matched = true
					result.PolicyID = p.ID
					result.PolicyName = p.Name
					result.Action = p.Action.Type
					result.Certain = srcCertain && dstCertain
					if !result.Certain {
						result.Note = "matched, but one side had no specific IP filter (zone-wide or unresolved traffic-matching-list) — treat as a possible match, not a guarantee"
					}
					break
				}
			}
			if !result.Matched {
				result.Note = "no enabled policy's IP filters matched both --src and --dst against the synced ruleset"
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := cmd.OutOrStdout()
			if !result.Matched {
				fmt.Fprintln(w, result.Note)
				return nil
			}
			fmt.Fprintf(w, "%s -> %s: %s (policy %q, certain=%v)\n", result.Src, result.Dst, result.Action, result.PolicyName, result.Certain)
			if result.Note != "" {
				fmt.Fprintln(w, result.Note)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSite, "site", "", "Site id, internalReference, or name (default: the only synced site)")
	cmd.Flags().StringVar(&flagSrc, "src", "", "Source IP address or CIDR (required)")
	cmd.Flags().StringVar(&flagDst, "dst", "", "Destination IP address or CIDR (required)")
	cmd.Flags().StringVar(&flagPort, "port", "", "Destination port, for reference only — not used for matching")
	return cmd
}

// parseHostOrFirstOfSubnet accepts either a bare IP or a CIDR (using the
// network's first address as the representative host to test against
// policy filters).
func parseHostOrFirstOfSubnet(s string) (netip.Addr, error) {
	if addr, err := netip.ParseAddr(s); err == nil {
		return addr, nil
	}
	ip, _, err := net.ParseCIDR(s)
	if err != nil {
		return netip.Addr{}, err
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, fmt.Errorf("could not parse %q as an address", s)
	}
	return addr.Unmap(), nil
}
