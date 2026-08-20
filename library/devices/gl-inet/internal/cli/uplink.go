// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

const uplinkGatherCmd = `for i in $(iwinfo 2>/dev/null | awk '/ESSID/{print $1}'); do echo "##IFACE $i"; iwinfo "$i" info 2>/dev/null; done; echo "##PING"; ping -c2 -W2 1.1.1.1 2>&1`

// pp:data-source live
func newNovelUplinkCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "uplink",
		Short:       "Diagnose a weak/slow venue WiFi uplink and get ranked ways to improve it.",
		Long:        "Measure the repeater STA link locally (RSSI, bitrate, channel, congestion, latency) and rank concrete improvements. Local metrics only — no external speedtest.",
		Example:     "  gl-inet-pp-cli uplink --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			if cliutil.IsVerifyEnv() {
				raw, _ := json.Marshal(map[string]any{"status": "noop", "reason": "verify_short_circuit"})
				return printOutputWithFlags(out, raw, flags)
			}
			if dryRunOK(flags) {
				return nil
			}

			c, err := glClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			mode := ""
			if data, err := c.Call(ctx, "netmode", "get_mode", nil); err == nil {
				if m := jsonObjField(data, "mode"); m != nil {
					_ = json.Unmarshal(m, &mode)
				}
			}
			if mode != "" && mode != "repeater" {
				raw, _ := json.Marshal(map[string]any{
					"mode": mode,
					"note": fmt.Sprintf("uplink diagnostics apply to a WiFi (repeater) uplink; current mode is %q", mode),
				})
				return printOutputWithFlags(out, raw, flags)
			}

			cfg, err := glSSHConfig(c)
			if err != nil {
				return classifyGLError(err, flags)
			}
			gather, err := glssh.Run(ctx, cfg, uplinkGatherCmd)
			if err != nil {
				return classifyGLError(err, flags)
			}
			ifaces := parseIfaceBlocks(gather)
			staName, sta := pickStaInterface(ifaces)

			result := map[string]any{
				"mode":      mode,
				"interface": staName,
				"metrics": map[string]any{
					"essid":         sta.ESSID,
					"channel":       sta.Channel,
					"signal_dbm":    sta.Signal,
					"bitrate_mbits": sta.BitRate,
					"band":          bandName(sta.Channel),
				},
			}

			// Latency from the gather's ping block.
			if i := strings.Index(gather, "##PING"); i >= 0 {
				pingOut := gather[i:]
				if ms, ok := parsePingLatency(pingOut); ok {
					result["latency_ms"] = ms
				}
				result["egress_reachable"] = pingReachable(pingOut)
			}

			// Channel congestion via a scan (bounded under dogfood).
			congestion := map[int]int{}
			if staName != "" && !cliutil.IsDogfoodEnv() {
				if scanOut, serr := glssh.Run(ctx, cfg, "iwinfo "+shellQuoteSingle(staName)+" scan 2>/dev/null"); serr == nil {
					congestion = channelCounts(parseIwinfoScan(scanOut))
				}
			}
			result["channel_congestion"] = congestion

			result["suggestions"] = uplinkSuggestions(sta, congestion, has5GHzRadio(ifaces), result)

			raw, _ := json.Marshal(result)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	return cmd
}

func bandName(channel int) string {
	switch {
	case channel == 0:
		return ""
	case channel <= 14:
		return "2.4GHz"
	default:
		return "5GHz"
	}
}

// uplinkSuggestions ranks concrete improvements from the measured metrics.
func uplinkSuggestions(sta iwinfoInfo, congestion map[int]int, have5GHz bool, result map[string]any) []string {
	var sugg []string
	if sta.Signal != 0 && sta.Signal <= -75 {
		sugg = append(sugg, fmt.Sprintf("weak signal (%d dBm): reposition the router closer to the source AP or remove obstructions", sta.Signal))
	}
	if sta.Channel >= 1 && sta.Channel <= 14 && have5GHz {
		sugg = append(sugg, "uplink is on 2.4GHz while a 5GHz radio is present: if the AP also broadcasts 5GHz, joining that band is usually faster")
	}
	if n := congestion[sta.Channel]; n > 3 {
		sugg = append(sugg, fmt.Sprintf("channel %d is congested (%d nearby APs): the venue AP, if movable, would benefit from a clearer channel", sta.Channel, n))
	}
	if ms, ok := result["latency_ms"].(float64); ok && ms > 100 {
		sugg = append(sugg, fmt.Sprintf("high latency (%.0f ms): consider USB tethering as an alternative uplink (gl-inet-pp-cli wan mode tethering)", ms))
	}
	if len(sugg) == 0 {
		sugg = append(sugg, "uplink metrics look healthy; no action needed")
	}
	return sugg
}
