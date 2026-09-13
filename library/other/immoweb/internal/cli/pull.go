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
		addNovelCommandIfAbsent(root, newPullCmd(flags))
	})
}

type pullReport struct {
	Scope        string            `json:"scope"`
	Communes     []resolvedCommune `json:"resolved_communes"`
	TotalOnSite  int               `json:"total_on_immoweb"`
	Stored       int               `json:"stored"`
	Complete     bool              `json:"complete"`
	MarkedGone   int               `json:"marked_gone"`
	WithoutPrice int               `json:"without_price"`
	Requests     int               `json:"requests"`
	Note         string            `json:"note,omitempty"`
}

// scopeHarvest harvests an area and, when the harvest was complete and the
// criteria only constrain deal/type/location, marks locally stored listings
// of that area that Immoweb no longer returns as gone.
func scopeHarvest(ctx context.Context, c *client.Client, db *store.Store, crit immo.Criteria, maxPages int) (pullReport, error) {
	rep := pullReport{Communes: make([]resolvedCommune, 0)}
	crit, resolved, err := resolveCriteria(ctx, c, crit)
	if err != nil {
		return rep, err
	}
	rep.Communes = resolved
	rep.Scope = crit.Key()
	started := time.Now().Add(-time.Second) // last_seen has second precision
	hv, err := harvest(ctx, c, db, crit.Params(), maxPages)
	rep.Requests = hv.Calls
	if err != nil {
		return rep, err
	}
	rep.TotalOnSite, rep.Stored, rep.Complete = hv.Total, len(hv.Listings), hv.Complete
	if hv.Priced > 0 { // unknown when the harvest fell back to search paging
		rep.WithoutPrice = hv.Total - hv.Priced
	}
	if hv.PricedComplete && postcodeScoped(crit) {
		existing, err := db.QueryListings(ctx, store.ListingFilter{Deal: crit.Deal, Types: crit.Types, PostalCodes: crit.PostalCodes})
		if err != nil {
			return rep, err
		}
		current := map[int64]bool{}
		for _, l := range hv.Listings {
			current[l.ID] = true
		}
		gone := []int64{}
		for _, e := range existing {
			// Listings without a published price are invisible to price bands.
			if !current[e.ID] && (e.Price != nil || hv.Complete) {
				gone = append(gone, e.ID)
			}
		}
		marked, err := db.MarkGone(ctx, gone, started, time.Now())
		if err != nil {
			return rep, err
		}
		rep.MarkedGone = marked
	}
	switch {
	case !hv.PricedComplete:
		rep.Note = fmt.Sprintf("stored %d of %d listings; narrow the area or raise --max-pages to get them all", len(hv.Listings), hv.Total)
	case !hv.Complete:
		rep.Note = fmt.Sprintf("%d listings without a published price ('price on request') are not included", hv.Total-hv.Priced)
	}
	if err := db.RecordPull(ctx, rep.Scope, rep.Stored, time.Now()); err != nil {
		return rep, err
	}
	if err := db.SaveSyncState(store.ImmoResource, "", rep.Stored); err != nil {
		return rep, err
	}
	return rep, nil
}

func newPullCmd(flags *rootFlags) *cobra.Command {
	var cf critFlags
	var maxPages int
	var dbPath string
	cmd := &cobra.Command{
		Use:   "pull",
		Short: "Harvest every listing of an area into the local store (feeds market, deal, yield, drops)",
		Long: `Collect all listings of an area and deal type into the local store, beating the
30-per-page and 9,969-result caps. The CLI counts the area first, takes one map call
(up to 200 listings) when it fits, and otherwise splits it into price bands of at most
200 listings each, one map call per band. Listings without a published price cannot be
reached through price bands and are reported as without_price. Listings that vanished
since the previous pull of the same area are marked gone (sold, rented or withdrawn).

Use pull (not the generic sync command) to fill the store: sync has no area filter.`,
		Example: strings.Trim(`
  immoweb-pp-cli pull --commune ixelles --type apartment --deal rent
  immoweb-pp-cli pull --commune liege --type apartment --deal sale --max-pages 40 --agent`, "\n"),
		Annotations: map[string]string{
			"mcp:local-write": "true",
			"pp:data-source":  "live",
			"pp:happy-args":   "--commune=ixelles;--type=apartment;--deal=rent",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "harvest an area into the local store")
			}
			useHarvestRate(cmd, flags)
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
			if len(crit.Communes)+len(crit.PostalCodes)+len(crit.Provinces)+len(crit.Districts) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an area is required (--commune, --postcode, --province or --district)"))
			}
			ctx, cancel := harvestCtx(cmd, flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			db, err := openImmoStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rep, err := scopeHarvest(ctx, c, db, crit, dogfoodPages(maxPages))
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Stored %d of %d listings (%d requests); %d marked gone.\n", rep.Stored, rep.TotalOnSite, rep.Requests, rep.MarkedGone)
			if rep.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), rep.Note)
			}
			return nil
		},
	}
	addCritFlags(cmd, &cf, false)
	cmd.Flags().IntVar(&maxPages, "max-pages", 40, "Search pages (30 listings each) to walk for areas with more than 200 listings")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local store path (default: the CLI's database)")
	_ = cmd.Flags().MarkHidden("db") // not an MCP tool argument: agents must not point the store at other files
	return cmd
}

// ensureArea makes sure the store holds a harvest of the given criteria no
// older than maxAge; it harvests the area when missing or stale.
func ensureArea(ctx context.Context, cmd *cobra.Command, c *client.Client, db *store.Store, crit immo.Criteria, maxAge time.Duration, maxPages int) (pullReport, bool, error) {
	resolvedCrit, resolved, err := resolveCriteria(ctx, c, crit)
	if err != nil {
		return pullReport{}, false, err
	}
	last, n, err := db.LastPull(ctx, resolvedCrit.Key())
	if err != nil {
		return pullReport{}, false, err
	}
	if !last.IsZero() && time.Since(last) < maxAge {
		return pullReport{Scope: resolvedCrit.Key(), Communes: resolved, Complete: true, Stored: n, TotalOnSite: -1}, false, nil
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "pulling %s %s listings for %s from Immoweb...\n", strings.ToLower(strings.Join(crit.Types, ",")), strings.ToLower(crit.Deal), strings.Join(append(append([]string{}, crit.Communes...), crit.PostalCodes...), ","))
	rep, err := scopeHarvest(ctx, c, db, resolvedCrit, maxPages)
	rep.Communes = resolved
	return rep, true, err
}
