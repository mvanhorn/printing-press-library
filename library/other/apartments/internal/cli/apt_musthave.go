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
			for _, r := range rows {
				li := r.Data
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
			return printJSONFiltered(cmd.OutOrStdout(), matched, flags)
		},
	}
	return cmd
}
