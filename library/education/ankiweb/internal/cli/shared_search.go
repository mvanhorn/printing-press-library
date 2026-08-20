// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

func newSharedSearchCmd(flags *rootFlags) *cobra.Command {
	var flagSearch string
	var hasAudio bool
	var hasImages bool

	cmd := &cobra.Command{
		Use:         "search [term]",
		Short:       "Search the shared-deck catalog by keyword",
		Example:     "  ankiweb-pp-cli shared search spanish --has-audio\n  ankiweb-pp-cli shared search --search japanese --json",
		Annotations: map[string]string{"pp:endpoint": "shared.search", "pp:method": "GET", "pp:path": "/svc/shared/list-decks", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			term := flagSearch
			if term == "" && len(args) > 0 {
				term = args[0]
			}
			if term == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "GET %s/svc/shared/list-decks?search=%s\n", "https://ankiweb.net", term)
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []svc.SharedDeck{}, flags)
			}

			decks, err := flags.loadSharedDecks(cmd, term)
			if err != nil {
				return err
			}

			filtered := decks[:0]
			for _, d := range decks {
				if hasAudio && d.Audio <= 0 {
					continue
				}
				if hasImages && d.Images <= 0 {
					continue
				}
				filtered = append(filtered, d)
			}
			return printJSONFiltered(cmd.OutOrStdout(), filtered, flags)
		},
	}
	cmd.Flags().StringVar(&flagSearch, "search", "", "Search term (e.g. spanish, anatomy, MCAT). Also accepted as a positional arg.")
	cmd.Flags().BoolVar(&hasAudio, "has-audio", false, "Keep only decks that include audio")
	cmd.Flags().BoolVar(&hasImages, "has-images", false, "Keep only decks that include images")

	return cmd
}
