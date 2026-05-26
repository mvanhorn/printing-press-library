// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
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

// orderTicket is a ticket nested inside an order payload, populated by the
// enriched orderSelection which adds tickets { id total ticketType { name }
// priceTier { name } }. The per-ticket total (integer cents) is the attribution
// amount for axis-scoped revenue grouping.
type orderTicket struct {
	ID         string `json:"id"`
	Total      int64  `json:"total"`
	TicketType struct {
		Name string `json:"name"`
	} `json:"ticketType"`
	PriceTier struct {
		Name string `json:"name"`
	} `json:"priceTier"`
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
	Tickets []orderTicket `json:"tickets"`
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

// revenueRow is one per-event revenue aggregate.
type revenueRow struct {
	EventID     string  `json:"event_id"`
	EventName   string  `json:"event_name"`
	Gross       float64 `json:"gross"`
	DiceFees    float64 `json:"dice_fees"`
	Net         float64 `json:"net"`
	OrdersCount int     `json:"orders_count"`
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
		name        string
		grossCents  int64
		diceCents   int64
		ordersCount int
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
	}

	rows := make([]revenueRow, 0, len(groups))
	for id, g := range groups {
		gross := float64(g.grossCents) / 100.0
		fees := float64(g.diceCents) / 100.0
		rows = append(rows, revenueRow{
			EventID:     id,
			EventName:   g.name,
			Gross:       round2(gross),
			DiceFees:    round2(fees),
			Net:         round2(gross - fees),
			OrdersCount: g.ordersCount,
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
	cmd.Flags().StringVar(&byAxis, "by-axis", "", "Group ticket revenue by a normalized tier axis (access_class|sales_stage|entry_window_type|group_size|comp_flag); falls back to raw ticketType.name if normalize has not been run")
	return cmd
}

// validByAxisValues lists the recognized tier axis names for --by-axis.
var validByAxisValues = map[string]bool{
	"access_class":      true,
	"sales_stage":       true,
	"entry_window_type": true,
	"group_size":        true,
	"comp_flag":         true,
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

// revenueByAxisRow is one axis-value bucket of ticket revenue.
type revenueByAxisRow struct {
	AxisValue    string  `json:"axis_value"`
	TicketCount  int     `json:"ticket_count"`
	TotalRevenue float64 `json:"total_revenue"`
}

// revenueByAxisResult is the full result of computeRevenueByAxis.
type revenueByAxisResult struct {
	Axis       string             `json:"axis"`
	Normalized bool               `json:"normalized"`
	Warning    string             `json:"warning,omitempty"`
	Rows       []revenueByAxisRow `json:"rows"`
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

// groupTicketRevenueByRaw groups ticket counts and ticketType.price sums by the
// raw ticketType.name. Used as the fallback when no crosswalk rows exist.
func groupTicketRevenueByRaw(ctx context.Context, db *sql.DB) ([]revenueByAxisRow, error) {
	sqlRows, err := db.QueryContext(ctx, `
		SELECT
			json_extract(data, '$.ticketType.name') AS axis_value,
			COUNT(*)                                  AS ticket_count,
			COALESCE(SUM(json_extract(data, '$.ticketType.price')), 0) AS total_cents
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
		"access_class":      "ta.access_class",
		"sales_stage":       "ta.sales_stage",
		"entry_window_type": "ta.entry_window_type",
		"group_size":        "CAST(ta.group_size AS TEXT)",
		"comp_flag":         "CAST(ta.comp_flag AS TEXT)",
	}
	axisCol := colMap[axis]

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
			COALESCE(SUM(json_extract(r.data, '$.ticketType.price')), 0) AS total_cents
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

// computeRevenueByAxisScoped groups ticket revenue from the orders store table
// by a normalized tier axis with optional event-ID and/or show-date scoping.
// It uses orders as the join point because orders carry both their event (for
// date/event filtering) and their nested tickets (for per-ticket axis lookup).
//
// Filtering rules:
//   - eventFilter, when non-empty, restricts to orders for that event ID.
//   - fromDate / toDate (YYYY-MM-DD), when set, restrict to orders whose
//     event's startDatetime falls in the inclusive window, via
//     eligibleEventsByDate.
//
// Per-ticket attribution:
//   - For each qualifying order, every nested ticket is iterated individually.
//   - The ticket's ticketType.name is looked up in entity_crosswalk and
//     tier_attributes to resolve the axis value.
//   - The ticket's own total (integer cents) is summed into the resolved bucket.
//   - An order with mixed ticket types is split per ticket — no revenue is
//     attributed at the order level.
//
// Bucket semantics (same as the unscoped path):
//   - "(not applicable)" when the ticket type IS in the crosswalk (method !=
//     'unmatched') but the axis column is NULL/empty.
//   - "(unclassified)" when the ticket type has no crosswalk row or method =
//     'unmatched'.
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

	if crosswalkCount == 0 {
		// Fallback: group by raw ticketType.name from the nested tickets.
		return scopedFallbackByRaw(axis, qualifying)
	}

	// Normalized path: load the crosswalk + tier_attributes for all ticket
	// type names into an in-process map so each ticket lookup is O(1).
	crosswalkMap, err := loadTicketTypeCrosswalk(ctx, db, axis)
	if err != nil {
		return nil, err
	}

	type bucket struct {
		count int
		cents int64
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
			name := tk.TicketType.Name
			axisVal := resolveAxisValue(crosswalkMap, name)
			b := ensureBucket(axisVal)
			b.count++
			b.cents += tk.Total
		}
	}

	rows := make([]revenueByAxisRow, 0, len(buckets))
	for k, b := range buckets {
		rows = append(rows, revenueByAxisRow{
			AxisValue:    k,
			TicketCount:  b.count,
			TotalRevenue: round2(float64(b.cents) / 100.0),
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
	if rows == nil {
		rows = []revenueByAxisRow{}
	}
	return &revenueByAxisResult{
		Axis:       axis,
		Normalized: true,
		Rows:       rows,
	}, nil
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
		"access_class":      "ta.access_class",
		"sales_stage":       "ta.sales_stage",
		"entry_window_type": "ta.entry_window_type",
		"group_size":        "CAST(ta.group_size AS TEXT)",
		"comp_flag":         "CAST(ta.comp_flag AS TEXT)",
	}
	axisCol := colMap[axis]

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
// no crosswalk rows exist, mirroring the unscoped fallback behavior.
func scopedFallbackByRaw(axis string, orders []storeOrder) (*revenueByAxisResult, error) {
	type bucket struct {
		count int
		cents int64
	}
	buckets := map[string]*bucket{}
	for _, o := range orders {
		for _, tk := range o.Tickets {
			name := tk.TicketType.Name
			if name == "" {
				name = "(unknown)"
			}
			b := buckets[name]
			if b == nil {
				b = &bucket{}
				buckets[name] = b
			}
			b.count++
			b.cents += tk.Total
		}
	}
	rows := make([]revenueByAxisRow, 0, len(buckets))
	for k, b := range buckets {
		rows = append(rows, revenueByAxisRow{
			AxisValue:    k,
			TicketCount:  b.count,
			TotalRevenue: round2(float64(b.cents) / 100.0),
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
	if rows == nil {
		rows = []revenueByAxisRow{}
	}
	return &revenueByAxisResult{
		Axis:       axis,
		Normalized: false,
		Warning:    "normalization has not been run; grouping by raw ticketType.name — run 'normalize --tiers' to enable axis grouping",
		Rows:       rows,
	}, nil
}
