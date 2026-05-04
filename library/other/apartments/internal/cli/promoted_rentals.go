// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

// rentalsFlags holds every flag the rentals command (and its sync-search
// sibling) consume — kept here so other commands can reuse the binding
// helper.
type rentalsFlags struct {
	city     string
	state    string
	zip      string
	beds     int
	bedsMin  int
	studio   bool
	baths    int
	bathsMin int
	priceMin int
	priceMax int
	pets     string
	typ      string
	page     int
	limit    int
	all      bool
}

func (rf *rentalsFlags) toOptions() apt.SearchOptions {
	return apt.SearchOptions{
		City:     rf.city,
		State:    rf.state,
		Zip:      rf.zip,
		Beds:     rf.beds,
		BedsMin:  rf.bedsMin,
		Studio:   rf.studio,
		Baths:    rf.baths,
		BathsMin: rf.bathsMin,
		PriceMin: rf.priceMin,
		PriceMax: rf.priceMax,
		Pets:     rf.pets,
		Type:     rf.typ,
		Page:     rf.page,
	}
}

func (rf *rentalsFlags) validate() error {
	if rf.pets != "" {
		switch strings.ToLower(rf.pets) {
		case "any", "cat", "dog", "both", "none":
		default:
			return fmt.Errorf("invalid --pets %q: must be one of any|cat|dog|both|none", rf.pets)
		}
	}
	if rf.typ != "" {
		switch strings.ToLower(rf.typ) {
		case "apartment", "house", "condo", "townhome":
		default:
			return fmt.Errorf("invalid --type %q: must be one of apartment|house|condo|townhome", rf.typ)
		}
	}
	if rf.beds > 0 && rf.bedsMin > 0 {
		return fmt.Errorf("--beds and --beds-min are mutually exclusive")
	}
	if rf.zip == "" && rf.city == "" && rf.state == "" {
		return fmt.Errorf("provide --city + --state or --zip")
	}
	if rf.limit <= 0 {
		rf.limit = 60
	}
	return nil
}

// addRentalsFlags binds rentalsFlags to a cobra command. All commands
// that build a SearchOptions reuse this helper so the flag surface is
// identical across rentals, sync-search, and nearby.
func addRentalsFlags(cmd *cobra.Command, rf *rentalsFlags) {
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
	cmd.Flags().IntVar(&rf.limit, "limit", 60, "Max placards to return.")
	cmd.Flags().BoolVar(&rf.all, "all", false, "Auto-paginate up to 5 pages.")
}

func newRentalsCmd(flags *rootFlags) *cobra.Command {
	rf := &rentalsFlags{}

	cmd := &cobra.Command{
		Use:         "rentals",
		Short:       "Search apartments.com by city/state, beds, price, pets, etc.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli rentals --city austin --state tx
  apartments-pp-cli rentals --city austin --state tx --beds 2 --price-max 2500 --pets dog --json
  apartments-pp-cli rentals --zip 78704 --beds-min 1 --price-min 1500 --price-max 2500
  apartments-pp-cli rentals --city austin --state tx --type house --beds 3 --all
  apartments-pp-cli rentals --city austin --state tx --beds 2 --pets dog --dry-run
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				opts := rf.toOptions()
				fmt.Fprintln(cmd.OutOrStdout(), "would GET:", apt.BuildSearchURL(opts))
				return nil
			}
			if err := rf.validate(); err != nil {
				return usageErr(err)
			}
			opts := rf.toOptions()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var collected []apt.Placard
			page := opts.Page
			if page < 1 {
				page = 1
			}
			maxPages := 1
			if rf.all {
				maxPages = 5
			}

			for i := 0; i < maxPages; i++ {
				current := opts
				if i == 0 && page == 1 {
					current.Page = 0 // canonical: omit /1/
				} else {
					current.Page = page
				}
				path := apt.BuildSearchURL(current)
				data, gerr := c.Get(path, nil)
				if gerr != nil {
					if i == 0 {
						return classifyAPIError(gerr)
					}
					break
				}
				placards, perr := apt.ParsePlacards([]byte(data), c.BaseURL)
				if perr != nil {
					return apiErr(perr)
				}
				for _, p := range placards {
					p.SearchSlug = path
					collected = append(collected, p)
					if len(collected) >= rf.limit {
						break
					}
				}
				if len(collected) >= rf.limit || len(placards) == 0 {
					break
				}
				if !rf.all {
					break
				}
				page++
				time.Sleep(800 * time.Millisecond)
			}
			if len(collected) > rf.limit {
				collected = collected[:rf.limit]
			}
			if collected == nil {
				collected = []apt.Placard{}
			}
			return printJSONFiltered(cmd.OutOrStdout(), collected, flags)
		},
	}

	addRentalsFlags(cmd, rf)
	return cmd
}
