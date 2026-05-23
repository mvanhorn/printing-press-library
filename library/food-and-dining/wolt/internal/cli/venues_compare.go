// Copyright 2026 amit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/wolt/internal/cliutil"
)

type venueCompareRow struct {
	Slug                string `json:"slug"`
	Open                *bool  `json:"open,omitempty"`
	OpenStatusText      string `json:"open_status_text,omitempty"`
	NextClose           string `json:"next_close,omitempty"`
	NextOpen            string `json:"next_open,omitempty"`
	DeliveryConfigCount int    `json:"delivery_config_count,omitempty"`
	OrderMinimum        any    `json:"order_minimum,omitempty"`
	Error               string `json:"error,omitempty"`
}

func newVenuesCompareCmd(flags *rootFlags) *cobra.Command {
	var slugsCSV, deliveryMethod string
	cmd := &cobra.Command{
		Use:   "venues-compare",
		Short: "Compare open status, next-close time, delivery configs, and order minimum across multiple venues",
		Long: "Fans out the per-venue dynamic endpoint for each slug and joins the\n" +
			"results into one structured payload. Useful for agent decisions that need\n" +
			"to weigh several venues at once. Returns open status, next-close/open\n" +
			"timestamps, delivery config count, and order minimum per venue. For\n" +
			"delivery-time ETAs in minutes, use `venues-now` instead — that data\n" +
			"comes from the discovery endpoint, not the per-venue dynamic one.",
		Example: "  wolt-pp-cli venues-compare --slugs noodle-story-kamppi,puttes-bar-pizza --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			slugs := splitCSVLowerWCompare(slugsCSV)
			if len(slugs) < 2 {
				return fmt.Errorf("must pass at least 2 slugs via --slugs a,b[,c]")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// PATCH(venues-compare-parallel-fanout): fan out per-venue dynamic
			// fetches concurrently via cliutil.FanoutRun. Sequential N calls
			// scaled linearly with the slug count; parallelizing them with
			// the default concurrency of 4 caps end-to-end latency at roughly
			// the slowest single call regardless of N (for typical N <= 4).
			fetchOne := func(ctx context.Context, slug string) (venueCompareRow, error) {
				row := venueCompareRow{Slug: slug}
				// PATCH(venues-compare-url-escape): escape slug + delivery method
				// before building the URL; without this a slug containing %, +, or
				// other URL-special characters produces a malformed request.
				path := "https://consumer-api.wolt.com/order-xp/web/v1/venue/slug/" +
					url.PathEscape(slug) + "/dynamic/?selected_delivery_method=" + url.QueryEscape(deliveryMethod)
				raw, err := c.Get(ctx, path, nil)
				if err != nil {
					row.Error = err.Error()
					return row, nil
				}
				var dyn struct {
					Venue struct {
						Online             *bool `json:"online,omitempty"`
						DeliveryOpenStatus struct {
							Value     string `json:"value,omitempty"`
							IsOpen    *bool  `json:"is_open,omitempty"`
							NextClose string `json:"next_close,omitempty"`
							NextOpen  string `json:"next_open,omitempty"`
						} `json:"delivery_open_status"`
						DeliveryConfigs []any `json:"delivery_configs,omitempty"`
					} `json:"venue"`
					OrderMinimum any `json:"order_minimum,omitempty"`
				}
				if err := json.Unmarshal(raw, &dyn); err != nil {
					row.Error = "parse: " + err.Error()
					return row, nil
				}
				if dyn.Venue.DeliveryOpenStatus.IsOpen != nil {
					row.Open = dyn.Venue.DeliveryOpenStatus.IsOpen
				} else if dyn.Venue.Online != nil {
					row.Open = dyn.Venue.Online
				}
				row.OpenStatusText = strings.TrimSpace(dyn.Venue.DeliveryOpenStatus.Value)
				row.NextClose = dyn.Venue.DeliveryOpenStatus.NextClose
				row.NextOpen = dyn.Venue.DeliveryOpenStatus.NextOpen
				row.DeliveryConfigCount = len(dyn.Venue.DeliveryConfigs)
				row.OrderMinimum = dyn.OrderMinimum
				return row, nil
			}
			results, _ := cliutil.FanoutRun(
				cmd.Context(),
				slugs,
				func(s string) string { return s },
				fetchOne,
			)
			out := struct {
				DeliveryMethod string            `json:"delivery_method"`
				Count          int               `json:"count"`
				Venues         []venueCompareRow `json:"venues"`
			}{DeliveryMethod: deliveryMethod}
			for _, r := range results {
				out.Venues = append(out.Venues, r.Value)
			}
			out.Count = len(out.Venues)
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&slugsCSV, "slugs", "", "Comma-separated venue slugs (required, >=2)")
	cmd.Flags().StringVar(&deliveryMethod, "delivery-method", "homedelivery", "Delivery method: homedelivery, takeaway, eatin")
	return cmd
}

func splitCSVLowerWCompare(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
