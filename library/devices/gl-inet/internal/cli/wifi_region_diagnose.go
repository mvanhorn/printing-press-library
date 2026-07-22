// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelWifiRegionDiagnoseCmd(flags *rootFlags) *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:         "diagnose",
		Short:       "Find out why a foreign network is invisible and fix the regulatory domain in one command.",
		Long:        "Scan nearby APs and flag any on a 2.4GHz channel (e.g. 12/13) that the current regulatory domain forbids, then name a country that would permit it. --fix sets the region.",
		Example:     "  gl-inet-pp-cli wifi region diagnose\n  gl-inet-pp-cli wifi region diagnose --fix --yes",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if cliutil.IsVerifyEnv() {
				raw, _ := json.Marshal(map[string]any{"status": "noop", "reason": "verify_short_circuit"})
				return printOutputWithFlags(out, raw, flags)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(out, "dry-run: would scan each radio for nearby APs and check channels against the current regdomain")
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			info, _, _ := fetchSystemInfo(ctx, c)
			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			wireless, err := glssh.UCIShow(ctx, cfg, "wireless")
			if err != nil {
				return classifyGLError(err, flags)
			}
			wmap := parseUCIShow(wireless)
			devices := extractWifiDevices(wmap)
			region := extractRadioCountries(wmap)

			country := info.CountryCode
			if country == "" {
				for _, v := range region {
					if v != "" {
						country = v
						break
					}
				}
			}

			// Scan each radio (bounded under dogfood) and flag disallowed 2.4GHz APs.
			scanDevices := devices
			if cliutil.IsDogfoodEnv() && len(scanDevices) > 1 {
				scanDevices = scanDevices[:1]
			}
			type blockedAP struct {
				SSID       string `json:"ssid"`
				Channel    int    `json:"channel"`
				Radio      string `json:"radio"`
				SuggestCC  string `json:"suggest_country"`
				SuggestMsg string `json:"suggestion"`
			}
			var blocked []blockedAP
			maxBlockedCh := 0
			for _, dev := range scanDevices {
				scanOut, serr := glssh.Run(ctx, cfg, "iwinfo "+shellQuoteSingle(dev)+" scan 2>/dev/null")
				if serr != nil {
					continue
				}
				for _, ap := range parseIwinfoScan(scanOut) {
					if ap.Channel < 1 || ap.Channel > 14 {
						continue // only the 2.4GHz regdomain case
					}
					if channel24Allowed(country, ap.Channel) {
						continue
					}
					cc := countryAllowing24Channel(ap.Channel)
					blocked = append(blocked, blockedAP{
						SSID: ap.SSID, Channel: ap.Channel, Radio: dev, SuggestCC: cc,
						SuggestMsg: fmt.Sprintf("channel %d is disallowed under region %q; set region to a country that allows it, e.g. %s", ap.Channel, country, cc),
					})
					if ap.Channel > maxBlockedCh {
						maxBlockedCh = ap.Channel
					}
				}
			}

			result := map[string]any{
				"current_region": country,
				"radios":         region,
				"blocked_aps":    blocked,
				"blocked_count":  len(blocked),
				"note_5ghz":      "5GHz channel availability (especially DFS channels 52-144) also varies by regulatory domain; this check focuses on the common 2.4GHz ch12/13 case.",
			}

			if fix && len(blocked) > 0 {
				target := countryAllowing24Channel(maxBlockedCh)
				if cliutil.IsVerifyEnv() {
					result["fix"] = "would set region to " + target
				} else if isTerminal(out) && !flags.yes && !flags.agent && !flags.noInput {
					result["fix"] = map[string]any{
						"status": "needs_confirmation", "country": target,
						"hint": "re-run with --fix --yes to apply",
					}
				} else if len(devices) > 0 {
					script := buildRegionSetScript(devices, target)
					if _, err := glssh.Run(ctx, cfg, script); err != nil {
						return classifyGLError(err, flags)
					}
					result["fix"] = map[string]any{"status": "applied", "country": target, "radios": devices}
				}
			}

			raw, _ := json.Marshal(result)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "Set the regulatory domain to a country that permits the blocked channels")
	return cmd
}
