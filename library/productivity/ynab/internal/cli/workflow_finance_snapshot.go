// Copyright 2026 christophersilvestri and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(pp-ynab-finance-snapshot): Adds a YNAB-specific read-only compound workflow for finance decisions.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/productivity/ynab/internal/client"
	"github.com/spf13/cobra"
)

type ynabSnapshotAccount struct {
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Balance        float64 `json:"balance"`
	ClearedBalance float64 `json:"cleared_balance"`
}

type ynabSnapshotMonth struct {
	Month       string  `json:"month"`
	Spend       float64 `json:"spend"`
	Partial     bool    `json:"partial,omitempty"`
	SourceError string  `json:"source_error,omitempty"`
}

type ynabSnapshotCategory struct {
	Name        string  `json:"name"`
	MonthlyMean float64 `json:"monthly_mean"`
}

type ynabSnapshotRunway struct {
	Burn   float64 `json:"burn"`
	Months float64 `json:"months"`
}

type ynabFinanceSnapshot struct {
	GeneratedAt         string                 `json:"generated_at"`
	BudgetID            string                 `json:"budget_id"`
	OperatingAccounts   []ynabSnapshotAccount  `json:"operating_accounts"`
	AssetAccounts       []ynabSnapshotAccount  `json:"asset_accounts"`
	OtherAccounts       []ynabSnapshotAccount  `json:"other_accounts"`
	OperatingCash       float64                `json:"operating_cash"`
	ManualISA           float64                `json:"manual_isa,omitempty"`
	ManualBTC           float64                `json:"manual_btc,omitempty"`
	RunwayBase          float64                `json:"runway_base"`
	Months              []ynabSnapshotMonth    `json:"months"`
	AverageFullMonth    float64                `json:"average_full_month"`
	AverageLast3Full    float64                `json:"average_last_3_full"`
	Runway              []ynabSnapshotRunway   `json:"runway"`
	TopCategoryAverages []ynabSnapshotCategory `json:"top_category_averages"`
	Notes               []string               `json:"notes"`
}

type ynabAccountsResponse struct {
	Data struct {
		Accounts []struct {
			Name            string `json:"name"`
			Type            string `json:"type"`
			Closed          bool   `json:"closed"`
			Deleted         bool   `json:"deleted"`
			Balance         int64  `json:"balance"`
			ClearedBalance  int64  `json:"cleared_balance"`
			UnclearedAmount int64  `json:"uncleared_balance"`
		} `json:"accounts"`
	} `json:"data"`
}

type ynabMonthResponse struct {
	Data struct {
		Month struct {
			Month      string `json:"month"`
			Categories []struct {
				CategoryGroupName string `json:"category_group_name"`
				Name              string `json:"name"`
				Hidden            bool   `json:"hidden"`
				Deleted           bool   `json:"deleted"`
				Internal          bool   `json:"internal"`
				Activity          int64  `json:"activity"`
			} `json:"categories"`
		} `json:"month"`
	} `json:"data"`
}

func newWorkflowFinanceSnapshotCmd(flags *rootFlags) *cobra.Command {
	var budgetID string
	var months int
	var manualISA float64
	var manualBTC float64
	var noManual bool

	cmd := &cobra.Command{
		Use:   "finance-snapshot",
		Short: "Summarize YNAB operating cash, recent burn, and runway",
		Long: `Builds a read-only finance snapshot from YNAB accounts and monthly category activity.

The workflow treats ordinary checking/cash accounts as operating cash and separates
asset-like accounts such as ISA, trading, Strike, crypto, and BTC by name/type.
Use --manual-isa and --manual-btc for assets tracked outside YNAB or when YNAB's
tracking-account values are stale. The command never prints credentials.`,
		Example: `  # Snapshot the last six months from the last-used budget
  ynab-pp-cli workflow finance-snapshot --json

  # Use a specific budget and manual asset baselines
  ynab-pp-cli workflow finance-snapshot --budget-id last-used --manual-isa 18044.27 --manual-btc 0.467 --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if months < 1 || months > 24 {
				return fmt.Errorf("--months must be between 1 and 24")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			c.NoCache = true
			if budgetID == "" {
				budgetID = "last-used"
			}
			includeManual := !noManual
			snap, err := buildYNABFinanceSnapshot(cmd.Context(), c, budgetID, months, includeManual, manualISA, manualBTC)
			if err != nil {
				return err
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(snap)
			}
			printYNABFinanceSnapshot(cmd, snap)
			return nil
		},
	}
	cmd.Flags().StringVar(&budgetID, "budget-id", "last-used", "Budget ID, or last-used")
	cmd.Flags().IntVar(&months, "months", 6, "Months of category activity to inspect, including the current month")
	cmd.Flags().Float64Var(&manualISA, "manual-isa", 0, "Manual ISA/liquid investment value to add to runway")
	cmd.Flags().Float64Var(&manualBTC, "manual-btc", 0, "Manual BTC holding to report; not priced into runway")
	cmd.Flags().BoolVar(&noManual, "no-manual", false, "Do not include manual asset fields")
	return cmd
}

func buildYNABFinanceSnapshot(ctx context.Context, c *client.Client, budgetID string, monthsBack int, includeManual bool, manualISA float64, manualBTC float64) (ynabFinanceSnapshot, error) {
	snap := ynabFinanceSnapshot{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		BudgetID:    budgetID,
		Notes: []string{
			"YNAB amounts are converted from milliunits to currency units.",
			"Tax buckets and business-bank balances are only included if represented in this YNAB budget.",
			"For material purchase decisions, confirm current tax bucket, expected income, and manual assets.",
		},
	}
	if includeManual {
		snap.ManualISA = manualISA
		snap.ManualBTC = manualBTC
	}

	accountsRaw, err := c.GetNoCache(ctx, fmt.Sprintf("/budgets/%s/accounts", budgetID), nil)
	if err != nil {
		return snap, fmt.Errorf("fetching accounts: %w", err)
	}
	var accounts ynabAccountsResponse
	if err := json.Unmarshal(accountsRaw, &accounts); err != nil {
		return snap, fmt.Errorf("decoding accounts: %w", err)
	}
	for _, a := range accounts.Data.Accounts {
		if a.Closed || a.Deleted {
			continue
		}
		row := ynabSnapshotAccount{Name: a.Name, Type: a.Type, Balance: milliunits(a.Balance), ClearedBalance: milliunits(a.ClearedBalance)}
		if isYNABOperatingAccount(a.Name, a.Type) {
			snap.OperatingAccounts = append(snap.OperatingAccounts, row)
			snap.OperatingCash += row.Balance
		} else if isYNABAssetAccount(a.Name, a.Type) {
			snap.AssetAccounts = append(snap.AssetAccounts, row)
		} else {
			snap.OtherAccounts = append(snap.OtherAccounts, row)
		}
	}
	sortSnapshotAccounts(snap.OperatingAccounts)
	sortSnapshotAccounts(snap.AssetAccounts)
	sortSnapshotAccounts(snap.OtherAccounts)

	categoryTotals := map[string]float64{}
	categoryAverageMonthCount := 0
	currentMonth := firstOfMonth(time.Now())
	for i := monthsBack - 1; i >= 0; i-- {
		month := currentMonth.AddDate(0, -i, 0)
		label := month.Format("2006-01")
		entry := ynabSnapshotMonth{Month: label, Partial: month.Equal(currentMonth)}
		raw, err := c.GetNoCache(ctx, fmt.Sprintf("/budgets/%s/months/%s", budgetID, month.Format("2006-01-02")), nil)
		if err != nil {
			entry.SourceError = err.Error()
			snap.Months = append(snap.Months, entry)
			continue
		}
		var mr ynabMonthResponse
		if err := json.Unmarshal(raw, &mr); err != nil {
			entry.SourceError = err.Error()
			snap.Months = append(snap.Months, entry)
			continue
		}
		for _, cat := range mr.Data.Month.Categories {
			if !isYNABSpendingCategory(cat.CategoryGroupName, cat.Name, cat.Hidden, cat.Deleted, cat.Internal) {
				continue
			}
			spend := -milliunits(cat.Activity)
			entry.Spend += spend
			if !entry.Partial {
				categoryTotals[cat.CategoryGroupName+": "+cat.Name] += spend
			}
		}
		if !entry.Partial {
			categoryAverageMonthCount++
		}
		snap.Months = append(snap.Months, entry)
	}

	full := make([]ynabSnapshotMonth, 0, len(snap.Months))
	for _, m := range snap.Months {
		if !m.Partial && m.SourceError == "" {
			full = append(full, m)
		}
	}
	snap.AverageFullMonth = averageMonthSpend(full)
	last3 := full
	if len(last3) > 3 {
		last3 = last3[len(last3)-3:]
	}
	snap.AverageLast3Full = averageMonthSpend(last3)

	snap.RunwayBase = snap.OperatingCash
	if includeManual {
		snap.RunwayBase += manualISA
	}
	for _, burn := range []float64{2200, 2500, 3000, 3500, 4000} {
		snap.Runway = append(snap.Runway, ynabSnapshotRunway{Burn: burn, Months: round1(snap.RunwayBase / burn)})
	}

	divisor := float64(categoryAverageMonthCount)
	if divisor == 0 {
		divisor = 1
	}
	for name, total := range categoryTotals {
		snap.TopCategoryAverages = append(snap.TopCategoryAverages, ynabSnapshotCategory{Name: name, MonthlyMean: total / divisor})
	}
	sort.Slice(snap.TopCategoryAverages, func(i, j int) bool {
		return snap.TopCategoryAverages[i].MonthlyMean > snap.TopCategoryAverages[j].MonthlyMean
	})
	if len(snap.TopCategoryAverages) > 15 {
		snap.TopCategoryAverages = snap.TopCategoryAverages[:15]
	}
	return snap, nil
}

func printYNABFinanceSnapshot(cmd *cobra.Command, snap ynabFinanceSnapshot) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "YNAB finance snapshot — %s\n", snap.GeneratedAt)
	fmt.Fprintf(out, "Budget: %s\n\n", snap.BudgetID)
	fmt.Fprintln(out, "Operating accounts:")
	if len(snap.OperatingAccounts) == 0 {
		fmt.Fprintln(out, "- none found")
	}
	for _, a := range snap.OperatingAccounts {
		fmt.Fprintf(out, "- %s: %s balance / %s cleared (%s)\n", a.Name, formatGBP(a.Balance), formatGBP(a.ClearedBalance), a.Type)
	}
	fmt.Fprintf(out, "Operating cash: %s\n", formatGBP(snap.OperatingCash))
	if len(snap.AssetAccounts) > 0 {
		fmt.Fprintln(out, "\nAsset-like accounts excluded from operating cash:")
		for _, a := range snap.AssetAccounts {
			fmt.Fprintf(out, "- %s: %s balance / %s cleared (%s)\n", a.Name, formatGBP(a.Balance), formatGBP(a.ClearedBalance), a.Type)
		}
	}
	if snap.ManualISA > 0 || snap.ManualBTC > 0 {
		fmt.Fprintln(out, "")
		if snap.ManualISA > 0 {
			fmt.Fprintf(out, "Manual ISA: %s\n", formatGBP(snap.ManualISA))
		}
		if snap.ManualBTC > 0 {
			fmt.Fprintf(out, "Manual BTC: %.8g BTC (not priced)\n", snap.ManualBTC)
		}
	}
	fmt.Fprintln(out, "\nMonthly category burn:")
	for _, m := range snap.Months {
		partial := ""
		if m.Partial {
			partial = " partial"
		}
		if m.SourceError != "" {
			fmt.Fprintf(out, "- %s%s: error: %s\n", m.Month, partial, m.SourceError)
			continue
		}
		fmt.Fprintf(out, "- %s%s: %s\n", m.Month, partial, formatGBP(m.Spend))
	}
	fmt.Fprintf(out, "Average full-month burn: %s\n", formatGBP(snap.AverageFullMonth))
	fmt.Fprintf(out, "Average last 3 full months: %s\n", formatGBP(snap.AverageLast3Full))
	fmt.Fprintln(out, "\nRunway:")
	for _, r := range snap.Runway {
		fmt.Fprintf(out, "- at %s/mo: %.1f months\n", formatGBP(r.Burn), r.Months)
	}
	fmt.Fprintln(out, "\nTop category averages:")
	for _, cat := range snap.TopCategoryAverages {
		fmt.Fprintf(out, "- %s: %s/mo\n", cat.Name, formatGBP(cat.MonthlyMean))
	}
}

func milliunits(v int64) float64 { return float64(v) / 1000.0 }

func firstOfMonth(t time.Time) time.Time {
	y, m, _ := t.Date()
	return time.Date(y, m, 1, 0, 0, 0, 0, t.Location())
}

func isYNABOperatingAccount(name, typ string) bool {
	if hasYNABAssetKeyword(name) {
		return false
	}
	return typ == "checking" || typ == "cash"
}

func isYNABAssetAccount(name, typ string) bool {
	if hasYNABAssetKeyword(name) {
		return true
	}
	return typ == "savings" || typ == "otherAsset" || typ == "investmentAccount"
}

func hasYNABAssetKeyword(name string) bool {
	for _, token := range strings.FieldsFunc(strings.ToLower(name), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		switch token {
		case "isa", "trading", "strike", "crypto", "btc", "bitcoin":
			return true
		}
	}
	return false
}

func isYNABSpendingCategory(group, name string, hidden, deleted, internal bool) bool {
	if hidden || deleted || internal {
		return false
	}
	groupLower := strings.ToLower(group)
	nameLower := strings.ToLower(name)
	if groupLower == "internal master category" || groupLower == "credit card payments" {
		return false
	}
	if nameLower == "ready to assign" || nameLower == "inflow: ready to assign" || nameLower == "investment" || nameLower == "isa" || nameLower == "savings" {
		return false
	}
	return true
}

func averageMonthSpend(months []ynabSnapshotMonth) float64 {
	if len(months) == 0 {
		return 0
	}
	var total float64
	for _, m := range months {
		total += m.Spend
	}
	return total / float64(len(months))
}

func sortSnapshotAccounts(accounts []ynabSnapshotAccount) {
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

func formatGBP(v float64) string {
	if v < 0 {
		return fmt.Sprintf("-£%.2f", -v)
	}
	return fmt.Sprintf("£%.2f", v)
}
