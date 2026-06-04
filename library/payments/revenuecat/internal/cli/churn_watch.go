// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/revenuecat/internal/store"
	"github.com/spf13/cobra"
)

// churnStatuses is the set of RevenueCat subscription statuses that represent a
// churn or at-risk-of-churn state. RevenueCat models a voluntary cancellation as
// auto_renewal_status=will_not_renew (often while status is still "active" during
// the remaining paid period) rather than as a distinct status, so buildChurnWatch
// also includes any subscription whose auto_renewal_status is will_not_renew —
// see willNotRenewStatus and the filter in buildChurnWatch.
var churnStatuses = map[string]bool{
	"expired":          true,
	"in_grace_period":  true,
	"in_billing_retry": true,
}

// willNotRenewStatus is RevenueCat's auto_renewal_status value for a subscription
// the customer has cancelled (auto-renew turned off). These are churn even when
// status is still "active" until the paid period ends.
const willNotRenewStatus = "will_not_renew"

type churnRow struct {
	SubscriptionID    string    `json:"subscription_id"`
	Status            string    `json:"status"`
	AutoRenewalStatus string    `json:"auto_renewal_status,omitempty"`
	CustomerID        string    `json:"customer_id"`
	ProductID         string    `json:"product_id,omitempty"`
	PeriodEndsAt      string    `json:"current_period_ends_at,omitempty"`
	endsAtTime        time.Time `json:"-"`
	ExposureUSD       float64   `json:"exposure_usd"`
}

type churnWatchView struct {
	ProjectID      string     `json:"project_id"`
	Rows           []churnRow `json:"rows"`
	Since          string     `json:"since"`
	WindowStart    string     `json:"window_start"`
	WindowEnd      string     `json:"window_end"`
	Count          int        `json:"count"`
	DollarExposure float64    `json:"dollar_exposure_usd"`
	ChartChurnRate float64    `json:"chart_churn_rate,omitempty"`
	Note           string     `json:"note,omitempty"`
}

func newNovelChurnWatchCmd(flags *rootFlags) *cobra.Command {
	var projectFlag string
	var dbPath string
	var since string
	cmd := &cobra.Command{
		Use:   "churn-watch",
		Short: "Subscriptions that flipped to billing-issue / grace / expired in a window, with dollar exposure",
		Long: `Reads the local 'subscriptions' mirror, filters to the churn set
(in_grace_period, in_billing_retry, expired) whose current_period_ends_at falls
inside the window, and sums each subscription's total_revenue_in_usd as the
dollar exposure. With --data-source auto/live it also fetches the live 'churn'
chart for the headline churn rate.

Use this command for who churned and the dollar exposure. Do NOT use it for the
recoverable still-failing window (subs still in grace/billing-issue with unpaid
invoices); use 'dunning-alert' instead.

Data source: auto (local subscriptions mirror; live churn chart when reachable).`,
		Example: "  revenuecat-pp-cli churn-watch --project proj1ab2c3d4 --since 30d --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "auto",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if err := validateDataSourceStrategy(flags, "auto"); err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan the local subscriptions mirror for churned subs and sum dollar exposure")
				return nil
			}
			projectID, err := resolveProjectID(projectFlag)
			if err != nil {
				return err
			}
			if since == "" {
				since = "30d"
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("revenuecat-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "subscriptions", nil, flags.maxAge)

			view, err := buildChurnWatch(db, projectID, window, since)
			if err != nil {
				return err
			}
			// Best-effort live cross-ref of the churn chart unless the caller
			// pinned --data-source local.
			if flags.dataSource != "local" {
				if c, cerr := flags.newClient(); cerr == nil {
					if rate, ok := fetchChurnRate(cmd.Context(), c, projectID); ok {
						view.ChartChurnRate = rate
					}
				}
			}
			return emitChurnWatch(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&projectFlag, "project", "", "RevenueCat project id (or set REVENUECAT_PROJECT_ID)")
	cmd.Flags().StringVar(&since, "since", "30d", "Window for the churn period-end (e.g. 24h, 7d, 30d)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

func emitChurnWatch(cmd *cobra.Command, flags *rootFlags, view churnWatchView) error {
	if len(view.Rows) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Rows))
		for _, r := range view.Rows {
			items = append(items, map[string]any{
				"subscription_id": r.SubscriptionID,
				"status":          r.Status,
				"auto_renewal":    r.AutoRenewalStatus,
				"customer_id":     r.CustomerID,
				"product_id":      r.ProductID,
				"period_ends_at":  r.PeriodEndsAt,
				"exposure_usd":    fmt.Sprintf("%.2f", r.ExposureUSD),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nTotal exposure: $%.2f across %d subscription(s) in the last %s.\n",
			view.DollarExposure, view.Count, view.Since)
		if view.ChartChurnRate > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Live churn-chart rate: %.4f\n", view.ChartChurnRate)
		}
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

// churnWatchSubScanCap bounds the subscriptions scan.
// Aligned with loadSubscriptionStatusCap (rc_helpers.go) so the same
// subscriptions scan is truncated identically across commands.
const churnWatchSubScanCap = 500000

func buildChurnWatch(db *store.Store, projectID string, window time.Duration, sinceLabel string) (churnWatchView, error) {
	now := time.Now().UTC()
	cutoff := now.Add(-window)
	view := churnWatchView{
		ProjectID:   projectID,
		Rows:        []churnRow{},
		Since:       sinceLabel,
		WindowStart: cutoff.Format(time.RFC3339),
		WindowEnd:   now.Format(time.RFC3339),
	}

	rows, err := db.Query(
		`SELECT data FROM resources
		 WHERE resource_type IN ('subscriptions','customers_subscriptions') LIMIT ?`,
		churnWatchSubScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying subscriptions: %w", err)
	}
	defer rows.Close()
	scanned := 0

	for rows.Next() {
		scanned++
		var data sql.NullString
		if rows.Scan(&data) != nil || !data.Valid {
			continue
		}
		var obj map[string]any
		if json.Unmarshal([]byte(data.String), &obj) != nil {
			continue
		}
		status, _ := obj["status"].(string)
		autoRenewal, _ := obj["auto_renewal_status"].(string)
		// A subscription is churn (or churning) if its status is a churn state
		// OR the customer has turned off auto-renew (will_not_renew), which
		// RevenueCat reports via auto_renewal_status while status may still be
		// "active" for the remaining paid period.
		if !churnStatuses[status] && autoRenewal != willNotRenewStatus {
			continue
		}
		ends := rcEpochMSToTime(obj["current_period_ends_at"])
		// When the period-end is unknown we cannot place it in a window, so
		// keep the row only if it is unambiguously in-window.
		if ends.IsZero() || ends.Before(cutoff) {
			continue
		}
		row := churnRow{
			SubscriptionID: toStringRC(obj["id"]),
			Status:         status,
			CustomerID:     toStringRC(obj["customer_id"]),
			ProductID:      toStringRC(obj["product_id"]),
			PeriodEndsAt:   ends.Format(time.RFC3339),
			endsAtTime:     ends,
			ExposureUSD:    monetaryGrossUSD(obj["total_revenue_in_usd"]),
		}
		row.AutoRenewalStatus = autoRenewal
		view.Rows = append(view.Rows, row)
		view.DollarExposure += row.ExposureUSD
	}
	if err := rows.Err(); err != nil {
		return view, fmt.Errorf("iterating subscriptions: %w", err)
	}
	if scanned >= churnWatchSubScanCap {
		fmt.Fprintf(os.Stderr, "warning: churn-watch hit the %d-subscription scan cap; results may be incomplete\n", churnWatchSubScanCap)
		view.Note = fmt.Sprintf("hit the %d-subscription scan cap; results may be incomplete.", churnWatchSubScanCap)
	}
	sort.Slice(view.Rows, func(i, j int) bool {
		return view.Rows[i].endsAtTime.After(view.Rows[j].endsAtTime)
	})
	view.Count = len(view.Rows)
	if view.Count == 0 && view.Note == "" {
		view.Note = fmt.Sprintf("no churned or cancelled (will_not_renew) subscriptions with a period-end in the last %s", sinceLabel)
	}
	return view, nil
}

// fetchChurnRate fetches the live churn chart and returns its latest data
// point's first series value (best-effort; false on any failure).
func fetchChurnRate(ctx context.Context, c *client.Client, projectID string) (float64, bool) {
	cd, err := fetchChart(ctx, c, projectID, "churn", nil)
	if err != nil {
		return 0, false
	}
	pts := cd.points()
	if len(pts) == 0 {
		return 0, false
	}
	sort.Slice(pts, func(i, j int) bool { return pts[i].When.Before(pts[j].When) })
	return pts[len(pts)-1].firstSeriesValue(), true
}
