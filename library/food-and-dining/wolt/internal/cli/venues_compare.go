// Copyright 2026 amit. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
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
		Short: "Compare open status, ETA window, and delivery configs across multiple venues",
		Long: "Fans out the per-venue dynamic endpoint for each slug and joins the\n" +
			"results into one structured payload. Useful for agent decisions that need\n" +
			"to weigh several venues at once.",
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
			out := struct {
				DeliveryMethod string            `json:"delivery_method"`
				Count          int               `json:"count"`
				Venues         []venueCompareRow `json:"venues"`
			}{DeliveryMethod: deliveryMethod}
			for _, slug := range slugs {
				row := venueCompareRow{Slug: slug}
				path := "https://consumer-api.wolt.com/order-xp/web/v1/venue/slug/" + slug + "/dynamic/?selected_delivery_method=" + deliveryMethod
				raw, err := c.Get(cmd.Context(), path, nil)
				if err != nil {
					row.Error = err.Error()
					out.Venues = append(out.Venues, row)
					continue
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
					out.Venues = append(out.Venues, row)
					continue
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
				out.Venues = append(out.Venues, row)
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
