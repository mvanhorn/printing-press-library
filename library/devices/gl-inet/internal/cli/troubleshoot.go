// Copyright 2026 Paul Bockewitz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"

	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/devices/gl-inet/internal/glssh"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelTroubleshootCmd(flags *rootFlags) *cobra.Command {
	var fix bool
	cmd := &cobra.Command{
		Use:         "troubleshoot",
		Short:       "Diagnose why the router has no internet and get the exact fix command.",
		Long:        "Walk a decision tree over the router's mode, status, and uplink to name the most likely cause of no-internet and the exact command to fix it.",
		Example:     "  gl-inet-pp-cli troubleshoot --agent",
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

			diag := map[string]any{}

			mode := ""
			if data, err := c.Call(ctx, "netmode", "get_mode", nil); err == nil {
				if m := jsonObjField(data, "mode"); m != nil {
					_ = json.Unmarshal(m, &mode)
				}
			}
			diag["mode"] = mode
			if status, err := c.Call(ctx, "system", "get_status", nil); err == nil {
				diag["status"] = json.RawMessage(status)
			}

			cause := ""
			fixCmd := ""

			// Repeater mode: the most common travel failure is "up but not joined".
			if mode == "repeater" {
				if rstatus, err := c.Call(ctx, "repeater", "get_status", nil); err == nil {
					diag["repeater_status"] = json.RawMessage(rstatus)
					connected, ssid := detectRepeaterConnected(rstatus)
					if !connected {
						cause = "router is up but not joined to any WiFi in repeater mode"
						fixCmd = "gl-inet-pp-cli venue connect <ssid>"
						diag["joined_ssid"] = ssid
					}
				}
			}

			// Tethering mode: surface tethering status.
			if cause == "" && mode == "tethering" {
				if tstatus, err := c.Call(ctx, "tethering", "get_status", nil); err == nil {
					diag["tethering_status"] = json.RawMessage(tstatus)
				}
			}

			// DNS/internet egress reachability check.
			if cause == "" {
				reachable, detail := checkEgress(ctx, c, flags)
				diag["egress_check"] = detail
				if !reachable {
					cause = "no internet egress: cannot reach 1.1.1.1 from the router"
					switch mode {
					case "repeater":
						fixCmd = "gl-inet-pp-cli venue connect <ssid>"
					case "tethering":
						fixCmd = "check the tethered device's data connection; gl-inet-pp-cli system status"
					default:
						fixCmd = "check the WAN cable, then: gl-inet-pp-cli wan mode ethernet"
					}
				}
			}

			if cause == "" {
				cause = "no obvious fault detected; the router appears to have connectivity"
			}
			diag["likely_cause"] = cause
			if fixCmd != "" {
				diag["fix_command"] = fixCmd
			}
			if fix {
				diag["fix"] = "troubleshoot --fix performs only safe, non-destructive checks; run the suggested fix_command to remediate"
			}

			raw, _ := json.Marshal(diag)
			return printOutputWithFlags(out, raw, flags)
		},
	}
	cmd.Flags().BoolVar(&fix, "fix", false, "Apply only safe remedies (informational; prints the fix command to run)")
	return cmd
}

// checkEgress tests internet reachability, preferring the GL diag.ping RPC and
// falling back to an SSH ping. Returns (reachable, detail).
func checkEgress(ctx context.Context, c *client.Client, flags *rootFlags) (bool, map[string]any) {
	detail := map[string]any{"target": "1.1.1.1"}
	if data, err := c.Call(ctx, "diag", "ping", map[string]any{"addr": "1.1.1.1"}); err == nil {
		detail["method"] = "rpc"
		detail["raw"] = json.RawMessage(data)
		// Best-effort interpretation of the RPC result.
		var m map[string]any
		if json.Unmarshal(data, &m) == nil {
			if s, ok := m["ping"].(string); ok && pingReachable(s) {
				return true, detail
			}
			if loss, ok := m["loss"].(float64); ok {
				return loss < 100, detail
			}
		}
	}
	// SSH fallback.
	cfg, err := glSSHConfig(c)
	if err != nil {
		detail["method"] = "unavailable"
		detail["error"] = err.Error()
		return false, detail
	}
	pout, perr := glssh.Run(ctx, cfg, "ping -c1 -W2 1.1.1.1 2>&1")
	detail["method"] = "ssh"
	if perr != nil {
		detail["error"] = perr.Error()
	}
	reachable := pingReachable(pout)
	detail["reachable"] = reachable
	if ms, ok := parsePingLatency(pout); ok {
		detail["latency_ms"] = ms
	}
	return reachable, detail
}
