// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type periodSummary struct {
	From          string  `json:"from"`
	To            string  `json:"to"`
	Orders        int     `json:"orders"`
	Revenue       float64 `json:"revenue"`
	AvgOrderValue float64 `json:"avg_order_value"`
}

type revenueDelta struct {
	Revenue       float64 `json:"revenue"`
	RevenuePct    float64 `json:"revenue_pct"`
	Orders        int     `json:"orders"`
	AvgOrderValue float64 `json:"avg_order_value"`
}

type revenueContribution struct {
	Factor      string  `json:"factor"`
	Amount      float64 `json:"amount"`
	Explanation string  `json:"explanation,omitempty"`
}

type productMover struct {
	ProductID      string  `json:"product_id"`
	Name           string  `json:"name"`
	CurrentRevenue float64 `json:"current_revenue"`
	PriorRevenue   float64 `json:"prior_revenue"`
	Delta          float64 `json:"delta"`
}

type revenueExplainView struct {
	Current          periodSummary         `json:"current"`
	Prior            periodSummary         `json:"prior"`
	Delta            revenueDelta          `json:"delta"`
	Contributions    []revenueContribution `json:"contributions"`
	TopProductMovers []productMover        `json:"top_product_movers"`
	ScannedOrders    int                   `json:"scanned_orders"`
	Note             string                `json:"note,omitempty"`
}

func newNovelRevenueExplainCmd(flags *rootFlags) *cobra.Command {
	var flagWindow string
	var flagAgainst string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "explain",
		Short: "Decompose a revenue change into order-count, basket-size, refund and coupon contributions",
		Long: "Compares the most recent window against the window immediately before it and attributes the\n" +
			"revenue delta to its causes. The Reports API returns one period's totals and cannot\n" +
			"decompose a change, so this reads the local mirror.\n\n" +
			"Use this command to explain a change in revenue between two time periods, broken into\n" +
			"order-count, basket-size, product, refund, and coupon contributions.\n" +
			"Do NOT use this command to slice a single period by one dimension; use\n" +
			"'analytics --type orders --group-by <field>' instead.\n" +
			"Do NOT use it for product-level return quality; use 'refund-rate' instead.",
		Example: "  woocommerce-pp-cli revenue explain --window 7d --against prior --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--window=7d",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would decompose revenue from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "revenue explain"); err != nil {
				return err
			}
			window, err := windowOrDefault(flagWindow, "7d")
			if err != nil {
				return err
			}
			if against := flagAgainst; against != "" && against != "prior" {
				return usageErr(fmt.Errorf("--against only supports \"prior\" (the immediately preceding window of equal length); got %q", against))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = resolveDBPath(dbPath)
			if localStoreMissing(cmd, flags, dbPath, "orders") {
				return nil
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "orders") {
				hintIfStale(cmd, db, "orders", flags.maxAge)
			}

			orders, err := loadWooOrders(ctx, db)
			if err != nil {
				return err
			}
			labels, err := loadProductLabels(ctx, db)
			if err != nil {
				return err
			}

			view := buildRevenueExplainView(orders, labels, window, time.Now().UTC())
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagWindow, "window", "7d", "Length of the period to analyze (7d, 30d, 4w)")
	cmd.Flags().StringVar(&flagAgainst, "against", "prior", "Comparison baseline; only \"prior\" is supported")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// buildRevenueExplainView is pure so the decomposition identity is testable.
//
// The order-count and basket-size contributions are constructed to sum exactly
// to the revenue delta:
//
//	(Nc-Np)*Ap + (Ac-Ap)*Nc == Nc*Ac - Np*Ap
func buildRevenueExplainView(orders []wooOrder, labels map[string]productLabel, window time.Duration, now time.Time) revenueExplainView {
	curStart := now.Add(-window)
	priStart := curStart.Add(-window)

	var curRevenue, priRevenue, curRefunds, priRefunds, curCoupons, priCoupons float64
	var curCount, priCount int
	curProducts := make(map[string]float64)
	priProducts := make(map[string]float64)

	for _, o := range orders {
		t, ok := o.EffectiveTime()
		if !ok {
			continue
		}
		inCurrent := !t.Before(curStart) && !t.After(now)
		inPrior := !t.Before(priStart) && t.Before(curStart)
		if !inCurrent && !inPrior {
			continue
		}
		// Refund and coupon totals are tracked across all statuses; revenue only
		// counts orders that actually settled.
		var refundTotal float64
		for _, r := range o.Refunds {
			refundTotal += math.Abs(parseMoney(r.Total))
		}
		var couponTotal float64
		for _, c := range o.CouponLines {
			couponTotal += parseMoney(c.Discount)
		}
		if inCurrent {
			curRefunds += refundTotal
			curCoupons += couponTotal
		} else {
			priRefunds += refundTotal
			priCoupons += couponTotal
		}
		if !revenueBearingStatuses[o.Status] {
			continue
		}
		if inCurrent {
			curRevenue += o.Total
			curCount++
		} else {
			priRevenue += o.Total
			priCount++
		}
		for _, li := range o.LineItems {
			pid := fmt.Sprintf("%d", li.ProductID)
			if inCurrent {
				curProducts[pid] += parseMoney(li.Total)
			} else {
				priProducts[pid] += parseMoney(li.Total)
			}
		}
	}

	avg := func(rev float64, n int) float64 {
		if n == 0 {
			return 0
		}
		return rev / float64(n)
	}
	curAvg, priAvg := avg(curRevenue, curCount), avg(priRevenue, priCount)
	revDelta := curRevenue - priRevenue
	var revPct float64
	if priRevenue != 0 {
		revPct = revDelta / priRevenue * 100
	}

	countContrib := float64(curCount-priCount) * priAvg
	basketContrib := (curAvg - priAvg) * float64(curCount)

	contributions := []revenueContribution{
		{
			Factor: "order_count",
			Amount: round2(countContrib),
			Explanation: fmt.Sprintf("%d %s orders at the prior average basket of %.2f",
				abs(curCount-priCount), moreOrFewer(curCount-priCount), priAvg),
		},
		{
			Factor: "basket_size",
			Amount: round2(basketContrib),
			Explanation: fmt.Sprintf("average order value %s %.2f across %d orders",
				roseOrFell(curAvg-priAvg), math.Abs(curAvg-priAvg), curCount),
		},
		{Factor: "refunds", Amount: round2(-(curRefunds - priRefunds)), Explanation: "change in refunded value versus the prior window"},
		{Factor: "coupon_discount", Amount: round2(-(curCoupons - priCoupons)), Explanation: "change in coupon discount granted versus the prior window"},
	}

	movers := make([]productMover, 0, len(curProducts)+len(priProducts))
	seen := make(map[string]bool)
	for pid := range curProducts {
		seen[pid] = true
	}
	for pid := range priProducts {
		seen[pid] = true
	}
	for pid := range seen {
		c, p := curProducts[pid], priProducts[pid]
		m := productMover{ProductID: pid, CurrentRevenue: round2(c), PriorRevenue: round2(p), Delta: round2(c - p)}
		if l, ok := labels[pid]; ok {
			m.Name = l.Name
		}
		movers = append(movers, m)
	}
	sort.SliceStable(movers, func(i, j int) bool {
		if math.Abs(movers[i].Delta) != math.Abs(movers[j].Delta) {
			return math.Abs(movers[i].Delta) > math.Abs(movers[j].Delta)
		}
		return movers[i].ProductID < movers[j].ProductID
	})
	if len(movers) > 10 {
		movers = movers[:10]
	}

	view := revenueExplainView{
		Current: periodSummary{
			From: curStart.Format(time.RFC3339), To: now.Format(time.RFC3339),
			Orders: curCount, Revenue: round2(curRevenue), AvgOrderValue: round2(curAvg),
		},
		Prior: periodSummary{
			From: priStart.Format(time.RFC3339), To: curStart.Format(time.RFC3339),
			Orders: priCount, Revenue: round2(priRevenue), AvgOrderValue: round2(priAvg),
		},
		Delta: revenueDelta{
			Revenue: round2(revDelta), RevenuePct: round2(revPct),
			Orders: curCount - priCount, AvgOrderValue: round2(curAvg - priAvg),
		},
		Contributions:    contributions,
		TopProductMovers: movers,
		ScannedOrders:    curCount + priCount,
	}
	if curCount == 0 && priCount == 0 {
		view.Note = "no revenue-bearing orders in either window; widen --window or run 'woocommerce-pp-cli sync --resources orders'"
	}
	return view
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func moreOrFewer(v int) string {
	switch {
	case v < 0:
		return "fewer"
	case v == 0:
		return "same count of"
	}
	return "more"
}

func roseOrFell(v float64) string {
	switch {
	case v < 0:
		return "fell"
	case v == 0:
		return "was unchanged at"
	}
	return "rose by"
}
