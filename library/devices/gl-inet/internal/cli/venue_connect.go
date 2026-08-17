// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelVenueConnectCmd(flags *rootFlags) *cobra.Command {
	var key string
	cmd := &cobra.Command{
		Use:         "connect <ssid>",
		Short:       "One command to get online at a new venue: scan, region-check, join, and prep for captive portals.",
		Long:        "Scan for the target SSID, warn if its channel is disallowed by the current regulatory domain, join it in repeater mode, then verify the link.",
		Example:     "  gl-inet-pp-cli venue connect 'Hotel WiFi' --key roompass --agent",
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ssid := args[0]
			out := cmd.OutOrStdout()

			if cliutil.IsVerifyEnv() {
				fmt.Fprintf(out, "would scan, region-check, and connect to SSID %q in repeater mode\n", ssid)
				return nil
			}
			connectArgs := map[string]any{"ssid": ssid}
			if key != "" {
				connectArgs["key"] = key
			}
			if dryRunOK(flags) {
				b, _ := json.Marshal(connectArgs)
				fmt.Fprintf(out, "dry-run: repeater.scan; repeater.connect %s; repeater.get_status\n", string(b))
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			result := map[string]any{"ssid": ssid}

			// Scan + region pre-check (best effort).
			info, _, _ := fetchSystemInfo(ctx, c)
			country := info.CountryCode
			if scan, serr := c.Call(ctx, "repeater", "scan", nil); serr == nil {
				if ch, ok := findAPChannel(scan, ssid); ok {
					result["channel"] = ch
					if ch >= 1 && ch <= 14 && country != "" && !channel24Allowed(country, ch) {
						result["region_warning"] = fmt.Sprintf("SSID %q is on channel %d, disallowed by region %q; run 'gl-inet-pp-cli wifi region set %s' first",
							ssid, ch, country, countryAllowing24Channel(ch))
					}
				} else {
					result["scan_note"] = "target SSID not found in scan results; attempting connect anyway"
				}
			}

			// Join.
			connRes, err := c.Call(ctx, "repeater", "connect", connectArgs)
			if err != nil {
				return classifyGLError(err, flags)
			}
			result["connect"] = json.RawMessage(connRes)

			// Verify.
			if status, serr := c.Call(ctx, "repeater", "get_status", nil); serr == nil {
				result["status"] = json.RawMessage(status)
			}

			result["captive_portal_note"] = strings.TrimSpace(
				"MAC cloning is unavailable on firmware 4.8.1 (the macclone RPC returns 'Method not found'). " +
					"If a captive portal blocks access, open http://192.168.8.1 or the venue's portal URL in a browser to finish sign-in.")

			raw, _ := json.Marshal(result)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "Pre-shared key/password for the target network")
	return cmd
}
