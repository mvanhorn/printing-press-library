// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output: flags and hooks for the dual transport,
// wired through the novel hook so root.go stays as generated.

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/config"
	"github.com/mvanhorn/printing-press-library/library/devices/unifi/internal/store"
	"github.com/spf13/cobra"
)

// Package-level because rootFlags is generated and the durable-extension rule
// forbids adding fields to it.
var unifiTarget client.TargetOverride

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.PersistentFlags().StringVar(&unifiTarget.ConsoleID, "console", "", "Console (host) id to target through the cloud connector (env: UNIFI_CONSOLE_ID)")
		root.PersistentFlags().StringVar(&unifiTarget.Gateway, "gateway", "", "Gateway address for direct local access, e.g. 192.168.1.1 (env: UNIFI_GATEWAY)")
		root.PersistentFlags().StringVar(&unifiTarget.Transport, "transport", "", "Force a route when both are configured: auto, cloud or local (env: UNIFI_TRANSPORT)")
		// The walker has no generic "not for MCP" list, and its root-flag list predates
		// these flags. mcp:read-flags is the available lever: it drops a persistent
		// flag from every mirrored schema and rejects it as an argument. Its name says
		// file path, but the effect is what is wanted, and refuseMCPChildArguments
		// holds the same line in the child if a future walker ignores this.
		if root.Annotations == nil {
			root.Annotations = map[string]string{}
		}
		root.Annotations["mcp:read-flags"] = strings.TrimLeft(root.Annotations["mcp:read-flags"]+","+strings.Join(mcpRefusedRouteFlags, ","), ",")
		generated := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if err := refuseMCPChildArguments(os.Args[1:], flags); err != nil {
				return usageErr(err)
			}
			// Local readers build no URL, so the transport's check never sees their
			// --console, and they would show a fenced console's mirror rows.
			if named := strings.TrimSpace(unifiTarget.ConsoleID); named != "" {
				if err := client.ConsoleAllowed(named); err != nil {
					return configErr(err)
				}
			}
			// Scope before the generated hook: it may open the store, and the
			// first read or write must already see the right console.
			scopeMirrorToConfiguredConsole(flags)
			return generated(cmd, args)
		}
	})
	// A local-only operator has a working credential that the generated auth checks
	// cannot see, because they only know about the cloud key.
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		doctorAuthConfiguredHook = func() (bool, string) {
			cfg, err := config.Load(flags.configPath)
			if err != nil || cfg.UniFiTarget().LocalAPIKey == "" {
				return false, ""
			}
			return true, "local gateway key (no cloud key: Site Manager commands will refuse)"
		}
	})
	registerClientHook(func(c *client.Client) error {
		applyUniFiTargetOverride(c)
		c.SetConsoleResolvedHook(store.SetActiveConsole)
		c.SetMutationSuccessHook(cleanupMirrorAfterDelete)
		return nil
	})
	classifyHTMLPayload = classifyConsolePage
}

func applyUniFiTargetOverride(c *client.Client) { c.SetTargetOverride(unifiTarget) }

// mcpRefusedRouteFlags decide where a key is sent, so they belong to whoever configured
// the server. Tool arguments are attacker-influenced: --gateway would name the host that
// receives UNIFI_LOCAL_API_KEY and step around UNIFI_CONSOLE_ALLOWLIST. --console stays
// available because the allowlist is checked against it and the host stays fixed.
var mcpRefusedRouteFlags = []string{"gateway", "transport"}

// refuseMCPChildArguments reads argv because parsing erases the difference that matters.
// The mirror renders only a boolean true as bare --confirm; "true", 1 and ["x"] all become
// --confirm=<value>, which pflag accepts as true.
func refuseMCPChildArguments(argv []string, flags *rootFlags) error {
	if !cliutil.IsMCPChildEnv() {
		return nil
	}
	if unifiTarget.Gateway != "" || unifiTarget.Transport != "" {
		return fmt.Errorf("--%s can only be set by the operator through the environment or config, not by an MCP caller", strings.Join(mcpRefusedRouteFlags, " and --"))
	}
	if named := strings.TrimSpace(unifiTarget.ConsoleID); named != "" {
		// The environment is read directly as well, so a broken config cannot lift the pin.
		pinned := strings.TrimSpace(cliutil.EnvOverride("UNIFI_CONSOLE_ID"))
		if cfg, err := config.Load(flags.configPath); err == nil {
			target := cfg.UniFiTarget()
			pinned = target.ConsoleID
			// On a configured gateway route --console picks no host; it is only a label on
			// mirror rows and journal entries. A caller who could change it could file this
			// gateway's data under another console and satisfy undo's console check.
			if target.Gateway != "" && !strings.EqualFold(target.Transport, "cloud") && named != target.ConsoleID {
				return errors.New("--console cannot relabel the gateway this server is configured to use")
			}
		}
		// An organization key reaches every console, so the console an operator named is
		// the only statement of what this server is for. Going further takes the operator
		// listing it, never a tool argument.
		if pinned != "" && named != pinned && !client.ConsoleExplicitlyAllowed(named) {
			return fmt.Errorf("console %q refused: this MCP server is pinned to %q by UNIFI_CONSOLE_ID. An operator can allow it by adding it to UNIFI_CONSOLE_ALLOWLIST", named, pinned)
		}
	}
	for _, arg := range argv {
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--confirm=") {
			return errors.New("confirm must be boolean true; nothing was sent")
		}
	}
	return nil
}

// scopeMirrorToConfiguredConsole handles commands that never build a client (the
// local-only readers). An auto-selected console arrives later through the hook.
func scopeMirrorToConfiguredConsole(flags *rootFlags) {
	// A broken config must not cost the flags: with no scope, local readers show every
	// console in the mirror, fenced ones included.
	var target config.UniFiTarget
	if cfg, err := config.Load(flags.configPath); err == nil {
		target = cfg.UniFiTarget()
	}
	if v := strings.TrimSpace(unifiTarget.ConsoleID); v != "" {
		target.ConsoleID = v
	}
	if v := strings.TrimSpace(unifiTarget.Gateway); v != "" {
		target.Gateway = v
	}
	if v := strings.TrimSpace(unifiTarget.Transport); v != "" {
		target.Transport = v
	}
	switch {
	case target.ConsoleID != "":
		store.SetActiveConsole(target.ConsoleID)
	// Under --transport cloud the gateway is not the route, so its rows are not this
	// command's rows; the auto-selected console arrives through the client hook instead.
	case target.Gateway != "" && !strings.EqualFold(target.Transport, "cloud"):
		if key := client.GatewayMirrorKey(target.Gateway); key != "" {
			store.SetActiveConsole(key)
		}
	}
}

var consolePageMarkers = [][]byte{
	[]byte("UniFi OS"), []byte("unifi-portal"), []byte("UniFi Network"), []byte("/manage/account/login"), []byte("ubnt"),
}

// classifyConsolePage: a console answers paths it does not serve with its own
// login page rather than a JSON 404. Reporting that as an auth failure (exit 4)
// sends people to rotate a key that was fine and tells scripts to re-authenticate.
func classifyConsolePage(trimmed []byte) error {
	for _, marker := range consolePageMarkers {
		if bytes.Contains(trimmed, marker) {
			return apiErr(fmt.Errorf("the console answered with its own HTML page instead of JSON, so the request did not reach the API. The key may be fine. Check the path and the route with 'unifi-pp-cli doctor', and run 'unifi-pp-cli reach' to see which connector path shape this console accepts"))
		}
	}
	return nil
}

// Single-id DELETE endpoints whose rows are mirrored. Bulk voucher delete is
// absent on purpose: its filter is opaque server-side, so the removed ids are
// unknown and clearing the site's rows would show live vouchers as gone.
var mirroredDeletePatterns = []struct {
	pattern      *regexp.Regexp
	resourceType string
}{
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/acl-rules/([^/]+)$`), "network_integration_v1_sites_acl_rules"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/dns/policies/([^/]+)$`), "network_integration_v1_sites_dns_policies"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/firewall/policies/([^/]+)$`), "network_integration_v1_sites_firewall_policies"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/firewall/zones/([^/]+)$`), "network_integration_v1_sites_firewall_zones"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/hotspot/vouchers/([^/]+)$`), "network_integration_v1_sites_hotspot_vouchers"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/networks/([^/]+)$`), "network_integration_v1_sites_networks"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/traffic-matching-lists/([^/]+)$`), "network_integration_v1_sites_traffic_matching_lists"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/wifi/broadcasts/([^/]+)$`), "network_integration_v1_sites_wifi_broadcasts"},
	{regexp.MustCompile(`^/network/integration/v1/sites/([^/]+)/devices/([^/]+)$`), "network_integration_v1_sites_devices"},
}

// cleanupMirrorAfterDelete keeps drift, search, topology and rule-predict from
// showing a deleted object until the next sync. Best effort: the live delete
// already succeeded and a mirror problem must not turn that into a failure.
func cleanupMirrorAfterDelete(method, path string) {
	if method != http.MethodDelete {
		return
	}
	for _, candidate := range mirroredDeletePatterns {
		match := candidate.pattern.FindStringSubmatch(path)
		if match == nil {
			continue
		}
		dbPath := defaultDBPath("unifi-pp-cli")
		if _, err := os.Stat(dbPath); err != nil {
			return
		}
		db, err := store.OpenWithContext(context.Background(), dbPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: local mirror cleanup skipped: %v\n", err)
			return
		}
		defer db.Close()
		if _, err := db.DeleteResource(candidate.resourceType, match[2], match[1]); err != nil {
			fmt.Fprintf(os.Stderr, "warning: local mirror cleanup failed for %s %s: %v\n", candidate.resourceType, match[2], err)
		}
		return
	}
}
