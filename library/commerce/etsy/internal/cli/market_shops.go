// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"
)

type mktShopRow struct {
	ShopID        int64    `json:"shop_id"`
	ShopName      any      `json:"shop_name"`
	Sales         any      `json:"transaction_sold_count"`
	ActiveListing any      `json:"listing_active_count"`
	AgeYears      any      `json:"age_years"`
	Rating        any      `json:"review_average"`
	Reviews       any      `json:"review_count"`
	TopAppearance int      `json:"top_n_appearances"`
	Appearances   int      `json:"appearances"`
	NewShop       any      `json:"new_shop"` // true | false | null (unknown)
	Why           []string `json:"new_shop_reasons,omitempty"`
}

func newNovelMarketShopsCmd(flags *rootFlags) *cobra.Command {
	var flagTop int
	var flagAge, flagSnapshot = "1y", int64(0)
	var flagSales int

	cmd := &cobra.Command{
		Use:   "shops",
		Short: "Profile the shops behind the results: sales, age, rating, and how many top positions new versus established shops hold.",
		Long: `Profile the shops behind the results: sales, age, rating, and how many top positions new versus established shops hold.

A shop is flagged new when it is younger than --new-shop-age OR has fewer than --new-shop-sales sales.
The crowd-out summary shows the share of the top-N positions (per term) held by new, established
and unknown-age shops.`,
		Example:     "  etsy-pp-cli market shops --top 10 --new-shop-age 1y --new-shop-sales 100",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := mktValidateSnapshotFlag(cmd, flagSnapshot); err != nil {
				return err
			}
			if dryRunOK(flags) {
				return mktDryRun(cmd, flags, "market shops")
			}
			age, err := mktParseWindow(flagAge)
			if err != nil {
				return usageErr(fmt.Errorf("--new-shop-age: %w", err))
			}
			if flagTop < 1 {
				return usageErr(fmt.Errorf("--top must be at least 1"))
			}
			db, err := mktOpenRead(cmd.Context())
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			if db != nil {
				defer db.Close()
			}
			snap, note, err := mktPickSnapshot(db, flagSnapshot)
			if err != nil {
				return err
			}
			if snap == nil {
				return mktEmitReport(cmd, flags, map[string]any{"results": make([]any, 0), "note": note})
			}
			rows, err := mktLoadResults(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			shops, err := mktLoadShops(db.DB(), snap.ID)
			if err != nil {
				return err
			}
			return mktEmitReport(cmd, flags, mktShopsReport(snap, rows, shops, flagTop, age, flagSales, time.Now()))
		},
	}
	cmd.Flags().IntVar(&flagTop, "top", 10, "Top-N positions per term that count as prime placement")
	cmd.Flags().StringVar(&flagAge, "new-shop-age", "1y", "Shops younger than this are new")
	cmd.Flags().IntVar(&flagSales, "new-shop-sales", 100, "Shops with fewer sales than this are new")
	cmd.Flags().Int64Var(&flagSnapshot, "snapshot", 0, mktSnapshotUsage)
	mktHideZeroDefault(cmd, "snapshot")
	return cmd
}

// mktShopClass returns "new", "established" or "unknown" for a shop.
func mktShopClass(s mktShop, known bool, age time.Duration, minSales int, now time.Time) (string, []string) {
	if !known {
		return "unknown", nil
	}
	var why []string
	if s.Created.Valid && now.Sub(time.Unix(s.Created.Int64, 0)) < age {
		why = append(why, "younger than --new-shop-age")
	}
	if s.Sold.Valid && s.Sold.Int64 < int64(minSales) {
		why = append(why, "fewer than --new-shop-sales sales")
	}
	if len(why) > 0 {
		return "new", why
	}
	if !s.Created.Valid && !s.Sold.Valid {
		return "unknown", nil
	}
	return "established", nil
}

func mktShopsReport(snap *mktSnapshot, rows []mktRow, shops map[int64]mktShop, top int, age time.Duration, minSales int, now time.Time) map[string]any {
	apps := map[int64]int{}
	topApps := map[int64]int{}
	slots, newSlots, estSlots, unkSlots := 0, 0, 0, 0
	ids := make([]int64, 0)
	for _, r := range rows {
		if !r.ShopID.Valid {
			continue
		}
		id := r.ShopID.Int64
		if _, ok := apps[id]; !ok {
			ids = append(ids, id)
		}
		apps[id]++
		if r.Position <= top {
			topApps[id]++
			slots++
			s, known := shops[id]
			switch c, _ := mktShopClass(s, known, age, minSales, now); c {
			case "new":
				newSlots++
			case "established":
				estSlots++
			default:
				unkSlots++
			}
		}
	}
	out := make([]mktShopRow, 0, len(ids))
	for _, id := range ids {
		s, known := shops[id]
		r := mktShopRow{ShopID: id, TopAppearance: topApps[id], Appearances: apps[id]}
		if known && s.Name.Valid && s.Name.String != "" {
			r.ShopName = s.Name.String
		}
		if known {
			if s.Sold.Valid {
				r.Sales = s.Sold.Int64
			}
			if s.Active.Valid {
				r.ActiveListing = s.Active.Int64
			}
			if s.Created.Valid {
				r.AgeYears = mktRound(now.Sub(time.Unix(s.Created.Int64, 0)).Hours()/24/365, 1)
			}
			if s.RatingAvg.Valid {
				r.Rating = s.RatingAvg.Float64
			}
			if s.Reviews.Valid {
				r.Reviews = s.Reviews.Int64
			}
		}
		switch c, why := mktShopClass(s, known, age, minSales, now); c {
		case "new":
			r.NewShop, r.Why = true, why
		case "established":
			r.NewShop = false
		}
		out = append(out, r)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].TopAppearance != out[j].TopAppearance {
			return out[i].TopAppearance > out[j].TopAppearance
		}
		if out[i].Appearances != out[j].Appearances {
			return out[i].Appearances > out[j].Appearances
		}
		return out[i].ShopID < out[j].ShopID
	})
	share := func(n int) any {
		if slots == 0 {
			return nil
		}
		return mktRound(float64(n)/float64(slots), 3)
	}
	return map[string]any{
		"snapshot_id": snap.ID,
		"results":     out,
		"crowd_out": map[string]any{
			"top_n":               top,
			"top_n_slots":         slots,
			"new_shop_slots":      newSlots,
			"established_slots":   estSlots,
			"unknown_slots":       unkSlots,
			"new_shop_share":      share(newSlots),
			"established_share":   share(estSlots),
			"unknown_share":       share(unkSlots),
			"new_shop_definition": fmt.Sprintf("younger than %s, or fewer than %s", mktWindowLabel(age), mktCount(minSales, "sale")),
		},
	}
}
