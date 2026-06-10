// Copyright 2026 richardadonnell. Licensed under Apache-2.0. See LICENSE.
// Hand-written: rich goquery `get` command (full detail + price research).

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/motohunt/internal/motohunt"

	"github.com/spf13/cobra"
)

func newGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch a listing's full detail including MotoHunt price research (base MSRP, ALP, deal rating)",
		Long: `Fetch the /l/{id} detail page and parse it into one struct: title, condition,
dealer, location, price, mileage, color, age, stock #, VIN, certified-pre-owned,
description, images, and the MotoHunt Price Research block (base_msrp, alp,
deal_rating). Honors --site (moto|atv).`,
		Example:     "  motohunt-pp-cli get 13256655 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			site, err := siteConfigFor(flags)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				id := "<id>"
				if len(args) > 0 {
					id = args[0]
				}
				fmt.Fprintf(cmd.OutOrStdout(), "would GET %s\n", site.DetailURL(id))
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("listing id required: motohunt-pp-cli get <id>"))
			}
			id := args[0]
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would get (verify env)")
				return printJSONFiltered(cmd.OutOrStdout(), motohunt.ListingDetail{ID: id}, flags)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			client := scrapeClient(flags)

			doc, ferr := client.Fetch(ctx, site.DetailURL(id))
			if ferr != nil {
				return apiErr(ferr)
			}
			detail := motohunt.ParseDetail(doc, site, id)
			if detail.Title == "" {
				// MotoHunt serves HTTP 200 for unknown ids; an empty title means
				// the id does not resolve to a real listing.
				return notFoundErr(fmt.Errorf("listing %q not found", id))
			}
			return printDomainJSON(cmd.OutOrStdout(), detail, flags)
		},
	}
	return cmd
}
