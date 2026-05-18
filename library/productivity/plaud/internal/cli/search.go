// Copyright 2026 jnalv414. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newSearchCmd(flags *rootFlags) *cobra.Command {
	var flagSpeaker, flagSince string
	var flagLimit int

	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Full-text search across speaker-diarized transcripts (FTS5)",
		Long: "Search every utterance in your local transcript store using FTS5.\n" +
			"Run `plaud-pp-cli sync` first to populate the store.\n\n" +
			"FTS5 syntax is supported: phrase queries (\"exact phrase\"), prefix (term*),\n" +
			"AND/OR/NOT operators, column filtering.",
		Example: `  plaud-pp-cli search "pricing"
  plaud-pp-cli search "renewal" --speaker Sandra --since 90d
  plaud-pp-cli search '"launch date"' --json --select recording_id,speaker,content`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.TrimSpace(args[0])
			if query == "" {
				return usageErr(fmt.Errorf("search query is required"))
			}
			since, err := parseSinceFlag(flagSince)
			if err != nil {
				return usageErr(err)
			}

			s, err := openPlaudStore(cmd.Context())
			if err != nil {
				return err
			}
			defer s.Close()

			limit := flagLimit
			if limit <= 0 {
				limit = 50
			}

			// Typed-domain search method — store.SearchTranscripts handles the
			// FTS5 MATCH + speaker + since filtering and returns
			// []store.TranscriptSearchResult instead of raw json maps.
			results, err := s.SearchTranscripts(query, flagSpeaker, since, limit)
			if err != nil {
				return apiErr(fmt.Errorf("search query: %w", err))
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&flagSpeaker, "speaker", "", "Restrict to one speaker (partial name match)")
	cmd.Flags().StringVar(&flagSince, "since", "", "Only segments from recordings after this time (e.g. 30d, 12h, ISO)")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Max results")
	return cmd
}
