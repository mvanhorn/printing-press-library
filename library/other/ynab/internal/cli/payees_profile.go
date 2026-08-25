// Copyright 2026 mikesnowbie and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command scaffold. Implement the RunE body before shipping.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source live
// No local mirror exists for this CLI (no sync command was generated for
// this spec), so this command reads live from the YNAB API on every call.

package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

// ynabPayeeTransactionsEnvelope mirrors YNAB's TransactionsResponse for the
// payee-scoped transactions endpoint, whose items are TransactionDetail
// (richer than TransactionSummary — includes resolved category_name).
type ynabPayeeTransactionsEnvelope struct {
	Data struct {
		Transactions []ynabPayeeTransaction `json:"transactions"`
	} `json:"data"`
}

type ynabPayeeTransaction struct {
	ID             string  `json:"id"`
	Date           string  `json:"date"`
	AmountCurrency float64 `json:"amount_currency"`
	CategoryName   *string `json:"category_name"`
	Deleted        bool    `json:"deleted"`
}

type payeeMonthlyBucket struct {
	Month string  `json:"month"`
	Total float64 `json:"total"`
	Count int     `json:"count"`
}

type payeeProfileResult struct {
	PayeeID            string               `json:"payee_id"`
	PeriodMonths       int                  `json:"period_months"`
	TransactionCount   int                  `json:"transaction_count"`
	TotalSpend         float64              `json:"total_spend"`
	AverageTransaction float64              `json:"average_transaction"`
	MonthlyBreakdown   []payeeMonthlyBucket `json:"monthly_breakdown"`
	TopCategories      []categoryFrequency  `json:"top_categories,omitempty"`
}

type categoryFrequency struct {
	Category string `json:"category"`
	Count    int    `json:"count"`
}

var periodPattern = regexp.MustCompile(`^(\d+)(m|mo|y)$`)

// parsePeriodMonths converts a shorthand period like "6m", "3mo", or "1y"
// into a month count. Empty input defaults to 12 months (matches the API's
// own one-year default lookback when since_date is omitted).
func parsePeriodMonths(period string) (int, error) {
	if period == "" {
		return 12, nil
	}
	m := periodPattern.FindStringSubmatch(period)
	if m == nil {
		return 0, fmt.Errorf("--period %q is not a valid shorthand; use forms like 3m, 6m, or 1y", period)
	}
	n, err := strconv.Atoi(m[1])
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("--period %q is not a valid shorthand; use forms like 3m, 6m, or 1y", period)
	}
	if m[2] == "y" {
		n *= 12
	}
	return n, nil
}

func newNovelPayeesProfileCmd(flags *rootFlags) *cobra.Command {
	var flagPeriod string
	var flagPlan string

	cmd := &cobra.Command{
		Use:     "profile <payee-id>",
		Short:   "Aggregated monthly spend stats, average transaction size, and typical categories for a single payee.",
		Example: "  ynab-pp-cli payees profile a1b2c3d4 --period 6m --agent",
		// YNAB's payee-transactions endpoint returns HTTP 200 with an empty
		// transactions array for an unknown payee_id rather than 404ing, so
		// this command cannot distinguish "bad ID" from "real payee with no
		// transactions in the window" without an extra payee-lookup call.
		// pp:happy-args gives the live dogfood happy-path check a
		// syntactically-valid fixture ID so it actually exercises a real
		// request (which gracefully returns zero transactions) instead of
		// being skipped for lacking required positional input.
		Annotations: map[string]string{
			"mcp:read-only":          "true",
			"pp:no-error-path-probe": "true",
			"pp:happy-args":          "<payee-id>=00000000-0000-0000-0000-000000000000",
			"pp:typed-exit-codes":    "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "payees profile")
			}
			if len(args) == 0 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required argument <payee-id>\nUsage: %s <payee-id>", cmd.CommandPath()))
			}
			payeeID := args[0]

			months, err := parsePeriodMonths(flagPeriod)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/plans/{plan_id}/payees/{payee_id}/transactions"
			path = replacePathParam(path, "plan_id", flagPlan)
			path = replacePathParam(path, "payee_id", payeeID)
			sinceDate := time.Now().AddDate(0, -months, 0).Format("2006-01-02")
			params := map[string]string{"since_date": sinceDate}
			raw, err := c.GetWithHeadersNoCache(ctx, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var envelope ynabPayeeTransactionsEnvelope
			if err := json.Unmarshal(raw, &envelope); err != nil {
				return fmt.Errorf("parsing transactions response: %w", err)
			}

			buckets := map[string]*payeeMonthlyBucket{}
			categoryCounts := map[string]int{}
			var total float64
			count := 0
			for _, t := range envelope.Data.Transactions {
				if t.Deleted {
					continue
				}
				count++
				total += t.AmountCurrency
				month := t.Date
				if len(month) >= 7 {
					month = month[:7] // YYYY-MM
				}
				b, ok := buckets[month]
				if !ok {
					b = &payeeMonthlyBucket{Month: month}
					buckets[month] = b
				}
				b.Total += t.AmountCurrency
				b.Count++
				if t.CategoryName != nil && *t.CategoryName != "" && *t.CategoryName != "Split" {
					categoryCounts[*t.CategoryName]++
				}
			}

			monthly := make([]payeeMonthlyBucket, 0, len(buckets))
			for _, b := range buckets {
				b.Total = math.Round(b.Total*100) / 100
				monthly = append(monthly, *b)
			}
			sort.Slice(monthly, func(i, j int) bool { return monthly[i].Month > monthly[j].Month })

			topCategories := make([]categoryFrequency, 0, len(categoryCounts))
			for name, n := range categoryCounts {
				topCategories = append(topCategories, categoryFrequency{Category: name, Count: n})
			}
			sort.Slice(topCategories, func(i, j int) bool {
				if topCategories[i].Count != topCategories[j].Count {
					return topCategories[i].Count > topCategories[j].Count
				}
				return topCategories[i].Category < topCategories[j].Category
			})
			if len(topCategories) > 5 {
				topCategories = topCategories[:5]
			}

			avg := 0.0
			if count > 0 {
				avg = math.Round((total/float64(count))*100) / 100
			}

			result := payeeProfileResult{
				PayeeID:            payeeID,
				PeriodMonths:       months,
				TransactionCount:   count,
				TotalSpend:         math.Round(total*100) / 100,
				AverageTransaction: avg,
				MonthlyBreakdown:   monthly,
				TopCategories:      topCategories,
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if count == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No transactions found for this payee in the last %d month(s).\n", months)
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Transactions:        %d\n", result.TransactionCount)
			fmt.Fprintf(cmd.OutOrStdout(), "Total spend:         %.2f\n", result.TotalSpend)
			fmt.Fprintf(cmd.OutOrStdout(), "Average transaction: %.2f\n\n", result.AverageTransaction)
			fmt.Fprintln(cmd.OutOrStdout(), "Monthly breakdown:")
			for _, b := range monthly {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %8.2f  (%d)\n", b.Month, b.Total, b.Count)
			}
			if len(topCategories) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTop categories:")
				for _, cat := range topCategories {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-30s %d\n", cat.Category, cat.Count)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagPeriod, "period", "12m", "Lookback window as shorthand (e.g. 3m, 6m, 1y); defaults to 12 months")
	cmd.Flags().StringVar(&flagPlan, "plan", "last-used", `Plan (budget) ID, or "last-used" for the most recently used plan`)
	return cmd
}
