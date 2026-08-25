// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// safety.go implements the `safety` command: FDA adverse-event signals for a
// drug. The name is first resolved through RxNorm so a brand name folds to its
// ingredient, then FAERS is queried for the most-reported reactions. A drug with
// no FAERS reports returns an honest zero rather than an error.
package cli

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/source"

	"github.com/spf13/cobra"
)

// reactionEntry is one FAERS reaction tally. The source package's reaction type
// is unexported, so safety re-projects it into this local shape for output.
type reactionEntry struct {
	Reaction string `json:"reaction"`
	Count    int    `json:"count"`
}

type safetyView struct {
	Query          string          `json:"query"`
	NormalizedName string          `json:"normalized_name"`
	Resolved       bool            `json:"resolved"`
	SearchedAs     string          `json:"searched_as"`
	TotalReports   int             `json:"total_reports"`
	TopReactions   []reactionEntry `json:"top_reactions,omitempty"`
	Note           string          `json:"note,omitempty"`
}

// pp:data-source live
func newNovelSafetyCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "safety <drug>",
		Short: "Surface FDA adverse-event signals for a trial's intervention drug.",
		Long: "Resolve a drug name through RxNorm (folding brand names to the ingredient),\n" +
			"then query the FDA Adverse Event Reporting System (FAERS) for the total report\n" +
			"count and the most frequently reported reactions.",
		Example: "  clinical-trials-pp-cli safety pembrolizumab --json\n" +
			"  clinical-trials-pp-cli safety Keytruda --limit 15",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would surface FDA adverse-event signals")
				return nil
			}
			query := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			src := flags.sourceClient()

			drug, err := source.Normalize(ctx, src, query)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// FAERS indexes on the medicinal product name; the RxNorm-normalized
			// ingredient is the most reliable FAERS key, falling back to raw query.
			searchAs := drug.Name
			if searchAs == "" {
				searchAs = query
			}
			faers, err := source.AdverseEvents(ctx, src, searchAs, limit, openFDAKey())
			if err != nil {
				return classifyAPIError(err, flags)
			}

			view := safetyView{
				Query:          query,
				NormalizedName: drug.Name,
				Resolved:       drug.Resolved,
				SearchedAs:     searchAs,
				TotalReports:   faers.TotalReports,
			}
			for _, r := range faers.TopReactions {
				view.TopReactions = append(view.TopReactions, reactionEntry{Reaction: r.Reaction, Count: r.Count})
			}
			switch {
			case faers.Note != "":
				view.Note = faers.Note
			case !drug.Resolved:
				view.Note = "drug name could not be resolved via RxNorm; searched FAERS for the literal term"
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if err := printJSONFiltered(cmd.OutOrStdout(), view, flags); err != nil {
					return err
				}
			} else if err := renderSafetyHuman(cmd, view); err != nil {
				return err
			}
			if view.TotalReports == 0 && len(view.TopReactions) == 0 {
				return noResultsErr(query)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Max top reactions to list")
	return cmd
}

func renderSafetyHuman(cmd *cobra.Command, v safetyView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "FDA adverse-event signal — %s (searched as %q)\n", compareSafetyLabel(v), v.SearchedAs)
	fmt.Fprintf(w, "  Total FAERS reports: %d\n\n", v.TotalReports)
	if len(v.TopReactions) > 0 {
		fmt.Fprintln(w, "Top reported reactions:")
		for _, r := range v.TopReactions {
			fmt.Fprintf(w, "  %-44s %d\n", truncate(r.Reaction, 44), r.Count)
		}
	}
	if v.Note != "" {
		fmt.Fprintf(w, "\nnote: %s\n", v.Note)
	}
	return nil
}

func compareSafetyLabel(v safetyView) string {
	if v.NormalizedName != "" {
		return v.NormalizedName
	}
	return v.Query
}
