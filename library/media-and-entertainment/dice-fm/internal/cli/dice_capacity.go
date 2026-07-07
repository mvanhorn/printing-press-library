// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "capacity" command: a cross-event sold-vs-capacity
// headroom rollup computed from the local store. Ported from the eventbrite
// CLI's `capacity` command; adapted to DICE's data model, where per-event
// capacity is the event's totalTicketAllocationQty and sold is the summed
// order quantity (DICE has no per-ticket-class sold counter in the synced
// connection, so order quantity is the real sold signal).
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// PATCH(amend-2026-05-23: port eventbrite capacity to dice-fm) — cross-event
// sold-vs-capacity headroom rollup over the local events + orders store.

// storeEvent is the slim shape of an `events` store node used across the
// analytics commands. Artists, genres, and genreTypes are included for the
// by-artist revenue rollup; totalTicketAllocationQty is event-level capacity.
type storeEvent struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	State         string `json:"state"`
	TotalAllocQty int64  `json:"totalTicketAllocationQty"`
	StartDatetime string `json:"startDatetime"`
	Artists       []struct {
		Name string `json:"name"`
	} `json:"artists"`
	Genres      []string          `json:"genres"`
	GenreTypes  []string          `json:"genreTypes"`
	TicketPools []storeTicketPool `json:"ticketPools"`
}

// storeTicketPool is one entry of an event's ticketPools allocation array.
// allocation is the pool's ticket allocation; tickets carry no pool reference,
// so sold-per-pool is not derivable from the synced connection.
type storeTicketPool struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Allocation int64  `json:"allocation"`
}

// eligibleEventsByDate returns the set of event IDs whose startDatetime falls in
// the [from, to] inclusive show-date window (YYYY-MM-DD). The second return is
// false when no window is set (both bounds empty), meaning "do not filter by
// date". Comparison is on the date prefix, correct for ISO-8601 timestamps.
//
// NOTE — bound semantics differ from sync by design: the analytics --to here is
// INCLUSIVE ("shows on or before <to>"), whereas sync's --events-to is EXCLUSIVE
// (it maps to the API's date filter). Both are documented in their respective
// --help; keep them in sync if either changes.
func eligibleEventsByDate(ctx context.Context, db *sql.DB, from, to string) (map[string]bool, bool, error) {
	if from == "" && to == "" {
		return nil, false, nil
	}
	events, err := readEvents(ctx, db)
	if err != nil {
		return nil, false, err
	}
	ids := make(map[string]bool)
	for _, e := range events {
		d := e.StartDatetime
		if len(d) >= 10 {
			d = d[:10]
		}
		if from != "" && d < from {
			continue
		}
		if to != "" && d > to {
			continue
		}
		ids[e.ID] = true
	}
	return ids, true, nil
}

// readEvents loads every `events` node from the store and unmarshals it. Rows
// that fail to unmarshal are skipped rather than aborting the scan.
func readEvents(ctx context.Context, db *sql.DB) ([]storeEvent, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'events'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storeEvent
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var e storeEvent
		if err := json.Unmarshal([]byte(data), &e); err != nil {
			continue
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// isLiveEventState reports whether a DICE event state counts as "live" for the
// cross-event capacity headroom view. DICE event states include draft, on-sale,
// off-sale, cancelled, postponed, and ended variants; the rollup keeps states
// that are currently or imminently on sale and an empty state (event row not
// synced, only its orders) rather than silently dropping it. Cancelled,
// postponed, and ended events are excluded so their headroom does not mislead.
func isLiveEventState(state string) bool {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", "live", "on-sale", "on_sale", "onsale", "announced", "off-sale", "off_sale", "offsale", "sold-out", "sold_out", "soldout":
		return true
	}
	return false
}

// capacityRow is one per-event sold-vs-capacity aggregate.
type capacityRow struct {
	EventID   string  `json:"event_id"`
	EventName string  `json:"event_name"`
	Sold      int64   `json:"sold"`
	Capacity  int64   `json:"capacity"`
	Remaining int64   `json:"remaining"`
	PctSold   float64 `json:"pct_sold"`
}

// computeCapacity rolls up sold (summed order quantity) against capacity
// (event totalTicketAllocationQty) per live event, returning headroom rows
// sorted by pct_sold descending. An optional eventFilter limits the rollup to a
// single event ID.
func computeCapacity(ctx context.Context, db *sql.DB, eventFilter string) ([]capacityRow, error) {
	events, err := readEvents(ctx, db)
	if err != nil {
		return nil, err
	}
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}

	type meta struct {
		name     string
		state    string
		capacity int64
		synced   bool
	}
	byEvent := map[string]*meta{}
	for _, e := range events {
		byEvent[e.ID] = &meta{name: e.Name, state: e.State, capacity: e.TotalAllocQty, synced: true}
	}

	sold := map[string]int64{}
	for _, o := range orders {
		id := o.Event.ID
		if id == "" {
			continue
		}
		qty := int64(o.Quantity)
		if qty <= 0 {
			qty = 1
		}
		sold[id] += qty
		m := byEvent[id]
		if m == nil {
			// Order references an event whose row was not synced; keep it so its
			// sold count is not silently dropped, but mark it unsynced so it
			// passes the live-state filter (empty state == live).
			m = &meta{name: o.Event.Name}
			byEvent[id] = m
		}
		if m.name == "" && o.Event.Name != "" {
			m.name = o.Event.Name
		}
	}

	rows := make([]capacityRow, 0, len(byEvent))
	for id, m := range byEvent {
		if eventFilter != "" && id != eventFilter {
			continue
		}
		if !isLiveEventState(m.state) {
			continue
		}
		s := sold[id]
		remaining := m.capacity - s
		pct := 0.0
		if m.capacity > 0 {
			pct = round2((float64(s) / float64(m.capacity)) * 100.0)
		}
		rows = append(rows, capacityRow{
			EventID:   id,
			EventName: m.name,
			Sold:      s,
			Capacity:  m.capacity,
			Remaining: remaining,
			PctSold:   pct,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].PctSold != rows[j].PctSold {
			return rows[i].PctSold > rows[j].PctSold
		}
		return rows[i].EventID < rows[j].EventID
	})
	return rows, nil
}

// poolRow is one per-ticket-pool allocation aggregate. PoolSum is the summed
// allocation across all of the event's pools; EventTotal is the event-level
// totalTicketAllocationQty, surfaced alongside so an operator can compare the
// pool-sum against the event total. SoldPerPool is intentionally absent: DICE
// tickets carry no pool reference, so sold-per-pool is not computable.
type poolRow struct {
	EventID    string `json:"event_id"`
	EventName  string `json:"event_name"`
	PoolID     string `json:"pool_id"`
	PoolName   string `json:"pool_name"`
	Allocation int64  `json:"allocation"`
	PoolSum    int64  `json:"pool_sum"`
	EventTotal int64  `json:"event_total"`
}

// computeCapacityPools reads events from the store and emits one row per ticket
// pool, carrying the pool allocation plus the per-event pool-sum and the event's
// totalTicketAllocationQty for comparison. Events without ticketPools are
// skipped. An optional eventFilter limits the scan to a single event ID. Rows
// are sorted by event ID then pool ID for deterministic output.
func computeCapacityPools(ctx context.Context, db *sql.DB, eventFilter string) ([]poolRow, error) {
	events, err := readEvents(ctx, db)
	if err != nil {
		return nil, err
	}

	rows := make([]poolRow, 0)
	for _, e := range events {
		if eventFilter != "" && e.ID != eventFilter {
			continue
		}
		if len(e.TicketPools) == 0 {
			continue
		}
		var poolSum int64
		for _, p := range e.TicketPools {
			poolSum += p.Allocation
		}
		for _, p := range e.TicketPools {
			rows = append(rows, poolRow{
				EventID:    e.ID,
				EventName:  e.Name,
				PoolID:     p.ID,
				PoolName:   p.Name,
				Allocation: p.Allocation,
				PoolSum:    poolSum,
				EventTotal: e.TotalAllocQty,
			})
		}
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].EventID != rows[j].EventID {
			return rows[i].EventID < rows[j].EventID
		}
		return rows[i].PoolID < rows[j].PoolID
	})
	return rows, nil
}

// newCapacityPoolsCmd surfaces per-ticket-pool allocation granularity, a finer
// view than the event-level capacity rollup. It only reads the local store.
func newCapacityPoolsCmd(flags *rootFlags) *cobra.Command {
	var event string
	var limit int
	cmd := &cobra.Command{
		Use:         "pools",
		Short:       "Per-ticket-pool allocation breakdown (pool-sum vs event total) from the local store",
		Example:     "  dice-fm-pp-cli capacity pools --limit 50 --select event_name,pool_name,allocation,pool_sum,event_total",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []poolRow{}, flags)
			}
			defer s.Close()
			rows, err := computeCapacityPools(cmd.Context(), s.DB(), event)
			if err != nil {
				return err
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Limit the breakdown to a single event ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}

// PATCH(amend-2026-05-23: port eventbrite capacity to dice-fm)
func newCapacityCmd(flags *rootFlags) *cobra.Command {
	var event string
	var limit int
	cmd := &cobra.Command{
		Use:         "capacity",
		Short:       "Cross-event sold-vs-capacity headroom rollup from the local store",
		Example:     "  dice-fm-pp-cli capacity --limit 20 --select event_name,sold,capacity,remaining,pct_sold",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []capacityRow{}, flags)
			}
			defer s.Close()
			rows, err := computeCapacity(cmd.Context(), s.DB(), event)
			if err != nil {
				return err
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Limit the rollup to a single event ID")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	cmd.AddCommand(newCapacityPoolsCmd(flags))
	return cmd
}
