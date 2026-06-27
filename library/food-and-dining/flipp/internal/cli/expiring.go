// Copyright 2026 mlabrenz and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func withinExpiringWindow(expiresAt, now, deadline time.Time) bool {
	return expiresAt.After(now) && !expiresAt.After(deadline)
}

// pp:data-source live
func newNovelExpiringCmd(flags *rootFlags) *cobra.Command {
	var flagDays string
	var flagZip string
	var flagLocale string
	var flagLimit int

	cmd := &cobra.Command{
		Use:         "expiring",
		Short:       "Find local flyer and coupon savings that expire within a chosen window.",
		Example:     "  flipp-pp-cli expiring --days 3 --zip 85001 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch Flipp flyer/coupon data near %s and filter expiring savings\n", flagZip)
				return nil
			}
			days, err := strconv.Atoi(flagDays)
			if err != nil || days < 0 {
				return usageErr(fmt.Errorf("--days must be a non-negative integer"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := fetchFlippData(ctx, c, flagZip, flagLocale)
			if err != nil {
				return err
			}
			now := time.Now()
			deadline := now.Add(time.Duration(days) * 24 * time.Hour)
			type expiringRow struct {
				Kind       string   `json:"kind"`
				ID         int      `json:"id"`
				Merchant   string   `json:"merchant,omitempty"`
				Name       string   `json:"name,omitempty"`
				Categories []string `json:"categories,omitempty"`
				ValidTo    string   `json:"valid_to,omitempty"`
				DaysLeft   int      `json:"days_left"`
			}
			rows := []expiringRow{}
			for _, flyer := range data.Flyers {
				t, ok := parseFlippTime(flyer.ValidTo)
				if !ok || !withinExpiringWindow(t, now, deadline) {
					continue
				}
				rows = append(rows, expiringRow{Kind: "flyer", ID: flyer.ID, Merchant: flyer.Merchant, Name: flyer.Name, Categories: flyer.Categories, ValidTo: flyer.ValidTo, DaysLeft: int(t.Sub(now).Hours() / 24)})
			}
			allCoupons := append([]flippCoupon{}, data.Coupons...)
			allCoupons = append(allCoupons, data.LoyaltyProgramCoupons...)
			allCoupons = append(allCoupons, data.FlyerItemCoupons...)
			for _, coupon := range allCoupons {
				t, ok := parseFlippTime(coupon.ValidTo)
				if !ok || !withinExpiringWindow(t, now, deadline) {
					continue
				}
				rows = append(rows, expiringRow{Kind: "coupon", ID: coupon.ID, Merchant: coupon.Merchant, Name: firstNonEmpty(coupon.Name, coupon.Description, coupon.Value), ValidTo: coupon.ValidTo, DaysLeft: int(t.Sub(now).Hours() / 24)})
			}
			if flagLimit > 0 && len(rows) > flagLimit {
				rows = rows[:flagLimit]
			}
			view := struct {
				Zip     string        `json:"zip"`
				Days    int           `json:"days"`
				Results []expiringRow `json:"results"`
			}{Zip: flagZip, Days: days, Results: rows}
			tableRows := [][]string{}
			for _, row := range rows {
				tableRows = append(tableRows, []string{row.Kind, strconv.Itoa(row.ID), row.Merchant, row.Name, row.ValidTo, strings.Join(row.Categories, ", ")})
			}
			return printRowsOrJSON(cmd, flags, view, []string{"Kind", "ID", "Merchant", "Name", "Valid To", "Categories"}, tableRows)
		},
	}
	cmd.Flags().StringVar(&flagDays, "days", "3", "Number of days ahead to include")
	cmd.Flags().StringVar(&flagZip, "zip", "85001", "ZIP or postal code used to find local savings")
	cmd.Flags().StringVar(&flagLocale, "locale", defaultFlippLocale, "Flipp locale, such as en-us or en-ca")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum expiring rows to return")
	return cmd
}
