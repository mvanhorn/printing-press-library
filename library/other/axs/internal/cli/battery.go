// Copyright 2026 Greg Stellato and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature for the SRAM AXS CLI. Not generated.
// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

type batteryRow struct {
	DeviceType    string   `json:"device_type"`
	DeviceLabel   string   `json:"device_label,omitempty"`
	BatteryStatus *float64 `json:"battery_status"`
	Voltage       *float64 `json:"voltage"`
	EndTS         float64  `json:"end_ts,omitempty"`
}

func newNovelBatteryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "battery",
		Short:       "Show latest AXS battery status from synced component summaries.",
		Long:        "Reads locally synced quarqnet component summaries and reports the latest battery_status, voltage, and end_ts per device_type. Run `axs-pp-cli sync --resources summaries` first.",
		Example:     "  axs-pp-cli battery --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if err := validateDataSourceStrategy(flags, "local"); err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			summaries, err := loadLocalSummaryResources(ctx, dbPath)
			if err != nil {
				if errors.Is(err, errNoLocalMirror) && (flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout())) {
					fmt.Fprintln(cmd.ErrOrStderr(), err.Error())
					return printJSONFiltered(cmd.OutOrStdout(), []batteryRow{}, flags)
				}
				return err
			}
			rows := latestBatteryRows(summaries)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no battery summaries found")
				return nil
			}
			tableRows := [][]string{}
			for _, row := range rows {
				tableRows = append(tableRows, []string{row.DeviceType, row.DeviceLabel, trimFloatPtr(row.BatteryStatus), trimFloatPtr(row.Voltage), trimFloat(row.EndTS)})
			}
			return flags.printTable(cmd, []string{"DEVICE_TYPE", "DEVICE", "BATTERY", "VOLTS", "END_TS"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type batteryAccumulator struct {
	row       batteryRow
	statusTS  float64
	voltageTS float64
}

func latestBatteryRows(summaries []summaryResource) []batteryRow {
	latest := map[string]*batteryAccumulator{}
	for _, summary := range summaries {
		deviceType := firstNonEmpty(gstr(summary.Data, "device_type"), gstr(summary.Item, "device_type"))
		if deviceType == "" {
			continue
		}
		status, hasStatus := gnum(summary.Data, "battery_status")
		if !hasStatus {
			status, hasStatus = gnum(summary.Item, "battery_status")
		}
		voltage, hasVoltage := gnum(summary.Data, "voltage")
		if !hasVoltage {
			voltage, hasVoltage = gnum(summary.Item, "voltage")
		}
		if !hasStatus && !hasVoltage {
			continue
		}
		endTS, _ := gnum(summary.Data, "end_ts")
		if endTS == 0 {
			endTS, _ = gnum(summary.Item, "end_ts")
		}
		acc := latest[deviceType]
		if acc == nil {
			acc = &batteryAccumulator{row: batteryRow{DeviceType: deviceType, DeviceLabel: deviceTypeLabel(deviceType)}}
			latest[deviceType] = acc
		}
		if hasStatus && (acc.row.BatteryStatus == nil || endTS >= acc.statusTS) {
			acc.row.BatteryStatus = &status
			acc.statusTS = endTS
		}
		if hasVoltage && (acc.row.Voltage == nil || endTS >= acc.voltageTS) {
			acc.row.Voltage = &voltage
			acc.voltageTS = endTS
		}
		acc.row.EndTS = max(acc.statusTS, acc.voltageTS)
	}
	rows := make([]batteryRow, 0, len(latest))
	for _, acc := range latest {
		rows = append(rows, acc.row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].DeviceType < rows[j].DeviceType
	})
	return rows
}
