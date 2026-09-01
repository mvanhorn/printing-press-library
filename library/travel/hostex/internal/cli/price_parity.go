// Hand-authored transcendence command. generate --force preserves this file.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelPriceParityCmd(flags *rootFlags) *cobra.Command {
	var flagProperty string
	var flagDays string

	cmd := &cobra.Command{
		Use:   "price-parity",
		Short: "Flag dates where a property's per-channel listing prices diverge across channels.",
		Long: "Queries Hostex live: reads the property's per-channel listings via pricing_ratios,\n" +
			"then pulls each listing's daily calendar and flags dates where the price spread\n" +
			"across channels exceeds the threshold — silent revenue loss no single call shows.\n\n" +
			"Live command (needs a valid token and multi-channel listings). For availability\n" +
			"vs channel-inventory mismatch use `oversell-watch` instead.",
		Example:     "  hostex-pp-cli price-parity --property 12345 --days 30 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--property=12704864"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would query pricing_ratios + listings/calendar for property %q over %s days\n", flagProperty, flagDays)
				return nil
			}
			if err := rejectLocalDataSource(flags); err != nil {
				return err
			}
			if strings.TrimSpace(flagProperty) == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--property is required"))
			}
			days, err := strconv.Atoi(strings.TrimSpace(flagDays))
			if err != nil || days <= 0 {
				return usageErr(fmt.Errorf("--days must be a positive integer"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// 1) Resolve the property's per-channel listings.
			ratRaw, err := c.Get(ctx, "/pricing_ratios", map[string]string{"property_id": flagProperty})
			if err != nil {
				return fmt.Errorf("querying pricing ratios: %w", err)
			}
			var rat struct {
				Channels []struct {
					ChannelType string `json:"channel_type"`
					ListingID   any    `json:"listing_id"`
					ListingName string `json:"listing_title"`
					Readonly    bool   `json:"readonly"`
				} `json:"channels"`
			}
			if err := json.Unmarshal(novUnwrapData(ratRaw), &rat); err != nil {
				return fmt.Errorf("decoding pricing ratios: %w", err)
			}
			if len(rat.Channels) < 2 {
				view := map[string]any{
					"property_id": flagProperty,
					"channels":    len(rat.Channels),
					"divergences": []any{},
					"note":        "fewer than two channel listings for this property; nothing to compare",
				}
				return novEmit(cmd, flags, view)
			}

			// 2) Pull each listing's daily calendar.
			now := nowUTC()
			start := now.Format("2006-01-02")
			end := now.AddDate(0, 0, days).Format("2006-01-02")
			listings := make([]map[string]any, 0, len(rat.Channels))
			for _, ch := range rat.Channels {
				listings = append(listings, map[string]any{"channel_type": ch.ChannelType, "listing_id": ch.ListingID})
			}
			calRaw, _, err := c.PostQueryWithParams(ctx, "/listings/calendar", nil, map[string]any{
				"start_date": start,
				"end_date":   end,
				"listings":   listings,
			})
			if err != nil {
				return fmt.Errorf("querying listing calendars: %w", err)
			}
			var cal struct {
				Listings []struct {
					ChannelType string `json:"channel_type"`
					ListingID   any    `json:"listing_id"`
					Calendar    []struct {
						Date  string `json:"date"`
						Price any    `json:"price"`
					} `json:"calendar"`
				} `json:"listings"`
			}
			if err := json.Unmarshal(novUnwrapData(calRaw), &cal); err != nil {
				return fmt.Errorf("decoding listing calendars: %w", err)
			}

			// 3) Index price per date per channel.
			type cell struct {
				Channel string  `json:"channel_type"`
				Price   float64 `json:"price"`
			}
			byDate := map[string][]cell{}
			for _, l := range cal.Listings {
				for _, d := range l.Calendar {
					p, ok := novToFloat(d.Price)
					if !ok || p <= 0 {
						continue
					}
					byDate[d.Date] = append(byDate[d.Date], cell{Channel: l.ChannelType, Price: p})
				}
			}

			type divRow struct {
				Date     string  `json:"date"`
				Min      float64 `json:"price_min"`
				Max      float64 `json:"price_max"`
				Spread   float64 `json:"price_spread"`
				Channels []cell  `json:"channels"`
			}
			rows := make([]divRow, 0)
			for date, cells := range byDate {
				if len(cells) < 2 {
					continue
				}
				min, max := cells[0].Price, cells[0].Price
				for _, c := range cells {
					if c.Price < min {
						min = c.Price
					}
					if c.Price > max {
						max = c.Price
					}
				}
				if max-min <= 0 {
					continue
				}
				rows = append(rows, divRow{Date: date, Min: min, Max: max, Spread: max - min, Channels: cells})
			}
			sort.SliceStable(rows, func(i, j int) bool {
				if rows[i].Spread != rows[j].Spread {
					return rows[i].Spread > rows[j].Spread
				}
				return rows[i].Date < rows[j].Date
			})

			view := struct {
				PropertyID    string   `json:"property_id"`
				Range         string   `json:"range"`
				Channels      int      `json:"channels"`
				DivergentDays int      `json:"divergent_days"`
				Divergences   []divRow `json:"divergences"`
			}{
				PropertyID:    flagProperty,
				Range:         start + ".." + end,
				Channels:      len(rat.Channels),
				DivergentDays: len(rows),
				Divergences:   rows,
			}
			return novEmit(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&flagProperty, "property", "", "Property ID to check (required)")
	cmd.Flags().StringVar(&flagDays, "days", "30", "Number of days forward to compare")
	return cmd
}
