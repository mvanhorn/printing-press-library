// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "returns anomalies" command: flags events whose return
// rate (returns/tickets_sold) meets or exceeds a threshold, computed from the
// local order and return stores.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

// returnsAnomalyRow is one flagged event.
type returnsAnomalyRow struct {
	EventID      string  `json:"event_id"`
	EventName    string  `json:"event_name"`
	OrdersCount  int     `json:"orders_count"`
	TicketsSold  int     `json:"tickets_sold"`
	ReturnsCount int     `json:"returns_count"`
	ReturnRate   float64 `json:"return_rate"`
}

// computeReturnsAnomalies counts orders and returns per event and returns the
// events whose return rate is >= threshold, sorted by return rate descending.
// Events with zero orders are skipped (an undefined rate).
func computeReturnsAnomalies(ctx context.Context, db *sql.DB, threshold float64, fromDate, toDate string) ([]returnsAnomalyRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	eligible, dateFiltered, err := eligibleEventsByDate(ctx, db, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	ordersByEvent := map[string]int{}
	ticketsByEvent := map[string]int{}
	for _, o := range orders {
		if o.Event.ID == "" {
			continue
		}
		if dateFiltered && !eligible[o.Event.ID] {
			continue
		}
		ordersByEvent[o.Event.ID]++
		// DICE return records are per-ticket, so the return rate must divide by
		// tickets sold (sum of order quantity), not order count — otherwise a
		// multi-ticket order can push the rate above 1.0. An order is at least
		// one ticket even when quantity is missing/zero.
		qty := o.Quantity
		if qty <= 0 {
			qty = 1
		}
		ticketsByEvent[o.Event.ID] += qty
	}

	returnsByEvent, namesByEvent, err := readReturnsByEvent(ctx, db)
	if err != nil {
		return nil, err
	}

	rows := make([]returnsAnomalyRow, 0)
	for eventID, ticketsCount := range ticketsByEvent {
		if ticketsCount == 0 {
			continue
		}
		returnsCount := returnsByEvent[eventID]
		rate := float64(returnsCount) / float64(ticketsCount)
		if rate < threshold {
			continue
		}
		rows = append(rows, returnsAnomalyRow{
			EventID:      eventID,
			EventName:    namesByEvent[eventID],
			OrdersCount:  ordersByEvent[eventID],
			TicketsSold:  ticketsCount,
			ReturnsCount: returnsCount,
			ReturnRate:   round4(rate),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].ReturnRate != rows[j].ReturnRate {
			return rows[i].ReturnRate > rows[j].ReturnRate
		}
		return rows[i].EventID < rows[j].EventID
	})
	return rows, nil
}

// readReturnsByEvent counts returns per event ID (return.order.event.id) and
// collects the event names found there.
func readReturnsByEvent(ctx context.Context, db *sql.DB) (counts map[string]int, names map[string]string, err error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'returns'`)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	counts = map[string]int{}
	names = map[string]string{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var r struct {
			Order struct {
				Event struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"event"`
			} `json:"order"`
		}
		if err := json.Unmarshal([]byte(data), &r); err != nil {
			continue
		}
		id := r.Order.Event.ID
		if id == "" {
			continue
		}
		counts[id]++
		if names[id] == "" && r.Order.Event.Name != "" {
			names[id] = r.Order.Event.Name
		}
	}
	return counts, names, rows.Err()
}

// pp:data-source local
func newReturnsAnomaliesCmd(flags *rootFlags) *cobra.Command {
	var threshold float64
	var from, to string
	cmd := &cobra.Command{
		Use:         "anomalies",
		Short:       "Flag events with an unusually high return rate",
		Example:     "  dice-fm-pp-cli returns anomalies --threshold 0.05 --from 2026-04-01 --to 2026-04-30 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if from, err = parseDateFlag("from", from); err != nil {
				return err
			}
			if to, err = parseDateFlag("to", to); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []returnsAnomalyRow{}, flags)
			}
			defer s.Close()
			rows, err := computeReturnsAnomalies(cmd.Context(), s.DB(), threshold, from, to)
			if err != nil {
				return fmt.Errorf("computing return anomalies: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().Float64Var(&threshold, "threshold", 0.05, "Minimum return rate (returns/tickets_sold) to flag an event")
	cmd.Flags().StringVar(&from, "from", "", "Only include shows on or after this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&to, "to", "", "Only include shows on or before this date (YYYY-MM-DD, by show date)")
	return cmd
}
