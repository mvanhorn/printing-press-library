// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// The live /recommendations payload is a single envelope of category groups,
// NOT a flat list of issued, timestamped recommendations:
//
//	{ "type": "recommendations",
//	  "recommendations": [
//	    { "categoryName": "supplements", "displayName": "Supplements",
//	      "recommendations": [
//	        { "name": "Omega-3 fatty acids", "Biomarkers": ["86009431", ...] } ] } ] }
//
// There is no id, no issued-date, and no title/body — only a name and the
// Quest biomarker codes the recommendation targets. So "stale" cannot mean
// "issued N rounds ago and ignored". It means: the recommendation's target
// biomarker(s) are STILL outside Function's optimal range — guidance the data
// says hasn't paid off yet. --min-rounds tightens this to persistence: the
// target has been out-of-optimal in at least N of the most recent rounds.
type recGroup struct {
	CategoryName string `json:"categoryName"`
	DisplayName  string `json:"displayName"`
	Items        []struct {
		Name       string            `json:"name"`
		Biomarkers []json.RawMessage `json:"Biomarkers"`
	} `json:"recommendations"`
}

type recEnvelope struct {
	Recommendations []recGroup `json:"recommendations"`
}

func newNovelRecommendationsStaleCmd(flags *rootFlags) *cobra.Command {
	var minRounds int
	var dbPath string
	var group string
	var limit int

	cmd := &cobra.Command{
		Use:         "stale",
		Short:       "Recommendations whose target biomarker is STILL outside Function-optimal range",
		Long:        "Joins each recommendation against the latest result for the biomarker(s) it targets (by Quest code) and surfaces the ones whose targets remain out of Function-optimal range — guidance the data says hasn't worked yet.\n\nThe /recommendations endpoint carries no issued-date, so staleness is defined by outcome (target biomarker still out of range), not by age. --min-rounds tightens it to persistence: the target has been out-of-optimal in at least N of the most recent rounds. Filter by --group (e.g. supplements, foods_to_eat, foods_to_avoid) to focus a large set.",
		Example:     "  function-health-pp-cli recommendations stale\n  function-health-pp-cli recommendations stale --group supplements --json\n  function-health-pp-cli recommendations stale --min-rounds 2",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if minRounds < 1 {
				minRounds = 1
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			s, err := openLocalStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer safeCloseStore(s)

			// Recommendations are stored as a single envelope (resource_type
			// 'recommendations', see sync.go). Read and parse the nested groups.
			var env recEnvelope
			{
				row := s.DB().QueryRowContext(ctx, `SELECT data FROM resources WHERE resource_type = 'recommendations' LIMIT 1`)
				var raw []byte
				if err := row.Scan(&raw); err != nil {
					return notFoundErr(fmt.Errorf("no synced recommendations; run `function-health-pp-cli sync --resources recommendations` first"))
				}
				if err := json.Unmarshal(raw, &env); err != nil || len(env.Recommendations) == 0 {
					return notFoundErr(fmt.Errorf("no synced recommendations; run `function-health-pp-cli sync --resources recommendations` first"))
				}
			}

			results, err := loadAllResults(ctx, s)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				return noStoreData("recommendations stale")
			}

			// Build Quest-code -> chronological result series (oldest..newest).
			byQuest := map[string][]resultRow{}
			for _, r := range results {
				if r.QuestCode != "" {
					byQuest[r.QuestCode] = append(byQuest[r.QuestCode], r)
				}
			}
			for k := range byQuest {
				series := byQuest[k]
				sort.Slice(series, func(i, j int) bool { return series[i].DrawDate < series[j].DrawDate })
				byQuest[k] = series
			}

			type staleRec struct {
				Recommendation string   `json:"recommendation"`
				Group          string   `json:"group"`
				Biomarkers     []string `json:"out_of_optimal_biomarkers"`
				RoundsOut      int      `json:"rounds_out_of_optimal"`
			}
			var list []staleRec
			groupFilter := strings.ToLower(strings.TrimSpace(group))
			for _, g := range env.Recommendations {
				if groupFilter != "" &&
					!strings.Contains(strings.ToLower(g.CategoryName), groupFilter) &&
					!strings.Contains(strings.ToLower(g.DisplayName), groupFilter) {
					continue
				}
				for _, it := range g.Items {
					var outNames []string
					maxRoundsOut := 0
					for _, code := range it.Biomarkers {
						series := byQuest[questCodeString(code)]
						if len(series) == 0 {
							continue
						}
						ro := trailingRoundsOutOfOptimal(series)
						if ro >= minRounds {
							latest := series[len(series)-1]
							name := latest.BiomarkerName
							if name == "" {
								name = questCodeString(code)
							}
							outNames = append(outNames, name)
							if ro > maxRoundsOut {
								maxRoundsOut = ro
							}
						}
					}
					if len(outNames) == 0 {
						continue
					}
					sort.Strings(outNames)
					label := g.DisplayName
					if label == "" {
						label = g.CategoryName
					}
					list = append(list, staleRec{
						Recommendation: it.Name,
						Group:          label,
						Biomarkers:     dedupeStrings(outNames),
						RoundsOut:      maxRoundsOut,
					})
				}
			}

			if len(list) == 0 {
				if flags != nil && flags.asJSON {
					return flags.printJSON(cmd, []staleRec{})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no stale recommendations — every recommendation's target biomarker is in Function-optimal range")
				return nil
			}
			// Most-persistent first, then most target biomarkers.
			sort.Slice(list, func(i, j int) bool {
				if list[i].RoundsOut != list[j].RoundsOut {
					return list[i].RoundsOut > list[j].RoundsOut
				}
				return len(list[i].Biomarkers) > len(list[j].Biomarkers)
			})
			if limit > 0 && len(list) > limit {
				list = list[:limit]
			}
			if flags != nil && flags.asJSON {
				return flags.printJSON(cmd, list)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintln(w, "Stale recommendations (target biomarker still outside Function-optimal range):")
			for _, e := range list {
				fmt.Fprintf(w, "  [%s] %s\n", e.Group, e.Recommendation)
				fmt.Fprintf(w, "    still out (>=%d rounds): %s\n", minRounds, truncate(strings.Join(e.Biomarkers, ", "), 100))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&minRounds, "min-rounds", 1, "Flag a recommendation only when a target biomarker has been out-of-optimal in at least N of the most recent rounds")
	cmd.Flags().StringVar(&group, "group", "", "Only this recommendation group (e.g. supplements, foods_to_eat, foods_to_avoid)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Cap the number of recommendations returned (0 = all)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Override local database path")
	return cmd
}

// trailingRoundsOutOfOptimal counts how many of the most recent consecutive
// draws (newest backward) are outside Function's optimal range. Draws without
// a defined optimal range break the streak (inconclusive, not "out"). Returns
// 0 when the latest draw is in range or unclassifiable.
func trailingRoundsOutOfOptimal(series []resultRow) int {
	n := 0
	for i := len(series) - 1; i >= 0; i-- {
		r := series[i]
		if !hasOptimal(r) {
			break
		}
		if optimalSign(r) == 0 {
			break
		}
		n++
	}
	return n
}

// questCodeString normalizes a Biomarkers entry, which the API encodes as
// either a JSON string ("86009431") or a number (86009431).
func questCodeString(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	s = strings.Trim(s, `"`)
	return s
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	out := in[:0:0]
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
