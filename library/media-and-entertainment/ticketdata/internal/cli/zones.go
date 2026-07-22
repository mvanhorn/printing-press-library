// pp:data-source local
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
	"github.com/spf13/cobra"
)

type zoneRow struct {
	Zone         string  `json:"zone"`
	CurrentPrice float64 `json:"current_price"`
	ZoneLow      float64 `json:"zone_low"`
	ZoneHigh     float64 `json:"zone_high"`
	PctAboveLow  float64 `json:"pct_above_low"`
	Points       int     `json:"points"`
}

type zonesView struct {
	Rows []zoneRow `json:"rows"`
}

func newNovelZonesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "zones <event-id>",
		Short:       "Rank an event's zones by current get-in price and by each zone's drop versus its own history to surface the underpriced",
		Long:        "Use `zones` to rank an event's zones by price and by opportunity vs their history. For the plain section name catalog use `events sections`.",
		Example:     "  ticketdata-pp-cli zones 22323960 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "22323960"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank local zone price history")
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("event id is required"))
			}
			if len(args) > 1 {
				return usageErr(fmt.Errorf("zones accepts one event id"))
			}
			if !isNumericEventID(args[0]) {
				return usageErr(fmt.Errorf("event id must be numeric, e.g. 22323960"))
			}
			if !cmd.Flags().Changed("db") {
				dbPath = defaultDBPath("ticketdata-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			points, err := db.ZonePoints(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			rows := buildZoneRows(points)
			if len(rows) == 0 {
				hintIfUnsynced(cmd, db, "")
			}
			view := zonesView{Rows: rows}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printZonesTable(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("ticketdata-pp-cli"), "SQLite database file path")
	return cmd
}

func buildZoneRows(points []store.TDZonePoint) []zoneRow {
	byZone := make(map[string][]store.TDZonePoint)
	for _, point := range points {
		byZone[point.Zone] = append(byZone[point.Zone], point)
	}
	rows := make([]zoneRow, 0, len(byZone))
	for zone, pts := range byZone {
		sort.SliceStable(pts, func(i, j int) bool { return pts[i].InsertedAt < pts[j].InsertedAt })
		low, high := pts[0].GetInPrice, pts[0].GetInPrice
		for _, point := range pts {
			if point.GetInPrice < low {
				low = point.GetInPrice
			}
			if point.GetInPrice > high {
				high = point.GetInPrice
			}
		}
		current := pts[len(pts)-1].GetInPrice
		pctAboveLow := 0.0
		if low > 0 {
			pctAboveLow = (current - low) / low * 100
		}
		rows = append(rows, zoneRow{Zone: zone, CurrentPrice: current, ZoneLow: low, ZoneHigh: high, PctAboveLow: pctAboveLow, Points: len(pts)})
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].CurrentPrice < rows[j].CurrentPrice })
	return rows
}

func printZonesTable(cmd *cobra.Command, rows []zoneRow) error {
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no zone price history found")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "ZONE\tCURRENT\tLOW\tHIGH\tABOVE LOW\tPOINTS")
	for _, row := range rows {
		fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\t%.2f%%\t%d\n", truncate(row.Zone, 36), row.CurrentPrice, row.ZoneLow, row.ZoneHigh, row.PctAboveLow, row.Points)
	}
	return tw.Flush()
}
