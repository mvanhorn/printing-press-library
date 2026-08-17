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

type firmwareRow struct {
	DeviceType      string  `json:"device_type"`
	DeviceLabel     string  `json:"device_label,omitempty"`
	FirmwareVersion *string `json:"fw_version"`
	EndTS           float64 `json:"end_ts,omitempty"`
}

func newNovelFirmwareCheckCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "firmware-check",
		Aliases:     []string{"firmware"},
		Short:       "Show latest AXS firmware version from synced component summaries.",
		Long:        "Reads locally synced quarqnet component summaries and reports the latest fw_version and end_ts per device_type. Run `axs-pp-cli sync --resources summaries` first.",
		Example:     "  axs-pp-cli firmware-check --json",
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
					return printJSONFiltered(cmd.OutOrStdout(), []firmwareRow{}, flags)
				}
				return err
			}
			rows := latestFirmwareRows(summaries)
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no firmware summaries found")
				return nil
			}
			tableRows := [][]string{}
			for _, row := range rows {
				tableRows = append(tableRows, []string{row.DeviceType, row.DeviceLabel, stringPtrValue(row.FirmwareVersion), trimFloat(row.EndTS)})
			}
			return flags.printTable(cmd, []string{"DEVICE_TYPE", "DEVICE", "FW_VERSION", "END_TS"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func summaryFirmwareVersion(summary summaryResource) (string, bool) {
	version := firstNonEmpty(
		gstr(summary.Data, "fw_version", "firmware_version"),
		gstr(summary.Item, "fw_version", "firmware_version"),
	)
	return version, version != ""
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func latestFirmwareRows(summaries []summaryResource) []firmwareRow {
	latest := map[string]firmwareRow{}
	for _, summary := range summaries {
		deviceType := firstNonEmpty(gstr(summary.Data, "device_type"), gstr(summary.Item, "device_type"))
		firmwareVersion, hasFirmware := summaryFirmwareVersion(summary)
		if deviceType == "" || !hasFirmware {
			continue
		}
		endTS, _ := gnum(summary.Data, "end_ts")
		if endTS == 0 {
			endTS, _ = gnum(summary.Item, "end_ts")
		}
		if row, ok := latest[deviceType]; ok && row.EndTS > endTS {
			continue
		}
		latest[deviceType] = firmwareRow{
			DeviceType:      deviceType,
			DeviceLabel:     deviceTypeLabel(deviceType),
			FirmwareVersion: &firmwareVersion,
			EndTS:           endTS,
		}
	}
	rows := make([]firmwareRow, 0, len(latest))
	for _, row := range latest {
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return rows[i].DeviceType < rows[j].DeviceType
	})
	return rows
}
