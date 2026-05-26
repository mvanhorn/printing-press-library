// Copyright 2026 quantumglitch. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
	moba "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/mobalytics-lol/internal/mobalytics"
)

// newDuoFinderCmd ranks synergy WRs for an ADC against a candidate
// support pool — coaches' pools are fixed, so the candidate filter
// matters more than the global "best support" answer.
func newDuoFinderCmd(flags *rootFlags) *cobra.Command {
	var bot, supportsFrom string
	cmd := &cobra.Command{
		Use:         "duo-finder",
		Short:       "Best support pairings for a given ADC, restricted to a candidate pool.",
		Example:     `  mobalytics-lol-pp-cli duo-finder --bot jinx --supports-from nautilus,leona,thresh,lulu`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if bot == "" || supportsFrom == "" {
				return fmt.Errorf("both --bot and --supports-from are required")
			}
			if dryRunOK(flags) {
				return nil
			}
			pool := splitCSV(supportsFrom)
			poolSet := map[string]bool{}
			for _, p := range pool {
				poolSet[p] = true
			}
			client := moba.NewClient(flags.timeout)
			html, err := client.Fetch(moba.ChampionPath(bot, "build"))
			if err != nil {
				return err
			}
			syn := moba.ParseSynergies(html, bot)
			out := []moba.SynergyRow{}
			for _, s := range syn {
				if poolSet[s.PartnerSlug] {
					out = append(out, s)
				}
			}
			sort.SliceStable(out, func(i, j int) bool { return out[i].WinRate > out[j].WinRate })
			// Mobalytics's per-champion synergy block lists a fixed top-N
			// partners; supports outside that set produce no row. Emit a
			// stderr hint so an empty result is distinguishable from a
			// broken extractor.
			if len(out) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "duo-finder: 0 of %d candidate supports appear in Mobalytics's synergy block for %s (fetched %d synergies total). Try a wider pool or check 'mobalytics-lol-pp-cli champion synergies %s' to see who is listed.\n", len(pool), bot, len(syn), bot)
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&bot, "bot", "", "ADC slug (required).")
	cmd.Flags().StringVar(&supportsFrom, "supports-from", "", "CSV of candidate support slugs (required).")
	return cmd
}
