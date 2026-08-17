// watch: save a named search and, on each re-run, diff the current live scan
// against the last stored snapshot to surface new listings and price drops.
// Novel feature — the headline capability; Zameen has no API and no alerts.
// pp:data-source live
package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/store"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/types"
	"github.com/mvanhorn/printing-press-library/library/commerce/zameen/internal/zameen"
)

// savedSearch is the JSON-persisted definition of a named search.
type savedSearch struct {
	City         string  `json:"city"`
	Location     string  `json:"location"`
	Area         string  `json:"area"`
	Purpose      string  `json:"purpose"`
	Type         string  `json:"type"`
	MinPrice     int     `json:"min_price"`
	MaxPrice     int     `json:"max_price"`
	MinBeds      int     `json:"min_beds"`
	MaxBeds      int     `json:"max_beds"`
	MinBaths     int     `json:"min_baths"`
	MinArea      float64 `json:"min_area"`
	MaxArea      float64 `json:"max_area"`
	Verified     bool    `json:"verified"`
	Sort         string  `json:"sort"`
	MaxScanPages int     `json:"max_scan_pages"`
}

func (s savedSearch) toSearchFlags() searchFlags {
	sf := searchFlags{
		city: s.City, location: s.Location, area: s.Area, purpose: s.Purpose,
		propertyType: s.Type, minPrice: s.MinPrice, maxPrice: s.MaxPrice,
		minBeds: s.MinBeds, maxBeds: s.MaxBeds, minBaths: s.MinBaths,
		minAreaMarla: s.MinArea, maxAreaMarla: s.MaxArea, verifiedOnly: s.Verified,
		sortKey: s.Sort, limit: s.MaxScanPages * zameen.PageSize, maxScanPages: s.MaxScanPages,
	}
	if sf.maxScanPages <= 0 {
		sf.maxScanPages = 5
		sf.limit = 5 * zameen.PageSize
	}
	return sf
}

type priceDrop struct {
	types.Listing
	OldPrice int     `json:"old_price"`
	NewPrice int     `json:"new_price"`
	Drop     int     `json:"drop"`
	PctDrop  float64 `json:"pct_drop"`
}

type watchResult struct {
	Name         string          `json:"name"`
	FirstRun     bool            `json:"first_run"`
	TotalCurrent int             `json:"total_current"`
	NewCount     int             `json:"new_count"`
	DropCount    int             `json:"drop_count"`
	NewListings  []types.Listing `json:"new_listings"`
	PriceDrops   []priceDrop     `json:"price_drops"`
}

func newNovelWatchCmd(flags *rootFlags) *cobra.Command {
	var sf searchFlags
	var dbPath string
	var list bool
	cmd := &cobra.Command{
		Use:   "watch <name>",
		Short: "Re-run a saved search and see exactly which listings are new and which dropped in price since last time.",
		Long: "Save a named search (pass the same filter flags 'find' takes). On each re-run, watch " +
			"scans Zameen live and diffs the results against the last stored snapshot, reporting " +
			"newly-added listings and listings whose price dropped.\n\n" +
			"First run records a baseline. Use --list to see saved searches.",
		Example: "  zameen-pp-cli watch dha-homes --city Islamabad --purpose buy --type Homes --area DHA_Defence",
		// Mutates the local snapshot store only (no external writes).
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would diff a saved search against its last snapshot")
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("zameen-pp-cli")
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := db.EnsureWatchTables(ctx); err != nil {
				return err
			}

			if list {
				saved, lErr := db.ListWatchSearches(ctx)
				if lErr != nil {
					return lErr
				}
				return emitObject(cmd, flags, saved)
			}

			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a saved-search <name> is required (or use --list)"))
			}
			name := args[0]

			// Load or define the search.
			def, ok, err := db.GetWatchSearch(ctx, name)
			if err != nil {
				return err
			}
			hasFilters := cmd.Flags().Changed("city") || cmd.Flags().Changed("location")
			if hasFilters {
				ss := savedSearch{
					City: sf.city, Location: sf.location, Area: sf.area, Purpose: sf.purpose,
					Type: sf.propertyType, MinPrice: sf.minPrice, MaxPrice: sf.maxPrice,
					MinBeds: sf.minBeds, MaxBeds: sf.maxBeds, MinBaths: sf.minBaths,
					MinArea: sf.minAreaMarla, MaxArea: sf.maxAreaMarla, Verified: sf.verifiedOnly,
					Sort: sf.sortKey, MaxScanPages: sf.maxScanPages,
				}
				raw, _ := json.Marshal(ss)
				if err := db.SaveWatchSearch(ctx, name, string(raw), time.Now().Unix()); err != nil {
					return err
				}
				def = string(raw)
			} else if !ok {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("no saved search named %q; define it by passing --city/--location and other filters, e.g.\n  zameen-pp-cli watch %s --city Islamabad --purpose buy --type Homes", name, name))
			}

			var ss savedSearch
			if err := json.Unmarshal([]byte(def), &ss); err != nil {
				return fmt.Errorf("reading saved search %q: %w", name, err)
			}
			tsf := ss.toSearchFlags()
			params, err := tsf.toParams()
			if err != nil {
				return err
			}

			c := zameen.NewClient(flags.timeout)
			res, err := c.Search(ctx, params)
			if err != nil {
				var rl *cliutil.RateLimitError
				if errors.As(err, &rl) {
					return rateLimitErr(err)
				}
				return apiErr(err)
			}

			if res.PartialError != "" {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: scan stopped early after a fetch error: %s (diff may miss listings)\n", res.PartialError)
			}
			byID := make(map[string]types.Listing, len(res.Listings))
			priceByID := make(map[string]int, len(res.Listings))
			for _, l := range res.Listings {
				if l.ExternalId == "" {
					continue
				}
				byID[l.ExternalId] = l
				priceByID[l.ExternalId] = l.Price
			}

			prev, err := db.LoadWatchSnapshot(ctx, name)
			if err != nil {
				return err
			}
			out := watchResult{
				Name:         name,
				TotalCurrent: len(priceByID),
				FirstRun:     len(prev) == 0,
				NewListings:  []types.Listing{},
				PriceDrops:   []priceDrop{},
			}
			if !out.FirstRun {
				for id, l := range byID {
					oldPrice, seen := prev[id]
					if !seen {
						out.NewListings = append(out.NewListings, l)
						continue
					}
					if l.Price > 0 && l.Price < oldPrice {
						drop := oldPrice - l.Price
						pct := 0.0
						if oldPrice > 0 {
							pct = math.Round(float64(drop)/float64(oldPrice)*1000) / 10
						}
						out.PriceDrops = append(out.PriceDrops, priceDrop{
							Listing: l, OldPrice: oldPrice, NewPrice: l.Price, Drop: drop, PctDrop: pct,
						})
					}
				}
			}
			sort.SliceStable(out.NewListings, func(i, j int) bool {
				return out.NewListings[i].CreatedAt > out.NewListings[j].CreatedAt
			})
			sort.SliceStable(out.PriceDrops, func(i, j int) bool {
				return out.PriceDrops[i].PctDrop > out.PriceDrops[j].PctDrop
			})
			out.NewCount = len(out.NewListings)
			out.DropCount = len(out.PriceDrops)

			if err := db.ReplaceWatchSnapshot(ctx, name, priceByID, time.Now().Unix()); err != nil {
				return fmt.Errorf("saving snapshot: %w", err)
			}

			if out.FirstRun {
				fmt.Fprintf(cmd.ErrOrStderr(), "baseline recorded for %q: %d listings tracked. Re-run to see new listings and price drops.\n", name, out.TotalCurrent)
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "%q: %d new, %d price drops (of %d current)\n", name, out.NewCount, out.DropCount, out.TotalCurrent)
			}
			return emitObject(cmd, flags, out)
		},
	}
	addSearchFlags(cmd, &sf, true)
	cmd.Flags().BoolVar(&list, "list", false, "List saved searches instead of running one")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard data dir)")
	return cmd
}
