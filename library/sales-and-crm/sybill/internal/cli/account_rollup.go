// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: account roll-up (offline join across entities).

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

type rollupDeal struct {
	DealID string  `json:"dealId"`
	Name   string  `json:"name,omitempty"`
	Stage  string  `json:"stage,omitempty"`
	Amount float64 `json:"amount,omitempty"`
	Closed bool    `json:"closed"`
}

type rollupContact struct {
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
	Title string `json:"title,omitempty"`
}

type accountRollup struct {
	AccountID        string          `json:"accountId,omitempty"`
	Name             string          `json:"name"`
	Website          string          `json:"website,omitempty"`
	Owner            string          `json:"owner,omitempty"`
	LastActivityDate string          `json:"lastActivityDate,omitempty"`
	CallCount        int             `json:"callCount"`
	LastCall         string          `json:"lastCall,omitempty"`
	OpenDealCount    int             `json:"openDealCount"`
	TotalOpenAmount  float64         `json:"totalOpenAmount,omitempty"`
	DealsByStage     map[string]int  `json:"dealsByStage,omitempty"`
	Deals            []rollupDeal    `json:"deals"`
	Contacts         []rollupContact `json:"contacts,omitempty"`
}

func newNovelAccountRollupCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "rollup <account>",
		Short: "One offline view per account: call count, open deals by stage, contacts, and last activity.",
		Long: `Join everything the local store knows about one account — its deals (grouped
by stage), the calls linked to it, and its contacts — into a single view. The
account can be named by its id or its name. Run 'sync' first.`,
		Example: strings.Trim(`
  # Roll up an account by name
  sybill-pp-cli account rollup "Acme Corp"

  # By id, as JSON
  sybill-pp-cli account rollup acc_12345 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			target := args[0]
			out := cmd.OutOrStdout()
			if dbPath == "" {
				dbPath = defaultDBPath("sybill-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sybill-pp-cli sync' first.", err)
			}
			defer db.Close()

			accounts, err := loadRecords(db, "accounts")
			if err != nil {
				return err
			}
			deals, err := loadRecords(db, "deals")
			if err != nil {
				return err
			}
			convs, err := loadRecords(db, "conversations")
			if err != nil {
				return err
			}

			// Resolve the account record (by id or name), if it was synced.
			var acct map[string]any
			for _, a := range accounts {
				if firstStr(a, "accountId", "account_id", "id") == target || strings.EqualFold(firstStr(a, "name"), target) {
					acct = a
					break
				}
			}

			accountName := target
			if acct != nil {
				accountName = firstStr(acct, "name")
			}

			// Deals belonging to this account (match by accountName, or by id if
			// the account wasn't synced and the user passed a name).
			var acctDeals []map[string]any
			for _, d := range deals {
				if accountName != "" && strings.EqualFold(dealAccount(d), accountName) {
					acctDeals = append(acctDeals, d)
				}
			}
			if accountName == "" && acct == nil {
				return fmt.Errorf("no account, deal, or activity found for %q in the local store; run 'sybill-pp-cli sync' first", target)
			}

			roll := accountRollup{
				Name:         accountName,
				DealsByStage: map[string]int{},
			}
			if acct != nil {
				roll.AccountID = firstStr(acct, "accountId", "account_id", "id")
				roll.Website = firstStr(acct, "website")
				if o := nestedObj(acct, "owner"); o != nil {
					roll.Owner = firstStr(o, "name", "email")
				} else {
					roll.Owner = firstStr(acct, "owner")
				}
				roll.LastActivityDate = firstStr(acct, "lastActivityDate", "last_activity_date")
			}

			seenContacts := map[string]bool{}
			addContacts := func(obj map[string]any) {
				arr, ok := obj["contacts"].([]any)
				if !ok {
					return
				}
				for _, item := range arr {
					m, ok := item.(map[string]any)
					if !ok {
						continue
					}
					c := rollupContact{
						Name:  firstStr(m, "name"),
						Email: firstStr(m, "email"),
						Title: firstStr(m, "title"),
					}
					key := strings.ToLower(c.Email + "|" + c.Name)
					if key == "|" || seenContacts[key] {
						continue
					}
					seenContacts[key] = true
					roll.Contacts = append(roll.Contacts, c)
				}
			}
			if acct != nil {
				addContacts(acct)
			}

			for _, d := range acctDeals {
				rd := rollupDeal{DealID: dealID(d), Name: dealName(d), Stage: dealStage(d), Closed: dealClosed(d)}
				if amt, ok := floatField(d, "amount"); ok {
					rd.Amount = amt
				}
				roll.Deals = append(roll.Deals, rd)
				if !rd.Closed {
					roll.OpenDealCount++
					roll.TotalOpenAmount += rd.Amount
					stage := rd.Stage
					if stage == "" {
						stage = "(unset)"
					}
					roll.DealsByStage[stage]++
				}
				addContacts(d)
			}

			// Calls linked to the account (directly or via its deals).
			var latestCall time.Time
			for _, c := range convs {
				if !convMatchesAccount(c, accountName, acctDeals) {
					continue
				}
				roll.CallCount++
				if t, ok := convStart(c); ok && t.After(latestCall) {
					latestCall = t
				}
			}
			if !latestCall.IsZero() {
				roll.LastCall = latestCall.Format(time.RFC3339)
			}

			sort.SliceStable(roll.Deals, func(i, j int) bool { return roll.Deals[i].Amount > roll.Deals[j].Amount })

			if novelMachineOutput(out, flags) {
				return printJSONFiltered(out, roll, flags)
			}

			fmt.Fprintf(out, "Account: %s\n", roll.Name)
			if roll.Website != "" {
				fmt.Fprintf(out, "Website: %s\n", roll.Website)
			}
			if roll.Owner != "" {
				fmt.Fprintf(out, "Owner:   %s\n", roll.Owner)
			}
			fmt.Fprintf(out, "Calls:   %d", roll.CallCount)
			if roll.LastCall != "" {
				if t, ok := parseTime(roll.LastCall); ok {
					fmt.Fprintf(out, "  (last: %s)", t.Format("2006-01-02"))
				}
			}
			fmt.Fprintln(out)
			fmt.Fprintf(out, "Open deals: %d", roll.OpenDealCount)
			if roll.TotalOpenAmount > 0 {
				fmt.Fprintf(out, "  (open value: %.0f)", roll.TotalOpenAmount)
			}
			fmt.Fprintln(out)
			if len(roll.DealsByStage) > 0 {
				stages := make([]string, 0, len(roll.DealsByStage))
				for s := range roll.DealsByStage {
					stages = append(stages, s)
				}
				sort.Strings(stages)
				for _, s := range stages {
					fmt.Fprintf(out, "  %-22s %d\n", s, roll.DealsByStage[s])
				}
			}
			if len(roll.Contacts) > 0 {
				fmt.Fprintf(out, "Contacts: %d\n", len(roll.Contacts))
				for _, c := range roll.Contacts {
					line := c.Name
					if c.Title != "" {
						line += " (" + c.Title + ")"
					}
					if c.Email != "" {
						line += " <" + c.Email + ">"
					}
					fmt.Fprintf(out, "  - %s\n", strings.TrimSpace(line))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}
