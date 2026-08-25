// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: true-cost fee comparison across an area.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type fpFeeRow struct {
	Code           string  `json:"code"`
	Name           string  `json:"name"`
	DeliveryFee    float64 `json:"delivery_fee"`
	MinOrderAmount float64 `json:"minimum_order_amount"`
	ServiceFeePct  float64 `json:"service_fee_percentage"`
	VatPct         float64 `json:"vat_percentage"`
	ServiceTaxPct  float64 `json:"service_tax_percentage"`
	EstimatedTotal float64 `json:"estimated_total_on_basket"`
	Overhead       float64 `json:"overhead_over_basket"`
	Rating         float64 `json:"rating"`
}

type fpFeeView struct {
	BasketValue            float64    `json:"basket_value"`
	Currency               string     `json:"currency"`
	Vendors                []fpFeeRow `json:"vendors"`
	ScannedVendors         int        `json:"scanned_vendors"`
	AvailableCount         int        `json:"available_count"`
	MaxScanPages           int        `json:"max_scan_pages"`
	DeliveryFeePricedCount int        `json:"delivery_fee_priced_count"`
	DeliveryFeeTotal       int        `json:"delivery_fee_vendor_count"`
	UniformFees            bool       `json:"uniform_fee_structure"`
	Note                   string     `json:"note,omitempty"`
}

// fpFeesAreUniform reports whether every row carries an identical cost
// structure. When it does, the --sort key has nothing to order by and the
// result looks like a broken sort unless the flatness is stated outright.
func fpFeesAreUniform(rows []fpFeeRow) bool {
	if len(rows) < 2 {
		return false
	}
	first := rows[0]
	for _, r := range rows[1:] {
		if r.DeliveryFee != first.DeliveryFee ||
			r.MinOrderAmount != first.MinOrderAmount ||
			r.ServiceFeePct != first.ServiceFeePct ||
			r.EstimatedTotal != first.EstimatedTotal {
			return false
		}
	}
	return true
}

func newNovelFeesCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng     float64
		basket       float64
		sortKey      string
		limit        int
		maxScanPages int
		country      string
		currency     string
	)

	cmd := &cobra.Command{
		Use:   "fees",
		Short: "Compare the full cost structure across an area: delivery fee, minimum order, service fee and VAT together.",
		Long: "Compare what ordering actually costs across every vendor in an area.\n\n" +
			"Headline delivery fee is often not the real cost: minimum order, service fee and\n" +
			"VAT can dominate. This estimates total spend on a basket of a given size.\n" +
			"Do NOT use this for your saved address specifically; 'home' does that.",
		Example:     "  foodpanda-pp-cli fees --latitude 24.8607 --longitude 67.0011 --sort total --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "fees")
			}
			if lat == 0 || lng == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--latitude and --longitude are required"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			sweep, err := fpSweep(ctx, c, fpDiscoVendors, fpQuery{Lat: lat, Lng: lng, Country: country}, maxScanPages)
			if err != nil {
				return err
			}

			rows := make([]fpFeeRow, 0, len(sweep.Vendors))
			for _, v := range sweep.Vendors {
				// Effective basket is at least the vendor's minimum order.
				eff := basket
				if v.MinOrderAmount > eff {
					eff = v.MinOrderAmount
				}
				svc := eff * v.ServiceFeePct / 100
				tax := eff * v.ServiceTaxPct / 100
				vat := eff * v.VatPct / 100
				total := eff + v.MinDeliveryFee + svc + tax + vat
				rows = append(rows, fpFeeRow{
					Code: v.Code, Name: v.Name,
					DeliveryFee: fpRound2(v.MinDeliveryFee), MinOrderAmount: fpRound2(v.MinOrderAmount),
					ServiceFeePct: v.ServiceFeePct, VatPct: v.VatPct, ServiceTaxPct: v.ServiceTaxPct,
					EstimatedTotal: fpRound2(total), Overhead: fpRound2(total - eff), Rating: v.Rating,
				})
			}

			switch sortKey {
			case "total":
				sortFeeRows(rows, func(a, b fpFeeRow) bool { return a.EstimatedTotal < b.EstimatedTotal })
			case "overhead":
				sortFeeRows(rows, func(a, b fpFeeRow) bool { return a.Overhead < b.Overhead })
			case "fee":
				sortFeeRows(rows, func(a, b fpFeeRow) bool { return a.DeliveryFee < b.DeliveryFee })
			case "min-order":
				sortFeeRows(rows, func(a, b fpFeeRow) bool { return a.MinOrderAmount < b.MinOrderAmount })
			case "rating":
				sortFeeRows(rows, func(a, b fpFeeRow) bool { return a.Rating > b.Rating })
			}
			rows = fpTrim(rows, limit)

			view := fpFeeView{
				BasketValue: fpRound2(basket), Currency: currency, Vendors: rows,
				ScannedVendors: sweep.Scanned, AvailableCount: sweep.Available, MaxScanPages: sweep.MaxScanPage,
			}
			pricedN, totalN, feeNote := fpFeePricing(sweep.Vendors)
			feeMissing := feeNote != ""
			view.DeliveryFeePricedCount, view.DeliveryFeeTotal = pricedN, totalN
			if feeNote != "" {
				view.Note = feeNote
			}
			// A flat market makes --sort look broken: every row is identical,
			// so the requested ordering visibly does nothing. Say so, rather
			// than letting the user assume the sort was ignored.
			view.UniformFees = fpFeesAreUniform(rows)
			if view.UniformFees && !feeMissing {
				view.Note = fmt.Sprintf("every one of the %d vendors here has the same fee structure (delivery %.0f, minimum order %.0f), so --sort %s cannot change the order; foodpanda is running a single flat rate at this coordinate",
					len(rows), rows[0].DeliveryFee, rows[0].MinOrderAmount, sortKey)
			}
			if len(rows) == 0 && sweep.ScanCapHit {
				view.Note = fmt.Sprintf("scanned %d vendors across %d page(s) without a match; raise --max-scan-pages to widen the search",
					sweep.Scanned, sweep.PagesRead)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No vendors found at that coordinate.")
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "True cost on a %s %.0f basket (%d of %d vendors scanned)\n",
				currency, basket, sweep.Scanned, sweep.Available)
			if feeMissing {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %s\n", feeNote)
				fmt.Fprintf(cmd.OutOrStdout(), "note: only %d of %d vendors are delivery-fee priced; 0 means unpriced, not free\n", pricedN, totalN)
			} else if view.UniformFees {
				fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n", view.Note)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Code, truncate(r.Name, 30),
					fmt.Sprintf("%.0f", r.DeliveryFee), fmt.Sprintf("%.0f", r.MinOrderAmount),
					fmt.Sprintf("%.1f%%", r.ServiceFeePct), fmt.Sprintf("%.0f", r.Overhead),
					fmt.Sprintf("%.0f", r.EstimatedTotal),
				})
			}
			return flags.printTable(cmd, []string{"CODE", "NAME", "FEE", "MIN ORDER", "SVC", "OVERHEAD", "EST TOTAL"}, out)
		},
	}

	cmd.Flags().Float64Var(&lat, "latitude", 0, "Latitude of the delivery point (required)")
	cmd.Flags().Float64Var(&lng, "longitude", 0, "Longitude of the delivery point (required)")
	cmd.Flags().Float64Var(&basket, "basket", 1000, "Basket value used to estimate percentage-based fees")
	cmd.Flags().StringVar(&sortKey, "sort", "total", "Sort by: total, overhead, fee, min-order, rating")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum vendors to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 4, "Maximum listing pages to scan (48 vendors per page)")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().StringVar(&currency, "currency", "PKR", "Currency label for output")
	return cmd
}

func sortFeeRows(rows []fpFeeRow, less func(a, b fpFeeRow) bool) {
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && less(rows[j], rows[j-1]); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
}
