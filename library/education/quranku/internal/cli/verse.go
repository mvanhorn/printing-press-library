// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: cross-source single-verse lookup (Arabic + Tafsiriyah).

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelVerseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "verse <surah:verse>",
		Short: "Get a single verse (e.g. 2:255) with Arabic and the Tafsiriyah translation joined together.",
		Long: "Look up one verse by its reference (surah:verse, e.g. 2:255) and return the Arabic text " +
			"and the Indonesian Tarjamah Tafsiriyah together from the local store.",
		Example:     "  quranku-pp-cli verse 2:255 --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "look up a verse")
			}
			if len(args) == 0 {
				return qkInputError(cmd, flags, "a verse reference (surah:verse, e.g. 2:255) is required")
			}
			surah, verse, ok := qkParseRef(args[0])
			if !ok {
				return qkRefError(cmd, flags, args[0])
			}
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

			v, err := qkGetVerse(s, surah, verse)
			if err != nil {
				return err
			}
			if v == nil {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "null")
				}
				return usageErr(fmt.Errorf("verse %d:%d not found (surah has fewer verses?)", surah, verse))
			}
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), v, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s) — ayat %d\n%s\n\n%s\n", v.SurahName, v.Key, v.Verse, v.Arabic, v.Tafsiriyah)
			return nil
		},
	}
	return cmd
}
