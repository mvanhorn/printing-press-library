// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Revenue report: payments/refunds by status, type, currency over a window.
// pp:data-source local
package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/payments/quentli/internal/store"
	"github.com/spf13/cobra"
)

type revenuePayment struct {
	ID          string  `json:"id"`
	Amount      float64 `json:"amount"`
	Currency    string  `json:"currency"`
	Status      string  `json:"status"`
	Type        string  `json:"type"`
	CreatedAt   string  `json:"createdAt"`
	IsCompleted bool    `json:"isCompleted"`
}

type currencyBucket struct {
	Currency           string  `json:"currency"`
	CompletedAmount    float64 `json:"completed_amount"`
	CompletedFormatted string  `json:"completed_formatted"`
	RefundedAmount     float64 `json:"refunded_amount"`
	RefundedFormatted  string  `json:"refunded_formatted"`
	NetAmount          float64 `json:"net_amount"`
	NetFormatted       string  `json:"net_formatted"`
	PaymentCount       int     `json:"payment_count"`
}

type revenueView struct {
	Period     string           `json:"period"`
	ByStatus   map[string]int   `json:"by_status"`
	ByCurrency []currencyBucket `json:"by_currency"`
}

func newNovelRevenueCmd(flags *rootFlags) *cobra.Command {
	var flagSince string
	var dbPath string
	cmd := &cobra.Command{
		Use:         "revenue",
		Short:       "Aggregate payments and refunds by status, type, and currency to see net collected vs returned",
		Example:     "  quentli-pp-cli revenue --since 30d --csv",
		Long:        "Reach for this to answer how much money actually landed last month and what came back in refunds.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "revenue")
			}
			var since time.Duration
			if flagSince != "" {
				parsed, err := cliutil.ParseDurationLoose(flagSince)
				if err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("invalid --since %q: %w", flagSince, err))
				}
				since = parsed
			}
			if dbPath == "" {
				dbPath = defaultDBPath("quentli-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: quentli-pp-cli sync --resources payments --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), revenueView{}, flags)
				}
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			hintIfUnsynced(cmd, db, "payments")

			dbg, err := loadAll[revenuePayment](db, "payments")
			if err != nil {
				return err
			}
			now := time.Now().UTC()
			byStatus := map[string]int{}
			bucket := map[string]*currencyBucket{}
			for _, p := range dbg {
				if since > 0 && p.CreatedAt != "" {
					if ct := cliutil.ParseStoredTime(p.CreatedAt); !ct.IsZero() && now.Sub(ct) > since {
						continue
					}
				}
				status := p.Status
				if status == "" {
					status = "unknown"
				}
				byStatus[status]++
				key := p.Currency
				if key == "" {
					key = "MXN"
				}
				b := bucket[key]
				if b == nil {
					b = &currencyBucket{Currency: key}
					bucket[key] = b
				}
				b.PaymentCount++
				if strings.EqualFold(p.Type, "refund") || p.Amount < 0 {
					b.RefundedAmount += absVal(p.Amount)
				} else if p.IsCompleted || strings.EqualFold(status, "COMPLETED") {
					b.CompletedAmount += p.Amount
				}
			}
			buckets := make([]currencyBucket, 0, len(bucket))
			for _, b := range bucket {
				b.NetAmount = b.CompletedAmount - b.RefundedAmount
				b.CompletedFormatted = formatMoneyMinor(b.CompletedAmount, b.Currency)
				b.RefundedFormatted = formatMoneyMinor(b.RefundedAmount, b.Currency)
				b.NetFormatted = formatMoneyMinor(b.NetAmount, b.Currency)
				buckets = append(buckets, *b)
			}
			sort.Slice(buckets, func(i, j int) bool { return buckets[i].NetAmount > buckets[j].NetAmount })
			view := revenueView{Period: flagSince, ByStatus: byStatus, ByCurrency: buckets}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, b := range buckets {
				fmt.Fprintf(cmd.OutOrStdout(), "%s  collected %s  refunded %s  net %s  (%d payments)\n", b.Currency, b.CompletedFormatted, b.RefundedFormatted, b.NetFormatted, b.PaymentCount)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "", "only consider payments in this window (e.g. 30d, 1w)")
	cmd.Flags().StringVar(&dbPath, "db", "", "path to the local database")
	return cmd
}

func absVal(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
