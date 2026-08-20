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

type shiftsRow struct {
	StartTS       float64 `json:"start_ts"`
	FDShiftCount  float64 `json:"fd_shift_count"`
	RDShiftCount  float64 `json:"rd_shift_count"`
	NumChainrings float64 `json:"num_chainrings"`
	NumCogs       float64 `json:"num_cogs"`
	FDGear        []any   `json:"fd_gear,omitempty"`
	RDGear        []any   `json:"rd_gear,omitempty"`
}

type shiftsTotalsRow struct {
	Rides         int     `json:"rides"`
	TotalFDShifts float64 `json:"total_fd_shifts"`
	TotalRDShifts float64 `json:"total_rd_shifts"`
}

func newNovelShiftsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var totals bool
	var gearSeries bool
	cmd := &cobra.Command{
		Use:         "shifts",
		Short:       "Show per-ride AXS shift counts from synced summaries.",
		Long:        "Reads locally synced quarqnet component summaries and reports per-ride front/rear shift counts plus chainring/cog counts. Gear series arrays are only emitted with --gear-series --json.",
		Example:     "  axs-pp-cli shifts --json\n  axs-pp-cli shifts --totals --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if gearSeries && !flags.asJSON {
				return fmt.Errorf("--gear-series requires --json")
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
					if totals {
						return printJSONFiltered(cmd.OutOrStdout(), shiftsTotalsRow{}, flags)
					}
					return printJSONFiltered(cmd.OutOrStdout(), []shiftsRow{}, flags)
				}
				return err
			}
			rows := []shiftsRow{}
			for _, summary := range summaries {
				detail := summary.Data
				startTS, _ := gnum(detail, "start_ts")
				if startTS == 0 {
					startTS, _ = gnum(summary.Item, "start_ts")
				}
				fdShiftCount, hasFD := gnum(detail, "fd_shift_count")
				if !hasFD {
					fdShiftCount, hasFD = gnum(summary.Item, "fd_shift_count")
				}
				rdShiftCount, hasRD := gnum(detail, "rd_shift_count")
				if !hasRD {
					rdShiftCount, hasRD = gnum(summary.Item, "rd_shift_count")
				}
				if !hasFD && !hasRD {
					continue
				}
				numChainrings, _ := gnum(detail, "num_chainrings")
				if numChainrings == 0 {
					numChainrings, _ = gnum(summary.Item, "num_chainrings")
				}
				numCogs, _ := gnum(detail, "num_cogs")
				if numCogs == 0 {
					numCogs, _ = gnum(summary.Item, "num_cogs")
				}
				row := shiftsRow{
					StartTS:       startTS,
					FDShiftCount:  fdShiftCount,
					RDShiftCount:  rdShiftCount,
					NumChainrings: numChainrings,
					NumCogs:       numCogs,
				}
				if gearSeries {
					if gear, ok := detail["fd_gear"].([]any); ok {
						row.FDGear = gear
					} else if gear, ok := summary.Item["fd_gear"].([]any); ok {
						row.FDGear = gear
					}
					if gear, ok := detail["rd_gear"].([]any); ok {
						row.RDGear = gear
					} else if gear, ok := summary.Item["rd_gear"].([]any); ok {
						row.RDGear = gear
					}
				}
				rows = append(rows, row)
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].StartTS < rows[j].StartTS
			})
			if totals {
				out := shiftsTotalsRow{Rides: len(rows)}
				for _, row := range rows {
					out.TotalFDShifts += row.FDShiftCount
					out.TotalRDShifts += row.RDShiftCount
				}
				if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
					return printJSONFiltered(cmd.OutOrStdout(), out, flags)
				}
				return flags.printTable(cmd, []string{"RIDES", "FD_SHIFTS", "RD_SHIFTS"}, [][]string{{fmt.Sprintf("%d", out.Rides), trimFloat(out.TotalFDShifts), trimFloat(out.TotalRDShifts)}})
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no shift summaries found")
				return nil
			}
			tableRows := [][]string{}
			for _, row := range rows {
				tableRows = append(tableRows, []string{trimFloat(row.StartTS), trimFloat(row.FDShiftCount), trimFloat(row.RDShiftCount), trimFloat(row.NumChainrings), trimFloat(row.NumCogs)})
			}
			return flags.printTable(cmd, []string{"START_TS", "FD_SHIFTS", "RD_SHIFTS", "CHAINRINGS", "COGS"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&totals, "totals", false, "Show lifetime totals over returned shift records")
	cmd.Flags().BoolVar(&gearSeries, "gear-series", false, "Include fd_gear and rd_gear arrays in JSON output")
	return cmd
}
