// Copyright 2026 David Bryson and contributors. Licensed under Apache-2.0. See LICENSE.
//
// `show` — full place detail by slug or numeric id (hand-authored).
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newShowCmd(flags *rootFlags) *cobra.Command {
	var short bool

	cmd := &cobra.Command{
		Use:   "show <id-or-slug>",
		Short: "Show full detail for an Atlas Obscura place",
		Long: "Show a place's full detail: description, coordinates, address, category tags,\n" +
			"and the \"Know Before You Go\" practical notes. Accepts a slug\n" +
			"(gustave-eiffels-secret-apartment) or a numeric id. Use --short for the lean\n" +
			"JSON record only. Community-sourced from atlasobscura.com; not an official API.",
		Example: "  atlas-obscura-pp-cli show gustave-eiffels-secret-apartment\n" +
			"  atlas-obscura-pp-cli show 27604 --json\n" +
			"  atlas-obscura-pp-cli show winchester-mystery-house --short",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch a place detail page")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a place id or slug is required"))
			}
			idOrSlug := args[0]

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			var place AOPlace
			if short {
				place, err = aoFetchPlaceShort(cmd.Context(), c, idOrSlug)
			} else {
				place, err = aoFetchPlaceFull(cmd.Context(), c, idOrSlug)
				// Fall back to short JSON for coords/id if the HTML parse came up thin.
				if err == nil && (place.ID == 0 || place.Lat == 0) {
					if sp, serr := aoFetchPlaceShort(cmd.Context(), c, idOrSlug); serr == nil {
						if place.ID == 0 {
							place.ID = sp.ID
						}
						if place.Lat == 0 && place.Lng == 0 {
							place.Lat, place.Lng = sp.Lat, sp.Lng
						}
						if place.Title == "" {
							place.Title = sp.Title
						}
						if place.Subtitle == "" {
							place.Subtitle = sp.Subtitle
						}
						if place.Location == "" {
							place.Location = sp.Location
						}
					}
				}
			}
			if err != nil {
				return classifyAPIError(err, flags)
			}
			place.Score = aoScore(place)

			if s, derr := aoDB(cmd.Context()); derr == nil {
				cachePlace(s, place)
				_ = s.Close()
			}

			env := map[string]any{"source": aoSourceNote, "place": place}
			return aoEmit(cmd, flags, env)
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "Fetch only the compact JSON record (1 request, no description/categories/KBYG)")
	return cmd
}
