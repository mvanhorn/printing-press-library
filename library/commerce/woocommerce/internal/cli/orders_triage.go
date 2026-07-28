// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type triageOrder struct {
	ID            string  `json:"id"`
	Number        string  `json:"number"`
	Status        string  `json:"status"`
	Total         float64 `json:"total"`
	AgeHours      float64 `json:"age_hours"`
	PaymentMethod string  `json:"payment_method"`
}

type triageBucket struct {
	Bucket    string        `json:"bucket"`
	Threshold string        `json:"threshold,omitempty"`
	Count     int           `json:"count"`
	Orders    []triageOrder `json:"orders"`
}

type gatewayFailureRate struct {
	PaymentMethod    string  `json:"payment_method"`
	TotalOrders      int     `json:"total_orders"`
	FailedOrders     int     `json:"failed_orders"`
	FailureRate      float64 `json:"failure_rate"`
	PriorFailureRate float64 `json:"prior_failure_rate"`
	Trend            string  `json:"trend"`
}

type triageView struct {
	Buckets             []triageBucket       `json:"buckets"`
	GatewayFailureRates []gatewayFailureRate `json:"gateway_failure_rates"`
	ScannedOrders       int                  `json:"scanned_orders"`
	Note                string               `json:"note,omitempty"`
}

// gatewayWindowStats accumulates one gateway's counts split across the recent
// and prior halves of the synced date range.
type gatewayWindowStats struct {
	recentTotal  int
	recentFailed int
	priorTotal   int
	priorFailed  int
}

const minGatewayOrdersForRate = 5

func newNovelOrdersTriageCmd(flags *rootFlags) *cobra.Command {
	var flagStuckAfter string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "triage",
		Short: "Split the order queue into actionable buckets and flag failing payment gateways",
		Long: "Buckets synced orders by status and age, then compares each payment gateway's recent\n" +
			"failure rate against the earlier half of the synced range so degrading gateways surface.\n" +
			"Reads the local mirror, so run 'sync' first.\n\n" +
			"Use this command to decide what to act on in the order queue right now.\n" +
			"Do NOT use this command to explain a revenue change over time; use 'revenue explain' instead.",
		Example: "  woocommerce-pp-cli orders triage --stuck-after 24h --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--stuck-after=24h",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would triage synced orders from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "orders triage"); err != nil {
				return err
			}
			stuckAfter, err := windowOrDefault(flagStuckAfter, "24h")
			if err != nil {
				return err
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
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

			view := buildTriageView(orders, stuckAfter, flagLimit, time.Now().UTC())
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagStuckAfter, "stuck-after", "24h", "Age at which a processing or on-hold order counts as stuck (24h, 7d, 1w)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum orders listed per bucket")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

// buildTriageView is pure so bucket and trend logic stays unit-testable without
// a database.
func buildTriageView(orders []wooOrder, stuckAfter time.Duration, limit int, now time.Time) triageView {
	stuckProcessing := make([]triageOrder, 0)
	agingOnHold := make([]triageOrder, 0)
	failedRecent := make([]triageOrder, 0)

	// Split the synced range in half so "is this gateway getting worse" compares
	// like with like. Without stored history this comparison is impossible.
	var earliest, latest time.Time
	for _, o := range orders {
		t, ok := o.EffectiveTime()
		if !ok {
			continue
		}
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
		if latest.IsZero() || t.After(latest) {
			latest = t
		}
	}
	var midpoint time.Time
	if !earliest.IsZero() && !latest.IsZero() {
		midpoint = earliest.Add(latest.Sub(earliest) / 2)
	}

	gateways := make(map[string]*gatewayWindowStats)

	for _, o := range orders {
		t, ok := o.EffectiveTime()
		var age time.Duration
		if ok {
			age = now.Sub(t)
		}
		entry := triageOrder{
			ID:            o.ID,
			Number:        o.Number,
			Status:        o.Status,
			Total:         round2(o.Total),
			AgeHours:      round2(age.Hours()),
			PaymentMethod: o.PaymentMethod,
		}
		switch {
		case o.Status == "processing" && ok && age >= stuckAfter:
			stuckProcessing = append(stuckProcessing, entry)
		case o.Status == "on-hold" && ok && age >= stuckAfter:
			agingOnHold = append(agingOnHold, entry)
		case o.Status == "failed" && ok && age <= 7*24*time.Hour:
			failedRecent = append(failedRecent, entry)
		}

		name := o.PaymentMethod
		if name == "" {
			name = "(unspecified)"
		}
		g, seen := gateways[name]
		if !seen {
			g = &gatewayWindowStats{}
			gateways[name] = g
		}
		if ok && !midpoint.IsZero() && !t.Before(midpoint) {
			g.recentTotal++
			if o.Status == "failed" {
				g.recentFailed++
			}
		} else {
			g.priorTotal++
			if o.Status == "failed" {
				g.priorFailed++
			}
		}
	}

	// Oldest-first is what an operator wants to clear first.
	for _, b := range []*[]triageOrder{&stuckProcessing, &agingOnHold, &failedRecent} {
		bucket := *b
		sort.SliceStable(bucket, func(i, j int) bool { return bucket[i].AgeHours > bucket[j].AgeHours })
	}

	rates := make([]gatewayFailureRate, 0, len(gateways))
	for name, g := range gateways {
		total := g.recentTotal + g.priorTotal
		if total < minGatewayOrdersForRate {
			continue
		}
		var recentRate, priorRate float64
		if g.recentTotal > 0 {
			recentRate = float64(g.recentFailed) / float64(g.recentTotal)
		}
		if g.priorTotal > 0 {
			priorRate = float64(g.priorFailed) / float64(g.priorTotal)
		}
		trend := "stable"
		switch {
		case g.priorTotal == 0 || g.recentTotal == 0:
			trend = "insufficient_history"
		case recentRate > priorRate*1.2:
			trend = "worsening"
		case recentRate < priorRate*0.8:
			trend = "improving"
		}
		rates = append(rates, gatewayFailureRate{
			PaymentMethod:    name,
			TotalOrders:      total,
			FailedOrders:     g.recentFailed + g.priorFailed,
			FailureRate:      round4(recentRate),
			PriorFailureRate: round4(priorRate),
			Trend:            trend,
		})
	}
	sort.SliceStable(rates, func(i, j int) bool {
		if rates[i].FailureRate != rates[j].FailureRate {
			return rates[i].FailureRate > rates[j].FailureRate
		}
		return rates[i].PaymentMethod < rates[j].PaymentMethod
	})

	view := triageView{
		Buckets: []triageBucket{
			{Bucket: "stuck_processing", Threshold: stuckAfter.String(), Count: len(stuckProcessing), Orders: capTriage(stuckProcessing, limit)},
			{Bucket: "aging_on_hold", Threshold: stuckAfter.String(), Count: len(agingOnHold), Orders: capTriage(agingOnHold, limit)},
			{Bucket: "failed_recent", Count: len(failedRecent), Orders: capTriage(failedRecent, limit)},
		},
		GatewayFailureRates: rates,
		ScannedOrders:       len(orders),
	}
	if len(orders) == 0 {
		view.Note = "no orders in the local mirror; run 'woocommerce-pp-cli sync --resources orders' first"
	}
	return view
}

func capTriage(in []triageOrder, limit int) []triageOrder {
	out := make([]triageOrder, 0, len(in))
	out = append(out, in...)
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
