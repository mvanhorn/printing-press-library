// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

type mrrWeekBucket struct {
	WeekStart    string  `json:"week_start"`
	InvoiceCount int     `json:"invoice_count"`
	NewRevenue   float64 `json:"new_revenue_usd"`
	RenewalRev   float64 `json:"renewal_revenue_usd"`
	RefundedRev  float64 `json:"refunded_revenue_usd"`
	NetMRR       float64 `json:"net_mrr_usd"`
	NetDelta     float64 `json:"net_delta_usd"`
}

type mrrTrendView struct {
	Weeks           []mrrWeekBucket `json:"weeks"`
	WindowWeeks     int             `json:"window_weeks"`
	WindowStartDate string          `json:"window_start_date"`
	WindowEndDate   string          `json:"window_end_date"`
	Note            string          `json:"note,omitempty"`
}

func newNovelMrrTrendCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int
	cmd := &cobra.Command{
		Use:   "mrr-trend",
		Short: "Weekly MRR over a sliding window, classified as new / renewal / refunded with week-over-week delta",
		Long: `Weekly MRR trend computed from the local 'subscription-invoices' mirror.

Each row buckets paid invoices by ISO-week, separating new subscriptions (first
invoice per subscription_id) from renewals (subsequent invoices), and reports
the week-over-week net delta.

Use this command for time-series MRR. Do NOT use this for a single point-in-time
revenue rollup that includes one-off orders; use 'revenue-snapshot' instead.

Data source: local. Run 'sync --resources subscriptions,subscription-invoices' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources subscriptions,subscription-invoices\n  lemonsqueezy-pp-cli mrr-trend --weeks 12 --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if weeks <= 0 {
				weeks = 12
			}
			if weeks > 104 {
				return usageErr(fmt.Errorf("--weeks must be between 1 and 104"))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			if !hintIfUnsynced(cmd, db, "subscription-invoices") {
				hintIfStale(cmd, db, "subscription-invoices", flags.maxAge)
			}

			view, err := buildMrrTrend(db, weeks)
			if err != nil {
				return err
			}
			return flags.printJSON(cmd, view)
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of trailing ISO weeks to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func buildMrrTrend(db *store.Store, weeks int) (mrrTrendView, error) {
	view := mrrTrendView{Weeks: []mrrWeekBucket{}, WindowWeeks: weeks}

	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'subscription-invoices' LIMIT 20000`,
	)
	if err != nil {
		return view, fmt.Errorf("querying subscription-invoices: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -7*weeks)
	firstSeenSub := map[string]time.Time{}

	type invoiceRow struct {
		when     time.Time
		subID    string
		status   string
		amount   float64
		refunded float64
	}
	var invoices []invoiceRow

	for rows.Next() {
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			Attributes struct {
				SubscriptionID    any    `json:"subscription_id"`
				Status            string `json:"status"`
				CreatedAt         string `json:"created_at"`
				Total             any    `json:"total"`
				TotalUSD          any    `json:"total_usd"`
				Refunded          any    `json:"refunded"`
				RefundedAmountUSD any    `json:"refunded_amount_usd"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		when := parseLSTime(env.Attributes.CreatedAt)
		if when.IsZero() {
			continue
		}
		amt := toFloatLS(env.Attributes.TotalUSD)
		if amt == 0 {
			amt = toFloatLS(env.Attributes.Total)
		}
		refunded := toFloatLS(env.Attributes.RefundedAmountUSD)
		if refunded == 0 && toBoolLS(env.Attributes.Refunded) {
			refunded = amt
		}
		subID := toStringLS(env.Attributes.SubscriptionID)
		invoices = append(invoices, invoiceRow{
			when:     when,
			subID:    subID,
			status:   env.Attributes.Status,
			amount:   amt / 100.0,
			refunded: refunded / 100.0,
		})
		if first, ok := firstSeenSub[subID]; !ok || when.Before(first) {
			firstSeenSub[subID] = when
		}
	}

	buckets := map[string]*mrrWeekBucket{}
	for _, inv := range invoices {
		if inv.when.Before(cutoff) {
			continue
		}
		weekStart := startOfISOWeek(inv.when)
		key := weekStart.Format("2006-01-02")
		b, ok := buckets[key]
		if !ok {
			b = &mrrWeekBucket{WeekStart: key}
			buckets[key] = b
		}
		if inv.status == "paid" || inv.status == "" {
			isNew := firstSeenSub[inv.subID].Equal(inv.when)
			if isNew {
				b.NewRevenue += inv.amount
			} else {
				b.RenewalRev += inv.amount
			}
			b.InvoiceCount++
		}
		b.RefundedRev += inv.refunded
	}

	var weekStarts []time.Time
	w := startOfISOWeek(cutoff)
	end := startOfISOWeek(now)
	for !w.After(end) {
		weekStarts = append(weekStarts, w)
		w = w.AddDate(0, 0, 7)
	}
	sort.Slice(weekStarts, func(i, j int) bool { return weekStarts[i].Before(weekStarts[j]) })

	var prevNet float64
	for _, ws := range weekStarts {
		key := ws.Format("2006-01-02")
		b, ok := buckets[key]
		if !ok {
			b = &mrrWeekBucket{WeekStart: key}
		}
		b.NetMRR = b.NewRevenue + b.RenewalRev - b.RefundedRev
		b.NetDelta = b.NetMRR - prevNet
		prevNet = b.NetMRR
		view.Weeks = append(view.Weeks, *b)
	}
	if len(view.Weeks) > 0 {
		view.WindowStartDate = view.Weeks[0].WeekStart
		view.WindowEndDate = view.Weeks[len(view.Weeks)-1].WeekStart
	}
	if len(invoices) == 0 {
		view.Note = "no subscription-invoices in local mirror; run 'sync --resources subscription-invoices' first"
	}
	return view, nil
}

func startOfISOWeek(t time.Time) time.Time {
	t = t.UTC()
	wd := int(t.Weekday())
	if wd == 0 {
		wd = 7
	}
	monday := t.AddDate(0, 0, -(wd - 1))
	return time.Date(monday.Year(), monday.Month(), monday.Day(), 0, 0, 0, 0, time.UTC)
}
