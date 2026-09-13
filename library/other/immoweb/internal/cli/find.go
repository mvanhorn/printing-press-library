// pp:data-source live

package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newFindCmd(flags))
	})
}

type findView struct {
	Criteria      immo.Criteria     `json:"criteria"`
	Communes      []resolvedCommune `json:"resolved_communes"`
	TotalMatching int               `json:"total_matching"`
	Fetched       int               `json:"fetched"`
	PagesFetched  int               `json:"pages_fetched"`
	Returned      int               `json:"returned"`
	Filtered      map[string]int    `json:"filtered_out,omitempty"`
	Results       []immo.Listing    `json:"results"`
	Stored        bool              `json:"stored_locally"`
	Note          string            `json:"note,omitempty"`
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var cf critFlags
	var pages, limit int
	var privateOnly, hideUnderOption, includeHidden, noStore bool
	var near string
	var radiusKm float64
	var dbPath string

	cmd := &cobra.Command{
		Use:   "find",
		Short: "Search live Immoweb listings with friendly filters (commune names, sale/rent, price, bedrooms, EPC...)",
		Long: `Search live Immoweb listings. Commune names are resolved to Immoweb postal codes,
every page fetched is stored locally (price history, watch, market, deal build on it),
and local-only filters (--private-only, --hide-under-option, --near) apply on top.

Paste an Immoweb search URL with --url to start from a search built on the website.`,
		Example: strings.Trim(`
  immoweb-pp-cli find --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --sort newest
  immoweb-pp-cli find --type house --deal sale --province walloon_brabant --max-price 400000 --garden --pages 3 --agent
  immoweb-pp-cli find --url "https://www.immoweb.be/fr/recherche/maison/a-vendre/liege/4000?maxPrice=300000"
  immoweb-pp-cli find --type house --deal sale --commune uccle --private-only --hide-under-option --csv`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true", // reads Immoweb only; results land in the CLI's own cache
			"pp:data-source": "live",
			"pp:happy-args":  "--type=apartment;--deal=rent;--commune=ixelles;--max-price=1500",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search Immoweb listings")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			crit, err := cf.build()
			if err != nil {
				return usageErr(err)
			}
			if err := crit.Validate(); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			var nearLat, nearLng float64
			if near != "" {
				parts := strings.Split(near, ",")
				if len(parts) != 2 {
					return usageErr(fmt.Errorf("--near must be \"lat,lng\" (e.g. 50.8466,4.3528)"))
				}
				la, e1 := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
				ln, e2 := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
				if e1 != nil || e2 != nil {
					return usageErr(fmt.Errorf("--near must be two numbers \"lat,lng\""))
				}
				nearLat, nearLng = la, ln
			}
			if pages < 1 {
				pages = 1
			}
			pages = dogfoodPages(pages)
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			crit, resolved, err := resolveCriteria(ctx, c, crit)
			if err != nil {
				return err
			}
			params := crit.Params()
			view := findView{Criteria: crit, Communes: resolved, Results: make([]immo.Listing, 0), Filtered: map[string]int{}, Stored: !noStore}
			var fetched []immo.Listing
			var db *store.Store
			if !noStore {
				db, err = openImmoStore(ctx, dbPath)
				if err != nil {
					return err
				}
				defer db.Close()
			}
			total := 0
			for page := 1; page <= pages; page++ {
				sp, err := fetchResults(ctx, c, "/en/search-results", params, page, immoPageSize, total)
				if err != nil {
					if len(fetched) > 0 {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: stopped at page %d: %v\n", page, err)
						break
					}
					return err
				}
				total = sp.Total
				view.PagesFetched = page
				if db != nil && len(sp.Items) > 0 {
					if err := db.UpsertImmoListings(ctx, sp.Items, sp.Raw, time.Now()); err != nil {
						return err
					}
				}
				fetched = append(fetched, sp.Items...)
				if len(sp.Items) < immoPageSize || page*immoPageSize >= min(total, immoResultsCap) {
					break
				}
			}
			if n, err := fetchCount(ctx, c, params); err == nil {
				total = n // search pages cap totalItems at 9,969
			}
			view.TotalMatching = total
			view.Fetched = len(fetched)
			hidden := map[int64]bool{}
			if db != nil && !includeHidden {
				if hidden, err = db.HiddenSet(ctx); err != nil {
					return err
				}
			}
			for _, l := range fetched {
				switch {
				case hidden[l.ID]:
					view.Filtered["hidden"]++
					continue
				case privateOnly && !l.Private:
					view.Filtered["agency"]++
					continue
				case hideUnderOption && l.UnderOption:
					view.Filtered["under_option"]++
					continue
				}
				if near != "" {
					if l.Lat == nil || l.Lng == nil || immo.Haversine(nearLat, nearLng, *l.Lat, *l.Lng) > radiusKm {
						view.Filtered["outside_radius"]++
						continue
					}
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
			}
			if total > view.Fetched && view.Note == "" {
				view.Note = fmt.Sprintf("%d of %d matching listings fetched; use --pages to fetch more (30 per page)", view.Fetched, total)
			}
			if flags.csv {
				rows := make([]map[string]any, 0, len(view.Results))
				for _, l := range view.Results {
					rows = append(rows, listingRow(l))
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			for _, rc := range resolved {
				fmt.Fprintf(cmd.ErrOrStderr(), "%s → %s (%s)\n", rc.Name, rc.Label, strings.Join(rc.PostalCodes, ","))
			}
			if len(view.Results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No matching listings (%d on Immoweb for these filters).\n", total)
				return nil
			}
			if err := printListingTable(cmd.OutOrStdout(), view.Results); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\n%d shown · %d matching on Immoweb · open one with: immoweb-pp-cli show <id>\n", view.Returned, total)
			if view.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), view.Note)
			}
			return nil
		},
	}
	addCritFlags(cmd, &cf, true)
	cmd.Flags().IntVar(&pages, "pages", 1, "Result pages to fetch (30 listings per page)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum listings to return after local filters (0 = all fetched)")
	cmd.Flags().BoolVar(&privateOnly, "private-only", false, "Only private sellers (listings without an agency)")
	cmd.Flags().BoolVar(&hideUnderOption, "hide-under-option", false, "Drop listings already under option")
	cmd.Flags().BoolVar(&includeHidden, "include-hidden", false, "Include listings you hid with 'immoweb-pp-cli hide'")
	cmd.Flags().StringVar(&near, "near", "", "Keep listings within --radius-km of \"lat,lng\"")
	cmd.Flags().Float64Var(&radiusKm, "radius-km", 2, "Radius for --near in kilometres")
	cmd.Flags().BoolVar(&noStore, "no-store", false, "Do not record results in the local store")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}
