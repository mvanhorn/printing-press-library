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

	"github.com/spf13/cobra"
)

// diceCLIName is the binary/store name used to resolve the local SQLite path.
const diceCLIName = "dice-fm-pp-cli"

// storeOrder is the slim shape of an `orders` store node the analytics commands
// read. Money fields are integer cents as stored by sync.
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
	var event, from, to string
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
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), []revenueRow{}, flags)
			}
			defer s.Close()
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
	return cmd
}
