// Copyright 2026 rderwin. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/apartments/internal/apt"

	"github.com/spf13/cobra"
)

func newMustHaveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "must-have <term> [term...]",
		Short:       "Filter synced listings to those whose amenities array contains ALL listed terms.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: strings.Trim(`
  apartments-pp-cli must-have "in unit washer" --json
  apartments-pp-cli must-have pool gym garage
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			terms := make([]string, 0, len(args))
			for _, a := range args {
				a = strings.ToLower(strings.TrimSpace(a))
				if a != "" {
					terms = append(terms, a)
				}
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
			var matched []apt.Listing
			var withAmenities int
			for _, r := range rows {
				li := r.Data
				if len(li.Amenities) > 0 {
					withAmenities++
				}
				lowered := make([]string, len(li.Amenities))
				for i, a := range li.Amenities {
					lowered[i] = strings.ToLower(a)
				}
				ok := true
				for _, t := range terms {
					hit := false
					for _, a := range lowered {
						if strings.Contains(a, t) {
							hit = true
							break
						}
					}
					if !hit {
						ok = false
						break
					}
				}
				if ok {
					matched = append(matched, li)
				}
			}
			if matched == nil {
				matched = []apt.Listing{}
			}
			// Empty-result envelope: when no listings match, surface why.
			// This prevents an agent from pivoting on '[]' alone.
			if len(matched) == 0 {
				out := map[string]any{
					"terms":                 terms,
					"matches":               matched,
					"cached_listings":       len(rows),
					"cached_with_amenities": withAmenities,
				}
				switch {
				case len(rows) == 0:
					out["hint"] = "no listings cached locally — run 'apartments-pp-cli sync-search <slug> --city <city> --state <st>' first"
				case withAmenities == 0:
					out["hint"] = "cached listings have no amenities populated — apartments.com listing detail pages 403-block; amenities are populated only when 'listing get' succeeds for individual URLs"
				default:
					out["hint"] = "no cached listing has all of these terms in its amenities array; try fewer terms or different wording"
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), matched, flags)
		},
	}
	return cmd
}
