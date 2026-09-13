// Copyright 2026 sambassio and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

type triageRow struct {
	Rank        int                 `json:"rank"`
	Score       float64             `json:"score"`
	ID          int64               `json:"id"`
	URL         string              `json:"url"`
	Price       *float64            `json:"price,omitempty"`
	PricePerSqm *float64            `json:"price_per_m2,omitempty"`
	Bedrooms    *int                `json:"bedrooms,omitempty"`
	Surface     *float64            `json:"surface_m2,omitempty"`
	Locality    string              `json:"locality"`
	Seller      string              `json:"seller"`
	DaysListed  *int                `json:"days_listed,omitempty"`
	EPC         string              `json:"epc,omitempty"`
	Factors     []immo.TriageFactor `json:"factors"`
}

type triageView struct {
	Search     string         `json:"search"`
	Source     string         `json:"source"`
	Candidates int            `json:"candidates"`
	Excluded   map[string]int `json:"excluded,omitempty"`
	Enriched   int            `json:"enriched"`
	Results    []triageRow    `json:"results"`
}

func newNovelTriageCmd(flags *rootFlags) *cobra.Command {
	var limit, enrich, maxPages int
	var includeUnderOption, includeRooms bool
	var dbPath string
	var cf critFlags
	cmd := &cobra.Command{
		Use:   "triage [saved search name]",
		Short: "Ranks the current matches of a saved search by a transparent score (price vs commune, freshness, price cut, private seller, EPC)",
		Long: `Rank the current matches of a saved search so you know who to call first.
Each listing gets a 0-100 score with the per-factor breakdown: €/m² percentile
against comparable stored listings in the same postal code (35%), freshness (25%),
price cut (15%), private seller (10%) and EPC label (15%, fetched with --enrich).
Hidden listings, listings under option and student rooms or per-room lets
(--include-rooms keeps them) are excluded. Pass a saved search name,
or the same filters as find (--type, --deal, --commune, --max-price, ...) for an
ad-hoc ranking.

Use this command to decide which current matches of a saved search to contact first.
Do NOT use this command for what changed since the last run; use 'watch' instead.
Do NOT use it for an in-depth look at a single listing; use 'deal' instead.`,
		Example: strings.Trim(`
  immoweb-pp-cli triage ixelles-2bed --agent
  immoweb-pp-cli triage --type apartment --deal rent --commune ixelles --max-price 1500 --min-bedrooms 2 --agent
  immoweb-pp-cli triage ixelles-2bed --enrich 5 --limit 5`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":       "true", // reads Immoweb only; results land in the CLI's own cache
			"pp:data-source":      "auto",
			"pp:happy-args":       "--type=apartment;--deal=rent;--commune=ixelles;--max-price=1500;--min-bedrooms=2",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "triage")
			}
			useHarvestRate(cmd, flags)
			var adhoc *immo.Criteria
			if len(args) == 0 {
				crit, err := cf.build()
				if err != nil {
					return usageErr(err)
				}
				if err := crit.Validate(); err != nil {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("give a saved search name (see: immoweb-pp-cli saved list) or search filters: %w", err))
				}
				adhoc = &crit
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openImmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var ss store.SavedSearch
			if adhoc != nil {
				ss = store.SavedSearch{Name: "ad-hoc", Criteria: *adhoc}
			} else if ss, err = loadSaved(cmd, db, args[0]); err != nil {
				return err
			}
			view := triageView{Search: ss.Name, Results: make([]triageRow, 0), Excluded: map[string]int{}}
			var candidates []store.StoredListing
			var liveClient *client.Client
			if flags.dataSource == "local" {
				view.Source = "local"
				if adhoc != nil {
					pcs := append([]string{}, adhoc.PostalCodes...)
					for _, name := range adhoc.Communes {
						more, err := localPostcodesFor(ctx, db, name)
						if err != nil {
							return err
						}
						pcs = append(pcs, more...)
					}
					if reason := localUnsupported(*adhoc); reason != "" {
						return usageErr(fmt.Errorf("--data-source local cannot evaluate %s from the local store; drop it or run live", reason))
					}
					if len(pcs) == 0 {
						return usageErr(fmt.Errorf("--data-source local needs --commune or --postcode for ad-hoc triage"))
					}
					rows, err := db.QueryListings(ctx, store.ListingFilter{Deal: adhoc.Deal, Types: adhoc.Types, PostalCodes: pcs})
					if err != nil {
						return err
					}
					for _, r := range rows {
						if matchesStored(r, *adhoc) {
							candidates = append(candidates, r)
						}
					}
				} else {
					seen, err := db.SeenIDs(ctx, ss.Name)
					if err != nil {
						return err
					}
					ids := make([]int64, 0, len(seen))
					for id := range seen {
						ids = append(ids, id)
					}
					if len(ids) > 0 {
						if candidates, err = db.QueryListings(ctx, store.ListingFilter{IDs: ids}); err != nil {
							return err
						}
					}
				}
			} else {
				view.Source = "live"
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				liveClient = c
				crit, _, err := resolveCriteria(ctx, c, ss.Criteria)
				if err != nil {
					return err
				}
				hv, err := harvest(ctx, c, db, crit.Params(), dogfoodPages(maxPages))
				if err != nil {
					return err
				}
				ids := make([]int64, 0, len(hv.Listings))
				for _, l := range hv.Listings {
					ids = append(ids, l.ID)
				}
				if len(ids) > 0 {
					if candidates, err = db.QueryListings(ctx, store.ListingFilter{IDs: ids}); err != nil {
						return err
					}
				}
			}
			hidden, err := db.HiddenSet(ctx)
			if err != nil {
				return err
			}
			hists, err := db.AllPriceHistories(ctx)
			if err != nil {
				return err
			}
			pools := map[string][]store.StoredListing{}
			score := func(ls []store.StoredListing) ([]triageRow, error) {
				out := make([]triageRow, 0, len(ls))
				now := time.Now()
				for _, l := range ls {
					key := l.Deal + "|" + l.Type + "|" + l.PostalCode
					if _, ok := pools[key]; !ok {
						p, err := db.QueryListings(ctx, store.ListingFilter{Deal: l.Deal, Types: []string{l.Type}, PostalCodes: []string{l.PostalCode}, IncludeGone: true})
						if err != nil {
							return nil, err
						}
						pools[key] = p
					}
					var pct *float64
					if l.PricePerSqm != nil {
						vals := make([]float64, 0, len(pools[key]))
						for _, p := range pools[key] {
							// Rooms compare with rooms, whole homes with whole homes.
							if p.ID != l.ID && p.PricePerSqm != nil && immo.IsRoomLet(p.Listing) == immo.IsRoomLet(l.Listing) {
								vals = append(vals, *p.PricePerSqm)
							}
						}
						if len(vals) >= minComparables {
							if v, ok := immo.PercentileRank(vals, *l.PricePerSqm); ok {
								pct = &v
							}
						}
					}
					_, cut := priceCut(l, hists[l.ID])
					days := daysOf(l, now)
					var updated *int
					if d, ok := immo.DaysListed(l.ModifiedAt, now); ok && days == nil {
						updated = &d
					}
					s, factors := immo.TriageScore(immo.TriageInput{PricePercentile: pct, DaysListed: days, DaysSinceUpdate: updated, PriceCut: cut || l.NewPrice, Private: l.Private, EPC: l.EPC})
					out = append(out, triageRow{Score: s, ID: l.ID, URL: l.URL, Price: l.Price, PricePerSqm: l.PricePerSqm, Bedrooms: l.Bedrooms, Surface: l.Surface,
						Locality: locLabel(l.Listing), Seller: sellerLabel(l.Listing), DaysListed: days, EPC: l.EPC, Factors: factors})
				}
				sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
				return out, nil
			}
			exclude := func(ls []store.StoredListing, count bool) []store.StoredListing {
				kept := make([]store.StoredListing, 0, len(ls))
				for _, l := range ls {
					switch {
					case hidden[l.ID]:
						if count {
							view.Excluded["hidden"]++
						}
					case l.UnderOption && !includeUnderOption:
						if count {
							view.Excluded["under_option"]++
						}
					case !includeRooms && immo.IsRoomLet(l.Listing):
						if count {
							view.Excluded["room_let"]++
						}
					default:
						kept = append(kept, l)
					}
				}
				return kept
			}
			filtered := exclude(candidates, true)
			view.Candidates = len(filtered)
			rows, err := score(filtered)
			if err != nil {
				return err
			}
			if enrich > 0 && liveClient != nil {
				for i := 0; i < len(rows) && i < enrich; i++ {
					d, raw, err := fetchDetail(ctx, liveClient, rows[i].ID)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: enrich %d: %v\n", rows[i].ID, err)
						continue
					}
					if err := db.SaveDetail(ctx, d, raw, time.Now()); err != nil {
						return err
					}
					view.Enriched++
				}
				ids := make([]int64, 0, len(filtered))
				for _, l := range filtered {
					ids = append(ids, l.ID)
				}
				if len(ids) > 0 {
					if filtered, err = db.QueryListings(ctx, store.ListingFilter{IDs: ids}); err != nil {
						return err
					}
				}
				// The detail call may have revealed an option or a sale.
				before := len(filtered)
				filtered = exclude(filtered, false)
				view.Excluded["under_option_after_enrich"] += before - len(filtered)
				if view.Excluded["under_option_after_enrich"] == 0 {
					delete(view.Excluded, "under_option_after_enrich")
				}
				if rows, err = score(filtered); err != nil {
					return err
				}
			}
			for i := range rows {
				rows[i].Rank = i + 1
			}
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			view.Results = rows
			if len(view.Excluded) == 0 {
				view.Excluded = nil
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No current matches for %q.\n", ss.Name)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "#\tSCORE\tID\tPRICE\t€/M²\tBEDS\tDAYS\tEPC\tSELLER\tLOCALITY")
			for _, r := range rows {
				days := ""
				if r.DaysListed != nil {
					days = fmt.Sprint(*r.DaysListed)
				}
				fmt.Fprintf(tw, "%d\t%.0f\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", r.Rank, r.Score, r.ID, fmtPtrEUR(r.Price), fmtPtrEUR(r.PricePerSqm), intStr(r.Bedrooms), days, r.EPC, r.Seller, r.Locality)
			}
			_ = tw.Flush()
			fmt.Fprintln(cmd.OutOrStdout(), "\nscore = 35% price vs postcode · 25% freshness · 15% price cut · 10% private seller · 15% EPC (use --json for the breakdown)")
			return nil
		},
	}
	addCritFlags(cmd, &cf, false)
	cmd.Flags().IntVar(&limit, "limit", 10, "Number of ranked listings to return")
	cmd.Flags().IntVar(&enrich, "enrich", 0, "Fetch full details (EPC, creation date) for the top N before the final ranking")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Search pages (30 each) to walk for searches over 200 listings")
	cmd.Flags().BoolVar(&includeUnderOption, "include-under-option", false, "Keep listings already under option")
	cmd.Flags().BoolVar(&includeRooms, "include-rooms", false, "Keep student rooms (kots) and per-room lets, excluded by default")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

// localUnsupported names criteria the local store cannot evaluate.
func localUnsupported(c immo.Criteria) string {
	switch {
	case c.Garden || c.Terrace || c.Pool || c.NewBuild || c.LifeAnnuity || c.PublicSale || c.Furnished:
		return "feature filters (--garden, --terrace, --pool, --new-build, --life-annuity, --public-sale, --furnished)"
	case c.MinYear > 0:
		return "--min-year"
	case (len(c.Provinces) > 0 || len(c.Districts) > 0) && len(c.PostalCodes) == 0 && len(c.Communes) == 0:
		return "--province/--district"
	}
	return ""
}

// matchesStored applies the numeric and EPC criteria to a stored listing.
// Unknown values (nil) never exclude a listing.
func matchesStored(l store.StoredListing, c immo.Criteria) bool {
	inRange := func(v *float64, lo, hi int) bool {
		if v == nil {
			return true
		}
		return (lo == 0 || *v >= float64(lo)) && (hi == 0 || *v <= float64(hi))
	}
	if !inRange(l.Price, c.MinPrice, c.MaxPrice) || !inRange(l.Surface, c.MinSurface, c.MaxSurface) || !inRange(l.Land, c.MinLand, c.MaxLand) {
		return false
	}
	if l.Bedrooms != nil {
		b := float64(*l.Bedrooms)
		if !inRange(&b, c.MinBedrooms, c.MaxBedrooms) {
			return false
		}
	}
	if len(c.EPC) > 0 && l.EPC != "" {
		ok := false
		for _, e := range c.EPC {
			if strings.EqualFold(e, l.EPC) {
				ok = true
			}
		}
		if !ok {
			return false
		}
	}
	return true
}
