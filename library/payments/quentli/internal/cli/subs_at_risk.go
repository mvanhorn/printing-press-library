// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Subscriptions at-risk: active subs on broken/missing payment methods or recent failed collection.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
	"github.com/spf13/cobra"
)

type subRisk struct {
	ID                 string `json:"id"`
	Status             string `json:"status"`
	CustomerID         string `json:"customerId"`
	CollectionMethod   string `json:"collectionMethod"`
	PaymentMethodID    string `json:"paymentMethodId"`
	NextCollectionDate string `json:"nextCollectionDate"`
	IsActive           bool   `json:"isActive"`
}

type atRiskPayment struct {
	ID         string `json:"id"`
	CustomerID string `json:"customerId"`
	Status     string `json:"status"`
	Type       string `json:"type"`
	CreatedAt  string `json:"createdAt"`
}

type subRiskRow struct {
	SubscriptionID     string `json:"subscription_id"`
	CustomerID         string `json:"customer_id"`
	Status             string `json:"status"`
	CollectionMethod   string `json:"collection_method"`
	NextCollectionDate string `json:"next_collection_date,omitempty"`
	PaymentMethodID    string `json:"payment_method_id,omitempty"`
	Risk               string `json:"risk"`
	LastFailedAt       string `json:"last_failed_at,omitempty"`
}

func newNovelSubsAtRiskCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "at-risk",
		Short:       "Surface active subscriptions tied to broken or missing payment methods or recent failed collection attempts",
		Example:     "  quentli-pp-cli subs at-risk --json",
		Long:        "Whole-portfolio recovery queue; use 'customer balance <id>' for a single subscription's details.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "subs at-risk")
			}
			if dbPath == "" {
				dbPath = defaultDBPath("quentli-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: quentli-pp-cli sync --resources subscriptions,payments --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]subRiskRow, 0), flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "subscriptions")

			subs, err := loadAll[subRisk](db, "subscriptions")
			if err != nil {
				return err
			}
			dbg, err := loadAll[atRiskPayment](db, "payments")
			if err != nil {
				return err
			}
			// most recent failed collection per customer
			lastFailed := map[string]string{}
			for _, p := range dbg {
				if strings.EqualFold(p.Type, "refund") {
					continue
				}
				if strings.EqualFold(p.Status, "FAILED") || strings.EqualFold(p.Status, "REJECTED") {
					if lastFailed[p.CustomerID] == "" || p.CreatedAt > lastFailed[p.CustomerID] {
						lastFailed[p.CustomerID] = p.CreatedAt
					}
				}
			}

			rows := make([]subRiskRow, 0)
			for _, s := range subs {
				if !s.IsActive {
					continue
				}
				if strings.EqualFold(s.Status, "CANCELED") || strings.EqualFold(s.Status, "COMPLETED") {
					continue
				}
				row := subRiskRow{
					SubscriptionID:     s.ID,
					CustomerID:         s.CustomerID,
					Status:             s.Status,
					CollectionMethod:   s.CollectionMethod,
					NextCollectionDate: s.NextCollectionDate,
					PaymentMethodID:    s.PaymentMethodID,
				}
				if strings.EqualFold(s.CollectionMethod, "AUTOMATIC") && s.PaymentMethodID == "" {
					row.Risk = "missing_payment_method"
					rows = append(rows, row)
					continue
				}
				if lf, ok := lastFailed[s.CustomerID]; ok {
					row.Risk = "failed_collection"
					row.LastFailedAt = lf
					rows = append(rows, row)
				}
			}
			sort.Slice(rows, func(i, j int) bool {
				return rows[i].NextCollectionDate > rows[j].NextCollectionDate
			})
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No at-risk subscriptions found.")
				return nil
			}
			table := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				table = append(table, map[string]any{
					"subscription": r.SubscriptionID,
					"customer":     r.CustomerID,
					"collection":   r.CollectionMethod,
					"risk":         r.Risk,
					"last_failed":  r.LastFailedAt,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "path to the local database")
	return cmd
}
