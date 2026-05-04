// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

// nearbyEntry is one ranked placard from a nearby fan-out.
type nearbyEntry struct {
	URL         string  `json:"url"`
	PropertyID  string  `json:"property_id,omitempty"`
	Title       string  `json:"title,omitempty"`
	Beds        int     `json:"beds,omitempty"`
	Baths       float64 `json:"baths,omitempty"`
	MaxRent     int     `json:"max_rent,omitempty"`
	SearchSlug  string  `json:"search_slug,omitempty"`
	PricePerBed float64 `json:"price_per_bed,omitempty"`
}

func newNearbyCmd(flags *rootFlags) *cobra.Command {
	var rank string
	rf := &rentalsFlags{}

	cmd := &cobra.Command{
		Use:         "nearby <city-state> [city-state...]",
		Short:       "Fan out a search across multiple city-state slugs and return one ranked, deduped list.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli nearby austin-tx round-rock-tx --beds 2 --price-max 2500 --json
  apartments-pp-cli nearby brooklyn-ny queens-ny --pets dog --rank rent
  # Note: --rank sqft only ranks listings with both maxrent and sqft populated;
  # most placard summaries don't carry sqft, so those float to the bottom.
  apartments-pp-cli nearby austin-tx round-rock-tx pflugerville-tx --rank sqft
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			switch rank {
			case "", "rent", "bed", "sqft":
			default:
				return usageErr(fmt.Errorf("invalid --rank %q: must be rent|bed|sqft", rank))
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			seen := map[string]bool{}
			var collected []nearbyEntry
			for _, slug := range args {
				slug = strings.ToLower(strings.TrimSpace(slug))
				if slug == "" {
					continue
				}
				city, state := splitCityState(slug)
				localOpts := rf.toOptions()
				localOpts.City = city
				localOpts.State = state
				path := apt.BuildSearchURL(localOpts)
				data, gerr := c.Get(path, nil)
				if gerr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetch %s failed: %v\n", slug, gerr)
					continue
				}
				placards, perr := apt.ParsePlacards([]byte(data), c.BaseURL)
				if perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: parse %s failed: %v\n", slug, perr)
					continue
				}
				for _, p := range placards {
					if seen[p.URL] {
						continue
					}
					seen[p.URL] = true
					ne := nearbyEntry{
						URL:        p.URL,
						PropertyID: p.PropertyID,
						Title:      p.Title,
						Beds:       p.Beds,
						Baths:      p.Baths,
						MaxRent:    p.MaxRent,
						SearchSlug: slug,
					}
					if p.Beds > 0 && p.MaxRent > 0 {
						ne.PricePerBed = float64(p.MaxRent) / float64(p.Beds)
					}
					collected = append(collected, ne)
				}
				time.Sleep(800 * time.Millisecond)
			}

			rankBy := rank
			if rankBy == "" {
				rankBy = "rent"
			}
			sort.SliceStable(collected, func(i, j int) bool {
				a, b := collected[i], collected[j]
				switch rankBy {
				case "bed":
					ai := bigIfZero(a.PricePerBed)
					bj := bigIfZero(b.PricePerBed)
					return ai < bj
				case "sqft":
					// Placards don't carry sqft, so this rank tier
					// effectively groups MaxRent ascending; documented.
					return tieBreakRent(a, b)
				default: // rent
					return tieBreakRent(a, b)
				}
			})
			if collected == nil {
				collected = []nearbyEntry{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), collected, flags)
		},
	}
	cmd.Flags().StringVar(&rank, "rank", "rent", "Ranker: rent|bed|sqft.")
	addRentalsFlags(cmd, rf)
	return cmd
}

func bigIfZero(v float64) float64 {
	if v == 0 {
		return 1e18
	}
	return v
}

func tieBreakRent(a, b nearbyEntry) bool {
	ai := a.MaxRent
	bj := b.MaxRent
	if ai == 0 {
		ai = 1 << 30
	}
	if bj == 0 {
		bj = 1 << 30
	}
	return ai < bj
}

func splitCityState(slug string) (string, string) {
	idx := strings.LastIndex(slug, "-")
	if idx <= 0 || idx >= len(slug)-1 {
		return slug, ""
	}
	return slug[:idx], slug[idx+1:]
}
