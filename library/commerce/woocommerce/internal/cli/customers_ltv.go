// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/woocommerce/internal/store"
)

type ltvCustomer struct {
	CustomerID        string   `json:"customer_id"`
	Email             string   `json:"email,omitempty"`
	Name              string   `json:"name,omitempty"`
	Orders            int      `json:"orders"`
	LifetimeValue     float64  `json:"lifetime_value"`
	AvgOrderValue     float64  `json:"avg_order_value"`
	FirstOrder        string   `json:"first_order"`
	LastOrder         string   `json:"last_order"`
	DaysBetweenOrders *float64 `json:"days_between_orders"`
	IsGuest           bool     `json:"is_guest"`
}

type ltvCohort struct {
	Cohort          string  `json:"cohort"`
	Customers       int     `json:"customers"`
	RepeatCustomers int     `json:"repeat_customers"`
	RepeatRate      float64 `json:"repeat_rate"`
	TotalRevenue    float64 `json:"total_revenue"`
	AvgLTV          float64 `json:"avg_ltv"`
}

type ltvTotals struct {
	Customers       int     `json:"customers"`
	RepeatCustomers int     `json:"repeat_customers"`
	RepeatRate      float64 `json:"repeat_rate"`
	AvgLTV          float64 `json:"avg_ltv"`
}

type ltvView struct {
	Customers     []ltvCustomer `json:"customers"`
	Cohorts       []ltvCohort   `json:"cohorts"`
	Totals        ltvTotals     `json:"totals"`
	ScannedOrders int           `json:"scanned_orders"`
	// UnidentifiedGuestOrders are guest orders with no billing email. They are
	// deliberately excluded from every aggregate above rather than lumped into
	// one fake customer.
	UnidentifiedGuestOrders int    `json:"unidentified_guest_orders"`
	Note                    string `json:"note,omitempty"`
}

// customerAgg accumulates one customer's order history before summarising.
type customerAgg struct {
	orders  int
	revenue float64
	dates   []time.Time
	isGuest bool
}

func newNovelCustomersLtvCmd(flags *rootFlags) *cobra.Command {
	var flagCohort string
	var flagLimit int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "ltv",
		Short: "Per-customer lifetime value and repeat-purchase retention by cohort",
		Long: "Groups synced orders by customer for lifetime value and order cadence, then buckets customers\n" +
			"by their first-order month or week to show repeat-purchase retention. Cohort retention and LTV\n" +
			"are structurally absent from the Reports API, so this reads the local mirror.\n\n" +
			"Use this command to judge customer quality over time.\n" +
			"Do NOT use this command to explain a single period's revenue change; use 'revenue explain' instead.",
		Example: "  woocommerce-pp-cli customers ltv --cohort month --agent",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:happy-args":  "--cohort=month",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would compute customer LTV from the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags, "customers ltv"); err != nil {
				return err
			}
			cohort := flagCohort
			if cohort == "" {
				cohort = "month"
			}
			if cohort != "month" && cohort != "week" {
				return usageErr(fmt.Errorf("--cohort must be \"month\" or \"week\"; got %q", cohort))
			}
			if flagLimit <= 0 {
				return usageErr(fmt.Errorf("--limit must be greater than zero"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			dbPath = resolveDBPath(dbPath)
			if localStoreMissing(cmd, flags, dbPath, "orders,customers") {
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

			// Stage 1: drain orders fully.
			orders, err := loadWooOrders(ctx, db)
			if err != nil {
				return err
			}
			// Stage 2: safe to query customers now.
			profiles, err := loadCustomerProfiles(ctx, db)
			if err != nil {
				return err
			}

			view := buildLtvView(orders, profiles, cohort, flagLimit)
			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagCohort, "cohort", "month", "Cohort granularity: month or week")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum customers returned (cohorts are never capped)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

type customerProfile struct {
	Email string
	Name  string
}

func loadCustomerProfiles(ctx context.Context, db *store.Store) (map[string]customerProfile, error) {
	out := make(map[string]customerProfile)
	rows, err := db.DB().QueryContext(ctx, `
		SELECT id, COALESCE(email,''), COALESCE(first_name,''), COALESCE(last_name,'') FROM customers`)
	if err != nil {
		return out, nil // customers may not be synced; ids still work
	}
	for rows.Next() {
		var id, email, first, last string
		if err := rows.Scan(&id, &email, &first, &last); err != nil {
			_ = rows.Close()
			return out, fmt.Errorf("scanning customer: %w", err)
		}
		name := strings.TrimSpace(first + " " + last)
		out[id] = customerProfile{Email: email, Name: name}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return out, fmt.Errorf("iterating customers: %w", err)
	}
	if err := rows.Close(); err != nil {
		return out, fmt.Errorf("closing customer rows: %w", err)
	}
	return out, nil
}

// buildLtvView is pure so cohort bucketing stays unit-testable.
func buildLtvView(orders []wooOrder, profiles map[string]customerProfile, cohort string, limit int) ltvView {
	aggs := make(map[string]*customerAgg)
	scanned := 0
	unidentified := 0
	for _, o := range orders {
		if !revenueBearingStatuses[o.Status] {
			continue
		}
		t, ok := o.EffectiveTime()
		if !ok {
			continue
		}
		scanned++
		key := fmt.Sprintf("%d", o.CustomerID)
		guest := o.CustomerID == 0
		if guest {
			// Guests are still distinct people. Key them by billing email so a
			// guest-checkout store does not collapse into one synthetic
			// customer with the entire store's revenue as its LTV.
			if o.BillingEmail != "" {
				key = "guest:" + o.BillingEmail
			} else {
				key = ""
			}
		}
		if key == "" {
			unidentified++
			continue
		}
		a, seen := aggs[key]
		if !seen {
			a = &customerAgg{isGuest: guest}
			aggs[key] = a
		}
		a.orders++
		a.revenue += o.Total
		a.dates = append(a.dates, t)
	}

	customers := make([]ltvCustomer, 0, len(aggs))
	cohorts := make(map[string]*ltvCohort)
	var totalRevenue float64
	repeatTotal := 0

	for id, a := range aggs {
		sort.Slice(a.dates, func(i, j int) bool { return a.dates[i].Before(a.dates[j]) })
		first, last := a.dates[0], a.dates[len(a.dates)-1]
		c := ltvCustomer{
			CustomerID:    id,
			Orders:        a.orders,
			LifetimeValue: round2(a.revenue),
			AvgOrderValue: round2(a.revenue / float64(a.orders)),
			FirstOrder:    first.Format(time.RFC3339),
			LastOrder:     last.Format(time.RFC3339),
			IsGuest:       a.isGuest,
		}
		if p, ok := profiles[id]; ok {
			c.Email, c.Name = p.Email, p.Name
		} else if a.isGuest {
			c.Email = strings.TrimPrefix(id, "guest:")
		}
		if a.orders > 1 {
			gap := last.Sub(first).Hours() / 24 / float64(a.orders-1)
			g := round2(gap)
			c.DaysBetweenOrders = &g
			repeatTotal++
		}
		customers = append(customers, c)
		totalRevenue += a.revenue

		key := cohortKey(first, cohort)
		ch, seen := cohorts[key]
		if !seen {
			ch = &ltvCohort{Cohort: key}
			cohorts[key] = ch
		}
		ch.Customers++
		ch.TotalRevenue += a.revenue
		if a.orders > 1 {
			ch.RepeatCustomers++
		}
	}

	sort.SliceStable(customers, func(i, j int) bool {
		if customers[i].LifetimeValue != customers[j].LifetimeValue {
			return customers[i].LifetimeValue > customers[j].LifetimeValue
		}
		return customers[i].CustomerID < customers[j].CustomerID
	})
	capped := customers
	if limit > 0 && len(capped) > limit {
		capped = capped[:limit]
	}

	cohortList := make([]ltvCohort, 0, len(cohorts))
	for _, ch := range cohorts {
		if ch.Customers > 0 {
			ch.RepeatRate = round4(float64(ch.RepeatCustomers) / float64(ch.Customers))
			ch.AvgLTV = round2(ch.TotalRevenue / float64(ch.Customers))
		}
		ch.TotalRevenue = round2(ch.TotalRevenue)
		cohortList = append(cohortList, *ch)
	}
	sort.SliceStable(cohortList, func(i, j int) bool { return cohortList[i].Cohort < cohortList[j].Cohort })

	totals := ltvTotals{Customers: len(aggs), RepeatCustomers: repeatTotal}
	if len(aggs) > 0 {
		totals.RepeatRate = round4(float64(repeatTotal) / float64(len(aggs)))
		totals.AvgLTV = round2(totalRevenue / float64(len(aggs)))
	}

	view := ltvView{
		Customers:     capped,
		Cohorts:       cohortList,
		Totals:        totals,
		ScannedOrders: scanned,
	}
	view.UnidentifiedGuestOrders = unidentified
	switch {
	case scanned == 0:
		view.Note = "no revenue-bearing orders in the local mirror; run 'woocommerce-pp-cli sync --resources orders,customers' first"
	case unidentified > 0:
		view.Note = fmt.Sprintf("%d guest order(s) carried no billing email and are excluded from customers, cohorts, and totals; they cannot be attributed to a person", unidentified)
	}
	return view
}

// cohortKey buckets a customer by their first order. ISO year-week keeps weekly
// cohorts sortable as strings.
func cohortKey(t time.Time, granularity string) string {
	if granularity == "week" {
		y, w := t.ISOWeek()
		return fmt.Sprintf("%04d-W%02d", y, w)
	}
	return t.Format("2006-01")
}
