// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel command. pp:data-source local.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
	"net/netip"
)

// triState is why this evaluator is not a boolean: most of what can sit in a policy
// (domains, applications, regions, time schedules) cannot be decided from a packet
// description, and "unknown" must stop the walk rather than fall through. Falling
// through would confidently report a later policy that might never be reached.
type triState int

const (
	triNo triState = iota
	triYes
	triUnknown
)

type predictPacket struct {
	SrcZone  string
	DstZone  string
	SrcIP    string
	DstIP    string
	SrcMAC   string
	SrcPort  int
	DstPort  int
	Protocol string
}

type predictInputs struct {
	Policies []map[string]any
	ACLRules []map[string]any
	Zones    []map[string]any
	TMLs     map[string]map[string]any
}

type predictedPolicy struct {
	ID     string `json:"id"`
	Name   string `json:"name,omitempty"`
	Index  int    `json:"index"`
	Action string `json:"action,omitempty"`
}

type predictResult struct {
	// Verdict is "match" or "indeterminate". There is deliberately no "safe" or
	// "allowed for sure": no API confirms this, and a zone's built-in defaults and
	// legacy firewall rules can sit outside the model.
	Verdict           string           `json:"verdict"`
	PredictedAction   string           `json:"predicted_action,omitempty"`
	MatchedPolicy     *predictedPolicy `json:"matched_policy,omitempty"`
	UndecidedPolicy   *predictedPolicy `json:"undecided_policy,omitempty"`
	Reason            string           `json:"reason"`
	SkippedPolicies   int              `json:"policies_evaluated"`
	ACLRulesInTheWay  []string         `json:"acl_rules_may_apply,omitempty"`
	Prediction        bool             `json:"prediction"`
	ConfirmedByAPI    bool             `json:"confirmed_by_api"`
	AssumedConnection string           `json:"assumed_connection_state"`
	Notes             []string         `json:"notes"`
}

var rulePredictNotes = []string{
	"This is a prediction. No API simulates a packet, so nothing here is confirmed by the gateway.",
	"Legacy firewall rules and the classic controller's rule list are outside the model.",
}

func asMap(v any) map[string]any { m, _ := v.(map[string]any); return m }

func asString(v any) string { s, _ := v.(string); return s }

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case string:
		i, err := strconv.Atoi(n)
		return i, err == nil
	}
	return 0, false
}

func invertIf(result triState, opposite bool) triState {
	if !opposite || result == triUnknown {
		return result
	}
	if result == triYes {
		return triNo
	}
	return triYes
}

func matchesPortItems(items []any, port int) triState {
	for _, raw := range items {
		item := asMap(raw)
		switch asString(item["type"]) {
		case "PORT_NUMBER":
			value, ok := asInt(item["value"])
			if !ok {
				return triUnknown
			}
			if value == port {
				return triYes
			}
		case "PORT_NUMBER_RANGE":
			start, okStart := asInt(item["start"])
			stop, okStop := asInt(item["stop"])
			if !okStart || !okStop {
				return triUnknown
			}
			if port >= start && port <= stop {
				return triYes
			}
		default:
			// An item type this model does not know could match anything.
			return triUnknown
		}
	}
	return triNo
}

func evalPortFilter(filter map[string]any, port int, in predictInputs) triState {
	if filter == nil {
		return triYes
	}
	if port == 0 {
		return triUnknown
	}
	opposite, _ := filter["matchOpposite"].(bool)
	switch asString(filter["type"]) {
	case "PORTS":
		items, _ := filter["items"].([]any)
		return invertIf(matchesPortItems(items, port), opposite)
	case "TRAFFIC_MATCHING_LIST":
		list, ok := in.TMLs[asString(filter["trafficMatchingListId"])]
		if !ok || asString(list["type"]) != "PORTS" {
			return triUnknown
		}
		items, _ := list["items"].([]any)
		return invertIf(matchesPortItems(items, port), opposite)
	}
	return triUnknown
}

func matchesIPItems(items []any, rawIP string) triState {
	ip, err := netip.ParseAddr(rawIP)
	if err != nil {
		return triUnknown
	}
	// ::ffff:10.0.0.5 is 10.0.0.5, but Prefix.Contains and == treat the forms as different.
	ip = ip.Unmap()
	for _, rawItem := range items {
		item := asMap(rawItem)
		switch asString(item["type"]) {
		case "IP_ADDRESS":
			addr, err := netip.ParseAddr(asString(item["value"]))
			if err != nil {
				return triUnknown
			}
			if addr.Unmap() == ip {
				return triYes
			}
		case "SUBNET":
			prefix, err := netip.ParsePrefix(asString(item["value"]))
			if err != nil {
				return triUnknown
			}
			if prefixHolds(prefix, ip) {
				return triYes
			}
		case "IP_ADDRESS_RANGE":
			start, errStart := netip.ParseAddr(asString(item["start"]))
			stop, errStop := netip.ParseAddr(asString(item["stop"]))
			if errStart != nil || errStop != nil {
				return triUnknown
			}
			if ip.Compare(start.Unmap()) >= 0 && ip.Compare(stop.Unmap()) <= 0 {
				return triYes
			}
		default:
			return triUnknown
		}
	}
	return triNo
}

// prefixHolds compares an unmapped address against a prefix that may be written in the
// mapped form, ::ffff:10.0.0.0/104, which Contains would not match against 10.0.0.5.
func prefixHolds(prefix netip.Prefix, ip netip.Addr) bool {
	if addr := prefix.Addr(); addr.Is4In6() {
		if prefix.Bits() < 96 {
			return false
		}
		prefix = netip.PrefixFrom(addr.Unmap(), prefix.Bits()-96)
	}
	return prefix.Contains(ip)
}

func evalIPFilter(filter map[string]any, ip string, in predictInputs) triState {
	if ip == "" {
		return triUnknown
	}
	opposite, _ := filter["matchOpposite"].(bool)
	switch asString(filter["type"]) {
	case "IP_ADDRESSES":
		items, _ := filter["items"].([]any)
		return invertIf(matchesIPItems(items, ip), opposite)
	case "TRAFFIC_MATCHING_LIST":
		list, ok := in.TMLs[asString(filter["trafficMatchingListId"])]
		if !ok || !strings.Contains(asString(list["type"]), "ADDRESSES") {
			return triUnknown
		}
		items, _ := list["items"].([]any)
		return invertIf(matchesIPItems(items, ip), opposite)
	}
	return triUnknown
}

func andTri(a, b triState) triState {
	if a == triNo || b == triNo {
		return triNo
	}
	if a == triUnknown || b == triUnknown {
		return triUnknown
	}
	return triYes
}

// evalTrafficFilter applies one side's filter. Domain, application and region need
// data a packet description does not have, so they are unknown. A port mismatch still
// gives a definite no even then, because the policy needs both to match.
func evalTrafficFilter(filter map[string]any, ip, mac string, port int, in predictInputs) (triState, string) {
	if filter == nil {
		return triYes, ""
	}
	portResult := evalPortFilter(asMap(filter["portFilter"]), port, in)
	switch kind := asString(filter["type"]); kind {
	case "PORT":
		return portResult, "port filter needs the packet's port"
	case "IP_ADDRESS":
		return andTri(evalIPFilter(asMap(filter["ipAddressFilter"]), ip, in), portResult), "IP filter needs the packet's address and port"
	case "MAC_ADDRESS":
		macFilter := asMap(filter["macAddressFilter"])
		macs, _ := macFilter["macAddresses"].([]any)
		if mac == "" {
			return andTri(triUnknown, portResult), "MAC filter needs --src-mac"
		}
		result := triNo
		for _, candidate := range macs {
			if strings.EqualFold(asString(candidate), mac) {
				result = triYes
			}
		}
		opposite, _ := macFilter["matchOpposite"].(bool)
		return andTri(invertIf(result, opposite), portResult), "MAC filter"
	default:
		return andTri(triUnknown, portResult), kind + " filters cannot be evaluated from a packet description"
	}
}

func evalProtocol(scope map[string]any, protocol string) triState {
	filter := asMap(scope["protocolFilter"])
	if filter == nil {
		return triYes
	}
	if protocol == "" {
		return triUnknown
	}
	protocol = strings.ToLower(protocol)
	opposite, _ := filter["matchOpposite"].(bool)
	switch asString(filter["type"]) {
	case "NAMED_PROTOCOL":
		name := strings.ToLower(asString(asMap(filter["protocol"])["name"]))
		if name == "" {
			return triUnknown
		}
		matched := name == protocol || (name == "tcp_udp" && (protocol == "tcp" || protocol == "udp"))
		return invertIf(boolTri(matched), opposite)
	case "PRESET":
		name := strings.ToUpper(asString(asMap(filter["preset"])["name"]))
		if name == "TCP_UDP" {
			return invertIf(boolTri(protocol == "tcp" || protocol == "udp"), opposite)
		}
		return triUnknown
	}
	return triUnknown
}

// evalIPVersion: a policy scoped to one address family says nothing about the other,
// and a required field that is absent cannot be read as "both".
func evalIPVersion(scope map[string]any, p predictPacket) triState {
	version := strings.ToUpper(asString(scope["ipVersion"]))
	switch version {
	case "IPV4_AND_IPV6":
		return triYes
	case "IPV4", "IPV6":
	default:
		return triUnknown
	}
	family := ""
	for _, raw := range []string{p.SrcIP, p.DstIP} {
		if addr, err := netip.ParseAddr(raw); err == nil {
			family = "IPV6"
			if addr.Unmap().Is4() {
				family = "IPV4"
			}
			break
		}
	}
	if family == "" {
		return triUnknown
	}
	return boolTri(family == version)
}

func boolTri(b bool) triState {
	if b {
		return triYes
	}
	return triNo
}

// evalPolicy returns whether the policy applies, and if unknown, why.
func evalPolicy(policy map[string]any, p predictPacket, in predictInputs) (triState, string) {
	if enabled, ok := policy["enabled"].(bool); ok && !enabled {
		return triNo, ""
	}
	src, dst := asMap(policy["source"]), asMap(policy["destination"])
	// The zones are required by the spec. Without them the policy's scope is unknown,
	// and reading absence as "any zone" could report a confident ALLOW.
	if asString(src["zoneId"]) == "" || asString(dst["zoneId"]) == "" {
		return triUnknown, "the policy does not name both its zones in the mirror"
	}
	if asString(src["zoneId"]) != "" && p.SrcZone != "" && asString(src["zoneId"]) != p.SrcZone {
		return triNo, ""
	}
	if asString(dst["zoneId"]) != "" && p.DstZone != "" && asString(dst["zoneId"]) != p.DstZone {
		return triNo, ""
	}
	result := triYes
	reason := ""
	merge := func(r triState, why string) {
		if r == triUnknown && reason == "" {
			reason = why
		}
		result = andTri(result, r)
	}
	if p.SrcZone == "" || p.DstZone == "" {
		merge(triUnknown, "the packet's zones were not given")
	}
	if schedule := asMap(policy["schedule"]); schedule != nil {
		if mode := asString(schedule["mode"]); mode != "" && mode != "ALWAYS" {
			merge(triUnknown, "schedule "+mode+" depends on the time of the packet")
		}
	}
	if asString(policy["ipsecFilter"]) != "" {
		merge(triUnknown, "IPsec filter cannot be evaluated")
	}
	// Assumes a new connection: this tool answers whether one would be allowed.
	if states, ok := policy["connectionStateFilter"].([]any); ok && len(states) > 0 {
		isNew := false
		for _, state := range states {
			if strings.EqualFold(asString(state), "NEW") {
				isNew = true
			}
		}
		if !isNew {
			return triNo, ""
		}
	}
	scope := asMap(policy["ipProtocolScope"])
	merge(evalIPVersion(scope, p), "the packet's address family was not given or the policy's IP version is unknown")
	protoResult := evalProtocol(scope, p.Protocol)
	merge(protoResult, "the packet's protocol was not given or the protocol filter is unknown")
	srcResult, srcWhy := evalTrafficFilter(asMap(src["trafficFilter"]), p.SrcIP, p.SrcMAC, p.SrcPort, in)
	merge(srcResult, "source: "+srcWhy)
	dstResult, dstWhy := evalTrafficFilter(asMap(dst["trafficFilter"]), p.DstIP, "", p.DstPort, in)
	merge(dstResult, "destination: "+dstWhy)
	return result, reason
}

func policySummary(policy map[string]any) *predictedPolicy {
	index, _ := asInt(policy["index"])
	return &predictedPolicy{ID: asString(policy["id"]), Name: asString(policy["name"]), Index: index, Action: asString(asMap(policy["action"])["type"])}
}

// evalACL: ACLs are enforced on switches along the path, and the path is not modeled.
// A BLOCK that may match therefore makes the overall answer indeterminate instead of
// being applied or ignored.
func evalACLRules(rules []map[string]any, p predictPacket, in predictInputs) []string {
	sorted := append([]map[string]any(nil), rules...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, _ := asInt(sorted[i]["index"])
		b, _ := asInt(sorted[j]["index"])
		return a < b
	})
	blocking := make([]string, 0)
	for _, rule := range sorted {
		if enabled, ok := rule["enabled"].(bool); ok && !enabled {
			continue
		}
		if asString(rule["action"]) != "BLOCK" {
			continue
		}
		match := triUnknown
		if asString(rule["type"]) == "IPV4" {
			match = andTri(evalACLEndpoint(asMap(rule["sourceFilter"]), p.SrcIP, p.SrcPort), evalACLEndpoint(asMap(rule["destinationFilter"]), p.DstIP, p.DstPort))
			if protocols, ok := rule["protocolFilter"].([]any); ok && len(protocols) > 0 {
				found := triNo
				if p.Protocol == "" {
					found = triUnknown
				}
				for _, proto := range protocols {
					if strings.EqualFold(asString(proto), p.Protocol) {
						found = triYes
					}
				}
				match = andTri(match, found)
			}
		}
		if match != triNo {
			blocking = append(blocking, firstNonEmpty(asString(rule["name"]), asString(rule["id"])))
		}
	}
	return blocking
}

func evalACLEndpoint(filter map[string]any, ip string, port int) triState {
	if filter == nil {
		return triYes
	}
	portResult := triYes
	if ports, ok := filter["portFilter"].([]any); ok && len(ports) > 0 {
		if port == 0 {
			portResult = triUnknown
		} else {
			portResult = triNo
			for _, candidate := range ports {
				if value, ok := asInt(candidate); ok && value == port {
					portResult = triYes
				}
			}
		}
	}
	switch asString(filter["type"]) {
	case "PORTS":
		return portResult
	case "IP_ADDRESSES_OR_SUBNETS":
		if ip == "" {
			return andTri(triUnknown, portResult)
		}
		addr, err := netip.ParseAddr(ip)
		if err != nil {
			return triUnknown
		}
		addr = addr.Unmap()
		entries, _ := filter["ipAddressesOrSubnets"].([]any)
		result := triNo
		for _, entry := range entries {
			text := asString(entry)
			if prefix, err := netip.ParsePrefix(text); err == nil {
				if prefixHolds(prefix, addr) {
					result = triYes
				}
				continue
			}
			if candidate, err := netip.ParseAddr(text); err == nil && candidate.Unmap() == addr {
				result = triYes
			}
		}
		return andTri(result, portResult)
	}
	return andTri(triUnknown, portResult)
}

// predictRule walks policies in index order and stops at the first that matches or
// cannot be decided.
func predictRule(p predictPacket, in predictInputs) predictResult {
	result := predictResult{Prediction: true, ConfirmedByAPI: false, AssumedConnection: "new", Notes: rulePredictNotes}
	policies := append([]map[string]any(nil), in.Policies...)
	sort.SliceStable(policies, func(i, j int) bool {
		a, _ := asInt(policies[i]["index"])
		b, _ := asInt(policies[j]["index"])
		return a < b
	})
	result.ACLRulesInTheWay = evalACLRules(in.ACLRules, p, in)
	for _, policy := range policies {
		match, why := evalPolicy(policy, p, in)
		result.SkippedPolicies++
		if match == triNo {
			continue
		}
		// Sorting treats a missing index as 0, so this policy's turn is a guess, and a
		// first-match walk is only as good as its order.
		if _, ordered := asInt(policy["index"]); !ordered {
			result.Verdict = "indeterminate"
			result.UndecidedPolicy = policySummary(policy)
			result.Reason = "this policy has no index in the mirror, so evaluation order is unknown"
			return result
		}
		if match == triUnknown {
			result.Verdict = "indeterminate"
			result.UndecidedPolicy = policySummary(policy)
			result.Reason = "cannot decide whether this policy applies: " + why
			return result
		}
		result.Verdict = "match"
		result.MatchedPolicy = policySummary(policy)
		result.PredictedAction = result.MatchedPolicy.Action
		result.Reason = "first enabled policy in index order whose filters all match"
		if len(result.ACLRulesInTheWay) > 0 {
			result.Verdict = "indeterminate"
			result.PredictedAction = ""
			result.Reason = "a switch ACL with BLOCK may apply on the path, and the path is not modeled; the firewall policy alone would be " + result.MatchedPolicy.Name
		}
		return result
	}
	result.Verdict = "indeterminate"
	result.Reason = "no synced policy matches, and built-in defaults are not modeled"
	return result
}

// resolveZoneRef accepts a zone id or case-insensitive name.
func resolveZoneRef(ref string, zones []map[string]any) (string, error) {
	if ref == "" {
		return "", nil
	}
	names := make([]string, 0, len(zones))
	for _, zone := range zones {
		name := asString(zone["name"])
		names = append(names, name)
		if asString(zone["id"]) == ref || strings.EqualFold(name, ref) {
			return asString(zone["id"]), nil
		}
	}
	return "", usageErr(fmt.Errorf("zone %q is not in the mirror; known zones: %s", ref, strings.Join(names, ", ")))
}

func newNovelRulePredictCmd(flags *rootFlags) *cobra.Command {
	var dbFlag, siteFlag, srcZone, dstZone, srcIP, dstIP, srcMAC, protocol string
	var srcPort, dstPort int
	cmd := &cobra.Command{
		Use:   "rule-predict",
		Short: "Predict which firewall policy would match a hypothetical packet. This is a local prediction with no API confirmation.",
		Long: `Use this command to evaluate one hypothetical packet against the synced rules. Do NOT
use it to assess what a device, port or WiFi action would affect; use 'impact' instead.

Walks zone-based policies in index order and reports the first match, or indeterminate
when any policy on the way cannot be decided: domain, application, region and scheduled
filters always land there. No API simulates a packet, so the result is a prediction.`,
		Example: strings.Trim(`
  unifi-pp-cli rule-predict --src-zone LAN --dst-zone External --dst-port 443 --protocol tcp --agent
  unifi-pp-cli rule-predict --src-zone LAN --dst-zone External --src-ip 192.0.2.10 --dst-ip 192.0.2.20 --dst-port 22 --protocol tcp --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "--src-zone=LAN;--dst-zone=External;--dst-port=443;--protocol=tcp"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "rule-predict")
			}
			if srcZone == "" && dstZone == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--src-zone and --dst-zone are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath := mirrorDBPath(dbFlag)
			if mirrorMissing(dbPath) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: unifi-pp-cli sync --console <id> --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), predictResult{Verdict: "indeterminate", Reason: "no local mirror; run sync first", Prediction: true, AssumedConnection: "new", Notes: rulePredictNotes}, flags)
				}
				return nil
			}
			db, err := openMirror(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			console, err := resolveMirrorConsole(ctx, db)
			if err != nil {
				return err
			}
			rows, err := loadMirrorRows(ctx, db, console, rtFirewall, rtZones, rtACL, rtTML)
			if err != nil {
				return err
			}
			sites := map[string]bool{}
			for _, row := range rows {
				sites[row.Parent] = true
			}
			if siteFlag == "" && len(sites) > 1 {
				names := make([]string, 0, len(sites))
				for s := range sites {
					names = append(names, s)
				}
				sort.Strings(names)
				return usageErr(fmt.Errorf("the mirror holds %d sites for this console, choose one with --site: %s", len(sites), strings.Join(names, ", ")))
			}
			in := predictInputs{TMLs: map[string]map[string]any{}}
			for _, row := range rows {
				if siteFlag != "" && row.Parent != siteFlag {
					continue
				}
				switch row.Type {
				case rtFirewall:
					in.Policies = append(in.Policies, row.Data)
				case rtZones:
					in.Zones = append(in.Zones, row.Data)
				case rtACL:
					in.ACLRules = append(in.ACLRules, row.Data)
				case rtTML:
					in.TMLs[row.ID] = row.Data
				}
			}
			// Asking which zone was meant, from a blank list, would misdirect: the mirror is
			// empty. Same answer as a missing mirror, since it knows just as little.
			if len(in.Zones) == 0 {
				result := predictResult{Verdict: "indeterminate", Reason: noZonesReason(ctx, db, console), Prediction: true, AssumedConnection: "new", Notes: rulePredictNotes}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), result, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "verdict: %s\nreason: %s\n", result.Verdict, result.Reason)
				return nil
			}
			packet := predictPacket{SrcIP: srcIP, DstIP: dstIP, SrcMAC: srcMAC, SrcPort: srcPort, DstPort: dstPort, Protocol: protocol}
			if packet.SrcZone, err = resolveZoneRef(srcZone, in.Zones); err != nil {
				return err
			}
			if packet.DstZone, err = resolveZoneRef(dstZone, in.Zones); err != nil {
				return err
			}
			result := predictRule(packet, in)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "verdict: %s\nreason: %s\n", result.Verdict, result.Reason)
			if result.MatchedPolicy != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "policy: %s (index %d) action %s\n", result.MatchedPolicy.Name, result.MatchedPolicy.Index, result.MatchedPolicy.Action)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "This is a prediction; nothing here is confirmed by the gateway.")
			return nil
		},
	}
	cmd.Flags().StringVar(&dbFlag, "db", "", "Local mirror path (default: the standard mirror)")
	cmd.Flags().StringVar(&siteFlag, "site", "", "Network site id, required when the console has several sites")
	cmd.Flags().StringVar(&srcZone, "src-zone", "", "Source firewall zone, by name or id")
	cmd.Flags().StringVar(&dstZone, "dst-zone", "", "Destination firewall zone, by name or id")
	cmd.Flags().StringVar(&srcIP, "src-ip", "", "Source IP address")
	cmd.Flags().StringVar(&dstIP, "dst-ip", "", "Destination IP address")
	cmd.Flags().StringVar(&srcMAC, "src-mac", "", "Source MAC address, for MAC-matching policies")
	cmd.Flags().IntVar(&srcPort, "src-port", 0, "Source port")
	cmd.Flags().IntVar(&dstPort, "dst-port", 0, "Destination port")
	cmd.Flags().StringVar(&protocol, "protocol", "", "Protocol name, e.g. tcp, udp, icmp")
	return cmd
}

// noZonesReason separates "never synced" from "synced and there are no zones". On a
// legacy-firewall console the second is permanent, and telling that user to sync again
// sends them round a loop that can never produce an answer.
func noZonesReason(ctx context.Context, db *store.Store, console string) string {
	if snaps, err := db.ListSnapshots(ctx, console); err == nil && len(snaps) > 0 {
		return "the last sync returned no firewall zones, which usually means this console uses legacy firewall rules instead of zone-based firewall; legacy rules are outside this model, so no prediction is possible"
	}
	return "the mirror holds no firewall zones for this console and site; run: unifi-pp-cli sync"
}
