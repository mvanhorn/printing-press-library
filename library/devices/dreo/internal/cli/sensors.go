// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/devices/dreo/internal/cliutil"

	"github.com/spf13/cobra"
)

func newSensorsCmd(rflags *rootFlags) *cobra.Command {
	var (
		live    bool
		liveAll bool
	)
	cmd := &cobra.Command{
		Use:   "sensors",
		Short: "Whole-house temperature, humidity, and PM2.5 across every sensor-bearing device",
		Long: `Aggregate sensor readings across every Dreo device.

By default, uses the cached device_state from the last sync. Pass --live
to fetch fresh state for every device (slower).

Subcommands:
  record   Persist WebSocket state events to a local sensor_readings table.
  query    Query the local sensor_readings table over arbitrary time windows.`,
		Example: `  dreo-pp-cli sensors
  dreo-pp-cli sensors --json
  dreo-pp-cli sensors --live`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				if rflags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), []any{}, rflags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate sensors: skipped under PRINTING_PRESS_VERIFY")
				return nil
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			if cliutil.IsDogfoodEnv() && live {
				// avoid N-fold network calls inside the 30s dogfood window
				liveAll = false
				live = false
			}

			devs, err := listCachedOrFetch(ctx, rflags, false)
			if err != nil {
				return err
			}

			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			var c *client.Client
			if live || liveAll {
				cl, err := rflags.newClient()
				if err != nil {
					return err
				}
				c = cl
			}

			// Lazy live client: if --live wasn't requested but the local
			// state cache is empty, fall back to a live fetch so the very
			// first `sensors` invocation isn't an empty table.
			lazyClient := func() *client.Client {
				if c != nil {
					return c
				}
				cl, err := rflags.newClient()
				if err != nil {
					return nil
				}
				c = cl
				return c
			}

			rows := []map[string]any{}
			for _, d := range devs {
				var stateMap map[string]any
				// Try cache first
				if raw, _, err := st.GetDeviceState(ctx, d.Sn); err == nil && len(raw) > 0 {
					_ = json.Unmarshal(raw, &stateMap)
				}
				// Live fetch when cache is empty or --live forces refresh
				if stateMap == nil || live || liveAll {
					if lc := lazyClient(); lc != nil {
						if got, err := fetchDeviceState(ctx, rflags, lc, d.Sn); err == nil {
							stateMap = got
						}
					}
				}
				if stateMap == nil {
					continue
				}
				temp, hasTemp := asFloat(stateMap, "temperature")
				hum, hasHum := asFloat(stateMap, "humidity")
				pm25, hasPM := asFloat(stateMap, "pm25")
				if !hasTemp && !hasHum && !hasPM {
					continue
				}
				// PATCH(greptile/641): each device carries its own tempunit
				// (0=°C, 1=°F). Normalize to °C so a household with mixed-
				// unit devices doesn't display 22 next to 72 in the same
				// column. Mirrors rooms.go. Missing tempunit defaults to °C.
				if hasTemp {
					if u, ok := asFloat(stateMap, "tempunit"); ok && u == 1 {
						temp = (temp - 32) * 5.0 / 9.0
					}
				}
				row := map[string]any{
					"sn":    d.Sn,
					"name":  d.Name,
					"room":  d.Room,
					"model": d.Model,
				}
				if hasTemp {
					row["temperature_c"] = temp
				}
				if hasHum {
					row["humidity"] = hum
				}
				if hasPM {
					row["pm25"] = pm25
				}
				rows = append(rows, row)
			}

			// Rank: highest pm25 first, then highest temp.
			sort.SliceStable(rows, func(i, j int) bool {
				pi, _ := rows[i]["pm25"].(float64)
				pj, _ := rows[j]["pm25"].(float64)
				if pi != pj {
					return pi > pj
				}
				ti, _ := rows[i]["temperature_c"].(float64)
				tj, _ := rows[j]["temperature_c"].(float64)
				return ti > tj
			})

			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rows, rflags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No sensor data found. Run `dreo-pp-cli devices state --device-sn <sn>` to refresh.")
				return nil
			}
			headers := []string{"NAME", "ROOM", "MODEL", "TEMP_C", "HUMIDITY", "PM2.5"}
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					stringOf(r["name"]),
					stringOf(r["room"]),
					stringOf(r["model"]),
					floatOf(r["temperature_c"]),
					floatOf(r["humidity"]),
					floatOf(r["pm25"]),
				})
			}
			return rflags.printTable(cmd, headers, out)
		},
	}
	cmd.Flags().BoolVar(&live, "live", false, "Fetch fresh state from the API for every device")
	cmd.Flags().BoolVar(&liveAll, "live-all", false, "Alias for --live")

	cmd.AddCommand(newSensorsRecordCmd(rflags))
	cmd.AddCommand(newSensorsQueryCmd(rflags))
	return cmd
}

// stringOf and floatOf are small printers for sensor table rows.
func stringOf(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}
func floatOf(v any) string {
	if v == nil {
		return ""
	}
	f, ok := v.(float64)
	if !ok {
		return ""
	}
	return fmt.Sprintf("%g", f)
}
