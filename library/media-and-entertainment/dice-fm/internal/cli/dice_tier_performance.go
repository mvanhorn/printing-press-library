// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "tier-performance" command, ported in intent from the
// eventbrite CLI's `discount-performance`. The eventbrite original reports
// per-discount-code redemptions and a redemption rate (sold / cap). DICE has NO
// discount/promo/coupon entity anywhere in its synced data model (events/tickets/
// orders/returns/transfers/extras/genres/fans), and a ticket `code` is a
// per-ticket access barcode, not a discount code — so a literal port is
// impossible without fabricating data. The command is therefore named for what
// it actually measures on DICE: price-tier sell-through.
//
// The closest real "named pricing lever" DICE exposes is the `priceTier`
// ({id name price}) carried on each synced ticket. This command reports
// per-price-tier redemptions and each tier's SHARE of total redemptions
// (tier redemptions / total redemptions), computed entirely from real `tickets`
// store rows joined against `returns` (so returned tickets are not counted).
// The analytical intent matches the eventbrite original (which pricing lever
// earned what share of sales); the semantics differ because DICE has no
// allocation cap per tier, so redemption_rate is a share-of-sales rate, not a
// sold/cap rate. This divergence is recorded in .printing-press-patches.json.
// This file is NOT generated and survives `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"

	"github.com/spf13/cobra"
)

// PATCH(amend-2026-05-23: port eventbrite discount-performance into dice-fm as
// tier-performance, re-targeted to real priceTier data — DICE has no
// discount-code entity) — per-price-tier redemptions + share-of-redemptions
// rate from the local store.

// storeTicket is the slim shape of a `tickets` store node used across the
// analytics commands. priceTier is DICE's named pricing lever; holder carries
// fan identity for cross-join with orders; ticketType carries the named type
// (e.g. "GA", "VIP"). Note: `ticketSelection` in dice_query.go does not sync
// an event reference onto tickets (DICE links tickets to events server-side
// via TicketWhereInput), so per-event aggregations are done via orders, not
// tickets.
type storeTicket struct {
	ID     string `json:"id"`
	Holder struct {
		Email         string `json:"email"`
		FirstName     string `json:"firstName"`
		LastName      string `json:"lastName"`
		OptInPartners bool   `json:"optInPartners"`
	} `json:"holder"`
	TicketType struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Price int64  `json:"price"`
	} `json:"ticketType"`
	PriceTier struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Price int64  `json:"price"`
	} `json:"priceTier"`
}

// storeReturn is the slim shape of a `returns` store node — only the ticketId is
// needed to exclude returned tickets from the redemption count.
type storeReturn struct {
	TicketID string `json:"ticketId"`
}

// readTickets loads every `tickets` node from the store. Rows that fail to
// unmarshal are skipped rather than aborting the scan.
func readTickets(ctx context.Context, db *sql.DB) ([]storeTicket, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'tickets'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []storeTicket
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var t storeTicket
		if err := json.Unmarshal([]byte(data), &t); err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// readReturnedTicketIDs returns the set of ticket IDs that have been returned,
// so the redemption count excludes them (matches the door list's return join).
func readReturnedTicketIDs(ctx context.Context, db *sql.DB) (map[string]bool, error) {
	rows, err := db.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'returns'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	returned := map[string]bool{}
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			continue
		}
		var r storeReturn
		if err := json.Unmarshal([]byte(data), &r); err != nil {
			continue
		}
		if r.TicketID != "" {
			returned[r.TicketID] = true
		}
	}
	return returned, rows.Err()
}

// tierPerformanceRow is one per-price-tier redemption aggregate.
type tierPerformanceRow struct {
	TierID         string  `json:"tier_id"`
	TierName       string  `json:"tier_name"`
	Price          float64 `json:"price"`
	Redemptions    int64   `json:"redemptions"`
	RedemptionRate float64 `json:"redemption_rate"`
}

// computeTierPerformance counts non-returned tickets per price tier and computes
// each tier's share of total non-returned redemptions across all synced tickets.
// Sorted by redemptions descending, then tier name ascending.
func computeTierPerformance(ctx context.Context, db *sql.DB) ([]tierPerformanceRow, error) {
	tickets, err := readTickets(ctx, db)
	if err != nil {
		return nil, err
	}
	returned, err := readReturnedTicketIDs(ctx, db)
	if err != nil {
		return nil, err
	}

	type agg struct {
		name  string
		price int64
		count int64
	}
	groups := map[string]*agg{} // keyed by price-tier ID
	var total int64
	for _, t := range tickets {
		if t.ID != "" && returned[t.ID] {
			continue
		}
		tierID := t.PriceTier.ID
		if tierID == "" {
			// Tickets sold without a named price tier still count toward the
			// denominator and surface as an "(untiered)" bucket rather than being
			// silently dropped.
			tierID = "(untiered)"
		}
		g := groups[tierID]
		if g == nil {
			g = &agg{name: t.PriceTier.Name, price: t.PriceTier.Price}
			if g.name == "" {
				g.name = tierID
			}
			groups[tierID] = g
		}
		g.count++
		total++
	}

	rows := make([]tierPerformanceRow, 0, len(groups))
	for tierID, g := range groups {
		rate := 0.0
		if total > 0 {
			rate = round4(float64(g.count) / float64(total))
		}
		rows = append(rows, tierPerformanceRow{
			TierID:         tierID,
			TierName:       g.name,
			Price:          round2(float64(g.price) / 100.0),
			Redemptions:    g.count,
			RedemptionRate: rate,
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Redemptions != rows[j].Redemptions {
			return rows[i].Redemptions > rows[j].Redemptions
		}
		return rows[i].TierName < rows[j].TierName
	})
	return rows, nil
}

// PATCH(amend-2026-05-23: port eventbrite discount-performance into dice-fm as tier-performance)
func newTierPerformanceCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "tier-performance",
		Short: "Per price-tier redemptions and share-of-sales rate from the local store",
		Long: "Per price-tier redemptions and each tier's share of total redemptions " +
			"across all synced tickets (returned tickets excluded). DICE exposes no " +
			"discount-code entity, so price tiers are the named pricing lever this reports on.",
		Example:     "  dice-fm-pp-cli tier-performance --limit 20",
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
				return printJSONFiltered(cmd.OutOrStdout(), []tierPerformanceRow{}, flags)
			}
			defer s.Close()
			rows, err := computeTierPerformance(cmd.Context(), s.DB())
			if err != nil {
				return err
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}
