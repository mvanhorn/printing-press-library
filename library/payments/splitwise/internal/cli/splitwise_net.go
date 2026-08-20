package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type netCurrencySummary struct {
	CurrencyCode string  `json:"currency_code"`
	OwedToYou    float64 `json:"owed_to_you"`
	YouOwe       float64 `json:"you_owe"`
	Net          float64 `json:"net"`
}

type netTransfer struct {
	FriendID     int     `json:"friend_id"`
	FriendName   string  `json:"friend_name"`
	Direction    string  `json:"direction"`
	Amount       float64 `json:"amount"`
	CurrencyCode string  `json:"currency_code"`
}

type netSavings struct {
	CurrencyCode      string `json:"currency_code"`
	PerGroupTransfers int    `json:"per_group_transfers"`
	NettedTransfers   int    `json:"netted_transfers"`
	Saved             int    `json:"saved"`
}

type netResult struct {
	YouID      int                  `json:"you_id"`
	ByCurrency []netCurrencySummary `json:"by_currency"`
	Plan       []netTransfer        `json:"plan"`
	Savings    []netSavings         `json:"savings"`
}

func computeNetPlan(friends []Friend, youID int) netResult {
	agg := make(map[string]*netCurrencySummary)
	perGroupCounts := make(map[string]int)
	nettedCounts := make(map[string]int)
	plan := make([]netTransfer, 0)

	for _, f := range friends {
		name := strings.TrimSpace(friendDisplayName(f))
		if name == "" {
			name = fmt.Sprintf("friend %d", f.ID)
		}

		friendTotals := make(map[string]float64)
		for _, b := range f.Balance {
			amt := parseAmount(b.Amount)
			if amt == 0 {
				continue
			}
			cc := strings.TrimSpace(b.CurrencyCode)
			friendTotals[cc] += amt
		}

		for cc, total := range friendTotals {
			total = round2(total)
			if total == 0 {
				continue
			}
			if _, ok := agg[cc]; !ok {
				agg[cc] = &netCurrencySummary{CurrencyCode: cc}
			}
			if total > 0 {
				agg[cc].OwedToYou += total
			} else if total < 0 {
				agg[cc].YouOwe += -total
			}
			agg[cc].Net += total
			nettedCounts[cc]++
			direction := "you_pay"
			if total > 0 {
				direction = "they_pay"
			}
			plan = append(plan, netTransfer{
				FriendID:     f.ID,
				FriendName:   name,
				Direction:    direction,
				Amount:       round2(math.Abs(total)),
				CurrencyCode: cc,
			})
		}

		groupSums := make(map[string]float64)
		groupTxns := make(map[string]int)
		for _, g := range f.Groups {
			for _, b := range g.Balance {
				cc := strings.TrimSpace(b.CurrencyCode)
				amt := parseAmount(b.Amount)
				if amt == 0 {
					continue
				}
				groupSums[cc] += amt
				groupTxns[cc]++
			}
		}
		for cc, total := range friendTotals {
			total = round2(total)
			if total == 0 {
				continue
			}
			txns := groupTxns[cc]
			// per_group_transfers counts distinct settlement legs (group + non-group).
			remainder := round2(total - groupSums[cc])
			if remainder != 0 {
				txns++
			}
			perGroupCounts[cc] += txns
		}
	}

	byCurrency := make([]netCurrencySummary, 0, len(agg))
	for _, v := range agg {
		v.OwedToYou = round2(v.OwedToYou)
		v.YouOwe = round2(v.YouOwe)
		v.Net = round2(v.Net)
		if v.OwedToYou == 0 && v.YouOwe == 0 {
			continue
		}
		byCurrency = append(byCurrency, *v)
	}
	sort.Slice(byCurrency, func(i, j int) bool {
		return byCurrency[i].CurrencyCode < byCurrency[j].CurrencyCode
	})

	sort.Slice(plan, func(i, j int) bool {
		if plan[i].CurrencyCode != plan[j].CurrencyCode {
			return plan[i].CurrencyCode < plan[j].CurrencyCode
		}
		ai := math.Abs(plan[i].Amount)
		aj := math.Abs(plan[j].Amount)
		if ai != aj {
			return ai > aj
		}
		return plan[i].FriendName < plan[j].FriendName
	})

	currencySeen := make(map[string]struct{})
	for cc := range agg {
		currencySeen[cc] = struct{}{}
	}
	for cc := range perGroupCounts {
		currencySeen[cc] = struct{}{}
	}

	savings := make([]netSavings, 0, len(currencySeen))
	for cc := range currencySeen {
		perGroup := perGroupCounts[cc]
		netted := nettedCounts[cc]
		saved := perGroup - netted
		if saved < 0 {
			saved = 0
		}
		savings = append(savings, netSavings{
			CurrencyCode:      cc,
			PerGroupTransfers: perGroup,
			NettedTransfers:   netted,
			Saved:             saved,
		})
	}
	sort.Slice(savings, func(i, j int) bool {
		return savings[i].CurrencyCode < savings[j].CurrencyCode
	})

	return netResult{
		YouID:      youID,
		ByCurrency: byCurrency,
		Plan:       plan,
		Savings:    savings,
	}
}

func newNetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "net",
		Short:       "Net balances across all groups into the fewest direct transfers",
		Example:     "  splitwise-pp-cli net --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "would compute net settlement plan")
				return nil
			}

			db, err := openSplitwiseStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			hintIfUnsynced(cmd, db, "get-friends")
			hintIfStale(cmd, db, "get-friends", flags.maxAge)

			friends, err := loadFriends(db)
			if err != nil {
				return err
			}

			youID := loadCurrentUserID(db)
			res := computeNetPlan(friends, youID)

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, res)
			}

			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "CURRENCY\tOWED TO YOU\tYOU OWE\tNET")
			for _, row := range res.ByCurrency {
				_, _ = fmt.Fprintf(tw, "%s\t%.2f\t%.2f\t%.2f\n", row.CurrencyCode, row.OwedToYou, row.YouOwe, row.Net)
			}
			if err := tw.Flush(); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			tw2 := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw2, "FRIEND\tDIRECTION\tAMOUNT\tCURRENCY")
			for _, row := range res.Plan {
				_, _ = fmt.Fprintf(tw2, "%s\t%s\t%.2f\t%s\n", row.FriendName, row.Direction, row.Amount, row.CurrencyCode)
			}
			if err := tw2.Flush(); err != nil {
				return err
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout())
			out := cmd.OutOrStdout()
			for _, s := range res.Savings {
				_, _ = fmt.Fprintf(out, "%s: settles your account in %d transfers vs %d per-group (saves %d)\n", s.CurrencyCode, s.NettedTransfers, s.PerGroupTransfers, s.Saved)
			}
			return nil
		},
	}
	return cmd
}
