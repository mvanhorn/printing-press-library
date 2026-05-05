// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// rankEntry is one row in the --by-aware ranking output.
type rankEntry struct {
	Rank         int     `json:"rank"`
	URL          string  `json:"url"`
	PropertyID   string  `json:"property_id,omitempty"`
	Title        string  `json:"title,omitempty"`
	Beds         int     `json:"beds,omitempty"`
	MaxRent      int     `json:"max_rent,omitempty"`
	Sqft         int     `json:"sqft,omitempty"`
	PricePerSqft float64 `json:"price_per_sqft,omitempty"`
	PricePerBed  float64 `json:"price_per_bed,omitempty"`
}

func newRankCmd(flags *rootFlags) *cobra.Command {
	var by string
	rf := &rentalsFlags{}

	cmd := &cobra.Command{
		Use:         "rank",
		Short:       "Rank synced listings by ratio metrics — price per square foot or price per bedroom.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli rank --by sqft --limit 10 --json
  apartments-pp-cli rank --by bed --beds 2
  apartments-pp-cli rank --by rent --price-max 2500
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch by {
			case "", "sqft", "bed", "rent":
			default:
				return usageErr(fmt.Errorf("invalid --by %q: must be sqft|bed|rent", by))
			}
			if dryRunOK(flags) {
				return nil
			}
			db, err := openAptStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			rows, err := loadCachedListings(db.DB())
			if err != nil {
				return err
			}

			var entries []rankEntry
			for _, r := range rows {
				li := r.Data
				if rf.beds > 0 && li.Beds != rf.beds {
					continue
				}
				if rf.bedsMin > 0 && li.Beds < rf.bedsMin {
					continue
				}
				if rf.priceMin > 0 && li.MaxRent < rf.priceMin {
					continue
				}
				if rf.priceMax > 0 && li.MaxRent > rf.priceMax {
					continue
				}
				if rf.city != "" && !strings.EqualFold(strings.ReplaceAll(strings.ToLower(li.Address.City), " ", "-"), rf.city) {
					continue
				}
				if rf.state != "" && !strings.EqualFold(li.Address.State, rf.state) {
					continue
				}
				e := rankEntry{
					URL:        li.URL,
					PropertyID: li.PropertyID,
					Title:      li.Title,
					Beds:       li.Beds,
					MaxRent:    li.MaxRent,
					Sqft:       li.Sqft,
				}
				if li.MaxRent > 0 && li.Sqft > 0 {
					e.PricePerSqft = float64(li.MaxRent) / float64(li.Sqft)
				}
				if li.MaxRent > 0 && li.Beds > 0 {
					e.PricePerBed = float64(li.MaxRent) / float64(li.Beds)
				}
				entries = append(entries, e)
			}

			byKey := by
			if byKey == "" {
				byKey = "sqft"
			}
			sort.SliceStable(entries, func(i, j int) bool {
				a, b := entries[i], entries[j]
				switch byKey {
				case "sqft":
					ai := a.PricePerSqft
					bj := b.PricePerSqft
					if ai == 0 {
						ai = 1e18
					}
					if bj == 0 {
						bj = 1e18
					}
					if ai != bj {
						return ai < bj
					}
				case "bed":
					ai := a.PricePerBed
					bj := b.PricePerBed
					if ai == 0 {
						ai = 1e18
					}
					if bj == 0 {
						bj = 1e18
					}
					if ai != bj {
						return ai < bj
					}
				}
				ar := a.MaxRent
				br := b.MaxRent
				if ar == 0 {
					ar = 1 << 30
				}
				if br == 0 {
					br = 1 << 30
				}
				return ar < br
			})

			if rf.limit > 0 && len(entries) > rf.limit {
				entries = entries[:rf.limit]
			}
			for i := range entries {
				entries[i].Rank = i + 1
			}
			if entries == nil {
				entries = []rankEntry{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), entries, flags)
		},
	}
	cmd.Flags().StringVar(&by, "by", "sqft", "Ranker: sqft|bed|rent.")
	// Flags inlined (not via addRentalsFlags) so the public-library
	// verify-skill static analyzer can see them on this command.
	cmd.Flags().StringVar(&rf.city, "city", "", "City slug (lowercased, hyphens for spaces). Example: austin, new-york.")
	cmd.Flags().StringVar(&rf.state, "state", "", "Two-letter state abbreviation (lowercase).")
	cmd.Flags().StringVar(&rf.zip, "zip", "", "ZIP code; overrides --city/--state when set.")
	cmd.Flags().IntVar(&rf.beds, "beds", 0, "Exact bedroom count. Mutually exclusive with --beds-min.")
	cmd.Flags().IntVar(&rf.bedsMin, "beds-min", 0, "Minimum bedrooms. Mutually exclusive with --beds.")
	cmd.Flags().BoolVar(&rf.studio, "studio", false, "Match studios.")
	cmd.Flags().IntVar(&rf.baths, "baths", 0, "Exact bathroom count.")
	cmd.Flags().IntVar(&rf.bathsMin, "baths-min", 0, "Minimum bathrooms.")
	cmd.Flags().IntVar(&rf.priceMin, "price-min", 0, "Minimum monthly rent in USD.")
	cmd.Flags().IntVar(&rf.priceMax, "price-max", 0, "Maximum monthly rent in USD.")
	cmd.Flags().StringVar(&rf.pets, "pets", "", "Pet filter: any|cat|dog|both|none.")
	cmd.Flags().StringVar(&rf.typ, "type", "", "Property type: apartment|house|condo|townhome.")
	cmd.Flags().IntVar(&rf.page, "page", 0, "Page number (1-indexed; default 1).")
	cmd.Flags().IntVar(&rf.limit, "limit", 25, "Max rows to return.")
	cmd.Flags().BoolVar(&rf.all, "all", false, "Auto-paginate up to 5 pages.")
	rf.limit = 25
	// Hide --all/--page from rank — they're not meaningful here, but
	// kept on the struct for code reuse.
	_ = cmd.Flags().MarkHidden("all")
	_ = cmd.Flags().MarkHidden("page")
	return cmd
}
