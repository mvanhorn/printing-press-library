// Copyright 2026 Ryan Gravette and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: join purchases against users into a readable ledger.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

type purchaseObject struct {
	Amount int    `json:"amount"`
	Object string `json:"object"`
}

type purchaseRecord struct {
	ID             string           `json:"id"`
	Charge         string           `json:"charge"`
	Currency       string           `json:"currency"`
	State          string           `json:"state"`
	Status         string           `json:"status"`
	PurchasingUser string           `json:"purchasingUser"`
	User           []string         `json:"user"`
	Objects        []purchaseObject `json:"objects"`
	Created        string           `json:"created"`
}

type reconciledPurchase struct {
	PurchaseID   string           `json:"purchase_id"`
	Charge       string           `json:"charge"`
	Currency     string           `json:"currency"`
	State        string           `json:"state"`
	Buyer        string           `json:"buyer"`
	BuyerEmail   string           `json:"buyer_email"`
	BuyerMatched bool             `json:"buyer_matched"`
	Objects      []purchaseObject `json:"objects"`
	Created      string           `json:"created"`
}

// pp:data-source computed
func newNovelPurchaseReconcileCmd(flags *rootFlags) *cobra.Command {
	var flagLimit int
	cmd := &cobra.Command{
		Use:         "reconcile",
		Short:       "Join purchases against user records to show who paid for what.",
		Long:        "Join purchases against user records to show who paid for what.\n\nUse to reconcile Stripe-backed purchases against user records.",
		Example:     "  agilix-dawn-pp-cli purchase reconcile --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch purchases and users, then reconcile")
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			_, purchaseMatches, err := fetchSearch(ctx, c, "purchase", fmt.Sprintf(`{"limit":%d}`, flagLimit))
			if err != nil {
				return classifyAPIError(err, flags)
			}
			_, userMatches, err := fetchSearch(ctx, c, "user", `{"limit":1000}`)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			userByID := make(map[string]rosterUser, len(userMatches))
			for _, m := range userMatches {
				var u rosterUser
				if json.Unmarshal(m, &u) == nil && u.ID != "" {
					userByID[u.ID] = u
				}
			}
			out := make([]reconciledPurchase, 0, len(purchaseMatches))
			for _, m := range purchaseMatches {
				var p purchaseRecord
				if json.Unmarshal(m, &p) != nil {
					continue
				}
				buyerID := p.PurchasingUser
				if buyerID == "" && len(p.User) > 0 {
					buyerID = p.User[0]
				}
				row := reconciledPurchase{
					PurchaseID: p.ID, Charge: p.Charge, Currency: p.Currency,
					State: p.State, Objects: p.Objects, Created: p.Created,
				}
				if u, ok := userByID[buyerID]; ok {
					row.Buyer = u.GivenName + " " + u.FamilyName
					row.BuyerEmail = u.Email
					row.BuyerMatched = true
				} else {
					row.Buyer = buyerID
				}
				out = append(out, row)
			}
			if flags.asJSON {
				return flags.printJSON(cmd, out)
			}
			w := cmd.OutOrStdout()
			if len(out) == 0 {
				fmt.Fprintln(w, "no purchases found")
				return nil
			}
			for _, r := range out {
				items := ""
				for i, o := range r.Objects {
					if i > 0 {
						items += ", "
					}
					items += fmt.Sprintf("%s (%d)", o.Object, o.Amount)
				}
				fmt.Fprintf(w, "%s\t%s\t%s %s\t%s\t%s\n", r.PurchaseID, r.State, r.Currency, r.Charge, r.Buyer, items)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagLimit, "limit", 500, "Maximum purchases to reconcile")
	return cmd
}
