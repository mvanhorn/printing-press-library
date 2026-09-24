// pp:data-source live

package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/immovlan"
	"github.com/mvanhorn/printing-press-library/library/other/immovlan/internal/store"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newWatchCmd(flags))
	})
}

type watchPriceChange struct {
	immovlan.Listing
	OldPrice float64 `json:"old_price"`
	NewPrice float64 `json:"new_price"`
	Pct      float64 `json:"change_pct"`
}

type watchGone struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Title     string   `json:"title,omitempty"`
	Locality  string   `json:"locality,omitempty"`
	LastPrice *float64 `json:"last_price,omitempty"`
	LastSeen  string   `json:"last_seen"`
}

type watchReport struct {
	Search       string             `json:"search"`
	Baseline     bool               `json:"baseline"`
	Fetched      int                `json:"fetched"`
	Complete     bool               `json:"complete"`
	New          []immovlan.Listing `json:"new"`
	PriceChanges []watchPriceChange `json:"price_changes"`
	Gone         []watchGone        `json:"gone"`
	LeftSearch   []watchGone        `json:"left_search"`
	Note         string             `json:"note,omitempty"`
	Error        string             `json:"error,omitempty"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var all bool
	var pages int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "watch [saved search name]",
		Short: "What is new, cheaper or gone in a saved search since its last run",
		Long: `Run a saved search against Immovlan and diff it with the previous run: new
listings, price changes and listings that left the search. For searches limited
to deal, type and postal codes, a listing that left is reported as gone from
Immovlan; with price, bedroom or PEB filters it is reported under left_search.
The first run records a silent baseline.`,
		Example: strings.Trim(`
  immovlan-pp-cli watch sch-fg --agent
  immovlan-pp-cli watch --all --pages 5 --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write": "true",
			"pp:data-source":  "live",
			"pp:happy-args":   "--all",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "run saved searches and diff with the last run")
			}
			if err := rejectDataSource(flags, "live"); err != nil {
				return usageErr(err)
			}
			if len(args) == 0 && !all {
				return usageErr(fmt.Errorf("give a saved search name or --all"))
			}
			pages = dogfoodPages(max(pages, 1))
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			db, err := openVlanStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var searches []store.SavedSearch
			if all {
				if searches, err = db.ListSearches(ctx); err != nil {
					return err
				}
			} else {
				ss, err := db.GetSearch(ctx, args[0])
				if isNoRows(err) {
					return notFoundErr(fmt.Errorf("no saved search named %q (see: immovlan-pp-cli saved list)", args[0]))
				}
				if err != nil {
					return err
				}
				searches = []store.SavedSearch{ss}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			reports := make([]watchReport, 0, len(searches))
			var firstErr error
			for _, ss := range searches {
				rep, err := runWatch(ctx, c, db, ss, pages)
				if err != nil {
					rep.Error = termSafe(err.Error())
					if firstErr == nil {
						firstErr = err
					}
					if !all {
						return err
					}
				}
				reports = append(reports, rep)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if all {
					return printView(cmd.OutOrStdout(), reports, flags)
				}
				return printView(cmd.OutOrStdout(), reports[0], flags)
			}
			w := cmd.OutOrStdout()
			if len(reports) == 0 {
				fmt.Fprintln(w, "No saved searches. Add one with: immovlan-pp-cli saved add <name> ...")
				return nil
			}
			for _, rep := range reports {
				fmt.Fprintf(w, "%s: ", bold(rep.Search))
				switch {
				case rep.Error != "":
					fmt.Fprintf(w, "error: %s\n", rep.Error)
				case rep.Baseline:
					fmt.Fprintf(w, "baseline recorded (%d listings); run again later to see changes\n", rep.Fetched)
				default:
					fmt.Fprintf(w, "%d new · %d price changes · %d gone · %d left the search\n", len(rep.New), len(rep.PriceChanges), len(rep.Gone), len(rep.LeftSearch))
					for _, l := range rep.New {
						fmt.Fprintf(w, "  NEW   %s %s %s %s\n", l.ID, fmtPtrEUR(l.Price), locLabel(l), termSafe(l.URL))
					}
					for _, p := range rep.PriceChanges {
						fmt.Fprintf(w, "  PRICE %s %s → %s (%+.1f%%) %s\n", p.ID, fmtEUR(p.OldPrice), fmtEUR(p.NewPrice), p.Pct, termSafe(p.URL))
					}
					for _, g := range rep.Gone {
						fmt.Fprintf(w, "  GONE  %s %s %s\n", g.ID, fmtPtrEUR(g.LastPrice), termSafe(g.URL))
					}
					for _, g := range rep.LeftSearch {
						fmt.Fprintf(w, "  LEFT  %s %s %s\n", g.ID, fmtPtrEUR(g.LastPrice), termSafe(g.URL))
					}
				}
				if rep.Note != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "  note: %s\n", rep.Note)
				}
			}
			return firstErr
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Run every saved search")
	cmd.Flags().IntVar(&pages, "pages", 10, fmt.Sprintf("Result pages to walk per search (%d listings each)", vlanPageSize))
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db")
	return cmd
}

// scopePostcodes returns the postcodes a search is bound to, whether given as
// --postcode or as <postcode>-<slug> towns (URL / --commune searches).
func scopePostcodes(c immovlan.Criteria) []string {
	out := append([]string{}, c.PostalCodes...)
	for _, pc := range immovlan.TownsPostcodes(c.Towns) {
		out = appendUniq(out, pc)
	}
	return out
}

// postcodeScoped reports whether a search constrains only deal, type and
// postcodes, so a listing missing from a complete walk is gone from Immovlan.
func postcodeScoped(c immovlan.Criteria) bool {
	return len(scopePostcodes(c)) > 0 && len(c.EPC) == 0 && c.MinPrice == 0 && c.MaxPrice == 0 && c.MinBedrooms == 0 && c.MaxBedrooms == 0
}

func runWatch(ctx context.Context, c *client.Client, db *store.Store, ss store.SavedSearch, pages int) (watchReport, error) {
	rep := watchReport{Search: ss.Name, New: []immovlan.Listing{}, PriceChanges: []watchPriceChange{}, Gone: []watchGone{}, LeftSearch: []watchGone{}}
	rep.Baseline = ss.LastRunAt == ""
	start := time.Now()
	seen, err := db.SeenIDs(ctx, ss.Name)
	if err != nil {
		return rep, err
	}
	hidden, err := db.HiddenSet(ctx)
	if err != nil {
		return rep, err
	}
	var fetched []immovlan.Listing
	complete := false
	for page := 1; page <= pages; page++ {
		sp, err := fetchSearchPage(ctx, c, ss.Criteria, page)
		if err != nil {
			if len(fetched) == 0 {
				return rep, err
			}
			rep.Note = fmt.Sprintf("stopped at page %d: %s", page, termSafe(err.Error()))
			break
		}
		if err := db.UpsertVlanListings(ctx, sp.Items, time.Now()); err != nil {
			return rep, err
		}
		fetched = append(fetched, sp.Items...)
		// A short page is not the last one (promoted cards are skipped by the
		// parser); only the pagination says whether more pages exist.
		if !sp.HasNext {
			complete = true
			break
		}
		if len(sp.Items) == 0 {
			// every card unparsable on a page that says more follow: stop, but
			// do not treat the walk as complete (nothing may be marked gone).
			rep.Note = fmt.Sprintf("page %d had no parsable listing; disappearances not evaluated this run", page)
			break
		}
	}
	rep.Fetched, rep.Complete = len(fetched), complete
	current := map[string]bool{}
	prices := make(map[string]*float64, len(fetched))
	for _, l := range fetched {
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
			pct := pct1((*l.Price - old) / old)
			rep.PriceChanges = append(rep.PriceChanges, watchPriceChange{Listing: l, OldPrice: old, NewPrice: *l.Price, Pct: pct})
		}
	}
	left := []string{}
	if !rep.Baseline && complete {
		for id := range seen {
			if !current[id] {
				left = append(left, id)
			}
		}
	}
	if len(left) > 0 {
		rows, err := db.QueryVlanListings(ctx, store.ListingFilter{IDs: left, IncludeGone: true})
		if err != nil {
			return rep, err
		}
		scope := map[string]bool{}
		for _, pc := range scopePostcodes(ss.Criteria) {
			scope[pc] = true
		}
		pure := postcodeScoped(ss.Criteria)
		goneIDs := []string{}
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
		if _, err := db.MarkGone(ctx, goneIDs, start, time.Now()); err != nil {
			return rep, err
		}
	}
	if !complete && rep.Note == "" {
		rep.Note = fmt.Sprintf("only %d pages walked; disappearances not evaluated this run (raise --pages)", pages)
	}
	if err := db.RecordSearchRun(ctx, ss.Name, prices, left, time.Now()); err != nil {
		return rep, err
	}
	return rep, nil
}
