// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: offline full-text search over the local Arabic + Tafsiriyah corpus.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelFindCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "find <query>",
		Short: "Search the Qur'an offline by word or meaning across Arabic and the Indonesian Tafsiriyah.",
		Long: "Search the local corpus (Arabic text and the Indonesian Tarjamah Tafsiriyah) with SQLite " +
			"full-text search. The corpus loads automatically on first use. Works fully offline afterward.",
		Example:     "  quranku-pp-cli find \"kasih sayang\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "search the local Qur'an corpus")
			}
			if len(args) == 0 {
				return qkInputError(cmd, flags, "a search query is required")
			}
			query := args[0]
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			s, err := qkOpenStore(ctx)
			if err != nil {
				return err
			}
			defer s.Close()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if _, err := qkEnsureCorpus(ctx, c, s, cmd.ErrOrStderr(), false); err != nil {
				return fmt.Errorf("loading corpus: %w", err)
			}

			rows, err := s.Search(query, limit, "verse")
			if err != nil {
				return fmt.Errorf("searching: %w", err)
			}
			results := make([]qkVerse, 0, len(rows))
			for _, r := range rows {
				var v qkVerse
				if json.Unmarshal(r, &v) == nil {
					results = append(results, v)
				}
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no verses matched %q\n", query)
				return nil
			}
			for _, v := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n  %s\n", v.Key, v.Arabic, v.Tafsiriyah)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "maximum matching verses to return")
	return cmd
}
