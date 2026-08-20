// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "revenue summary" command and the shared local-store
// order-reading helpers used by the revenue/velocity/fans analytics commands.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// diceCLIName is the binary/store name used to resolve the local SQLite path.
const diceCLIName = "dice-fm-pp-cli"

// orderTicket is a ticket ID nested inside an order payload, populated by the
// lean orderSelectionWithTickets which fetches tickets { id } only. Type name
// and per-ticket total come from the local tickets table via a join in
// computeRevenueByAxisScoped rather than being carried on the order itself.
type orderTicket struct {
	ID string `json:"id"`
}

// orderAdjustment is one post-purchase fee/refund adjustment nested inside an
// order payload (Order.adjustments -> [Adjustment]). FeesChange.Promoter is the
// promoter-side fee delta in integer cents (negative for a refund/reduction);
// FeesChange.Dice is DICE's side. The adjustment's Ticket.ID identifies which
// ticket the change applies to, used to attribute the adjustment to the same
// axis bucket as that ticket. Adjustments are reported as a DISTINCT figure
// alongside gross revenue and are never merged into gross.
type orderAdjustment struct {
	FeesChange struct {
		Category string `json:"category"`
		Dice     int64  `json:"dice"`
		Promoter int64  `json:"promoter"`
	} `json:"feesChange"`
	ProcessedAt string `json:"processedAt"`
	Reason      string `json:"reason"`
	Ticket      struct {
		ID string `json:"id"`
	} `json:"ticket"`
}

// storeOrder is the slim shape of an `orders` store node the analytics commands
// read. Money fields are integer cents as stored by sync. Tickets carries the
// nested per-order tickets populated by the enriched orderSelection.
type storeOrder struct {
	ID          string `json:"id"`
	PurchasedAt string `json:"purchasedAt"`
	Quantity    int    `json:"quantity"`
	Total       int64  `json:"total"`
	DiceComm    int64  `json:"diceCommission"`
	IPCity      string `json:"ipCity"`
	IPCountry   string `json:"ipCountry"`
	Fan         struct {
		FirstName     string `json:"firstName"`
		LastName      string `json:"lastName"`
		Email         string `json:"email"`
		PhoneNumber   string `json:"phoneNumber"`
		OptInPartners bool   `json:"optInPartners"`
	} `json:"fan"`
	Event struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"event"`
	Tickets     []orderTicket     `json:"tickets"`
	Adjustments []orderAdjustment `json:"adjustments"`
}

// joinName concatenates a first and last name, trimming the gap when either is
// empty. Shared by the door list and the fan analytics commands.
func joinName(first, last string) string {
	switch {
	case first == "" && last == "":
		return ""
	case first == "":
		return last
	case last == "":
		return first
	default:
		return first + " " + last
	}
}

// readOrders loads every `orders` node from the store and unmarshals it. Rows
// that fail to unmarshal are skipped rather than aborting the scan.
func readOrders(ctx context.Context, db *sql.DB) ([]storeOrder, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'orders'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storeOrder
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var o storeOrder
		if err := json.Unmarshal([]byte(data), &o); err != nil {
			continue
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// round2 rounds a float to 2 decimal places.
func round2(f float64) float64 {
	return float64(int64(f*100+sign(f)*0.5)) / 100
}

func round4(f float64) float64 {
	return float64(int64(f*10000+sign(f)*0.5)) / 10000
}

func sign(f float64) float64 {
	if f < 0 {
		return -1
	}
	return 1
}

// dateFloorMatch reports whether purchasedAt (RFC3339) is >= from. from may be
// a YYYY-MM-DD date or a full RFC3339 timestamp; lexical comparison on the
// shared prefix is correct for RFC3339 because the format is big-endian.
func dateFloorMatch(purchasedAt, from string) bool {
	if from == "" {
		return true
	}
	// An order with no purchase date cannot satisfy a date floor — exclude it
	// rather than letting an empty string compare equal and silently inflate
	// filtered revenue/fan/velocity totals.
	if len(purchasedAt) < len(from) {
		return false
	}
	// RFC3339 timestamps and YYYY-MM-DD floors are lexicographically ordered,
	// so a prefix-length compare against `from` lets a date-only floor match a
	// full timestamp.
	return purchasedAt[:len(from)] >= from
}

// revenueRow is one per-event revenue aggregate. AdjustmentPromoter /
// AdjustmentCount report post-purchase fee/refund adjustments for the event as
// a DISTINCT figure (summed feesChange.promoter in dollars, negative for net
// refunds); they are never folded into Gross/Net.
type revenueRow struct {
	EventID            string  `json:"event_id"`
	EventName          string  `json:"event_name"`
	Gross              float64 `json:"gross"`
	DiceFees           float64 `json:"dice_fees"`
	Net                float64 `json:"net"`
	OrdersCount        int     `json:"orders_count"`
	AdjustmentCount    int     `json:"adjustment_count"`
	AdjustmentPromoter float64 `json:"adjustment_promoter"`
}

// computeRevenue aggregates orders into per-event gross/dice-fee/net rows,
// filtered by an optional event ID and an optional show-date window (events whose
// startDatetime falls in [fromDate, toDate]), sorted by gross descending.
func computeRevenue(ctx context.Context, db *sql.DB, eventFilter, fromDate, toDate string) ([]revenueRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	eligible, dateFiltered, err := eligibleEventsByDate(ctx, db, fromDate, toDate)
	if err != nil {
		return nil, err
	}
	type agg struct {
		name         string
		grossCents   int64
		diceCents    int64
		ordersCount  int
		adjCount     int
		adjPromoterC int64
	}
	groups := map[string]*agg{}
	for _, o := range orders {
		if eventFilter != "" && o.Event.ID != eventFilter {
			continue
		}
		if dateFiltered && !eligible[o.Event.ID] {
			continue
		}
		g := groups[o.Event.ID]
		if g == nil {
			g = &agg{name: o.Event.Name}
			groups[o.Event.ID] = g
		}
		if g.name == "" && o.Event.Name != "" {
			g.name = o.Event.Name
		}
		g.grossCents += o.Total
		g.diceCents += o.DiceComm
		g.ordersCount++
		// Adjustments are a distinct figure summed per event-group; they do not
		// touch gross/net.
		for _, adj := range o.Adjustments {
			g.adjCount++
			g.adjPromoterC += adj.FeesChange.Promoter
		}
	}

	rows := make([]revenueRow, 0, len(groups))
	for id, g := range groups {
		gross := float64(g.grossCents) / 100.0
		fees := float64(g.diceCents) / 100.0
		rows = append(rows, revenueRow{
			EventID:            id,
			EventName:          g.name,
			Gross:              round2(gross),
			DiceFees:           round2(fees),
			Net:                round2(gross - fees),
			OrdersCount:        g.ordersCount,
			AdjustmentCount:    g.adjCount,
			AdjustmentPromoter: round2(float64(g.adjPromoterC) / 100.0),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Gross != rows[j].Gross {
			return rows[i].Gross > rows[j].Gross
		}
		return rows[i].EventID < rows[j].EventID
	})
	return rows, nil
}

func newRevenueCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "revenue",
		Short: "Revenue analytics computed from the local order store",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newRevenueSummaryCmd(flags))
	cmd.AddCommand(newRevenueByArtistCmd(flags))
	return cmd
}

// pp:data-source local
func newRevenueSummaryCmd(flags *rootFlags) *cobra.Command {
	var event, from, to, byAxis string
	cmd := &cobra.Command{
		Use:         "summary",
		Short:       "Aggregate gross, Dice fees, and net per event from synced orders",
		Example:     "  dice-fm-pp-cli revenue summary --from 2026-04-01 --to 2026-04-30 --select event_name,gross,dice_fees,net,orders_count",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			if from, err = parseDateFlag("from", from); err != nil {
				return err
			}
			if to, err = parseDateFlag("to", to); err != nil {
				return err
			}
			if byAxis != "" {
				if err := validateByAxis(byAxis); err != nil {
					return err
				}
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				if byAxis != "" {
					return printJSONFiltered(cmd.OutOrStdout(), &revenueByAxisResult{}, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), []revenueRow{}, flags)
			}
			defer s.Close()
			if byAxis != "" {
				// Route to the scoped path when any of --event/--from/--to are
				// set; otherwise use the existing unscoped tickets-table path.
				scoped := cmd.Flags().Changed("event") ||
					cmd.Flags().Changed("from") ||
					cmd.Flags().Changed("to")
				var res *revenueByAxisResult
				if scoped {
					res, err = computeRevenueByAxisScoped(cmd.Context(), s.DB(), byAxis, event, from, to)
					if err != nil {
						return fmt.Errorf("computing scoped revenue by axis: %w", err)
					}
				} else {
					res, err = computeRevenueByAxis(cmd.Context(), s.DB(), byAxis)
					if err != nil {
						return fmt.Errorf("computing revenue by axis: %w", err)
					}
				}
				if !res.Normalized {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", res.Warning)
				}
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			rows, err := computeRevenue(cmd.Context(), s.DB(), event, from, to)
			if err != nil {
				return fmt.Errorf("computing revenue: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Limit to a single event ID")
	cmd.Flags().StringVar(&from, "from", "", "Only include shows on or after this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&to, "to", "", "Only include shows on or before this date (YYYY-MM-DD, by show date)")
	cmd.Flags().StringVar(&byAxis, "by-axis", "", "Group ticket revenue (by amount paid, ticket $.total) by a normalized tier axis (access_class|sales_stage|entry_window_type|group_size|comp_flag); falls back to raw ticketType.name if normalize has not been run")
	return cmd
}

// validByAxisValues lists the recognized tier axis names for --by-axis.
var validByAxisValues = map[string]bool{
	axisAccessClass:     true,
	axisSalesStage:      true,
	axisEntryWindowType: true,
	axisGroupSize:       true,
	axisCompFlag:        true,
}

// validateByAxis returns an error when axis is not one of the accepted values.
func validateByAxis(axis string) error {
	if !validByAxisValues[axis] {
		keys := make([]string, 0, len(validByAxisValues))
		for k := range validByAxisValues {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return fmt.Errorf("invalid --by-axis %q: must be one of %s", axis, strings.Join(keys, "|"))
	}
	return nil
}

// revenueByAxisRow is one axis-value bucket of ticket revenue. AdjustmentCount
// and AdjustmentPromoter report post-purchase fee/refund adjustments attributed
// to this axis as a DISTINCT figure: AdjustmentPromoter is the summed
// feesChange.promoter delta in dollars (negative for net refunds) and is never
// folded into TotalRevenue (gross stays clean).
type revenueByAxisRow struct {
	AxisValue          string  `json:"axis_value"`
	TicketCount        int     `json:"ticket_count"`
	TotalRevenue       float64 `json:"total_revenue"`
	AdjustmentCount    int     `json:"adjustment_count"`
	AdjustmentPromoter float64 `json:"adjustment_promoter"`
}

// revenueByAxisResult is the full result of computeRevenueByAxis.
// TotalAdjustmentPromoter / TotalAdjustmentCount sum the per-bucket adjustment
// figures for an at-a-glance total, distinct from gross revenue.
type revenueByAxisResult struct {
	Axis                    string             `json:"axis"`
	Normalized              bool               `json:"normalized"`
	Warning                 string             `json:"warning,omitempty"`
	Rows                    []revenueByAxisRow `json:"rows"`
	TotalAdjustmentCount    int                `json:"total_adjustment_count"`
	TotalAdjustmentPromoter float64            `json:"total_adjustment_promoter"`
}

// computeRevenueByAxis groups ticket revenue from the tickets store table by a
// normalized tier axis (e.g. access_class). When no entity_crosswalk rows exist
// for entity_type='ticket_type', it falls back to grouping by raw
// ticketType.name and sets Normalized=false.
//
// Monetary values come from ticketType.price (cents stored on each ticket). This
// is the only per-ticket monetary field reachable without joining through orders,
// since synced tickets carry no order-ID reference (see storeTicket comment in
// dice_tier_performance.go).
func computeRevenueByAxis(ctx context.Context, db *sql.DB, axis string) (*revenueByAxisResult, error) {
	// Check whether normalization has been run for ticket_type.
	var crosswalkCount int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entity_crosswalk WHERE entity_type = 'ticket_type'`,
	).Scan(&crosswalkCount)
	if err != nil {
		return nil, fmt.Errorf("checking crosswalk: %w", err)
	}

	if crosswalkCount == 0 {
		// Fallback: group by raw ticketType.name.
		rows, err := groupTicketRevenueByRaw(ctx, db)
		if err != nil {
			return nil, err
		}
		return &revenueByAxisResult{
			Axis:       axis,
			Normalized: false,
			Warning:    "normalization has not been run; grouping by raw ticketType.name — run 'normalize --tiers' to enable axis grouping",
			Rows:       rows,
		}, nil
	}

	// Normalized path: join tickets → crosswalk → tier_attributes, group by axis.
	rows, err := groupTicketRevenueByAxis(ctx, db, axis)
	if err != nil {
		return nil, err
	}
	return &revenueByAxisResult{
		Axis:       axis,
		Normalized: true,
		Rows:       rows,
	}, nil
}

// groupTicketRevenueByRaw groups ticket counts and paid-total sums by the raw
// ticketType.name. Used as the fallback when no crosswalk rows exist. Sums paid
// $.total (not list $.ticketType.price) to match the scoped raw fallback
// (scopedFallbackByRaw) and the normalized path.
func groupTicketRevenueByRaw(ctx context.Context, db *sql.DB) ([]revenueByAxisRow, error) {
	sqlRows, err := db.QueryContext(ctx, `
		SELECT
			json_extract(data, '$.ticketType.name') AS axis_value,
			COUNT(*)                                  AS ticket_count,
			COALESCE(SUM(json_extract(data, '$.total')), 0) AS total_cents
		FROM resources
		WHERE resource_type = 'tickets'
		  AND json_extract(data, '$.ticketType.name') IS NOT NULL
		GROUP BY json_extract(data, '$.ticketType.name')
		ORDER BY total_cents DESC, axis_value ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("grouping tickets by raw name: %w", err)
	}
	defer sqlRows.Close()
	return scanAxisRows(sqlRows)
}

// groupTicketRevenueByAxis joins tickets → entity_crosswalk → tier_attributes
// and groups by the requested axis column. The result uses two sentinel buckets:
//   - "(not applicable)" when the ticket type IS in the crosswalk (method !=
//     'unmatched') but the requested axis column is NULL/empty — the type was
//     classified but has no value for this axis.
//   - "(unclassified)" when the ticket type has no crosswalk row or its method
//     is 'unmatched'.
//
// No revenue is dropped — every ticket lands in some bucket.
func groupTicketRevenueByAxis(ctx context.Context, db *sql.DB, axis string) ([]revenueByAxisRow, error) {
	// axis is already validated against validByAxisValues before reaching here.
	// Use a fixed allow-list to build the column reference safely.
	colMap := map[string]string{
		axisAccessClass:     "ta.access_class",
		axisSalesStage:      "ta.sales_stage",
		axisEntryWindowType: "ta.entry_window_type",
		axisGroupSize:       "CAST(ta.group_size AS TEXT)",
		axisCompFlag:        "CAST(ta.comp_flag AS TEXT)",
	}
	axisCol := colMap[axis]
	if axisCol == "" {
		return nil, fmt.Errorf("unsupported axis %q", axis)
	}

	// CASE logic:
	//   - ec row absent or method='unmatched'  -> '(unclassified)'
	//   - ec row present + axis col NULL/empty -> '(not applicable)'
	//   - otherwise                            -> actual axis value
	query := fmt.Sprintf(`
		SELECT
			CASE
				WHEN ec.canonical_id IS NULL OR ec.method = 'unmatched' THEN '(unclassified)'
				WHEN COALESCE(%s, '') = ''                               THEN '(not applicable)'
				ELSE %s
			END AS axis_value,
			COUNT(*)                        AS ticket_count,
			-- Monetary basis: paid ticket $.total (the amount the buyer paid),
			-- to match the scoped path (loadLocalTicketMap, which sums $.total).
			-- Previously summed $.ticketType.price (list price), so the same axis
			-- reported different money scoped vs unscoped.
			COALESCE(SUM(json_extract(r.data, '$.total')), 0) AS total_cents
		FROM resources r
		LEFT JOIN entity_crosswalk ec
			ON ec.entity_type   = 'ticket_type'
			AND ec.source_system = 'dice'
			AND ec.source_value  = json_extract(r.data, '$.ticketType.name')
		LEFT JOIN tier_attributes ta
			ON ta.canonical_id = ec.canonical_id
		WHERE r.resource_type = 'tickets'
		GROUP BY axis_value
		ORDER BY total_cents DESC, axis_value ASC
	`, axisCol, axisCol)

	sqlRows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("grouping tickets by axis %q: %w", axis, err)
	}
	defer sqlRows.Close()
	return scanAxisRows(sqlRows)
}

// scanAxisRows reads (axis_value, ticket_count, total_cents) rows from a query
// result and converts total_cents to dollars.
func scanAxisRows(sqlRows *sql.Rows) ([]revenueByAxisRow, error) {
	var out []revenueByAxisRow
	for sqlRows.Next() {
		var axisValue string
		var ticketCount int
		var totalCents int64
		if err := sqlRows.Scan(&axisValue, &ticketCount, &totalCents); err != nil {
			return nil, fmt.Errorf("scanning axis row: %w", err)
		}
		out = append(out, revenueByAxisRow{
			AxisValue:    axisValue,
			TicketCount:  ticketCount,
			TotalRevenue: round2(float64(totalCents) / 100.0),
		})
	}
	if err := sqlRows.Err(); err != nil {
		return nil, err
	}
	if out == nil {
		out = []revenueByAxisRow{}
	}
	return out, nil
}

// localTicketInfo is the per-ticket data loaded from the local tickets table
// and used by computeRevenueByAxisScoped to resolve axis values and totals
// without relying on fields nested in the order payload.
type localTicketInfo struct {
	typeName   string
	totalCents int64
}

// loadLocalTicketMap builds a ticketID -> localTicketInfo map by querying the
// synced tickets table. Missing or NULL values are returned as zero values so
// callers can detect unresolvable IDs. The map is built once and reused for
// O(1) per-ticket lookup across all qualifying orders.
func loadLocalTicketMap(ctx context.Context, db *sql.DB) (map[string]localTicketInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id,
		       COALESCE(json_extract(data, '$.ticketType.name'), '') AS type_name,
		       COALESCE(json_extract(data, '$.total'), 0)            AS total_cents
		FROM resources
		WHERE resource_type = 'tickets'
	`)
	if err != nil {
		return nil, fmt.Errorf("loading local ticket map: %w", err)
	}
	defer rows.Close()
	m := map[string]localTicketInfo{}
	for rows.Next() {
		var id, typeName string
		var totalCents int64
		if err := rows.Scan(&id, &typeName, &totalCents); err != nil {
			return nil, fmt.Errorf("scanning ticket row: %w", err)
		}
		m[id] = localTicketInfo{typeName: typeName, totalCents: totalCents}
	}
	return m, rows.Err()
}

// computeRevenueByAxisScoped groups ticket revenue from the orders store table
// by a normalized tier axis with optional event-ID and/or show-date scoping.
// It uses orders as the join point because orders carry both their event (for
// date/event filtering) and their nested ticket IDs (for per-ticket axis
// lookup).
//
// Filtering rules:
//   - eventFilter, when non-empty, restricts to orders for that event ID.
//   - fromDate / toDate (YYYY-MM-DD), when set, restrict to orders whose
//     event's startDatetime falls in the inclusive window, via
//     eligibleEventsByDate.
//
// Per-ticket attribution:
//   - For each qualifying order, every nested ticket ID is looked up in the
//     local tickets table (loaded once as an in-process map). The ticket's
//     ticketType.name resolves the axis value via entity_crosswalk +
//     tier_attributes; the ticket's total (integer cents) is attributed to
//     that bucket.
//   - An order with mixed ticket types is split per ticket — no revenue is
//     attributed at the order level.
//   - A ticket ID present in the order but absent from the local tickets
//     table cannot be resolved; it is counted in "(unclassified)" with $0
//     revenue (the type and total are unknown without the synced ticket row).
//
// Bucket semantics (same as the unscoped path):
//   - "(not applicable)" when the ticket type IS in the crosswalk (method !=
//     'unmatched') but the axis column is NULL/empty.
//   - "(unclassified)" when the ticket type has no crosswalk row, method =
//     'unmatched', or the ticket ID is not in the local tickets table.
//
// When no crosswalk rows exist for entity_type='ticket_type', falls back to
// grouping by raw ticketType.name with Normalized=false.
func computeRevenueByAxisScoped(ctx context.Context, db *sql.DB, axis, eventFilter, fromDate, toDate string) (*revenueByAxisResult, error) {
	// Check whether normalization has been run.
	var crosswalkCount int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entity_crosswalk WHERE entity_type = 'ticket_type'`,
	).Scan(&crosswalkCount); err != nil {
		return nil, fmt.Errorf("checking crosswalk: %w", err)
	}

	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("reading orders: %w", err)
	}

	eligible, dateFiltered, err := eligibleEventsByDate(ctx, db, fromDate, toDate)
	if err != nil {
		return nil, fmt.Errorf("resolving date window: %w", err)
	}

	// Filter orders by event and/or date window.
	var qualifying []storeOrder
	for _, o := range orders {
		if eventFilter != "" && o.Event.ID != eventFilter {
			continue
		}
		if dateFiltered && !eligible[o.Event.ID] {
			continue
		}
		qualifying = append(qualifying, o)
	}

	// Orders synced without --order-tickets carry no nested ticket IDs, so a
	// scoped by-axis query would otherwise return an empty result with no
	// explanation. Detect that case (qualifying orders exist but none carry
	// any ticket IDs) and warn instead of silently returning nothing.
	ticketIDsInScope := 0
	for _, o := range qualifying {
		ticketIDsInScope += len(o.Tickets)
	}
	if len(qualifying) > 0 && ticketIDsInScope == 0 {
		return &revenueByAxisResult{
			Axis:       axis,
			Normalized: crosswalkCount > 0,
			Warning:    "orders in the selected window have no ticket data; date/event-scoped --by-axis requires orders synced with their tickets — run `sync --order-tickets`",
			Rows:       []revenueByAxisRow{},
		}, nil
	}

	// Load the local tickets table once for O(1) per-ticket lookup.
	localTickets, err := loadLocalTicketMap(ctx, db)
	if err != nil {
		return nil, err
	}

	if crosswalkCount == 0 {
		// Fallback: group by raw ticketType.name from the local tickets table.
		return scopedFallbackByRaw(axis, qualifying, localTickets)
	}

	// Normalized path: load the crosswalk + tier_attributes for all ticket
	// type names into an in-process map so each ticket lookup is O(1).
	crosswalkMap, err := loadTicketTypeCrosswalk(ctx, db, axis)
	if err != nil {
		return nil, err
	}

	type bucket struct {
		count            int
		cents            int64
		adjCount         int
		adjPromoterCents int64
	}
	buckets := map[string]*bucket{}
	ensureBucket := func(key string) *bucket {
		if b := buckets[key]; b != nil {
			return b
		}
		b := &bucket{}
		buckets[key] = b
		return b
	}

	for _, o := range qualifying {
		for _, tk := range o.Tickets {
			info, found := localTickets[tk.ID]
			if !found {
				// Ticket ID is not in the local table — count it in
				// (unclassified) with $0 revenue; type and total are unknown.
				ensureBucket("(unclassified)").count++
				continue
			}
			axisVal := resolveAxisValue(crosswalkMap, info.typeName)
			b := ensureBucket(axisVal)
			b.count++
			b.cents += info.totalCents
		}
		// Attribute each adjustment to the same axis bucket as its ticket,
		// resolved the same way tickets are (local tickets table -> crosswalk).
		// The promoter-side delta is tracked separately from gross (b.cents).
		for _, adj := range o.Adjustments {
			axisVal := adjustmentAxisValue(localTickets, crosswalkMap, adj.Ticket.ID, true)
			b := ensureBucket(axisVal)
			b.adjCount++
			b.adjPromoterCents += adj.FeesChange.Promoter
		}
	}

	rows := make([]revenueByAxisRow, 0, len(buckets))
	var totalAdjCount int
	var totalAdjCents int64
	for k, b := range buckets {
		totalAdjCount += b.adjCount
		totalAdjCents += b.adjPromoterCents
		rows = append(rows, revenueByAxisRow{
			AxisValue:          k,
			TicketCount:        b.count,
			TotalRevenue:       round2(float64(b.cents) / 100.0),
			AdjustmentCount:    b.adjCount,
			AdjustmentPromoter: round2(float64(b.adjPromoterCents) / 100.0),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ci := int64(rows[i].TotalRevenue * 100)
		cj := int64(rows[j].TotalRevenue * 100)
		if ci != cj {
			return ci > cj
		}
		return rows[i].AxisValue < rows[j].AxisValue
	})
	return &revenueByAxisResult{
		Axis:                    axis,
		Normalized:              true,
		Rows:                    rows,
		TotalAdjustmentCount:    totalAdjCount,
		TotalAdjustmentPromoter: round2(float64(totalAdjCents) / 100.0),
	}, nil
}

// adjustmentAxisValue resolves the axis bucket an adjustment belongs to by
// looking up its ticket in the local tickets table the same way ticket revenue
// is resolved. A ticket ID absent from the local table (or with no ID) lands in
// "(unclassified)". When normalized is true the type name is resolved through
// the crosswalk; otherwise the raw type name is used (with "(unknown)" for an
// empty name) to match the fallback path's bucketing.
func adjustmentAxisValue(localTickets map[string]localTicketInfo, crosswalkMap map[string]crosswalkEntry, ticketID string, normalized bool) string {
	info, found := localTickets[ticketID]
	if !found {
		return "(unclassified)"
	}
	if normalized {
		return resolveAxisValue(crosswalkMap, info.typeName)
	}
	if info.typeName == "" {
		return "(unknown)"
	}
	return info.typeName
}

// crosswalkEntry holds the resolved axis value for a ticket type name.
type crosswalkEntry struct {
	found     bool   // true when a non-unmatched crosswalk row exists
	axisValue string // empty when the row is present but the axis column is NULL
}

// loadTicketTypeCrosswalk fetches all entity_crosswalk + tier_attributes rows
// for entity_type='ticket_type' and builds a name → crosswalkEntry map for
// O(1) per-ticket lookup. axis must be pre-validated against validByAxisValues.
func loadTicketTypeCrosswalk(ctx context.Context, db *sql.DB, axis string) (map[string]crosswalkEntry, error) {
	colMap := map[string]string{
		axisAccessClass:     "ta.access_class",
		axisSalesStage:      "ta.sales_stage",
		axisEntryWindowType: "ta.entry_window_type",
		axisGroupSize:       "CAST(ta.group_size AS TEXT)",
		axisCompFlag:        "CAST(ta.comp_flag AS TEXT)",
	}
	axisCol := colMap[axis]
	if axisCol == "" {
		return nil, fmt.Errorf("unsupported axis %q", axis)
	}

	query := fmt.Sprintf(`
		SELECT ec.source_value, ec.method, COALESCE(%s, '') AS axis_val
		FROM entity_crosswalk ec
		LEFT JOIN tier_attributes ta ON ta.canonical_id = ec.canonical_id
		WHERE ec.entity_type   = 'ticket_type'
		  AND ec.source_system = 'dice'
	`, axisCol)
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("loading crosswalk for axis %q: %w", axis, err)
	}
	defer rows.Close()
	m := map[string]crosswalkEntry{}
	for rows.Next() {
		var sourceVal, method, axisVal string
		if err := rows.Scan(&sourceVal, &method, &axisVal); err != nil {
			return nil, fmt.Errorf("scanning crosswalk row: %w", err)
		}
		if method == "unmatched" {
			m[sourceVal] = crosswalkEntry{found: false, axisValue: ""}
		} else {
			m[sourceVal] = crosswalkEntry{found: true, axisValue: axisVal}
		}
	}
	return m, rows.Err()
}

// resolveAxisValue maps a raw ticketType name to its axis bucket using the
// preloaded crosswalk map. Returns the axis value, "(not applicable)", or
// "(unclassified)" per the bucket semantics documented on computeRevenueByAxisScoped.
func resolveAxisValue(crosswalkMap map[string]crosswalkEntry, name string) string {
	entry, ok := crosswalkMap[name]
	if !ok {
		return "(unclassified)"
	}
	if !entry.found {
		return "(unclassified)"
	}
	if entry.axisValue == "" {
		return "(not applicable)"
	}
	return entry.axisValue
}

// scopedFallbackByRaw groups scoped order tickets by raw ticketType.name when
// no crosswalk rows exist, mirroring the unscoped fallback behavior. Type name
// and total come from localTickets (the synced tickets table); ticket IDs not
// present in localTickets are bucketed as "(unknown)" with $0 revenue.
func scopedFallbackByRaw(axis string, orders []storeOrder, localTickets map[string]localTicketInfo) (*revenueByAxisResult, error) {
	type bucket struct {
		count            int
		cents            int64
		adjCount         int
		adjPromoterCents int64
	}
	buckets := map[string]*bucket{}
	ensureBucket := func(key string) *bucket {
		if b := buckets[key]; b != nil {
			return b
		}
		b := &bucket{}
		buckets[key] = b
		return b
	}
	for _, o := range orders {
		for _, tk := range o.Tickets {
			info, found := localTickets[tk.ID]
			var name string
			var cents int64
			if found {
				name = info.typeName
				cents = info.totalCents
			}
			if name == "" {
				name = "(unknown)"
			}
			b := ensureBucket(name)
			b.count++
			b.cents += cents
		}
		// Adjustments group by raw ticket type name (or "(unknown)") to match
		// the fallback bucketing; promoter delta tracked separately from gross.
		for _, adj := range o.Adjustments {
			name := adjustmentAxisValue(localTickets, nil, adj.Ticket.ID, false)
			b := ensureBucket(name)
			b.adjCount++
			b.adjPromoterCents += adj.FeesChange.Promoter
		}
	}
	rows := make([]revenueByAxisRow, 0, len(buckets))
	var totalAdjCount int
	var totalAdjCents int64
	for k, b := range buckets {
		totalAdjCount += b.adjCount
		totalAdjCents += b.adjPromoterCents
		rows = append(rows, revenueByAxisRow{
			AxisValue:          k,
			TicketCount:        b.count,
			TotalRevenue:       round2(float64(b.cents) / 100.0),
			AdjustmentCount:    b.adjCount,
			AdjustmentPromoter: round2(float64(b.adjPromoterCents) / 100.0),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		ci := int64(rows[i].TotalRevenue * 100)
		cj := int64(rows[j].TotalRevenue * 100)
		if ci != cj {
			return ci > cj
		}
		return rows[i].AxisValue < rows[j].AxisValue
	})
	return &revenueByAxisResult{
		Axis:                    axis,
		Normalized:              false,
		Warning:                 "normalization has not been run; grouping by raw ticketType.name — run 'normalize --tiers' to enable axis grouping",
		Rows:                    rows,
		TotalAdjustmentCount:    totalAdjCount,
		TotalAdjustmentPromoter: round2(float64(totalAdjCents) / 100.0),
	}, nil
}
