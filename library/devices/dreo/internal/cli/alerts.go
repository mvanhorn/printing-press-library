// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newAlertsCmd(rflags *rootFlags) *cobra.Command {
	var (
		pm25Above    float64
		filterBelow  int
		offlineAfter time.Duration
	)
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Report devices breaching health thresholds (filter life, water tank, offline, sensor)",
		Long: `Join the cached device catalog with the latest device_state and surface
devices crossing any of these thresholds:

  --pm25-above N    Surface devices reporting pm25 above N µg/m³.
  --filter-below N  Surface devices reporting filterLife percent below N (default 10).
  --offline-after D Surface devices last seen more than D ago (default 5m).

Also surfaces water-tank-empty (humidifier "wrong" == "Empty").

Acts entirely on cached data. To refresh the state snapshots that this
command reads, run 'dreo-pp-cli sensors --live' first — that path
iterates every device, calls /api/user-device/device/state, and writes
the flattened result to the local store. ('dreo-pp-cli devices state'
prints the raw API response but does NOT update the cache.)`,
		Example: `  dreo-pp-cli alerts
  dreo-pp-cli alerts --pm25-above 35 --filter-below 20 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
			defer cancel()

			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()
			devs, err := st.ListDevices(ctx)
			if err != nil {
				return err
			}
			alerts := []map[string]any{}
			for _, d := range devs {
				raw, fetchedAt, serr := st.GetDeviceState(ctx, d.Sn)
				var state map[string]any
				if serr == nil {
					_ = json.Unmarshal(raw, &state)
				}
				// Dreo's device list endpoint doesn't return `online`; the
				// authoritative connectivity signal lives in mixed.connected
				// from the state endpoint (flattened into the state map by
				// flattenState). d.Online stays a "have I ever seen this
				// device?" hint; "is it reachable right now?" comes from
				// state.connected. Only treat the device as offline when we
				// have a state snapshot AND it says disconnected — a missing
				// snapshot is "unknown" (covered by the stale_state alert
				// below), not "offline".
				if state != nil {
					if connected, ok := state["connected"].(bool); ok && !connected {
						alerts = append(alerts, map[string]any{
							"sn":    d.Sn,
							"name":  d.Name,
							"room":  d.Room,
							"kind":  "offline",
							"value": false,
						})
					}
				}
				if state != nil {
					if pm25, ok := asFloat(state, "pm25"); ok && pm25Above > 0 && pm25 > pm25Above {
						alerts = append(alerts, map[string]any{
							"sn":        d.Sn,
							"name":      d.Name,
							"room":      d.Room,
							"kind":      "pm25_high",
							"value":     pm25,
							"threshold": pm25Above,
						})
					}
					if fl, ok := asFloat(state, "filterLife", "filterlife"); ok && fl < float64(filterBelow) {
						alerts = append(alerts, map[string]any{
							"sn":        d.Sn,
							"name":      d.Name,
							"room":      d.Room,
							"kind":      "filter_low",
							"value":     fl,
							"threshold": filterBelow,
						})
					}
					if wrong, ok := state["wrong"].(string); ok && strings.EqualFold(wrong, "Empty") {
						alerts = append(alerts, map[string]any{
							"sn":   d.Sn,
							"name": d.Name,
							"room": d.Room,
							"kind": "water_tank_empty",
						})
					}
				}
				if !fetchedAt.IsZero() && offlineAfter > 0 && time.Since(fetchedAt) > offlineAfter {
					alerts = append(alerts, map[string]any{
						"sn":           d.Sn,
						"name":         d.Name,
						"room":         d.Room,
						"kind":         "stale_state",
						"last_fetched": fetchedAt.Format(time.RFC3339),
						"threshold":    offlineAfter.String(),
					})
				}
			}

			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), alerts, rflags)
			}
			if len(alerts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No alerts.")
				return nil
			}
			headers := []string{"KIND", "NAME", "ROOM", "VALUE"}
			out := make([][]string, 0, len(alerts))
			for _, a := range alerts {
				val := ""
				if v, ok := a["value"]; ok {
					val = fmt.Sprintf("%v", v)
				} else if v, ok := a["last_fetched"]; ok {
					val = fmt.Sprintf("%v", v)
				}
				out = append(out, []string{
					stringOf(a["kind"]),
					stringOf(a["name"]),
					stringOf(a["room"]),
					val,
				})
			}
			return rflags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().Float64Var(&pm25Above, "pm25-above", 0, "Surface devices with pm25 above this (µg/m³; 0=disabled)")
	cmd.Flags().IntVar(&filterBelow, "filter-below", 10, "Surface devices with filter life below this percent")
	cmd.Flags().DurationVar(&offlineAfter, "offline-after", 5*time.Minute, "Surface devices whose state hasn't been refreshed for this long")
	return cmd
}
