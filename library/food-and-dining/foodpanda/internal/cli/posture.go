// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live
// Novel command: advertising / placement posture across an area.
//
// IMPORTANT: these are advertising and placement signals only. foodpanda's
// "NCR" family means Non-Commission Revenue, i.e. the advertising business.
// Merchant commission rates are contract terms and appear in NO consumer-facing
// foodpanda response; this command does not and cannot report them.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type fpPostureRow struct {
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Cuisine          string  `json:"cuisine"`
	AdBuyer          bool    `json:"ad_buyer"`
	NCRPricingModel  string  `json:"ncr_pricing_model"`
	IsPromoted       bool    `json:"is_promoted"`
	IsPremium        bool    `json:"is_premium"`
	PremiumPosition  int     `json:"premium_position"`
	VendorPoints     int     `json:"vendor_points"`
	PreferredPartner bool    `json:"is_preferred_partner"`
	OwnFleet         bool    `json:"vendor_own_delivery"`
	DeliveryProvider string  `json:"delivery_provider,omitempty"`
	Rating           float64 `json:"rating"`
	ReviewCount      int     `json:"review_count"`
	Tag              string  `json:"tag,omitempty"`
}

type fpPostureView struct {
	Disclaimer     string         `json:"disclaimer"`
	Vendors        []fpPostureRow `json:"vendors"`
	AdBuyerCount   int            `json:"ad_buyer_count"`
	PromotedCount  int            `json:"promoted_count"`
	PremiumCount   int            `json:"premium_count"`
	OwnFleetCount  int            `json:"vendor_own_delivery_count"`
	ScannedVendors int            `json:"scanned_vendors"`
	AvailableCount int            `json:"available_count"`
	MaxScanPages   int            `json:"max_scan_pages"`
	Note           string         `json:"note,omitempty"`
}

const fpPostureDisclaimer = "Advertising and placement signals only. foodpanda's NCR family means " +
	"Non-Commission Revenue (its ad product). Merchant commission rates are contract terms and are " +
	"not present in any consumer-facing foodpanda response."

func newNovelPostureCmd(flags *rootFlags) *cobra.Command {
	var (
		lat, lng     float64
		sortKey      string
		limit        int
		maxScanPages int
		country      string
		adsOnly      bool
	)

	cmd := &cobra.Command{
		Use:   "posture",
		Short: "Rank vendors by advertising and placement signals: CPC ad participation, promoted and premium status, and ranking score.",
		Long: "Rank vendors in an area by observable advertising and placement posture.\n\n" +
			"Signals: ncr_pricing_model (foodpanda's CPC ad product), is_promoted, is_premium,\n" +
			"premium_position, vendor_points, is_preferred_partner and delivery provider.\n\n" +
			"This does NOT report merchant commission rates. Commission is a B2B contract term\n" +
			"that appears in no consumer-facing foodpanda response.",
		Example:     "  foodpanda-pp-cli posture --latitude 31.5204 --longitude 74.3587 --ads-only --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "posture")
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

			vendors := sweep.Vendors
			if adsOnly {
				kept := make([]fpVendor, 0, len(vendors))
				for _, v := range vendors {
					if v.AdBuyer() {
						kept = append(kept, v)
					}
				}
				vendors = kept
			}
			fpSortVendors(vendors, sortKey)

			view := fpPostureView{
				Disclaimer: fpPostureDisclaimer, ScannedVendors: sweep.Scanned,
				AvailableCount: sweep.Available, MaxScanPages: sweep.MaxScanPage,
			}
			// Counts describe the full swept set, not the trimmed output.
			for _, v := range sweep.Vendors {
				if v.AdBuyer() {
					view.AdBuyerCount++
				}
				if v.IsPromoted {
					view.PromotedCount++
				}
				if v.IsPremium {
					view.PremiumCount++
				}
				if !v.HasDeliveryProvider {
					view.OwnFleetCount++
				}
			}

			rows := make([]fpPostureRow, 0, len(vendors))
			for _, v := range fpTrim(vendors, limit) {
				rows = append(rows, fpPostureRow{
					Code: v.Code, Name: v.Name, Cuisine: v.PrimaryCuisine(),
					AdBuyer: v.AdBuyer(), NCRPricingModel: v.NCRPricingModel,
					IsPromoted: v.IsPromoted, IsPremium: v.IsPremium, PremiumPosition: int(v.PremiumPosition),
					VendorPoints: int(v.VendorPoints), PreferredPartner: v.IsPreferredPartner,
					OwnFleet: !v.HasDeliveryProvider, DeliveryProvider: v.DeliveryProvider,
					Rating: v.Rating, ReviewCount: int(v.ReviewNumber), Tag: v.Tag,
				})
			}
			view.Vendors = rows
			if len(rows) == 0 && sweep.ScanCapHit {
				view.Note = fmt.Sprintf("scanned %d vendors across %d page(s) without a match; raise --max-scan-pages to widen the search",
					sweep.Scanned, sweep.PagesRead)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No vendors matched.")
				if view.Note != "" {
					fmt.Fprintln(cmd.OutOrStdout(), view.Note)
				}
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d of %d scanned vendors buy placement  (promoted %d, premium %d, own-fleet %d)\n",
				view.AdBuyerCount, sweep.Scanned, view.PromotedCount, view.PremiumCount, view.OwnFleetCount)
			fmt.Fprintf(cmd.OutOrStdout(), "note: %s\n\n", fpPostureDisclaimer)
			out := make([][]string, 0, len(rows))
			for _, r := range rows {
				out = append(out, []string{
					r.Code, truncate(r.Name, 30),
					boolMark(r.AdBuyer), r.NCRPricingModel, boolMark(r.IsPromoted), boolMark(r.IsPremium),
					fmt.Sprintf("%d", r.PremiumPosition), fmt.Sprintf("%d", r.VendorPoints),
					boolMark(r.OwnFleet), fmt.Sprintf("%.1f", r.Rating),
				})
			}
			return flags.printTable(cmd,
				[]string{"CODE", "NAME", "ADS", "MODEL", "PROMO", "PREM", "POS", "POINTS", "OWNFLEET", "RATING"}, out)
		},
	}

	cmd.Flags().Float64Var(&lat, "latitude", 0, "Latitude of the area to profile (required)")
	cmd.Flags().Float64Var(&lng, "longitude", 0, "Longitude of the area to profile (required)")
	cmd.Flags().StringVar(&sortKey, "sort", "points", "Sort by: points, rating, name, distance, fee")
	cmd.Flags().IntVar(&limit, "limit", 30, "Maximum vendors to return")
	cmd.Flags().IntVar(&maxScanPages, "max-scan-pages", 4, "Maximum listing pages to scan (48 vendors per page)")
	cmd.Flags().StringVar(&country, "country", "pk", "Market code: pk, bd, sg, my, hk, th")
	cmd.Flags().BoolVar(&adsOnly, "ads-only", false, "Only show vendors participating in the CPC ad product")
	return cmd
}

func boolMark(b bool) string {
	if b {
		return "yes"
	}
	return "-"
}
