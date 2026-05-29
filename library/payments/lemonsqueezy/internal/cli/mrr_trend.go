// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
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
			// Read-only local query — no mutation to suppress under --dry-run;
			// we still run so --dry-run --json emits a real view.
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

			hintIfMultiUnsynced(cmd, db, "subscription-invoices",
				[]string{"subscriptions"}, flags.maxAge)

			view, err := buildMrrTrend(db, weeks)
			if err != nil {
				return err
			}
			return emitMrrTrend(cmd, flags, view)
		},
	}
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of trailing ISO weeks to include")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitMrrTrend(cmd *cobra.Command, flags *rootFlags, view mrrTrendView) error {
	if len(view.Weeks) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Weeks))
		for _, w := range view.Weeks {
			items = append(items, map[string]any{
				"week_start":   w.WeekStart,
				"new_usd":      fmt.Sprintf("%.2f", w.NewRevenue),
				"renewal_usd":  fmt.Sprintf("%.2f", w.RenewalRev),
				"refunded_usd": fmt.Sprintf("%.2f", w.RefundedRev),
				"net_mrr_usd":  fmt.Sprintf("%.2f", w.NetMRR),
				"net_delta":    fmt.Sprintf("%.2f", w.NetDelta),
				"invoices":     w.InvoiceCount,
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\n%d-week window: %s → %s\n",
			view.WindowWeeks, view.WindowStartDate, view.WindowEndDate)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// mrrTrendInvoiceCap is the in-window subscription-invoices scan cap. The
// query filters by created_at >= cutoff at SQL level so this cap applies to
// the working window, NOT the entire invoice history. A store with 1M
// lifetime invoices but 200K in-window will still saturate; one with 1M
// lifetime and 50K in-window won't.
const mrrTrendInvoiceCap = 200000

func buildMrrTrend(db *store.Store, weeks int) (mrrTrendView, error) {
	view := mrrTrendView{Weeks: []mrrWeekBucket{}, WindowWeeks: weeks}

	now := time.Now().UTC()
	cutoff := now.AddDate(0, 0, -7*weeks)

	// "New vs renewal" classification needs to know whether a subscription
	// existed before the window. We grab each sub's created_at once via the
	// subscriptions resource and treat an in-window invoice as NEW only
	// when its parent sub was also created in the window. This is robust
	// even when the invoice cap clips older invoices because the
	// subscription-level signal is independent of invoice volume.
	subCreatedAt := loadSubscriptionCreatedAt(db)

	// Filter invoices to the working window at SQL level. ORDER BY ASC keeps
	// chronological iteration for the bucket loop. CAST handles SQLite's
	// string-typed JSON comparison.
	rows, err := db.Query(
		`SELECT data FROM resources
		 WHERE resource_type = 'subscription-invoices'
		   AND CAST(json_extract(data, '$.attributes.created_at') AS TEXT) >= ?
		 ORDER BY CAST(json_extract(data, '$.attributes.created_at') AS TEXT) ASC
		 LIMIT ?`,
		cutoff.Format(time.RFC3339), mrrTrendInvoiceCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying subscription-invoices: %w", err)
	}
	defer rows.Close()
	scannedInvoices := 0

	type invoiceRow struct {
		when     time.Time
		subID    string
		id       string
		status   string
		amount   float64
		refunded float64
	}
	var invoices []invoiceRow

	for rows.Next() {
		scannedInvoices++
		var data sql.NullString
		if err := rows.Scan(&data); err != nil {
			continue
		}
		if !data.Valid {
			continue
		}
		var env struct {
			ID         string `json:"id"`
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
		if when.IsZero() || when.Before(cutoff) {
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
			id:       env.ID,
			status:   env.Attributes.Status,
			amount:   amt / 100.0,
			refunded: refunded / 100.0,
		})
	}
	// firstInWindow ensures only the earliest in-window invoice per sub
	// claims the "new" classification when the sub was newly created in
	// the window. Subsequent invoices for the same sub are renewals.
	type subStamp struct {
		when      time.Time
		invoiceID string
	}
	firstInWindow := map[string]subStamp{}
	for _, inv := range invoices {
		if cur, ok := firstInWindow[inv.subID]; !ok ||
			inv.when.Before(cur.when) ||
			(inv.when.Equal(cur.when) && inv.id < cur.invoiceID) {
			firstInWindow[inv.subID] = subStamp{when: inv.when, invoiceID: inv.id}
		}
	}
	// Cap warning fires when the in-window scan saturated — meaning the
	// query truncated. Honest signal: the user's working window has more
	// than `cap` invoices, so older weeks in this view may underreport.
	if scannedInvoices >= mrrTrendInvoiceCap {
		fmt.Fprintf(os.Stderr, "warning: mrr-trend hit the %d-invoice in-window scan cap; older weeks in this view may underreport revenue\n", mrrTrendInvoiceCap)
		view.Note = fmt.Sprintf("hit the %d-invoice in-window scan cap; older weeks may underreport revenue. Narrow --weeks or open an issue if your in-window invoice volume routinely exceeds this.", mrrTrendInvoiceCap)
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
			// "New" means: (a) this is the earliest in-window invoice for
			// the sub, AND (b) the sub itself was created in the window.
			// (b) is the sub-level signal that doesn't depend on having
			// every historical invoice synced.
			first := firstInWindow[inv.subID]
			isFirstInvoice := first.invoiceID == inv.id
			subCreated, hasSubInfo := subCreatedAt[inv.subID]
			isNewSub := hasSubInfo && !subCreated.Before(cutoff)
			if isFirstInvoice && isNewSub {
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
	if len(invoices) == 0 && view.Note == "" {
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
