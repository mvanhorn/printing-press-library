// pp:data-source live

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFindCmd(flags))
	})
}

type findView struct {
	Criteria      immovlan.Criteria  `json:"criteria"`
	TotalMatching int                `json:"total_matching"` // estimated from the pagination (20 per page)
	TotalIsExact  bool               `json:"total_is_exact"`
	Fetched       int                `json:"fetched"`
	PagesFetched  int                `json:"pages_fetched"`
	LastPage      int                `json:"last_page"`
	Returned      int                `json:"returned"`
	Filtered      map[string]int     `json:"filtered_out,omitempty"`
	Results       []immovlan.Listing `json:"results"`
	Stored        bool               `json:"stored_locally"`
	Note          string             `json:"note,omitempty"`
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var cf critFlags
	var pages, limit int
	var privateOnly, includeHidden, noStore bool
	var epcOnly string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search live Immovlan listings with friendly filters (communes, sale/rent/public-sale, price, bedrooms, PEB)",
		Long: `Search live Immovlan listings. Brussels commune names resolve offline to postal
codes; every page fetched (20 cards) is stored locally so watch, drops, enrich,
peb-trap, same-as and split-candidates can build on it. The PEB letter comes from
the card's watermark; --epc accepts letters (F,G) or Immovlan bands (bad, poor).

Paste an immovlan.be search URL with --url to start from a search built on the site.`,
		Example: strings.Trim(`
  immovlan-pp-cli find --type maison,appartement --deal sale --postcode 1030,1210 --epc F,G --min-price 500000 --max-price 2500000 --pages 3 --agent
  immovlan-pp-cli find --type appartement --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2
  immovlan-pp-cli find --deal public-sale --postcode 1030,1080 --agent
  immovlan-pp-cli find --url "https://immovlan.be/fr/immobilier?transactiontypes=a-vendre&propertytypes=maison&towns=1030-schaerbeek" --csv`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true", // reads immovlan.be only; results land in the CLI's own cache
			"pp:data-source": "live",
			"pp:happy-args":  "--type=maison;--deal=sale;--postcode=1030;--epc=F,G",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search Immovlan listings")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			if pages < 1 {
				pages = 1
			}
			pages = dogfoodPages(pages)
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			var db *store.Store
			var err error
			if !noStore {
				if db, err = openVlanStore(ctx, dbPath); err != nil {
					return err
				}
				defer db.Close()
			}
			crit, err := cf.build(ctx, db)
			if err != nil {
				return usageErr(err)
			}
			if err := crit.Validate(); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view := findView{Criteria: crit, Results: make([]immovlan.Listing, 0), Filtered: map[string]int{}, Stored: !noStore}
			var fetched []immovlan.Listing
			for page := 1; page <= pages; page++ {
				sp, err := fetchSearchPage(ctx, c, crit, page)
				if err != nil {
					if len(fetched) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped at page %d: %s\n", page, termSafe(err.Error()))
						break
					}
					return err
				}
				view.PagesFetched = page
				if sp.LastPage > view.LastPage {
					view.LastPage = sp.LastPage
				}
				if db != nil && len(sp.Items) > 0 {
					if err := db.UpsertVlanListings(ctx, sp.Items, time.Now()); err != nil {
						return err
					}
				}
				fetched = append(fetched, sp.Items...)
				if !sp.HasNext || len(sp.Items) == 0 {
					view.TotalIsExact = true
					break
				}
			}
			view.Fetched = len(fetched)
			view.TotalMatching = view.LastPage * vlanPageSize
			if view.TotalIsExact {
				view.TotalMatching = view.Fetched
			}
			hidden := map[string]bool{}
			if db != nil && !includeHidden {
				if hidden, err = db.HiddenSet(ctx); err != nil {
					return err
				}
			}
			letters, err := parseEPCLetters(epcOnly)
			if err != nil {
				return err
			}
			applyFindFilters(&view, fetched, hidden, privateOnly, letters, limit)
			if flags.csv {
				// Same columns as dump --format csv so both portals load into one sheet.
				rows := make([]store.StoredListing, 0, len(view.Results))
				for _, l := range view.Results {
					rows = append(rows, store.StoredListing{Listing: l})
				}
				return writeVlanCSV(cmd.OutOrStdout(), rows)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printView(cmd.OutOrStdout(), view, flags)
			}
			if len(view.Results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No matching listings on Immovlan for these filters.")
				return nil
			}
			if err := printListingTable(cmd.OutOrStdout(), view.Results); err != nil {
				return err
			}
			total := fmt.Sprintf("about %d", view.TotalMatching)
			if view.TotalIsExact {
				total = fmt.Sprintf("%d", view.TotalMatching)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d shown · %s matching on Immovlan · open one with: immovlan-pp-cli show <ref>\n", view.Returned, total)
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	addCritFlags(cmd, &cf, true)
	cmd.Flags().IntVar(&pages, "pages", 1, fmt.Sprintf("Result pages to fetch (%d listings per page)", vlanPageSize))
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum listings to return after local filters (0 = all fetched)")
	cmd.Flags().BoolVar(&privateOnly, "private-only", false, "Only private sellers")
	cmd.Flags().StringVar(&epcOnly, "epc-letter", "", "Keep only cards whose PEB watermark is one of these letters (F,G) or bands (bad); --epc searches by band")
	cmd.Flags().BoolVar(&includeHidden, "include-hidden", false, "Include listings you hid with 'immovlan-pp-cli hide'")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not record results in the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

// applyFindFilters keeps the fetched cards that pass the local filters and
// writes the counts and the note an agent needs to decide whether to fetch more.
func applyFindFilters(view *findView, fetched []immovlan.Listing, hidden map[string]bool, privateOnly bool, letters map[string]bool, limit int) {
	if view.Filtered == nil {
		view.Filtered = map[string]int{}
	}
	for _, l := range fetched {
		switch {
		case hidden[l.ID]:
			view.Filtered["hidden"]++
			continue
		case privateOnly && !l.Private:
			view.Filtered["agency"]++
			continue
		case len(letters) > 0 && !letters[strings.ToUpper(l.EPC)]:
			view.Filtered["epc_letter"]++
			continue
		}
		view.Results = append(view.Results, l)
		if limit > 0 && len(view.Results) >= limit {
			break
		}
	}
	view.Returned = len(view.Results)
	if len(view.Filtered) == 0 {
		view.Filtered = nil
	}
	if view.Returned == 0 && view.Fetched > 0 {
		view.Note = "every fetched listing was removed by local filters; raise --pages to scan more"
	} else if !view.TotalIsExact {
		view.Note = fmt.Sprintf("%d listings fetched from %d of %d pages (%d per page); use --pages to fetch more", view.Fetched, view.PagesFetched, view.LastPage, vlanPageSize)
	}
}
