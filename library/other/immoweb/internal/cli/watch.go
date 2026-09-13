// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/immo"
	"github.com/mvanhorn/printing-press-library/library/other/immoweb/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newWatchCmd(flags))
	})
}

type watchPriceChange struct {
	immo.Listing
	OldPrice float64 `json:"old_price"`
	NewPrice float64 `json:"new_price"`
	Pct      float64 `json:"change_pct"`
}

type watchGone struct {
	ID        int64    `json:"id"`
	URL       string   `json:"url"`
	Title     string   `json:"title,omitempty"`
	Locality  string   `json:"locality,omitempty"`
	LastPrice *float64 `json:"last_price,omitempty"`
	LastSeen  string   `json:"last_seen"`
}

type watchReport struct {
	Search       string             `json:"search"`
	Baseline     bool               `json:"baseline"`
	TotalOnSite  int                `json:"total_on_immoweb"`
	Fetched      int                `json:"fetched"`
	Complete     bool               `json:"complete"`
	New          []immo.Listing     `json:"new"`
	PriceChanges []watchPriceChange `json:"price_changes"`
	Gone         []watchGone        `json:"gone"`
	LeftSearch   []watchGone        `json:"left_search"`
	Note         string             `json:"note,omitempty"`
	Error        string             `json:"error,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var maxPages int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "watch [saved-search]",
		Short: "What is new, cheaper or gone in a saved search since its last run",
		Long: `Run a saved search against Immoweb and diff it with the previous run of that search:
new listings, price changes (against the price seen at the last watch) and listings
that left the search. For searches limited to deal, type and commune or postal code, a listing that
left is reported as gone from Immoweb (sold, rented or withdrawn); for searches with
price or other filters it is reported under left_search (it may only have moved out
of your filters). The first run records a silent baseline. Hidden listings are left
out. Output is JSON-friendly for notifiers (pipe --json into Telegram, email, ...).

Use this command for what changed since the last run of one saved search.
Do NOT use it to rank current matches; use 'triage' instead.`,
		Example: strings.Trim(`
  immoweb-pp-cli watch ixelles-2bed
  immoweb-pp-cli watch --all --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write":     "true",
			"pp:data-source":      "live",
			"pp:happy-args":       "name=ixelles-2bed",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "run saved searches and diff with the last run")
			}
			useHarvestRate(cmd, flags)
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 && !all {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("give a saved search name or --all"))
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openImmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var searches []store.SavedSearch
			if all {
				if searches, err = db.ListSearches(ctx); err != nil {
					return err
				}
				if len(searches) == 0 {
					return notFoundErr(fmt.Errorf("no saved searches yet; create one with: immoweb-pp-cli saved add <name> --type ... --deal ..."))
				}
			} else {
				ss, err := loadSaved(cmd, db, args[0])
				if err != nil {
					return err
				}
				searches = []store.SavedSearch{ss}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			pages := dogfoodPages(maxPages)
			reports := make([]watchReport, 0, len(searches))
			var firstErr error
			for _, ss := range searches {
				rep, err := runWatch(ctx, db, c, ss, pages)
				if err != nil {
					// Keep going: earlier searches already committed their run,
					// so their changes must still be printed.
					rep.Error = err.Error()
					if firstErr == nil {
						firstErr = fmt.Errorf("watch %s: %w", ss.Name, err)
					}
				}
				reports = append(reports, rep)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				var perr error
				if len(reports) == 1 {
					perr = printJSONFiltered(cmd.OutOrStdout(), reports[0], flags)
				} else {
					perr = printJSONFiltered(cmd.OutOrStdout(), reports, flags)
				}
				if perr != nil {
					return perr
				}
				return firstErr
			}
			w := cmd.OutOrStdout()
			for _, r := range reports {
				fmt.Fprintf(w, "%s — %d on Immoweb\n", bold(r.Search), r.TotalOnSite)
				if r.Error != "" {
					fmt.Fprintf(w, "  error: %s\n\n", r.Error)
					continue
				}
				if r.Baseline {
					fmt.Fprintf(w, "  baseline recorded (%d listings); run again later to see changes\n\n", r.Fetched)
					continue
				}
				if len(r.New)+len(r.PriceChanges)+len(r.Gone)+len(r.LeftSearch) == 0 {
					fmt.Fprintln(w, "  no changes since last run")
				}
				for _, l := range r.New {
					row := listingRow(l)
					fmt.Fprintf(w, "  NEW   %d  %s  %s beds  %s m²  %s  %s\n", l.ID, row["price"], row["beds"], row["m2"], row["locality"], l.URL)
				}
				for _, p := range r.PriceChanges {
					fmt.Fprintf(w, "  PRICE %d  %s → %s (%+.1f%%)  %s\n", p.ID, fmtEUR(p.OldPrice), fmtEUR(p.NewPrice), p.Pct, p.URL)
				}
				for _, g := range r.Gone {
					fmt.Fprintf(w, "  GONE  %d  %s  last seen %s\n", g.ID, g.Locality, shortDate(g.LastSeen))
				}
				for _, g := range r.LeftSearch {
					fmt.Fprintf(w, "  LEFT  %d  %s  (no longer matches the filters)\n", g.ID, g.Locality)
				}
				if r.Note != "" {
					fmt.Fprintf(w, "  note: %s\n", r.Note)
				}
				fmt.Fprintln(w)
			}
			return firstErr
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Watch every saved search")
	cmd.Flags().IntVar(&maxPages, "max-pages", 20, "Search pages (30 listings each) for the rare areas price bands cannot split")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

// postcodeScoped reports whether criteria constrain only deal, type and
// postal codes (communes resolve to postal codes), so a listing missing from
// a complete harvest is gone from Immoweb. Province- or district-only
// searches never qualify: a stored listing's area cannot be checked.
func postcodeScoped(c immo.Criteria) bool {
	return len(c.PostalCodes) > 0 && c.MinPrice == 0 && c.MaxPrice == 0 && c.MinBedrooms == 0 && c.MaxBedrooms == 0 &&
		c.MinSurface == 0 && c.MaxSurface == 0 && c.MinLand == 0 && c.MaxLand == 0 && c.MinYear == 0 && len(c.EPC) == 0 &&
		!c.Garden && !c.Terrace && !c.Pool && !c.NewBuild && !c.LifeAnnuity && !c.PublicSale && !c.Furnished
}

func shortDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func runWatch(ctx context.Context, db *store.Store, cl *client.Client, ss store.SavedSearch, pages int) (watchReport, error) {
	rep := watchReport{Search: ss.Name, New: make([]immo.Listing, 0), PriceChanges: make([]watchPriceChange, 0), Gone: make([]watchGone, 0), LeftSearch: make([]watchGone, 0)}
	crit, _, err := resolveCriteria(ctx, cl, ss.Criteria)
	if err != nil {
		return rep, err
	}
	seen, err := db.SeenIDs(ctx, ss.Name)
	if err != nil {
		return rep, err
	}
	rep.Baseline = ss.LastRunAt == ""
	hv, err := harvest(ctx, cl, db, crit.Params(), pages)
	if err != nil {
		return rep, err
	}
	rep.TotalOnSite, rep.Fetched, rep.Complete = hv.Total, len(hv.Listings), hv.Complete
	hidden, err := db.HiddenSet(ctx)
	if err != nil {
		return rep, err
	}
	current := map[int64]bool{}
	prices := make(map[int64]*float64, len(hv.Listings))
	for _, l := range hv.Listings {
		current[l.ID] = true
		prices[l.ID] = l.Price
		if rep.Baseline || hidden[l.ID] {
			continue
		}
		prev, ok := seen[l.ID]
		if !ok {
			rep.New = append(rep.New, l)
			continue
		}
		if prev.LastPrice != nil && l.Price != nil && *l.Price != *prev.LastPrice && *prev.LastPrice > 0 {
			old := *prev.LastPrice
			rep.PriceChanges = append(rep.PriceChanges, watchPriceChange{Listing: l, OldPrice: old, NewPrice: *l.Price, Pct: immo.Round1((*l.Price - old) / old * 100)})
		}
	}
	left := []int64{}
	if !rep.Baseline && hv.PricedComplete {
		for id, sl := range seen {
			// Unpriced listings are invisible to price bands; only judge them
			// when the harvest was complete.
			if !current[id] && (sl.LastPrice != nil || hv.Complete) {
				left = append(left, id)
			}
		}
	}
	if len(left) > 0 {
		rows, err := db.QueryListings(ctx, store.ListingFilter{IDs: left, IncludeGone: true})
		if err != nil {
			return rep, err
		}
		// A listing is gone from Immoweb only when the search constrains
		// nothing but deal, type and postal codes, and the listing's postal code
		// is still inside today's resolved area (commune resolution can
		// change between runs). Everything else merely left the search.
		scope := map[string]bool{}
		for _, pc := range crit.PostalCodes {
			scope[immo.BarePostalCode(pc)] = true
		}
		pure := postcodeScoped(crit)
		goneIDs := []int64{}
		for _, r := range rows {
			if hidden[r.ID] {
				continue
			}
			g := watchGone{ID: r.ID, URL: r.URL, Title: r.Title, Locality: locLabel(r.Listing), LastPrice: seen[r.ID].LastPrice, LastSeen: seen[r.ID].LastSeen}
			if pure && scope[r.PostalCode] {
				rep.Gone = append(rep.Gone, g)
				goneIDs = append(goneIDs, r.ID)
			} else {
				rep.LeftSearch = append(rep.LeftSearch, g)
			}
		}
		if len(goneIDs) > 0 {
			if err := db.MarkGone(ctx, goneIDs, time.Now()); err != nil {
				return rep, err
			}
		}
	}
	switch {
	case !hv.PricedComplete:
		rep.Note = fmt.Sprintf("only %d of %d listings fetched; disappearances not evaluated this run", len(hv.Listings), hv.Total)
	case !hv.Complete:
		rep.Note = fmt.Sprintf("%d listings without a published price are not tracked", hv.Total-hv.Priced)
	}
	if err := db.RecordSearchRun(ctx, ss.Name, prices, left, time.Now()); err != nil {
		return rep, err
	}
	return rep, nil
}
