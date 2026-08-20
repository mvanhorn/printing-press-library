// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source auto

package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/research"
	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"

	"github.com/spf13/cobra"
)

func newNovelResearchDriftCmd(flags *rootFlags) *cobra.Command {
	rf := &researchFlags{}
	var saveBaseline bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift [seed]",
		Short: "Compare a niche against a saved baseline to see what actually moved since last time.",
		Long: strings.Trim(`
Compare a niche against a saved baseline and report what actually moved.

EverBee exposes no history at all, so week-over-week movement is only knowable
from snapshots kept locally. Run with --save-baseline to record the current
verdict, then re-run later to see the deltas in demand, competition, opportunity,
and median price. Both fetch timestamps and the comparison window travel with the
result, so the period being compared is never ambiguous.

With no saved baseline, this reports that plainly and tells you how to save one —
it does not invent a zero-valued comparison.

Use this command to compare a niche against a saved baseline over time. For a
fresh point-in-time verdict, use 'research niche' instead.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli research drift "dad shirt" --save-baseline --agent
  everbee-pp-cli research drift "dad shirt" --agent
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "seed=dad shirt",
			// A nonsense seed is a valid search that finds nothing, not an error:
			// EverBee answers 200 and the command returns an honest zero-evidence
			// result with exit 0. Telling bad input apart from a valid empty result
			// would mean inventing API semantics EverBee does not have.
			"pp:no-error-path-probe": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would score the niche and diff it against the saved baseline")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a seed keyword is required, e.g. research drift \"dad shirt\""))
			}
			if err := rf.validate(); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			if dbPath == "" {
				dbPath = defaultDBPath("everbee-pp-cli")
			}
			db, err := store.OpenWithContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store at %s: %w", dbPath, err)
			}
			defer func() { _ = db.Close() }()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			seed := strings.Join(args, " ")
			verdict, err := research.Niche(ctx, c, rf.options(seed))
			if err != nil {
				return wrapResearchErr(err)
			}

			if saveBaseline {
				if err := research.SaveBaseline(ctx, db.DB(), verdict); err != nil {
					return err
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "saved baseline for %q\n", seed)
			}

			baseline, err := research.LoadBaseline(ctx, db.DB(), seed)
			if errors.Is(err, research.ErrNoBaseline) {
				fmt.Fprintf(cmd.ErrOrStderr(),
					"no baseline saved for %q yet\nsave one with: everbee-pp-cli research drift %q --save-baseline\n",
					seed, seed)
				// An absent baseline is a first-run state, not an error. Emit a
				// well-formed no-baseline result so an agent can branch on it.
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"seed":         seed,
					"baseline":     nil,
					"current":      verdict,
					"has_baseline": false,
					"warnings": []string{
						fmt.Sprintf("no baseline saved for %q; re-run with --save-baseline to start tracking it.", seed),
					},
				}, flags)
			}
			if err != nil {
				return err
			}

			drift := research.DiffBaseline(baseline, verdict)
			return printJSONFiltered(cmd.OutOrStdout(), drift, flags)
		},
	}
	cmd.Flags().StringVar(&rf.productType, "product", "", "Constrain evidence to a product type: physical, digital, or apparel (default: any)")
	cmd.Flags().BoolVar(&rf.excludeSVGPNG, "exclude-svg-png", false, "Exclude digital cut-file listings (SVG/PNG/Cricut) from the evidence count")
	cmd.Flags().Float64Var(&rf.minRelevance, "min-relevance", research.DefaultMinRelevance, "Relevance floor (0-1) a row must clear to count as evidence. Rows below it are still returned and annotated, never dropped.")
	cmd.Flags().IntVar(&rf.perPage, "limit", 20, "Number of listings to fetch as product evidence")
	cmd.Flags().BoolVar(&saveBaseline, "save-baseline", false, "Record the current verdict as the baseline for future drift runs")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's data directory)")
	return cmd
}
