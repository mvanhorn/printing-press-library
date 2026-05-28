// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "fans profile" command: per-fan rollup for a single email
// address, computed from orders and tickets in the local store. This file is
// NOT generated and survives `generate --force`.
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// fanProfileResult is the single-object output of fans profile.
type fanProfileResult struct {
	Found           bool     `json:"found"`
	Email           string   `json:"email"`
	Name            string   `json:"name"`
	OptedIn         bool     `json:"opted_in"`
	FirstOrderDate  string   `json:"first_order_date"`
	LastOrderDate   string   `json:"last_order_date"`
	FirstEvent      string   `json:"first_event"`
	LastEvent       string   `json:"last_event"`
	OrdersCount     int      `json:"orders_count"`
	TotalSpend      float64  `json:"total_spend"`
	VIPSpend        float64  `json:"vip_spend"`
	EventCountAll   int      `json:"event_count_all"`
	EventCountPaid  int      `json:"event_count_paid"`
	EventsPurchased []string `json:"events_purchased"`
	TicketTypes     []string `json:"ticket_types"`
}

// computeFanProfile builds a per-fan rollup for a single email. If the email
// has no orders in the store the result has found:false and zero/empty fields.
func computeFanProfile(ctx context.Context, db *sql.DB, email string) (fanProfileResult, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return fanProfileResult{}, err
	}

	// VIP detection: order's total counts as VIP spend when the fan has at
	// least one ticket for that order whose ticketType.name contains "vip"
	// (case-insensitive).
	tickets, err := readTickets(ctx, db)
	if err != nil {
		return fanProfileResult{}, err
	}
	// Build a set of order IDs that are "VIP" based on the holder's tickets.
	// Tickets don't carry an order ID, so we proxy via holder email + ticketType.
	// When a holder has any "vip" ticket type we mark ALL of that fan's orders
	// as VIP-eligible — this is intentionally loose (no order-to-ticket link in
	// the store), but it's the best available signal without fabricating joins.
	// The field is documented accordingly in the command help.
	wantEmail := strings.ToLower(email)
	holderHasVIP := false
	ticketTypeNames := map[string]bool{}
	for _, t := range tickets {
		if strings.ToLower(t.Holder.Email) != wantEmail {
			continue
		}
		if t.TicketType.Name != "" {
			ticketTypeNames[t.TicketType.Name] = true
		}
		if strings.Contains(strings.ToLower(t.TicketType.Name), "vip") {
			holderHasVIP = true
		}
	}

	type orderRecord struct {
		purchasedAt string
		eventID     string
		eventName   string
		total       int64
	}
	var fanOrders []orderRecord
	var fanName string
	fanOptedIn := false

	for _, o := range orders {
		if strings.ToLower(o.Fan.Email) != wantEmail {
			continue
		}
		if fanName == "" {
			fanName = joinName(o.Fan.FirstName, o.Fan.LastName)
		}
		if o.Fan.OptInPartners {
			fanOptedIn = true
		}
		fanOrders = append(fanOrders, orderRecord{
			purchasedAt: o.PurchasedAt,
			eventID:     o.Event.ID,
			eventName:   o.Event.Name,
			total:       o.Total,
		})
	}

	if len(fanOrders) == 0 {
		return fanProfileResult{
			Found:           false,
			Email:           email,
			EventsPurchased: []string{},
			TicketTypes:     []string{},
		}, nil
	}

	// Sort orders chronologically to find first/last.
	sort.Slice(fanOrders, func(i, j int) bool {
		return fanOrders[i].purchasedAt < fanOrders[j].purchasedAt
	})

	var totalCents, vipCents int64
	eventSet := map[string]bool{}
	paidEventSet := map[string]bool{}
	eventNames := map[string]string{} // id -> name

	for _, o := range fanOrders {
		totalCents += o.total
		if holderHasVIP {
			vipCents += o.total
		}
		if o.eventID != "" {
			eventSet[o.eventID] = true
			if o.eventName != "" {
				eventNames[o.eventID] = o.eventName
			}
			if o.total > 0 {
				paidEventSet[o.eventID] = true
			}
		}
	}

	// Collect distinct event names in first-seen order.
	seenEvents := map[string]bool{}
	var eventsPurchased []string
	for _, o := range fanOrders {
		if o.eventID != "" && !seenEvents[o.eventID] {
			seenEvents[o.eventID] = true
			name := o.eventName
			if name == "" {
				name = o.eventID
			}
			eventsPurchased = append(eventsPurchased, name)
		}
	}

	// Collect distinct ticket type names (from tickets store, sorted).
	ttNames := make([]string, 0, len(ticketTypeNames))
	for n := range ticketTypeNames {
		ttNames = append(ttNames, n)
	}
	sort.Strings(ttNames)

	first := fanOrders[0]
	last := fanOrders[len(fanOrders)-1]
	firstName := first.eventName
	if firstName == "" {
		firstName = first.eventID
	}
	lastName := last.eventName
	if lastName == "" {
		lastName = last.eventID
	}

	if eventsPurchased == nil {
		eventsPurchased = []string{}
	}
	if ttNames == nil {
		ttNames = []string{}
	}

	return fanProfileResult{
		Found:           true,
		Email:           email,
		Name:            fanName,
		OptedIn:         fanOptedIn,
		FirstOrderDate:  first.purchasedAt,
		LastOrderDate:   last.purchasedAt,
		FirstEvent:      firstName,
		LastEvent:       lastName,
		OrdersCount:     len(fanOrders),
		TotalSpend:      round2(float64(totalCents) / 100.0),
		VIPSpend:        round2(float64(vipCents) / 100.0),
		EventCountAll:   len(eventSet),
		EventCountPaid:  len(paidEventSet),
		EventsPurchased: eventsPurchased,
		TicketTypes:     ttNames,
	}, nil
}

func newFansProfileCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile <email>",
		Short: "Per-fan rollup for a single email: orders, spend, events, ticket types",
		Long: "Compute a per-fan rollup for the given email from orders and tickets in " +
			"the local store. Returns a single JSON object. " +
			"vip_spend sums order totals for fans who hold any ticket whose ticketType.name " +
			"contains \"VIP\" (case-insensitive); the store does not link tickets to orders, " +
			"so this is a fan-level signal, not per-order. " +
			"If the email has no orders, returns found:false with empty/zero fields.\n\n" +
			"Exit codes:\n  0  profile built (found:true) or email not found (found:false)",
		Example: "  dice-fm-pp-cli fans profile fan@example.com --json",
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:typed-exit-codes":    "0",
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				return fmt.Errorf("usage: fans profile <email>")
			}
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			if s == nil {
				return printJSONFiltered(cmd.OutOrStdout(), fanProfileResult{
					Found:           false,
					Email:           args[0],
					EventsPurchased: []string{},
					TicketTypes:     []string{},
				}, flags)
			}
			defer s.Close()
			result, err := computeFanProfile(cmd.Context(), s.DB(), args[0])
			if err != nil {
				return fmt.Errorf("computing fan profile: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}
