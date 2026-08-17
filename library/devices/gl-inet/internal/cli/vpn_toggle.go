// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelVpnToggleCmd(flags *rootFlags) *cobra.Command {
	var off bool
	cmd := &cobra.Command{
		Use:         "toggle <name>",
		Short:       "Start a VPN tunnel, arm the kill-switch, and confirm your public IP actually changed.",
		Long:        "Find a configured WireGuard or OpenVPN tunnel by name and start it (or stop it with --off), then confirm the host's public egress IP changed. If it did not change after starting, exits non-zero (possible leak).",
		Example:     "  gl-inet-pp-cli vpn toggle mullvad --agent",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,7"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := args[0]
			out := cmd.OutOrStdout()
			action := "start"
			if off {
				action = "stop"
			}

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would %s VPN tunnel %q and verify egress IP\n", action, name)
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			module, startArgs, found := findTunnel(ctx, c, name)
			if !found {
				return notFoundErr(fmt.Errorf("no VPN tunnel named %q in wg-client or ovpn-client config", name))
			}

			if dryRunOK(flags) {
				b, _ := json.Marshal(startArgs)
				fmt.Fprintf(out, "dry-run: %s.%s %s\n", module, action, string(b))
				return nil
			}

			result := map[string]any{"name": name, "module": module, "action": action}

			ipBefore := fetchPublicIP(ctx)
			result["egress_before"] = ipBefore

			if _, err := c.Call(ctx, module, action, startArgs); err != nil {
				return classifyGLError(err, flags)
			}
			// Give the tunnel a moment to come up before re-checking egress.
			if action == "start" {
				select {
				case <-ctx.Done():
				case <-time.After(2 * time.Second):
				}
			}
			ipAfter := fetchPublicIP(ctx)
			result["egress_after"] = ipAfter

			raw, _ := json.Marshal(result)
			if perr := printOutputWithFlags(out, raw, flags); perr != nil {
				return perr
			}

			// Leak gate: starting a tunnel that does not change the egress IP is
			// a likely misconfiguration or leak.
			if action == "start" && ipBefore != "" && ipAfter != "" && ipBefore == ipAfter {
				return &cliError{code: 7, err: fmt.Errorf("egress IP did not change after starting %q (was %s, still %s); possible VPN leak or tunnel failed to route", name, ipBefore, ipAfter)}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&off, "off", false, "Stop the tunnel instead of starting it")
	return cmd
}

// findTunnel locates a tunnel by name across wg-client and ovpn-client config
// lists, matching group_name (WireGuard) or username (OpenVPN). Returns the
// module and the start/stop argument map derived from the matched entry.
func findTunnel(ctx context.Context, c *client.Client, name string) (module string, startArgs map[string]any, found bool) {
	// Resolving the tunnel is a read; run it for real even under --dry-run so
	// the preview can show the exact start/stop args. Only the mutating
	// start/stop call below stays gated by dry-run.
	savedDry := c.DryRun
	c.DryRun = false
	defer func() { c.DryRun = savedDry }()
	for _, m := range []struct {
		module  string
		nameKey string
	}{
		{"wg-client", "group_name"},
		{"ovpn-client", "username"},
	} {
		data, err := c.Call(ctx, m.module, "get_all_config_list", nil)
		if err != nil {
			continue
		}
		var list []map[string]any
		if json.Unmarshal(jsonArrayField(data, "config_list"), &list) != nil {
			continue
		}
		for _, entry := range list {
			label, _ := entry[m.nameKey].(string)
			if !strings.EqualFold(strings.TrimSpace(label), strings.TrimSpace(name)) {
				continue
			}
			return m.module, tunnelStartArgs(entry), true
		}
	}
	return "", nil, false
}

// tunnelStartArgs extracts the identifying fields a start/stop call needs.
func tunnelStartArgs(entry map[string]any) map[string]any {
	args := map[string]any{}
	for _, k := range []string{"group_id", "peer_id", "id", "name", "group_name", "username"} {
		if v, ok := entry[k]; ok {
			args[k] = v
		}
	}
	// WireGuard peers carry their own peer id.
	if peers, ok := entry["peers"].([]any); ok && len(peers) > 0 {
		if p0, ok := peers[0].(map[string]any); ok {
			for _, k := range []string{"peer_id", "id"} {
				if v, ok := p0[k]; ok {
					args["peer_id"] = v
					break
				}
			}
		}
	}
	return args
}

// fetchPublicIP returns the host's public egress IP from a public echo service,
// or "" on any failure. Bounded by a short timeout so it never hangs a toggle.
func fetchPublicIP(ctx context.Context) string {
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, "https://api.ipify.org?format=json", nil)
	if err != nil {
		return ""
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var body struct {
		IP string `json:"ip"`
	}
	if json.NewDecoder(resp.Body).Decode(&body) != nil {
		return ""
	}
	return body.IP
}
