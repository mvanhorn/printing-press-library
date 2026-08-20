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

type wearRow struct {
	ID              string   `json:"id"`
	Serial          string   `json:"serial,omitempty"`
	Component       string   `json:"component,omitempty"`
	DeviceType      string   `json:"device_type,omitempty"`
	DeviceLabel     string   `json:"device_label,omitempty"`
	TotalDistance   float64  `json:"total_distance,omitempty"`
	ShiftCount      float64  `json:"shift_count,omitempty"`
	FDShiftCount    float64  `json:"fd_shift_count,omitempty"`
	RDShiftCount    float64  `json:"rd_shift_count,omitempty"`
	ActuationCount  float64  `json:"actuation_count,omitempty"`
	BatteryStatus   *float64 `json:"battery_status,omitempty"`
	FirmwareVersion string   `json:"firmware_version,omitempty"`
}

func newNovelWearCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "wear",
		Short:       "Rank component usage from AXS component summaries.",
		Long:        "Aggregates locally synced per-component summary fields and ranks components by distance, shifts, and actuations. Run `axs-pp-cli sync --resources summaries` first.",
		Example:     "  axs-pp-cli wear --json",
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
					return printJSONFiltered(cmd.OutOrStdout(), []wearRow{}, flags)
				}
				return err
			}
			byKey := map[string]*wearRow{}
			for _, summary := range summaries {
				item := summary.Item
				detail := summary.Data
				key := gstr(item, "nexus_component_id", "serial", "device_id", "id")
				if key == "" {
					key = summary.ID
				}
				row := byKey[key]
				if row == nil {
					deviceType := firstNonEmpty(gstr(detail, "device_type"), gstr(item, "device_type"))
					row = &wearRow{
						ID:              key,
						Serial:          gstr(item, "serial"),
						Component:       firstNonEmpty(gstr(detail, "component", "name"), gstr(item, "component", "name")),
						DeviceType:      deviceType,
						DeviceLabel:     deviceTypeLabel(deviceType),
						FirmwareVersion: firstNonEmpty(gstr(detail, "fw_version", "firmware_version"), gstr(item, "fw_version", "firmware_version")),
					}
					byKey[key] = row
				}
				if v, ok := gnum(detail, "distance", "total_distance"); ok {
					row.TotalDistance += v
				}
				applyWearShiftCounts(row, detail, item)
				if v, ok := gnum(detail, "actuation_count", "total_actuations"); ok {
					row.ActuationCount += v
				}
				applyWearBatteryStatus(row, detail, item)
			}
			rows := []wearRow{}
			for _, row := range byKey {
				rows = append(rows, *row)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].TotalDistance+rows[i].ShiftCount+rows[i].ActuationCount > rows[j].TotalDistance+rows[j].ShiftCount+rows[j].ActuationCount
			})
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no component summaries found")
				return nil
			}
			tableRows := [][]string{}
			for _, row := range rows {
				tableRows = append(tableRows, []string{row.DeviceType, row.DeviceLabel, row.Component, row.Serial, trimFloat(row.ShiftCount), trimFloat(row.TotalDistance), trimFloat(row.ActuationCount)})
			}
			return flags.printTable(cmd, []string{"DEVICE_TYPE", "DEVICE", "COMPONENT", "SERIAL", "SHIFTS", "DISTANCE", "ACTUATIONS"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func applyWearBatteryStatus(row *wearRow, detail, item map[string]any) {
	status, ok := gnum(detail, "battery_status")
	if !ok {
		status, ok = gnum(item, "battery_status")
	}
	if ok {
		row.BatteryStatus = &status
	}
}

func applyWearShiftCounts(row *wearRow, detail, item map[string]any) {
	rd, hasRD := gnum(detail, "rd_shift_count")
	if !hasRD {
		rd, hasRD = gnum(item, "rd_shift_count")
	}
	total, hasTotal := gnum(detail, "shift_count")
	if !hasTotal {
		total, hasTotal = gnum(item, "shift_count")
	}
	fd, hasFD := gnum(detail, "fd_shift_count")
	if !hasFD {
		fd, hasFD = gnum(item, "fd_shift_count")
	}
	if hasRD {
		row.RDShiftCount += rd
		row.ShiftCount += rd
	} else if hasTotal {
		row.ShiftCount += total
	}
	if hasFD {
		row.FDShiftCount += fd
		if hasRD || !hasTotal {
			row.ShiftCount += fd
		}
	}
}
