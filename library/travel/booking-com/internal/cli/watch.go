// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type watchRow struct {
	ID          int     `json:"id"`
	Slug        string  `json:"slug"`
	Country     string  `json:"country"`
	Checkin     string  `json:"checkin"`
	Checkout    string  `json:"checkout"`
	GroupAdults int     `json:"group_adults"`
	AddedAt     string  `json:"added_at"`
	LatestPrice float64 `json:"latest_price,omitempty"`
	MedianPrice float64 `json:"median_price,omitempty"`
	DropPct     float64 `json:"drop_pct,omitempty"`
	Currency    string  `json:"currency,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "watch", Short: "Track hotel price watches", Annotations: map[string]string{"mcp:read-only": "true"}, RunE: parentNoSubcommandRunE(flags)}
	cmd.AddCommand(newWatchAddCmd(flags), newWatchListCmd(flags), newWatchRunCmd(flags))
	return cmd
}

func newWatchAddCmd(flags *rootFlags) *cobra.Command {
	var slug, country, checkin, checkout string
	var adults int
	cmd := &cobra.Command{
		Use:         "add",
		Short:       "Add a local hotel price watch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]watchRow, 0))
			}
			if slug == "" || country == "" || checkin == "" || checkout == "" {
				return cmd.Help()
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("watch add: %w", err)
			}
			defer st.Close()
			added := time.Now().UTC().Format(time.RFC3339)
			res, err := st.DB().ExecContext(cmd.Context(), `INSERT INTO watches (slug,country,checkin,checkout,group_adults,added_at) VALUES (?,?,?,?,?,?)`, slug, country, checkin, checkout, adults, added)
			if err != nil {
				return fmt.Errorf("watch add: %w", err)
			}
			id, _ := res.LastInsertId()
			return flags.printJSON(cmd, []watchRow{{ID: int(id), Slug: slug, Country: country, Checkin: checkin, Checkout: checkout, GroupAdults: adults, AddedAt: added}})
		},
	}
	cmd.Flags().StringVar(&slug, "slug", "", "Hotel slug")
	cmd.Flags().StringVar(&country, "country", "", "Hotel country code")
	cmd.Flags().StringVar(&checkin, "checkin", "", "Check-in date YYYY-MM-DD")
	cmd.Flags().StringVar(&checkout, "checkout", "", "Check-out date YYYY-MM-DD")
	cmd.Flags().IntVar(&adults, "adults", 2, "Adult guests")
	return cmd
}

func newWatchListCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Short:       "List local hotel price watches",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]watchRow, 0))
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("watch list: %w", err)
			}
			defer st.Close()
			rows, err := st.DB().QueryContext(cmd.Context(), `SELECT id,slug,country,checkin,checkout,group_adults,added_at FROM watches ORDER BY id`)
			if err != nil {
				return fmt.Errorf("watch list: %w", err)
			}
			defer rows.Close()
			out := make([]watchRow, 0)
			for rows.Next() {
				var w watchRow
				if err := rows.Scan(&w.ID, &w.Slug, &w.Country, &w.Checkin, &w.Checkout, &w.GroupAdults, &w.AddedAt); err != nil {
					return fmt.Errorf("watch list: %w", err)
				}
				out = append(out, w)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("watch list: %w", err)
			}
			return flags.printJSON(cmd, out)
		},
	}
}

func newWatchRunCmd(flags *rootFlags) *cobra.Command {
	var minPct float64
	cmd := &cobra.Command{
		Use:         "run",
		Short:       "Refresh watches and report price drops",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return flags.printJSON(cmd, make([]watchRow, 0))
			}
			st, err := openBookingStore(cmd.Context())
			if err != nil {
				return fmt.Errorf("watch run: %w", err)
			}
			defer st.Close()
			watches, err := loadWatches(cmd.Context(), st.DB())
			if err != nil {
				return fmt.Errorf("watch run: %w", err)
			}
			c, err := flags.newClient()
			if err != nil {
				return fmt.Errorf("watch run: %w", err)
			}
			out := make([]watchRow, 0)
			for _, w := range watches {
				checkin, _ := time.Parse(dateOnly, w.Checkin)
				checkout, _ := time.Parse(dateOnly, w.Checkout)
				data, err := c.Get(hotelPath(w.Country, w.Slug), hotelParams(checkin, checkout, w.GroupAdults))
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: watch %d failed: %v\n", w.ID, err)
					continue
				}
				prop, err := parseHotel(data)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: watch %d parse failed: %v\n", w.ID, err)
					continue
				}
				price, currency := hotelPrice(prop)
				if price > 0 {
					_ = insertPrice(cmd.Context(), st.DB(), w.Slug, w.Country, w.Checkin, w.Checkout, w.GroupAdults, currency, price)
				}
				history, err := priceHistory(cmd.Context(), st.DB(), w)
				if err != nil || len(history) == 0 {
					continue
				}
				latest := history[len(history)-1]
				median := medianFloat(append([]float64(nil), history...))
				drop := 0.0
				if median > 0 {
					drop = (median - latest) / median * 100
				}
				if drop >= minPct {
					w.LatestPrice, w.MedianPrice, w.DropPct, w.Currency = latest, median, drop, currency
					out = append(out, w)
				}
			}
			sort.Slice(out, func(i, j int) bool { return out[i].DropPct > out[j].DropPct })
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().Float64Var(&minPct, "min-pct", 10, "Minimum drop percentage below trailing median")
	return cmd
}

func loadWatches(ctx context.Context, db *sql.DB) ([]watchRow, error) {
	rows, err := db.QueryContext(ctx, `SELECT id,slug,country,checkin,checkout,group_adults,added_at FROM watches ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]watchRow, 0)
	for rows.Next() {
		var w watchRow
		if err := rows.Scan(&w.ID, &w.Slug, &w.Country, &w.Checkin, &w.Checkout, &w.GroupAdults, &w.AddedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func priceHistory(ctx context.Context, db *sql.DB, w watchRow) ([]float64, error) {
	rows, err := db.QueryContext(ctx, `SELECT price FROM price_history WHERE slug=? AND country=? AND checkin=? AND checkout=? AND group_adults=? ORDER BY observed_at`,
		w.Slug, w.Country, w.Checkin, w.Checkout, w.GroupAdults)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]float64, 0)
	for rows.Next() {
		var price float64
		if err := rows.Scan(&price); err != nil {
			return nil, err
		}
		out = append(out, price)
	}
	return out, rows.Err()
}
