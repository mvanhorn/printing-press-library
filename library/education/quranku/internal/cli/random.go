// Copyright 2026 erikgunawans and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: random verse for reflection.

package cli

import (
	"fmt"
	"math/rand"

	"github.com/spf13/cobra"
)

// pp:data-source auto
func newNovelRandomCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "random",
		Short: "A random verse with its Tafsiriyah translation for reflection.",
		Long: "Return a random verse with its Indonesian Tarjamah Tafsiriyah from the local store. " +
			"Fully offline after the corpus loads.",
		Example:     "  quranku-pp-cli random --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return qkDryRun(cmd, flags, "return a random verse")
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
			// #nosec G404 -- non-cryptographic selection of a Qur'an verse for reflection; math/rand is appropriate here.
			v := verses[rand.Intn(len(verses))]
			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), v, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n%s\n\n%s\n", v.SurahName, v.Key, v.Arabic, v.Tafsiriyah)
			return nil
		},
	}
	return cmd
}
