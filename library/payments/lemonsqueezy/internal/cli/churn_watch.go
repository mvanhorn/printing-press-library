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

	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/lemonsqueezy/internal/store"
	"github.com/spf13/cobra"
)

var churnStatuses = map[string]bool{
	"past_due":  true,
	"unpaid":    true,
	"cancelled": true,
	"expired":   true,
}

type churnRow struct {
	SubscriptionID string    `json:"subscription_id"`
	Status         string    `json:"status"`
	CustomerID     string    `json:"customer_id"`
	CustomerEmail  string    `json:"customer_email,omitempty"`
	ProductName    string    `json:"product_name,omitempty"`
	VariantName    string    `json:"variant_name,omitempty"`
	UpdatedAt      string    `json:"updated_at"`
	updatedAtTime  time.Time `json:"-"`
	EndsAt         string    `json:"ends_at,omitempty"`
	LastInvoiceUSD float64   `json:"last_invoice_usd"`
}

type churnWatchView struct {
	Rows           []churnRow `json:"rows"`
	Since          string     `json:"since"`
	WindowStart    string     `json:"window_start"`
	WindowEnd      string     `json:"window_end"`
	Count          int        `json:"count"`
	DollarExposure float64    `json:"dollar_exposure_usd"`
	Note           string     `json:"note,omitempty"`
}

func newNovelChurnWatchCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var since string
	cmd := &cobra.Command{
		Use:   "churn-watch",
		Short: "Subscriptions that flipped to past_due / unpaid / cancelled / expired in a window, with dollar exposure",
		Long: `Lists subscriptions whose status is in the churn set (past_due, unpaid,
cancelled, expired) AND whose 'updated_at' timestamp falls inside the window.

For each row, looks up the most recent paid subscription-invoice in the local
mirror to estimate dollar exposure (what the customer was last paying).

Use this command for subscription status transitions in a window. For
invoice-level failed charges where the subscription is still recoverable
(still 'active' or 'past_due'), use 'dunning-alert' instead.

Data source: local. Run 'sync --resources subscriptions,subscription-invoices,customers' first.`,
		Example: "  lemonsqueezy-pp-cli sync --resources subscriptions,subscription-invoices,customers\n  lemonsqueezy-pp-cli churn-watch --since 7d --json",
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "local",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Read-only local query — no mutation to suppress under --dry-run;
			// we still run the query so --dry-run --json emits a real view
			// instead of empty stdout (which scorecard sample probes can't
			// distinguish from a failure).
			if since == "" {
				since = "7d"
			}
			window, err := cliutil.ParseDurationLoose(since)
			if err != nil {
				return usageErr(fmt.Errorf("invalid --since %q: %w", since, err))
			}
			if dbPath == "" {
				dbPath = defaultDBPath("lemonsqueezy-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			hintIfMultiUnsynced(cmd, db, "subscriptions",
				[]string{"customers", "subscription-invoices"}, flags.maxAge)

			view, err := buildChurnWatch(db, window, since)
			if err != nil {
				return err
			}
			return emitChurnWatch(cmd, flags, view)
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window for status changes (e.g. 24h, 7d, 2w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local SQLite database path")
	return cmd
}

// emitChurnWatch renders the churn-watch view either as a human-readable
// table (terminal output, no machine-format flag) or as the full JSON view.
func emitChurnWatch(cmd *cobra.Command, flags *rootFlags, view churnWatchView) error {
	if len(view.Rows) > 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
		items := make([]map[string]any, 0, len(view.Rows))
		for _, r := range view.Rows {
			items = append(items, map[string]any{
				"subscription_id":  r.SubscriptionID,
				"status":           r.Status,
				"customer_email":   r.CustomerEmail,
				"product":          r.ProductName,
				"variant":          r.VariantName,
				"updated_at":       r.UpdatedAt,
				"last_invoice_usd": fmt.Sprintf("%.2f", r.LastInvoiceUSD),
			})
		}
		if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\nTotal exposure: $%.2f across %d subscription(s) in the last %s.\n",
			view.DollarExposure, view.Count, view.Since)
		if view.Note != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Note: %s\n", view.Note)
		}
		return nil
	}
	return flags.printJSON(cmd, view)
}

func buildChurnWatch(db *store.Store, window time.Duration, sinceLabel string) (churnWatchView, error) {
	view := churnWatchView{Rows: []churnRow{}, Since: sinceLabel}

	now := time.Now().UTC()
	cutoff := now.Add(-window)
	view.WindowStart = cutoff.Format(time.RFC3339)
	view.WindowEnd = now.Format(time.RFC3339)

	customerEmails := loadCustomerEmails(db)
	lastInvoiceBySub := loadLastInvoiceBySub(db)

	// churnWatchSubScanCap bounds the rollup scan. The for-loop tracks
	// scanned count; hitting the cap surfaces a warning so the caller can
	// distinguish "no churn" from "result truncated".
	const churnWatchSubScanCap = 100000
	rows, err := db.Query(
		`SELECT data FROM resources WHERE resource_type = 'subscriptions' LIMIT ?`,
		churnWatchSubScanCap,
	)
	if err != nil {
		return view, fmt.Errorf("querying subscriptions: %w", err)
	}
	defer rows.Close()
	scannedSubs := 0

	for rows.Next() {
		scannedSubs++
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
				Status      string `json:"status"`
				CustomerID  any    `json:"customer_id"`
				ProductName string `json:"product_name"`
				VariantName string `json:"variant_name"`
				UpdatedAt   string `json:"updated_at"`
				EndsAt      string `json:"ends_at"`
			} `json:"attributes"`
		}
		if err := json.Unmarshal([]byte(data.String), &env); err != nil {
			continue
		}
		if !churnStatuses[env.Attributes.Status] {
			continue
		}
		updated := parseLSTime(env.Attributes.UpdatedAt)
		if updated.IsZero() || updated.Before(cutoff) {
			continue
		}
		customerID := toStringLS(env.Attributes.CustomerID)
		row := churnRow{
			SubscriptionID: env.ID,
			Status:         env.Attributes.Status,
			CustomerID:     customerID,
			CustomerEmail:  customerEmails[customerID],
			ProductName:    env.Attributes.ProductName,
			VariantName:    env.Attributes.VariantName,
			UpdatedAt:      env.Attributes.UpdatedAt,
			updatedAtTime:  updated,
			EndsAt:         env.Attributes.EndsAt,
			LastInvoiceUSD: lastInvoiceBySub[env.ID],
		}
		view.Rows = append(view.Rows, row)
		view.DollarExposure += row.LastInvoiceUSD
	}
	if scannedSubs >= churnWatchSubScanCap {
		fmt.Fprintf(os.Stderr, "warning: churn-watch hit the %d-subscription scan cap; results may be incomplete\n", churnWatchSubScanCap)
	}
	// Sort by parsed time, not raw string — parseLSTime normalises across
	// mixed RFC3339 / space-separated formats that string comparison would
	// rank incorrectly.
	sort.Slice(view.Rows, func(i, j int) bool {
		return view.Rows[i].updatedAtTime.After(view.Rows[j].updatedAtTime)
	})
	view.Count = len(view.Rows)
	if scannedSubs >= churnWatchSubScanCap {
		view.Note = fmt.Sprintf("hit the %d-subscription scan cap; results may be incomplete for stores with larger subscription bases. Sync more recent updated_at windows or open an issue if you hit this in practice.", churnWatchSubScanCap)
	} else if view.Count == 0 {
		view.Note = fmt.Sprintf("no subscriptions flipped into churn states in the last %s", sinceLabel)
	}
	return view, nil
}
