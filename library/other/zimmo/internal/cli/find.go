// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/zimmo/internal/zimmo"
)

type findView struct {
	Total    int             `json:"total_matching"`
	Returned int             `json:"returned"`
	Pages    int             `json:"pages_fetched"`
	Stored   bool            `json:"stored"`
	Criteria zimmo.Criteria  `json:"criteria"`
	Listings []zimmo.Listing `json:"listings"`
	Note     string          `json:"note,omitempty"`
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var cf critFlags
	var limit, pages int
	var noStore bool
	var dbPath string
	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search Zimmo listings by commune, type, price, bedrooms, surface, EPC and text, and store them",
		Long: `Search Zimmo's listings (for sale by default; rent, sold, rented and take-over
with --status) and store every result in the local SQLite store, which
powers watch, drops, peb-trap, underpriced, yield, motivated and same-as.
Communes accept French or Dutch names (ixelles, elsene) or postcodes; paste
any zimmo.be result page or advanced-search URL with --url.`,
		Example: strings.Trim(`
  zimmo-pp-cli find --postcode 1050 --type apartment --max-price 400000
  zimmo-pp-cli find --commune schaerbeek --type house --epc F,G --agent
  zimmo-pp-cli find --commune uccle --status sold --limit 50 --json
  zimmo-pp-cli find --url "https://www.zimmo.be/fr/bruxelles-1000/a-vendre/appartement"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "live", "pp:happy-args": "--postcode=1050;--type=apartment;--limit=5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search Zimmo listings and store them")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) > 0 {
				return usageErr(fmt.Errorf("find takes flags, not arguments (got %q); use --commune or --text", args[0]))
			}
			if limit <= 0 {
				return usageErr(fmt.Errorf("--limit must be positive"))
			}
			crit, err := cf.build()
			if err != nil {
				return err
			}
			flags.agentSource = "live"
			ctx, cancel := boundCtx(cmd.Context(), batchFlags(flags))
			defer cancel()
			zc := zimmoClient(flags)
			db, err := openZimmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			crit, err = resolveCriteria(ctx, zc, db, crit)
			if err != nil {
				return err
			}
			res, err := walkSearch(ctx, zc, crit, harnessPages(pages), limit)
			if err != nil {
				return err
			}
			view := findView{Total: res.Total, Returned: len(res.Listings), Pages: res.Pages, Criteria: crit, Listings: res.Listings}
			if view.Listings == nil {
				view.Listings = make([]zimmo.Listing, 0)
			}
			if !noStore && len(res.Listings) > 0 {
				if err := db.UpsertZimmoListings(ctx, res.Listings, time.Now(), false); err != nil {
					return err
				}
				_ = db.SaveSyncState("listings", "", len(res.Listings))
				view.Stored = true
			}
			if res.Total == 0 {
				view.Note = "no listing matches these criteria on Zimmo"
			} else if res.Truncated != nil {
				view.Note = fmt.Sprintf("stopped after %d listings: Zimmo refused a later page (%v)", len(res.Listings), res.Truncated)
			} else if len(res.Listings) < res.Total {
				view.Note = fmt.Sprintf("returned %d of %d matching listings; raise --limit/--pages for more", len(res.Listings), res.Total)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printZimmo(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Listings) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching listings.")
				return nil
			}
			if err := printListingTable(cmd.OutOrStdout(), view.Listings); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d of %d matching listings", view.Returned, view.Total)
			if view.Stored {
				fmt.Fprint(cmd.OutOrStdout(), " (stored)")
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	addCritFlags(cmd, &cf)
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum listings to return")
	cmd.Flags().IntVar(&pages, "pages", 10, "Maximum result pages (100 listings each) to fetch")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not write results to the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's data.db)")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFindCmd(flags))
	})
}
