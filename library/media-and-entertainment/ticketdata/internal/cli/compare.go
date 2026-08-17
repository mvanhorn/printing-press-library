// pp:data-source local
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
	"github.com/spf13/cobra"
)

type compareRow struct {
	EventID           string  `json:"event_id"`
	Title             string  `json:"title"`
	VenueName         string  `json:"venue_name"`
	GetInPrice        float64 `json:"get_in_price"`
	ThreeDayChangePct float64 `json:"three_day_change_pct"`
}

type compareView struct {
	Rows     []compareRow `json:"rows"`
	Cheapest string       `json:"cheapest,omitempty"`
}

func newNovelCompareCmd(flags *rootFlags) *cobra.Command {
	var performer string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "compare [event-id...]",
		Short:       "Rank multiple watched events, or all of one performer's watched events, by get-in price and percent change.",
		Long:        "Use `compare` to rank multiple watched events or one performer's events head-to-head. For a single event's own history use `stats`; for the full watchlist snapshot use `board`.",
		Example:     "  ticketdata-pp-cli compare --performer ariana-grande --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "22323960"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && performer == "" && commandNFlag(cmd) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would rank local watched event snapshots")
				return nil
			}
			if len(args) == 0 && performer == "" {
				return usageErr(fmt.Errorf("event ids or --performer are required"))
			}
			if !cmd.Flags().Changed("db") {
				dbPath = defaultDBPath("ticketdata-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			watches, err := db.ListWatch(cmd.Context())
			if err != nil {
				return err
			}
			ids, watchByID := compareIDs(args, performer, watches)
			snaps, err := db.AllLatestSnapshots(cmd.Context())
			if err != nil {
				return err
			}
			rows := make([]compareRow, 0, len(ids))
			for _, id := range ids {
				snap, ok := snaps[id]
				if !ok {
					continue
				}
				watch := watchByID[id]
				rows = append(rows, compareRow{EventID: id, Title: watch.Title, VenueName: watch.VenueName, GetInPrice: snap.GetInPrice, ThreeDayChangePct: snap.ThreeDayChangePct})
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].GetInPrice < rows[j].GetInPrice })
			view := compareView{Rows: rows}
			if len(rows) > 0 {
				view.Cheapest = rows[0].EventID
			} else {
				hintIfUnsynced(cmd, db, "")
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printCompareTable(cmd, view)
		},
	}
	cmd.Flags().StringVar(&performer, "performer", "", "Watched performer slug to compare")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("ticketdata-pp-cli"), "SQLite database file path")
	return cmd
}

func compareIDs(args []string, performer string, watches []store.TDWatch) ([]string, map[string]store.TDWatch) {
	byID := make(map[string]store.TDWatch, len(watches))
	for _, watch := range watches {
		byID[watch.EventID] = watch
	}
	ids := make([]string, 0, len(args))
	seen := make(map[string]bool)
	if performer != "" {
		for _, watch := range watches {
			if watch.PerformerSlug == performer && !seen[watch.EventID] {
				ids = append(ids, watch.EventID)
				seen[watch.EventID] = true
			}
		}
		return ids, byID
	}
	for _, id := range args {
		if id != "" && !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	return ids, byID
}

func printCompareTable(cmd *cobra.Command, view compareView) error {
	if len(view.Rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no comparable snapshots found")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "EVENT\tTITLE\tVENUE\tGET-IN\t3D CHANGE")
	for _, row := range view.Rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%.2f\t%.2f\n", row.EventID, truncate(row.Title, 38), truncate(row.VenueName, 28), row.GetInPrice, row.ThreeDayChangePct)
	}
	return tw.Flush()
}
