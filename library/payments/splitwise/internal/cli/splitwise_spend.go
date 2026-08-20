package cli

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func newSpendCmd(flags *rootFlags) *cobra.Command {
	groupBy := "category"
	cmd := &cobra.Command{
		Use:         "spend",
		Short:       "Show your spend (your share of each expense) grouped by category, group, or month",
		Example:     "  splitwise-pp-cli spend --group-by category --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute spend")
				return nil
			}

			if groupBy != "category" && groupBy != "group" && groupBy != "month" {
				return usageErr(fmt.Errorf("invalid --group-by value %q: must be category, group, or month", groupBy))
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-expenses")

			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}
			groups, err := loadGroups(db)
			if err != nil {
				return err
			}
			youID := loadCurrentUserID(db)

			groupNames := make(map[int]string)
			groupNames[0] = "Non-group"
			for _, g := range groups {
				groupNames[g.ID] = strings.TrimSpace(g.Name)
			}

			type key struct {
				Bucket       string
				CurrencyCode string
			}
			type agg struct {
				Total float64
				Count int
			}
			aggs := make(map[key]*agg)

			for _, e := range expenses {
				if e.Payment || expenseDeleted(e.DeletedAt) {
					continue
				}
				bucket := spendBucket(groupBy, e, groupNames)
				k := key{Bucket: bucket, CurrencyCode: strings.TrimSpace(e.CurrencyCode)}
				if _, ok := aggs[k]; !ok {
					aggs[k] = &agg{}
				}
				aggs[k].Total += userOwedShare(e, youID)
				aggs[k].Count++
			}

			type spendRow struct {
				Bucket       string  `json:"bucket"`
				CurrencyCode string  `json:"currency_code"`
				Total        float64 `json:"total"`
				Count        int     `json:"count"`
			}
			results := make([]spendRow, 0)
			for k, v := range aggs {
				results = append(results, spendRow{
					Bucket:       k.Bucket,
					CurrencyCode: k.CurrencyCode,
					Total:        round2(v.Total),
					Count:        v.Count,
				})
			}
			sort.Slice(results, func(i, j int) bool {
				if results[i].Total == results[j].Total {
					if results[i].Bucket == results[j].Bucket {
						return results[i].CurrencyCode < results[j].CurrencyCode
					}
					return results[i].Bucket < results[j].Bucket
				}
				return results[i].Total > results[j].Total
			})

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.emitStructured(cmd, results)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "BUCKET\tCURRENCY\tTOTAL\tCOUNT")
			for _, row := range results {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%.2f\t%d\n", row.Bucket, row.CurrencyCode, row.Total, row.Count)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "category", "Bucket spend by: category|group|month")
	return cmd
}

// userOwedShare returns the authenticated user's share of an expense (their
// actual spend), which is what "your spend" should total — not the full expense
// cost shared across everyone. Falls back to the full cost only when the
// current user id is unknown (no synced current-user record), so the command
// still produces a meaningful number rather than zero.
func userOwedShare(e Expense, youID int) float64 {
	if youID == 0 {
		return parseAmount(e.Cost)
	}
	for _, u := range e.Users {
		if u.UserID == youID {
			return parseAmount(u.OwedShare)
		}
	}
	return 0
}

func newLedgerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "ledger <group>",
		Short:       "Show group expense ledger and running balances",
		Example:     "  splitwise-pp-cli ledger \"Tahoe Trip\" --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would build ledger")
				return nil
			}
			if len(args) == 0 {
				return usageErr(errors.New("group name or id is required"))
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-groups")
			hintIfUnsynced(cmd, db, "get-expenses")

			groups, err := loadGroups(db)
			if err != nil {
				return err
			}

			// Rejoin a multi-word group name the MCP command-mirror split into
			// several positionals (see joinNameArgs in splitwise_settle.go).
			groupID, groupName, memberNames, err := resolveLedgerGroup(joinNameArgs(args), groups)
			if err != nil {
				return usageErr(err)
			}

			expenses, err := loadExpenses(db)
			if err != nil {
				return err
			}

			type ledgerExpense struct {
				Date         string `json:"date"`
				Description  string `json:"description"`
				Cost         string `json:"cost"`
				CurrencyCode string `json:"currency_code"`
				Payment      bool   `json:"payment"`
			}
			ledgerExpenses := make([]ledgerExpense, 0)

			type balKey struct {
				UserID       int
				CurrencyCode string
			}
			running := make(map[balKey]float64)

			for _, e := range expenses {
				if e.GroupID != groupID || expenseDeleted(e.DeletedAt) {
					continue
				}
				ledgerExpenses = append(ledgerExpenses, ledgerExpense{
					Date:         strings.TrimSpace(e.Date),
					Description:  strings.TrimSpace(e.Description),
					Cost:         strings.TrimSpace(e.Cost),
					CurrencyCode: strings.TrimSpace(e.CurrencyCode),
					Payment:      e.Payment,
				})
				for _, u := range e.Users {
					key := balKey{UserID: u.UserID, CurrencyCode: strings.TrimSpace(e.CurrencyCode)}
					running[key] += parseAmount(u.PaidShare) - parseAmount(u.OwedShare)
					if _, ok := memberNames[u.UserID]; !ok {
						name := strings.TrimSpace(strings.TrimSpace(u.User.FirstName) + " " + strings.TrimSpace(u.User.LastName))
						if name == "" {
							name = fmt.Sprintf("User %d", u.UserID)
						}
						memberNames[u.UserID] = name
					}
				}
			}

			sort.Slice(ledgerExpenses, func(i, j int) bool {
				if ledgerExpenses[i].Date == ledgerExpenses[j].Date {
					return ledgerExpenses[i].Description < ledgerExpenses[j].Description
				}
				return ledgerExpenses[i].Date < ledgerExpenses[j].Date
			})

			type balanceRow struct {
				UserID       int     `json:"user_id"`
				Name         string  `json:"name"`
				CurrencyCode string  `json:"currency_code"`
				Net          float64 `json:"net"`
			}
			runningBalances := make([]balanceRow, 0)
			for k, v := range running {
				runningBalances = append(runningBalances, balanceRow{
					UserID:       k.UserID,
					Name:         memberNames[k.UserID],
					CurrencyCode: k.CurrencyCode,
					Net:          round2(v),
				})
			}
			sort.Slice(runningBalances, func(i, j int) bool {
				if runningBalances[i].CurrencyCode == runningBalances[j].CurrencyCode {
					return runningBalances[i].Name < runningBalances[j].Name
				}
				return runningBalances[i].CurrencyCode < runningBalances[j].CurrencyCode
			})

			out := struct {
				GroupID         int             `json:"group_id"`
				GroupName       string          `json:"group_name"`
				Expenses        []ledgerExpense `json:"expenses"`
				RunningBalances []balanceRow    `json:"running_balances"`
			}{
				GroupID:         groupID,
				GroupName:       groupName,
				Expenses:        ledgerExpenses,
				RunningBalances: runningBalances,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.emitStructured(cmd, out)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Group: %s (%d)\n\n", groupName, groupID)
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "DATE\tDESCRIPTION\tCOST\tCURRENCY\tPAYMENT")
			for _, e := range ledgerExpenses {
				_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n", e.Date, e.Description, e.Cost, e.CurrencyCode, e.Payment)
			}
			_ = tw.Flush()

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			tw2 := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw2, "USER\tCURRENCY\tNET")
			for _, b := range runningBalances {
				_, _ = fmt.Fprintf(tw2, "%s\t%s\t%.2f\n", b.Name, b.CurrencyCode, b.Net)
			}
			return tw2.Flush()
		},
	}
	return cmd
}

func resolveLedgerGroup(input string, groups []Group) (int, string, map[int]string, error) {
	memberNames := make(map[int]string)
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return 0, "", memberNames, errors.New("group name or id is required")
	}

	if isAllDigits(trimmed) {
		id, _ := strconv.Atoi(trimmed)
		for _, g := range groups {
			if g.ID == id {
				for _, m := range g.Members {
					memberNames[m.ID] = strings.TrimSpace(strings.TrimSpace(m.FirstName) + " " + strings.TrimSpace(m.LastName))
				}
				return g.ID, g.Name, memberNames, nil
			}
		}
		return 0, "", memberNames, fmt.Errorf("group %q not found; run sync or use a numeric id", input)
	}

	matches := matchGroupsByName(input, groups)
	switch len(matches) {
	case 0:
		return 0, "", memberNames, fmt.Errorf("group %q not found; run sync or use a numeric id", input)
	case 1:
		g := matches[0]
		for _, m := range g.Members {
			memberNames[m.ID] = strings.TrimSpace(strings.TrimSpace(m.FirstName) + " " + strings.TrimSpace(m.LastName))
		}
		return g.ID, g.Name, memberNames, nil
	default:
		return 0, "", memberNames, ambiguousGroupErr(input, matches)
	}
}

func spendBucket(groupBy string, e Expense, groupNames map[int]string) string {
	switch groupBy {
	case "group":
		if name := strings.TrimSpace(groupNames[e.GroupID]); name != "" {
			return name
		}
		if e.GroupID == 0 {
			return "Non-group"
		}
		return fmt.Sprintf("Group %d", e.GroupID)
	case "month":
		d := strings.TrimSpace(e.Date)
		if len(d) >= 7 {
			return d[:7]
		}
		if d == "" {
			return "Unknown"
		}
		return d
	default:
		name := strings.TrimSpace(e.Category.Name)
		if name == "" {
			return "Uncategorized"
		}
		return name
	}
}

func expenseDeleted(deletedAt *string) bool {
	if deletedAt == nil {
		return false
	}
	v := strings.TrimSpace(*deletedAt)
	if v == "" {
		return false
	}
	return !strings.EqualFold(v, "null")
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
