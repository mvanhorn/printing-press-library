// pp:data-source local
// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/ticketdata/internal/store"
	"github.com/spf13/cobra"
)

type boardRow struct {
	EventID             string   `json:"event_id"`
	Title               string   `json:"title"`
	GetInPrice          float64  `json:"get_in_price"`
	ThreeDayChangePct   float64  `json:"three_day_change_pct"`
	ForecastValue       float64  `json:"forecast_value"`
	PriceTrendDirection string   `json:"price_trend_direction"`
	HistoryPercentile   *float64 `json:"history_percentile,omitempty"`
}

type boardView struct {
	Rows []boardRow `json:"rows"`
}

func newNovelBoardCmd(flags *rootFlags) *cobra.Command {
	var sortBy string
	var dbPath string

	cmd := &cobra.Command{
		Use:         "board",
		Short:       "One sortable table of every watched event: get-in price, N-day change, forecast direction",
		Long:        "Use `board` for a current snapshot of the whole watchlist. For what CHANGED since your last sync or for price-target alerts use `drift`; for one event's historical distribution use `stats`.",
		Example:     "  ticketdata-pp-cli board --sort change --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return usageErr(fmt.Errorf("board does not accept arguments"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would read latest local watchlist snapshots")
				return nil
			}
			if sortBy != "price" && sortBy != "change" && sortBy != "percentile" {
				return usageErr(fmt.Errorf("--sort must be one of price, change, percentile"))
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
			snaps, err := db.AllLatestSnapshots(cmd.Context())
			if err != nil {
				return err
			}

			eventIDs := make([]string, 0, len(watches))
			for _, watch := range watches {
				eventIDs = append(eventIDs, watch.EventID)
			}
			pointsByEvent, err := db.PricePointsForEvents(cmd.Context(), eventIDs)
			if err != nil {
				return err
			}

			rows := make([]boardRow, 0, len(watches))
			for _, watch := range watches {
				snap, ok := snaps[watch.EventID]
				if !ok {
					continue
				}
				var percentile *float64
				points := pointsByEvent[watch.EventID]
				if len(points) >= 2 {
					lte := 0
					for _, pt := range points {
						if pt.GetInPrice <= snap.GetInPrice {
							lte++
						}
					}
					p := float64(lte) / float64(len(points)) * 100
					percentile = &p
				}
				rows = append(rows, boardRow{
					EventID:             watch.EventID,
					Title:               watch.Title,
					GetInPrice:          snap.GetInPrice,
					ThreeDayChangePct:   snap.ThreeDayChangePct,
					ForecastValue:       snap.ForecastValue,
					PriceTrendDirection: snap.PriceTrendDirection,
					HistoryPercentile:   percentile,
				})
			}
			sortBoardRows(rows, sortBy)
			if len(rows) == 0 {
				hintIfUnsynced(cmd, db, "")
			}
			view := boardView{Rows: rows}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printBoardTable(cmd, rows)
		},
	}
	cmd.Flags().StringVar(&sortBy, "sort", "price", "Sort rows by price, change, or percentile")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("ticketdata-pp-cli"), "SQLite database file path")
	return cmd
}

func sortBoardRows(rows []boardRow, sortBy string) {
	sort.SliceStable(rows, func(i, j int) bool {
		switch sortBy {
		case "change":
			return rows[i].ThreeDayChangePct < rows[j].ThreeDayChangePct
		case "percentile":
			if rows[i].HistoryPercentile == nil {
				return false
			}
			if rows[j].HistoryPercentile == nil {
				return true
			}
			return *rows[i].HistoryPercentile < *rows[j].HistoryPercentile
		default:
			return rows[i].GetInPrice < rows[j].GetInPrice
		}
	})
}

func printBoardTable(cmd *cobra.Command, rows []boardRow) error {
	if len(rows) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no watched snapshots found")
		return nil
	}
	tw := newTabWriter(cmd.OutOrStdout())
	fmt.Fprintln(tw, "EVENT\tTITLE\tGET-IN\t3D CHANGE\tFORECAST\tTREND\tPERCENTILE")
	for _, row := range rows {
		p := ""
		if row.HistoryPercentile != nil {
			p = fmt.Sprintf("%.1f", *row.HistoryPercentile)
		}
		fmt.Fprintf(tw, "%s\t%s\t%.2f\t%.2f\t%.2f\t%s\t%s\n",
			row.EventID, truncate(row.Title, 42), row.GetInPrice, row.ThreeDayChangePct,
			row.ForecastValue, row.PriceTrendDirection, p)
	}
	return tw.Flush()
}
