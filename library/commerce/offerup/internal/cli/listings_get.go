// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/offerup/internal/offerup"
)

func newListingsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <listing-id>",
		Short: "Get the full detail for one listing (description, photos, seller)",
		Long: strings.Trim(`
Fetch one listing's full detail by id: title, description, price, condition,
photos, location, and the seller's profile and reputation badges. The listing
and seller are cached to the local store so seller-scan can build the seller's
inventory.`, "\n"),
		Example:     "  offerup-pp-cli listings get 967e9f5f-4153-3b8f-ac54-70e8433675b6",
		Annotations: map[string]string{"pp:endpoint": "listings.get", "pp:method": "GET", "pp:path": "/item/detail/{listing_id}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), &offerup.ListingDetail{}, flags)
			}
			client := newOfferupClient(flags)
			detail, err := client.GetItem(cmd.Context(), args[0])
			if err != nil {
				return classifyOfferupError(err)
			}
			if st, err := openOfferupStore(); err == nil {
				defer st.Close()
				_ = st.RecordDetail(detail)
				if detail.Seller != nil {
					_ = st.RecordSeller(detail.Seller)
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), detail, flags)
		},
	}
	return cmd
}
