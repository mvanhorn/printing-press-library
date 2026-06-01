package cli

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/mvanhorn/printing-press-library/library/payments/splitwise/internal/cliutil"
	"github.com/spf13/cobra"
)

// pp:data-source live
func newSplitCmd(flags *rootFlags) *cobra.Command {
	amount := 0.0
	description := "Shared expense"
	currency := "USD"
	equal := true
	exact := ""
	percent := ""
	shares := ""
	paidBy := 0
	record := false

	cmd := &cobra.Command{
		Use:   "split <group>",
		Short: "Build and preview an expense split, then optionally record it",
		Example: "  splitwise-pp-cli split \"Tahoe Trip\" --amount 84 --equal\n" +
			"  splitwise-pp-cli split \"Tahoe Trip\" --amount 90 --exact 12345:30,67890:60 --record",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute split plan")
				return nil
			}
			if len(args) == 0 {
				return usageErr(errors.New("group name or id is required"))
			}
			if amount <= 0 {
				return usageErr(errors.New("--amount must be > 0"))
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			groups, err := loadGroups(db)
			if err != nil {
				return err
			}
			// Rejoin a multi-word group name the MCP command-mirror split into
			// several positionals (see joinNameArgs in splitwise_settle.go).
			groupName := joinNameArgs(args)
			group, ok, ambErr := resolveSettleGroup(groupName, groups)
			if ambErr != nil {
				return usageErr(ambErr)
			}
			if !ok {
				return usageErr(fmt.Errorf("no group matches %q; run sync first", groupName))
			}

			payer := paidBy
			if payer == 0 {
				payer = loadCurrentUserID(db)
			}
			if payer == 0 {
				return usageErr(errors.New("--paid-by is required when current user is not synced"))
			}

			members := group.Members
			if len(members) == 0 {
				return usageErr(fmt.Errorf("group %q has no members", strings.TrimSpace(group.Name)))
			}
			memberSet := make(map[int]struct{})
			for _, m := range members {
				memberSet[m.ID] = struct{}{}
			}
			if _, ok := memberSet[payer]; !ok {
				return usageErr(fmt.Errorf("paid-by user %d is not a member of %q", payer, strings.TrimSpace(group.Name)))
			}

			mode, err := resolveSplitMode(cmd, equal, exact, percent, shares)
			if err != nil {
				return usageErr(err)
			}
			totalCents := dollarsToCents(amount)
			owedByUser := make(map[int]int64)

			sortedMembers := make([]GroupMember, 0, len(members))
			sortedMembers = append(sortedMembers, members...)
			sort.Slice(sortedMembers, func(i, j int) bool { return sortedMembers[i].ID < sortedMembers[j].ID })

			switch mode {
			case "equal":
				if err := allocateEqual(totalCents, sortedMembers, owedByUser); err != nil {
					return usageErr(err)
				}
			case "exact":
				if err := allocateExact(totalCents, exact, sortedMembers, memberSet, owedByUser); err != nil {
					return usageErr(err)
				}
			case "percent":
				if err := allocateWeighted(totalCents, percent, sortedMembers, memberSet, owedByUser, 100.0); err != nil {
					return usageErr(err)
				}
			case "shares":
				if err := allocateWeighted(totalCents, shares, sortedMembers, memberSet, owedByUser, 0); err != nil {
					return usageErr(err)
				}
			}

			users := make([]map[string]any, 0, len(sortedMembers))
			type shareRow struct {
				UserID    int    `json:"user_id"`
				Name      string `json:"name"`
				PaidShare string `json:"paid_share"`
				OwedShare string `json:"owed_share"`
			}
			rows := make([]shareRow, 0, len(sortedMembers))

			for _, m := range sortedMembers {
				owed := centsToMoneyString(owedByUser[m.ID])
				paid := "0.00"
				if m.ID == payer {
					paid = fmt.Sprintf("%.2f", amount)
				}
				users = append(users, map[string]any{"user_id": m.ID, "paid_share": paid, "owed_share": owed})
				name := strings.TrimSpace(strings.TrimSpace(m.FirstName) + " " + strings.TrimSpace(m.LastName))
				if name == "" {
					name = fmt.Sprintf("User %d", m.ID)
				}
				rows = append(rows, shareRow{UserID: m.ID, Name: name, PaidShare: paid, OwedShare: owed})
			}

			body := map[string]any{
				"cost":          fmt.Sprintf("%.2f", amount),
				"currency_code": strings.TrimSpace(currency),
				"description":   description,
				"group_id":      group.ID,
				"users":         users,
			}
			view := struct {
				Group       string     `json:"group"`
				GroupID     int        `json:"group_id"`
				Amount      float64    `json:"amount"`
				Currency    string     `json:"currency_code"`
				Mode        string     `json:"mode"`
				PaidBy      int        `json:"paid_by"`
				Shares      []shareRow `json:"shares"`
				ExpenseBody any        `json:"expense_body"`
			}{
				Group:       strings.TrimSpace(group.Name),
				GroupID:     group.ID,
				Amount:      amount,
				Currency:    strings.TrimSpace(currency),
				Mode:        mode,
				PaidBy:      payer,
				Shares:      rows,
				ExpenseBody: body,
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				if err := flags.printJSON(cmd, view); err != nil {
					return err
				}
			} else {
				tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
				_, _ = fmt.Fprintln(tw, "NAME\tPAID\tOWED")
				for _, row := range rows {
					_, _ = fmt.Fprintf(tw, "%s\t%s\t%s\n", row.Name, row.PaidShare, row.OwedShare)
				}
				if err := tw.Flush(); err != nil {
					return err
				}
			}

			if !record {
				_, _ = fmt.Fprintln(cmd.ErrOrStderr(), "preview only — re-run with --record to create the expense")
				return nil
			}

			if cliutil.IsVerifyEnv() {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would create expense (verify mode)")
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			respData, status, err := c.Post(cmd.Context(), "/create_expense", body)
			if err != nil {
				return err
			}
			if status < 200 || status >= 300 {
				return fmt.Errorf("create_expense failed: status %d", status)
			}
			// Splitwise returns HTTP 200 with a non-empty "errors" body when the
			// create is rejected, so the status check alone is not sufficient.
			if envErr := splitwiseMutationError(respData); envErr != nil {
				return fmt.Errorf("create_expense rejected: %w", envErr)
			}

			summary := map[string]any{"status_code": status, "created": true}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, summary)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "created expense (status %d)\n", status)
			return nil
		},
	}

	cmd.Flags().Float64Var(&amount, "amount", 0, "Total expense amount")
	cmd.Flags().StringVar(&description, "description", "Shared expense", "Expense description")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Expense currency code")
	cmd.Flags().BoolVar(&equal, "equal", true, "Split equally across all members")
	cmd.Flags().StringVar(&exact, "exact", "", "Exact amounts CSV: user_id:amount,user_id:amount")
	cmd.Flags().StringVar(&percent, "percent", "", "Percent CSV: user_id:pct,user_id:pct")
	cmd.Flags().StringVar(&shares, "shares", "", "Relative shares CSV: user_id:weight,user_id:weight")
	cmd.Flags().IntVar(&paidBy, "paid-by", 0, "User id who paid (default: current user)")
	cmd.Flags().BoolVar(&record, "record", false, "Create the expense after preview")
	return cmd
}

func resolveSplitMode(cmd *cobra.Command, equal bool, exact string, percent string, shares string) (string, error) {
	hasExact := strings.TrimSpace(exact) != ""
	hasPercent := strings.TrimSpace(percent) != ""
	hasShares := strings.TrimSpace(shares) != ""
	modes := 0
	if hasExact {
		modes++
	}
	if hasPercent {
		modes++
	}
	if hasShares {
		modes++
	}
	if modes > 1 {
		return "", errors.New("choose exactly one of --exact, --percent, or --shares")
	}
	if modes == 0 {
		if cmd.Flags().Changed("equal") && !equal {
			return "", errors.New("no split mode selected; use --equal or one of --exact/--percent/--shares")
		}
		return "equal", nil
	}
	if cmd.Flags().Changed("equal") && equal {
		return "", errors.New("--equal cannot be combined with --exact/--percent/--shares")
	}
	if hasExact {
		return "exact", nil
	}
	if hasPercent {
		return "percent", nil
	}
	return "shares", nil
}

func allocateEqual(totalCents int64, members []GroupMember, out map[int]int64) error {
	if len(members) == 0 {
		return errors.New("cannot split with zero members")
	}
	base := totalCents / int64(len(members))
	rem := totalCents % int64(len(members))
	for i, m := range members {
		v := base
		if int64(i) < rem {
			v++
		}
		out[m.ID] = v
	}
	return nil
}

func allocateExact(totalCents int64, input string, members []GroupMember, memberSet map[int]struct{}, out map[int]int64) error {
	for _, m := range members {
		out[m.ID] = 0
	}
	pairs, err := parsePairs(input)
	if err != nil {
		return err
	}
	sum := int64(0)
	for uid, val := range pairs {
		if _, ok := memberSet[uid]; !ok {
			return fmt.Errorf("user %d is not in group", uid)
		}
		cents := dollarsToCents(val)
		if cents < 0 {
			return fmt.Errorf("amount for user %d must be >= 0", uid)
		}
		out[uid] = cents
		sum += cents
	}
	if sum != totalCents {
		return fmt.Errorf("exact amounts must sum to %.2f", float64(totalCents)/100)
	}
	return nil
}

func allocateWeighted(totalCents int64, input string, members []GroupMember, memberSet map[int]struct{}, out map[int]int64, mustSum float64) error {
	for _, m := range members {
		out[m.ID] = 0
	}
	pairs, err := parsePairs(input)
	if err != nil {
		return err
	}
	totalWeight := 0.0
	for uid, w := range pairs {
		if _, ok := memberSet[uid]; !ok {
			return fmt.Errorf("user %d is not in group", uid)
		}
		if w < 0 {
			return fmt.Errorf("weight for user %d must be >= 0", uid)
		}
		totalWeight += w
	}
	if mustSum > 0 {
		if math.Abs(totalWeight-mustSum) > 0.000001 {
			return fmt.Errorf("percent values must sum to 100")
		}
	}
	if totalWeight <= 0 {
		return errors.New("weights must sum to > 0")
	}

	type remRow struct {
		id   int
		frac float64
	}
	remaining := totalCents
	rems := make([]remRow, 0, len(pairs))
	for uid, w := range pairs {
		raw := float64(totalCents) * (w / totalWeight)
		base := int64(math.Floor(raw))
		out[uid] = base
		remaining -= base
		rems = append(rems, remRow{id: uid, frac: raw - float64(base)})
	}
	sort.Slice(rems, func(i, j int) bool {
		if rems[i].frac == rems[j].frac {
			return rems[i].id < rems[j].id
		}
		return rems[i].frac > rems[j].frac
	})
	for i := int64(0); i < remaining; i++ {
		out[rems[i%int64(len(rems))].id]++
	}
	return nil
}

func parsePairs(input string) (map[int]float64, error) {
	out := make(map[int]float64)
	parts := strings.Split(strings.TrimSpace(input), ",")
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, ":", 2)
		if len(kv) != 2 {
			return nil, fmt.Errorf("invalid pair %q: expected user_id:value", p)
		}
		uid, err := strconv.Atoi(strings.TrimSpace(kv[0]))
		if err != nil || uid <= 0 {
			return nil, fmt.Errorf("invalid user_id in %q", p)
		}
		val, err := strconv.ParseFloat(strings.TrimSpace(kv[1]), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid numeric value in %q", p)
		}
		out[uid] = val
	}
	if len(out) == 0 {
		return nil, errors.New("no user:value pairs provided")
	}
	return out, nil
}

func dollarsToCents(amount float64) int64 {
	return int64(math.Round(amount * 100))
}

func centsToMoneyString(cents int64) string {
	return fmt.Sprintf("%.2f", float64(cents)/100)
}
