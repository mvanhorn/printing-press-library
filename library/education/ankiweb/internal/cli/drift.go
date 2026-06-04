// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/spf13/cobra"
)

// driftResult is the (currently always empty) drift report. download-count
// drift requires owned-shared-deck data that AnkiWeb's exposed endpoints do
// not return, so the command is honest about the gap rather than fabricating.
type driftResult struct {
	Decks     []any  `json:"decks"`
	Supported bool   `json:"supported"`
	Note      string `json:"note"`
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Track download-count changes on decks you've published (data not currently exposed)",
		Long: `Drift would report how the download counts on the shared decks you published
have changed between syncs. AnkiWeb's available endpoints do not expose
owned-shared-deck download counts, so this command reports the gap honestly and
returns an empty result rather than fabricating data.`,
		Example:     "  ankiweb-pp-cli drift --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			const note = "drift tracking needs owned-shared-deck download data, which AnkiWeb's available endpoints (deck-list-info, shared/*) do not expose. No data was fabricated."

			res := driftResult{Decks: []any{}, Supported: false, Note: note}

			if flags.asJSON || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(note))
			return nil
		},
	}
	return cmd
}
