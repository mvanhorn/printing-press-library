// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: deterministic date-seeded verse of the day.

package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelDailyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "A stable, date-seeded verse of the day with its Tafsiriyah translation.",
		Long: "Return a verse of the day. The pick is deterministic for a given date (seeded by the number " +
			"of days since the Unix epoch), so every run on the same day yields the same verse. Fully offline.",
		Example:     "  quranku-pp-cli daily --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "return the verse of the day")
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
			verses, err := qkAllVerses(s)
			if err != nil {
				return err
			}
			if len(verses) == 0 {
				return fmt.Errorf("no verses in the local store")
			}
			daySeed := time.Now().UTC().Unix() / 86400
			v := verses[int(daySeed%int64(len(verses)))]
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), v, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Ayat hari ini — %s (%s)\n%s\n\n%s\n", v.SurahName, v.Key, v.Arabic, v.Tafsiriyah)
			return nil
		},
	}
	return cmd
}
