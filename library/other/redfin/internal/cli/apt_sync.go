// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/redfin/internal/redfin"

	"github.com/spf13/cobra"
)

func newAptSyncCmd(flags *rootFlags) *cobra.Command {
	hf := &homesFlags{status: "for-sale"}

	cmd := &cobra.Command{
		Use:   "sync-search <slug>",
		Short: "Run a saved search and persist results to the local store.",
		Long: `Runs the gis search defined by the filter flags, then writes one row per
listing into both:
  • the canonical homes table (for transcendence reads), and
  • listing_snapshots tagged with the saved-search slug (for watch diffs).

The slug + filter options are also recorded in saved_searches so future
runs can re-execute the same query by name.`,
		Example:     `  redfin-pp-cli sync-search austin-3br --region-id 30772 --region-type 6 --beds-min 3 --price-max 700000`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/gis (saved-search slug required at runtime)")
					return nil
				}
				return cmd.Help()
			}
			slug := args[0]
			opts, oerr := optsFromFlags(hf)
			if oerr != nil {
				if dryRunOK(flags) {
					fmt.Fprintln(cmd.ErrOrStderr(), "would GET: /stingray/api/gis (saved-search "+slug+"; region required at runtime)")
					return nil
				}
				// Fall back to a previously-saved search if region wasn't
				// explicit on the CLI.
				s, oerr2 := openRedfinStore(cmd.Context())
				if oerr2 != nil {
					return oerr
				}
				saved, ok, gerr := redfin.GetSavedSearch(s.DB(), slug)
				s.Close()
				if gerr != nil || !ok {
					return oerr
				}
				opts = saved
			}
			if dryRunOK(flags) {
				printDryRunGet(cmd, "/stingray/api/gis", redfin.BuildSearchParams(opts))
				return nil
			}
			listings, err := runHomesSearch(cmd, flags, opts, hf.all)
			if err != nil {
				return err
			}
			s, err := openRedfinStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()
			now := time.Now()
			for i := range listings {
				listings[i].SearchSlug = slug
				if err := upsertListingHome(s, listings[i]); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: upsert homes %s: %v\n", listings[i].URL, err)
				}
				raw, _ := json.Marshal(listings[i])
				snap := redfin.Snapshot{
					ListingURL:  listings[i].URL,
					PropertyID:  listings[i].PropertyID,
					SavedSearch: slug,
					ObservedAt:  now,
					Status:      listings[i].Status,
					Price:       listings[i].Price,
					Beds:        listings[i].Beds,
					Baths:       listings[i].Baths,
					Sqft:        listings[i].Sqft,
					DOM:         listings[i].DOM,
					FetchStatus: 200,
					RawData:     raw,
				}
				if err := redfin.InsertSnapshot(s.DB(), snap); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: insert snapshot %s: %v\n", listings[i].URL, err)
				}
			}
			if err := redfin.UpsertSavedSearch(s.DB(), slug, opts, len(listings)); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: upsert saved search: %v\n", err)
			}
			out := map[string]any{
				"saved_search": slug,
				"count":        len(listings),
				"synced_at":    now.UTC().Format(time.RFC3339),
				"path":         s.Path(),
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}

	cmd.Flags().Int64Var(&hf.regionID, "region-id", 0, "Numeric Redfin region ID")
	cmd.Flags().IntVar(&hf.regionType, "region-type", 6, "Region type")
	cmd.Flags().StringVar(&hf.regionSlug, "region-slug", "", "Region slug")
	cmd.Flags().StringVar(&hf.status, "status", "for-sale", "Listing status: for-sale|sold|pending|coming-soon")
	cmd.Flags().StringVar(&hf.pType, "type", "", "Comma-separated property types")
	cmd.Flags().Float64Var(&hf.bedsMin, "beds-min", 0, "Minimum bedrooms")
	cmd.Flags().Float64Var(&hf.bathsMin, "baths-min", 0, "Minimum bathrooms")
	cmd.Flags().IntVar(&hf.priceMin, "price-min", 0, "Minimum price")
	cmd.Flags().IntVar(&hf.priceMax, "price-max", 0, "Maximum price")
	cmd.Flags().IntVar(&hf.sqftMin, "sqft-min", 0, "Minimum sqft")
	cmd.Flags().IntVar(&hf.sqftMax, "sqft-max", 0, "Maximum sqft")
	cmd.Flags().IntVar(&hf.yearMin, "year-min", 0, "Earliest year built")
	cmd.Flags().IntVar(&hf.yearMax, "year-max", 0, "Latest year built")
	cmd.Flags().IntVar(&hf.lotMin, "lot-min", 0, "Minimum lot size")
	cmd.Flags().IntVar(&hf.schoolsMin, "schools-min", 0, "Minimum school rating")
	cmd.Flags().StringVar(&hf.polygon, "polygon", "", "Bounding polygon")
	cmd.Flags().IntVar(&hf.page, "page", 1, "1-indexed page number")
	cmd.Flags().IntVar(&hf.limit, "limit", 50, "Listings per page (max 350)")
	cmd.Flags().BoolVar(&hf.all, "all", false, "Auto-paginate up to 5 pages")

	return cmd
}
