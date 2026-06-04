// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

type sellerScanResult struct {
	SellerID       string                  `json:"sellerId"`
	Seller         *offerup.Seller         `json:"seller"`
	InventoryCount int                     `json:"inventoryCount"`
	MedianAsking   float64                 `json:"medianAsking"`
	Inventory      []offerup.StoredListing `json:"inventory"`
}

// pp:data-source local
func newNovelSellerScanCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "seller-scan <seller-id>",
		Short: "Pull a seller's locally-known inventory alongside their reputation badges (business/dealer/TruYou), join date",
		Long: strings.Trim(`
Profile a seller from the local store. Returns the seller's reputation
(business/dealer/TruYou badges, join date) plus every listing of theirs you've
fetched with 'listings get', and the median asking price across that inventory.
OfferUp has no public endpoint that lists a seller's full catalog, so the
inventory grows as you fetch more of their items.`, "\n"),
		Example: "  offerup-pp-cli seller-scan 161842229 --agent",
		// An unknown seller id reads the local store and legitimately returns an
		// empty inventory (exit 0) — indistinguishable from a valid seller not yet
		// synced — so there is no error-path to probe.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sellerID := args[0]
			result := sellerScanResult{SellerID: sellerID, Inventory: []offerup.StoredListing{}}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			st, err := openOfferupStore()
			if err != nil {
				return configErr(err)
			}
			defer st.Close()
			seller, err := st.Seller(sellerID)
			if err != nil {
				return apiErr(err)
			}
			result.Seller = seller
			inv, err := st.SellerInventory(sellerID)
			if err != nil {
				return apiErr(err)
			}
			if inv != nil {
				result.Inventory = inv
			}
			result.InventoryCount = len(result.Inventory)
			result.MedianAsking = offerup.Median(result.Inventory)
			if result.InventoryCount == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"No locally-known listings for seller %s. Run 'offerup-pp-cli listings get <listing-id>' on the seller's items first to populate their inventory.\n",
					sellerID)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}
