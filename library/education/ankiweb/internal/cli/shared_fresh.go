// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"sort"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

// freshDeck augments a SharedDeck with a human-readable modified date.
type freshDeck struct {
	svc.SharedDeck
	ModifiedDate string `json:"modified_date"`
}

func newSharedFreshCmd(flags *rootFlags) *cobra.Command {
	var since string

	cmd := &cobra.Command{
		Use:         "fresh [term]",
		Short:       "List shared decks sorted by last-modified date (most recent first)",
		Example:     "  ankiweb-pp-cli shared fresh spanish --since 2024-01-01",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			term := args[0]
			sinceTS, err := parseSinceDate(since)
			if err != nil {
				return usageErr(err)
			}
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []freshDeck{}, flags)
			}

			decks, err := flags.loadSharedDecks(cmd, term)
			if err != nil {
				return err
			}

			kept := decks[:0]
			for _, d := range decks {
				if sinceTS > 0 && d.Modified < sinceTS {
					continue
				}
				kept = append(kept, d)
			}
			sort.SliceStable(kept, func(i, j int) bool {
				return kept[i].Modified > kept[j].Modified
			})

			out := make([]freshDeck, 0, len(kept))
			for _, d := range kept {
				out = append(out, freshDeck{SharedDeck: d, ModifiedDate: modifiedDate(d.Modified)})
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only show decks modified on or after this date (YYYY-MM-DD)")
	return cmd
}
