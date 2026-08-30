// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// similar.go implements the `similar` command: given one NCT ID, find other
// trials that share the same condition, phase, and intervention profile using
// only ClinicalTrials.gov registry signals — no external or ML-based matching.
package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// similarMatch is one result row in the similar output.
type similarMatch struct {
	NCTID      string   `json:"id"`
	Title      string   `json:"title,omitempty"`
	Status     string   `json:"status,omitempty"`
	Phase      string   `json:"phase,omitempty"`
	Conditions []string `json:"conditions,omitempty"`
	MatchScore int      `json:"match_score"`
}

// similarView is the JSON shape `similar` emits.
type similarView struct {
	SeedNCTID   string         `json:"seed_id"`
	SeedTitle   string         `json:"seed_title,omitempty"`
	MatchBasis  string         `json:"match_basis"`
	TotalFound  int            `json:"total_found"`
	Matches     []similarMatch `json:"matches"`
}

// scoreMatch computes a simple integer similarity score between a seed Trial
// and a candidate Trial using only in-memory registry fields:
//   - +2 for every shared phase
//   - +1 for every shared condition beyond the primary (already used for the query)
//   - +1 for every shared intervention name token (case-insensitive word overlap)
func scoreMatch(seed, candidate Trial) int {
	score := 0

	// Phase overlap.
	for _, sp := range seed.Phases {
		for _, cp := range candidate.Phases {
			if strings.EqualFold(sp, cp) {
				score += 2
				break
			}
		}
	}

	// Additional condition overlap (first condition drove the query, extras are bonus).
	seedConds := make(map[string]bool, len(seed.Conditions))
	for _, c := range seed.Conditions {
		seedConds[strings.ToLower(c)] = true
	}
	for i, c := range candidate.Conditions {
		if i == 0 {
			continue // already used as the search basis
		}
		if seedConds[strings.ToLower(c)] {
			score++
		}
	}

	// Intervention token overlap: check if any word from a seed intervention
	// appears in any candidate intervention name.
	seedTokens := make(map[string]bool)
	for _, iv := range seed.Interventions {
		for _, tok := range strings.Fields(strings.ToLower(iv)) {
			if len(tok) > 3 { // skip short noise words
				seedTokens[tok] = true
			}
		}
	}
	for _, iv := range candidate.Interventions {
		for _, tok := range strings.Fields(strings.ToLower(iv)) {
			if seedTokens[tok] {
				score++
				break // count each candidate intervention at most once
			}
		}
	}

	return score
}

// pp:data-source live
func newNovelSimilarCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "similar <nct-id>",
		Short: "Find trials similar to a given NCT ID by condition, phase, and intervention profile.",
		Long: "Fetches the seed trial, then queries ClinicalTrials.gov for trials on the same\n" +
			"primary condition. Candidates are scored by shared phases (+2 each), additional\n" +
			"shared conditions (+1 each), and overlapping intervention name tokens (+1 each)\n" +
			"— all from registry signals, no external or ML-based matching.",
		Example: "  clinical-trials-pp-cli similar NCT04280705\n" +
			"  clinical-trials-pp-cli similar NCT04280705 --limit 5 --json",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if limit < 1 {
				return usageErr(fmt.Errorf("--limit must be at least 1"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would find similar trials from ClinicalTrials.gov")
				return nil
			}
			nctID := strings.TrimSpace(args[0])
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			seed, ok, err := ctgovFetchOne(ctx, c, nctID)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if !ok {
				return usageErr(fmt.Errorf("no trial found for %q (check the NCT id)", nctID))
			}

			// Build the search query. Prefer the first condition; fall back to
			// a broad text search on the first word of the title.
			matchBasis := ""
			var params map[string]string
			titleWords := strings.Fields(seed.Title)
			if len(seed.Conditions) > 0 {
				matchBasis = fmt.Sprintf("condition: %s", seed.Conditions[0])
				params = ctgovParams("cond", seed.Conditions[0])
			} else if len(titleWords) > 0 {
				matchBasis = fmt.Sprintf("title keyword: %s (seed has no conditions posted)", titleWords[0])
				params = ctgovParams("term", titleWords[0])
			} else {
				matchBasis = "broad search (seed has no conditions or title)"
				params = ctgovParams("term", "")
			}

			candidates, err := ctgovFetch(ctx, c, params, 200, 1)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Score and rank, excluding the seed itself.
			type scored struct {
				trial Trial
				score int
			}
			var ranked []scored
			for _, t := range candidates {
				if strings.EqualFold(t.NCTID, seed.NCTID) {
					continue
				}
				ranked = append(ranked, scored{t, scoreMatch(seed, t)})
			}
			sort.Slice(ranked, func(i, j int) bool {
				if ranked[i].score != ranked[j].score {
					return ranked[i].score > ranked[j].score
				}
				return ranked[i].trial.NCTID < ranked[j].trial.NCTID
			})

			if len(ranked) > limit {
				ranked = ranked[:limit]
			}

			matches := make([]similarMatch, 0, len(ranked))
			for _, r := range ranked {
				matches = append(matches, similarMatch{
					NCTID:      r.trial.NCTID,
					Title:      r.trial.Title,
					Status:     r.trial.Status,
					Phase:      r.trial.Phase,
					Conditions: r.trial.Conditions,
					MatchScore: r.score,
				})
			}

			view := similarView{
				SeedNCTID:  seed.NCTID,
				SeedTitle:  seed.Title,
				MatchBasis: matchBasis,
				TotalFound: len(ranked),
				Matches:    matches,
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return renderSimilarHuman(cmd, view)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 10, "Maximum number of similar trials to return (min 1)")
	return cmd
}

func renderSimilarHuman(cmd *cobra.Command, v similarView) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Similar trials to %s\n", v.SeedNCTID)
	fmt.Fprintf(w, "  %s\n", truncate(v.SeedTitle, 80))
	fmt.Fprintf(w, "  match basis: %s\n\n", v.MatchBasis)
	if len(v.Matches) == 0 {
		fmt.Fprintln(w, "No similar trials found in this condition area.")
		return nil
	}
	fmt.Fprintf(w, "%-14s %-8s %-18s %s  %s\n", "NCT ID", "SCORE", "STATUS", "PHASE", "TITLE")
	for _, m := range v.Matches {
		fmt.Fprintf(w, "%-14s %-8d %-18s %-10s %s\n",
			m.NCTID, m.MatchScore, truncate(m.Status, 18), truncate(m.Phase, 10), truncate(m.Title, 60))
	}
	return nil
}
