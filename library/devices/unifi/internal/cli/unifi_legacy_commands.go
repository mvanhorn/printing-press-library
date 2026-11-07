// Hand-authored, not generator output. A separate file so regen-merge keeps it.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// legacyCommands maps commands from the earlier public release to their current names.
// The reprint grouped API commands under their product, and scripts written against the
// old names should keep working. Every pair was matched on HTTP method and path, and has
// the same positional arguments.
var legacyCommands = map[string]string{
	"countries":                                   "network list-countries",
	"dpi get-application-categories":              "network list-dpi-categories",
	"dpi get-applications":                        "network list-dpi-applications",
	"info":                                        "network get-info",
	"pending-devices":                             "network list-pending-devices",
	"sites acl-rules create":                      "network create-acl-rule",
	"sites acl-rules delete":                      "network delete-acl-rule",
	"sites acl-rules get":                         "network get-acl-rule",
	"sites acl-rules get-ordering":                "network get-acl-rule-ordering",
	"sites acl-rules get-page":                    "network list-acl-rules",
	"sites acl-rules update":                      "network update-acl-rule",
	"sites acl-rules update-ordering":             "network set-acl-rule-ordering",
	"sites clients execute-connected-action":      "network run-client-action",
	"sites clients get-connected-details":         "network get-client",
	"sites clients get-connected-overview-page":   "network list-clients",
	"sites device-tags get-page":                  "network list-device-tags",
	"sites devices adopt":                         "network adopt-device",
	"sites devices execute-adopted-action":        "network run-device-action",
	"sites devices execute-port-action":           "network run-port-action",
	"sites devices get-adopted-details":           "network get-device",
	"sites devices get-adopted-latest-statistics": "network get-device-statistics",
	"sites devices get-adopted-overview-page":     "network list-devices",
	"sites devices remove":                        "network remove-device",
	"sites dns create-policy":                     "network create-dns-policy",
	"sites dns delete-policy":                     "network delete-dns-policy",
	"sites dns get-policy":                        "network get-dns-policy",
	"sites dns get-policy-page":                   "network list-dns-policies",
	"sites dns update-policy":                     "network update-dns-policy",
	"sites firewall create-policy":                "network create-firewall-policy",
	"sites firewall create-zone":                  "network create-firewall-zone",
	"sites firewall delete-policy":                "network delete-firewall-policy",
	"sites firewall delete-zone":                  "network delete-firewall-zone",
	"sites firewall get-policies":                 "network list-firewall-policies",
	"sites firewall get-policy":                   "network get-firewall-policy",
	"sites firewall get-policy-ordering":          "network get-firewall-policy-ordering",
	"sites firewall get-zone":                     "network get-firewall-zone",
	"sites firewall get-zones":                    "network list-firewall-zones",
	"sites firewall patch-policy":                 "network patch-firewall-policy",
	"sites firewall update-policy":                "network update-firewall-policy",
	"sites firewall update-policy-ordering":       "network set-firewall-policy-ordering",
	"sites firewall update-zone":                  "network update-firewall-zone",
	"sites hotspot create-vouchers":               "network create-hotspot-voucher",
	"sites hotspot delete-voucher":                "network delete-hotspot-voucher",
	"sites hotspot delete-vouchers":               "network delete-hotspot-vouchers",
	"sites hotspot get-voucher":                   "network get-hotspot-voucher",
	"sites hotspot get-vouchers":                  "network list-hotspot-vouchers",
	"sites networks create":                       "network create",
	"sites networks delete":                       "network delete",
	"sites networks get-details":                  "network get",
	"sites networks get-overview-page":            "network list",
	"sites networks get-references":               "network get-references",
	"sites networks update":                       "network update",
	"sites radius get-profile-overview-page":      "network list-radius-profiles",
	"sites switching get-lag":                     "network get-switching-lag",
	"sites switching get-lag-page":                "network list-switching-lags",
	"sites switching get-mc-lag-domain":           "network get-switching-mc-lag-domain",
	"sites switching get-mc-lag-domain-page":      "network list-switching-mc-lag-domains",
	"sites switching get-switch-stack":            "network get-switching-switch-stack",
	"sites switching get-switch-stack-page":       "network list-switching-switch-stacks",
	"sites traffic-matching-lists create":         "network create-traffic-matching-list",
	"sites traffic-matching-lists delete":         "network delete-traffic-matching-list",
	"sites traffic-matching-lists get":            "network list-traffic-matching-lists",
	"sites traffic-matching-lists get-sites":      "network get-traffic-matching-list",
	"sites traffic-matching-lists update":         "network update-traffic-matching-list",
	"sites vpn get-server-page":                   "network list-vpn-servers",
	"sites vpn get-site-to-site-tunnel-page":      "network list-vpn-site-to-site-tunnels",
	"sites wans get-overview-page":                "network list-wans",
	"sites wifi create-broadcast":                 "network create-wifi-broadcast",
	"sites wifi delete-broadcast":                 "network delete-wifi-broadcast",
	"sites wifi get-broadcast-details":            "network get-wifi-broadcast",
	"sites wifi get-broadcast-page":               "network list-wifi-broadcasts",
	"sites wifi update-broadcast":                 "network update-wifi-broadcast",
}

// removedCommands no longer exist because the features were dropped. The API does not
// report the PoE wattage port-audit needed, and guest report had no recurring use. They
// fail with the commands that cover the same ground rather than an unknown-command error.
var removedCommands = map[string]string{
	"port-audit":   "removed. Use 'network list-devices <siteId>' and 'network get-device <siteId> <deviceId>'",
	"guest report": "removed. Use 'network list-hotspot-vouchers <siteId>' and 'network list-clients <siteId>'",
}

const (
	legacyAliasAnnotation   = "unifi:legacy-target"
	legacyRemovedAnnotation = "unifi:legacy-removed"
)

// legacyInvocation checks argv against a throwaway command tree that also holds the old
// names, so cobra's own parser can tell a command from a flag value such as "--console info".
// The real tree never contains them, which keeps them out of help, MCP tools and dogfood,
// all of which walk it and would otherwise treat each alias as a command to exercise.
func legacyInvocation(args []string) ([]string, string, error) {
	var flags rootFlags
	probe := newRootCmd(&flags)
	addLegacyCommands(probe)
	if cmd, _, err := probe.Find(args); err == nil && cmd != nil {
		if why := cmd.Annotations[legacyRemovedAnnotation]; why != "" {
			return nil, "", fmt.Errorf("%s: %s", strings.TrimPrefix(cmd.CommandPath(), probe.Name()+" "), why)
		}
	}
	rewritten, note := rewriteLegacyArgs(probe, args)
	return rewritten, note, nil
}

func addLegacyCommands(root *cobra.Command) {
	for path, target := range legacyCommands {
		legacyCommandAt(root, path).Annotations = map[string]string{legacyAliasAnnotation: target}
	}
	for path, why := range removedCommands {
		legacyCommandAt(root, path).Annotations = map[string]string{legacyRemovedAnnotation: why}
	}
}

// legacyCommandAt walks to a path, reusing real commands like "sites" and creating hidden
// groups for the rest. Old groups such as "sites clients" point at "network".
func legacyCommandAt(root *cobra.Command, path string) *cobra.Command {
	parent := root
	for _, name := range strings.Fields(path) {
		var next *cobra.Command
		for _, child := range parent.Commands() {
			if child.Name() == name {
				next = child
				break
			}
		}
		if next == nil {
			next = &cobra.Command{Use: name, Hidden: true, Annotations: map[string]string{legacyAliasAnnotation: "network"}}
			next.DisableFlagParsing = true
			parent.AddCommand(next)
		}
		parent = next
	}
	return parent
}

// rewriteLegacyArgs swaps an old command name for its current one and leaves everything
// else in place, so flags on either side and positional arguments carry over unchanged.
func rewriteLegacyArgs(root *cobra.Command, args []string) ([]string, string) {
	cmd, _, err := root.Find(args)
	if err != nil || cmd == nil {
		return args, ""
	}
	target := cmd.Annotations[legacyAliasAnnotation]
	if target == "" {
		return args, ""
	}
	old := strings.Fields(strings.TrimPrefix(cmd.CommandPath(), root.Name()+" "))
	for start := 0; start+len(old) <= len(args); start++ {
		if strings.Join(args[start:start+len(old)], " ") != strings.Join(old, " ") {
			continue
		}
		// A global flag's value can equal an old name, and only the real position counts.
		if start > 0 && strings.HasPrefix(args[start-1], "-") && !strings.Contains(args[start-1], "=") && flagTakesValue(root, args[start-1]) {
			continue
		}
		rewritten := append(append(append([]string{}, args[:start]...), strings.Fields(target)...), args[start+len(old):]...)
		return rewritten, fmt.Sprintf("note: '%s' has moved to '%s'\n", strings.Join(old, " "), target)
	}
	return args, ""
}

func flagTakesValue(root *cobra.Command, token string) bool {
	flag := root.PersistentFlags().Lookup(strings.TrimLeft(token, "-"))
	return flag != nil && flag.NoOptDefVal == ""
}
