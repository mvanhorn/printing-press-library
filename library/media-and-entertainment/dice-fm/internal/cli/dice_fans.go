// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE "fans" analytics commands: repeat buyers, top spenders,
// and opted-in contact export, all computed from the local order store.
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

// fansRepeatRow is one repeat-buyer aggregate.
type fansRepeatRow struct {
	Email       string `json:"email"`
	Name        string `json:"name"`
	EventsCount int    `json:"events_count"`
	// TotalSpend is the sum of the fan's paid order totals; 0 for fans whose
	// orders were all free/comp/guest-list, so a $0 row with a high
	// EventsCount is a free attendee, not a summing gap.
	TotalSpend float64  `json:"total_spend"`
	EventIDs   []string `json:"event_ids"`
}

// computeFansRepeat groups orders by fan email and keeps fans who bought into at
// least minEvents distinct events, optionally floored by a since date. Sorted by
// events_count desc, then total_spend desc.
func computeFansRepeat(ctx context.Context, db *sql.DB, minEvents int, since string) ([]fansRepeatRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	type agg struct {
		name       string
		totalCents int64
		eventSet   map[string]bool
		order      []string
	}
	groups := map[string]*agg{}
	for _, o := range orders {
		email := o.Fan.Email
		if email == "" {
			continue
		}
		if !dateFloorMatch(o.PurchasedAt, since) {
			continue
		}
		g := groups[email]
		if g == nil {
			g = &agg{eventSet: map[string]bool{}}
			groups[email] = g
		}
		if g.name == "" {
			g.name = joinName(o.Fan.FirstName, o.Fan.LastName)
		}
		g.totalCents += o.Total
		if o.Event.ID != "" && !g.eventSet[o.Event.ID] {
			g.eventSet[o.Event.ID] = true
			g.order = append(g.order, o.Event.ID)
		}
	}

	rows := make([]fansRepeatRow, 0)
	for email, g := range groups {
		if len(g.eventSet) < minEvents {
			continue
		}
		rows = append(rows, fansRepeatRow{
			Email:       email,
			Name:        g.name,
			EventsCount: len(g.eventSet),
			TotalSpend:  round2(float64(g.totalCents) / 100.0),
			EventIDs:    g.order,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].EventsCount != rows[j].EventsCount {
			return rows[i].EventsCount > rows[j].EventsCount
		}
		if rows[i].TotalSpend != rows[j].TotalSpend {
			return rows[i].TotalSpend > rows[j].TotalSpend
		}
		return rows[i].Email < rows[j].Email
	})
	return rows, nil
}

// fansTopRow is one top-spender aggregate.
type fansTopRow struct {
	Email       string  `json:"email"`
	Name        string  `json:"name"`
	TotalSpend  float64 `json:"total_spend"`
	OrdersCount int     `json:"orders_count"`
}

// computeFansTop groups orders by fan email, optionally filtered to one event,
// sorted by total spend desc and limited to n rows (n<=0 returns all).
func computeFansTop(ctx context.Context, db *sql.DB, eventFilter string, n int) ([]fansTopRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	type agg struct {
		name        string
		totalCents  int64
		ordersCount int
	}
	groups := map[string]*agg{}
	for _, o := range orders {
		if eventFilter != "" && o.Event.ID != eventFilter {
			continue
		}
		email := o.Fan.Email
		if email == "" {
			continue
		}
		g := groups[email]
		if g == nil {
			g = &agg{}
			groups[email] = g
		}
		if g.name == "" {
			g.name = joinName(o.Fan.FirstName, o.Fan.LastName)
		}
		g.totalCents += o.Total
		g.ordersCount++
	}

	rows := make([]fansTopRow, 0, len(groups))
	for email, g := range groups {
		rows = append(rows, fansTopRow{
			Email:       email,
			Name:        g.name,
			TotalSpend:  round2(float64(g.totalCents) / 100.0),
			OrdersCount: g.ordersCount,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalSpend != rows[j].TotalSpend {
			return rows[i].TotalSpend > rows[j].TotalSpend
		}
		return rows[i].Email < rows[j].Email
	})
	if n > 0 && len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}

// fansOptinRow is one opted-in contact row.
type fansOptinRow struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
	City      string `json:"city"`
	Country   string `json:"country"`
}

// computeFansOptin returns deduplicated opted-in fan contacts, filtered by an
// optional event, country (case-insensitive equality), and city (case-insensitive
// substring).
func computeFansOptin(ctx context.Context, db *sql.DB, eventFilter, country, city string) ([]fansOptinRow, error) {
	orders, err := readOrders(ctx, db)
	if err != nil {
		return nil, err
	}
	wantCountry := strings.ToLower(country)
	wantCity := strings.ToLower(city)

	seen := map[string]bool{}
	rows := make([]fansOptinRow, 0)
	for _, o := range orders {
		if !o.Fan.OptInPartners {
			continue
		}
		if eventFilter != "" && o.Event.ID != eventFilter {
			continue
		}
		if wantCountry != "" && strings.ToLower(o.IPCountry) != wantCountry {
			continue
		}
		if wantCity != "" && !strings.Contains(strings.ToLower(o.IPCity), wantCity) {
			continue
		}
		email := o.Fan.Email
		// An opt-in export feeds an email tool; a fan with no email is useless
		// and an unbounded number of them would emit duplicate blank rows.
		if email == "" {
			continue
		}
		if seen[email] {
			continue
		}
		seen[email] = true
		rows = append(rows, fansOptinRow{
			FirstName: o.Fan.FirstName,
			LastName:  o.Fan.LastName,
			Email:     o.Fan.Email,
			Phone:     o.Fan.PhoneNumber,
			City:      o.IPCity,
			Country:   o.IPCountry,
		})
	}
	return rows, nil
}

func newFansCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fans",
		Short: "Fan analytics: repeat buyers, top spenders, opted-in contacts, segments, profiles",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newFansRepeatCmd(flags))
	cmd.AddCommand(newFansTopCmd(flags))
	cmd.AddCommand(newFansOptinCmd(flags))
	cmd.AddCommand(newFansSegmentCmd(flags))
	cmd.AddCommand(newFansProfileCmd(flags))
	return cmd
}

// pp:data-source local
func newFansRepeatCmd(flags *rootFlags) *cobra.Command {
	var minEvents int
	var since string
	cmd := &cobra.Command{
		Use:   "repeat",
		Short: "Find fans who bought tickets to multiple events",
		Long: "Find fans who bought tickets to at least --min-events distinct events.\n" +
			"Sorted by events_count desc, then total_spend desc.\n\n" +
			"total_spend sums each fan's paid order value (DICE order totals). Free,\n" +
			"comp, and guest-list attendees show total_spend=0 even with a high\n" +
			"events_count — that is faithful order data, not an undercount. Treat a\n" +
			"$0 repeat attendee as a free/RSVP fan, not a buyer; use 'fans top' to\n" +
			"rank by spend.",
		Example:     "  dice-fm-pp-cli fans repeat --min-events 2 --since 2026-01-01 --json",
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
				return printJSONFiltered(cmd.OutOrStdout(), []fansRepeatRow{}, flags)
			}
			defer s.Close()
			rows, err := computeFansRepeat(cmd.Context(), s.DB(), minEvents, since)
			if err != nil {
				return fmt.Errorf("computing repeat fans: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().IntVar(&minEvents, "min-events", 2, "Minimum distinct events a fan must have bought into")
	cmd.Flags().StringVar(&since, "since", "", "Only count orders purchased on or after this date (YYYY-MM-DD)")
	return cmd
}

func newFansTopCmd(flags *rootFlags) *cobra.Command {
	var event string
	var n int
	cmd := &cobra.Command{
		Use:         "top",
		Short:       "Rank ticket buyers by total spend",
		Example:     "  dice-fm-pp-cli fans top --event RXZlbnQ6MTIzNDU= --n 20 --json",
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
				return printJSONFiltered(cmd.OutOrStdout(), []fansTopRow{}, flags)
			}
			defer s.Close()
			rows, err := computeFansTop(cmd.Context(), s.DB(), event, n)
			if err != nil {
				return fmt.Errorf("computing top fans: %w", err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Limit to a single event ID")
	cmd.Flags().IntVar(&n, "n", 20, "Maximum number of fans to return (0 = all)")
	return cmd
}

func newFansOptinCmd(flags *rootFlags) *cobra.Command {
	var event, country, city string
	cmd := &cobra.Command{
		Use:         "optin",
		Short:       "Export opted-in fan contacts, filterable by geography",
		Example:     "  dice-fm-pp-cli fans optin --country GB --city London --csv",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := openStoreForRead(cmd.Context(), diceCLIName)
			if err != nil {
				return err
			}
			var rows []fansOptinRow
			if s != nil {
				defer s.Close()
				rows, err = computeFansOptin(cmd.Context(), s.DB(), event, country, city)
				if err != nil {
					return fmt.Errorf("computing opted-in fans: %w", err)
				}
			}
			if rows == nil {
				rows = []fansOptinRow{}
			}
			arr, err := json.Marshal(rows)
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), arr, flags)
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "Limit to a single event ID")
	cmd.Flags().StringVar(&country, "country", "", "Filter by IP country code (case-insensitive exact match)")
	cmd.Flags().StringVar(&city, "city", "", "Filter by IP city (case-insensitive substring match)")
	return cmd
}
