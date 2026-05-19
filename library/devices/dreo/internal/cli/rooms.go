// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

func newRoomsCmd(rflags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "rooms",
		Short:       "Per-room aggregates (device count, on-count, avg temp, avg humidity)",
		Example:     "  dreo-pp-cli rooms\n  dreo-pp-cli rooms --json",
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

			type acc struct {
				count   int
				onCount int
				tempSum float64
				tempN   int
				humSum  float64
				humN    int
			}
			rooms := map[string]*acc{}
			for _, d := range devs {
				roomKey := d.Room
				if roomKey == "" {
					roomKey = "(no room)"
				}
				a := rooms[roomKey]
				if a == nil {
					a = &acc{}
					rooms[roomKey] = a
				}
				a.count++
				raw, _, serr := st.GetDeviceState(ctx, d.Sn)
				if serr != nil {
					continue
				}
				var state map[string]any
				if err := json.Unmarshal(raw, &state); err != nil {
					continue
				}
				if power, ok := state["poweron"].(bool); ok && power {
					a.onCount++
				}
				// PATCH(greptile/641): each Dreo device carries its own
				// tempunit (0=°C, 1=°F). Convert °F to °C before summing
				// so households with mixed-unit devices get a meaningful
				// average instead of a mash of raw 22-ish and 72-ish
				// numbers. Missing tempunit defaults to °C (the Dreo
				// REST default).
				if t, ok := asFloat(state, "temperature"); ok {
					if u, ok := asFloat(state, "tempunit"); ok && u == 1 {
						t = (t - 32) * 5.0 / 9.0
					}
					a.tempSum += t
					a.tempN++
				}
				if h, ok := asFloat(state, "humidity"); ok {
					a.humSum += h
					a.humN++
				}
			}

			type roomRow struct {
				Room        string  `json:"room"`
				DeviceCount int     `json:"device_count"`
				OnCount     int     `json:"on_count"`
				AvgTempC    float64 `json:"avg_temperature_c,omitempty"`
				AvgHumidity float64 `json:"avg_humidity,omitempty"`
			}
			result := make([]roomRow, 0, len(rooms))
			for room, a := range rooms {
				row := roomRow{
					Room:        room,
					DeviceCount: a.count,
					OnCount:     a.onCount,
				}
				if a.tempN > 0 {
					row.AvgTempC = a.tempSum / float64(a.tempN)
				}
				if a.humN > 0 {
					row.AvgHumidity = a.humSum / float64(a.humN)
				}
				result = append(result, row)
			}
			sort.Slice(result, func(i, j int) bool { return result[i].Room < result[j].Room })

			if rflags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, rflags)
			}
			if len(result) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No rooms found. Run `dreo-pp-cli devices list` first.")
				return nil
			}
			headers := []string{"ROOM", "DEVICES", "ON", "AVG_TEMP_C", "AVG_HUMIDITY"}
			rows := make([][]string, 0, len(result))
			for _, r := range result {
				tempStr, humStr := "", ""
				if r.AvgTempC != 0 {
					tempStr = fmt.Sprintf("%.1f", r.AvgTempC)
				}
				if r.AvgHumidity != 0 {
					humStr = fmt.Sprintf("%.1f", r.AvgHumidity)
				}
				rows = append(rows, []string{
					r.Room,
					fmt.Sprintf("%d", r.DeviceCount),
					fmt.Sprintf("%d", r.OnCount),
					tempStr,
					humStr,
				})
			}
			return rflags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}
