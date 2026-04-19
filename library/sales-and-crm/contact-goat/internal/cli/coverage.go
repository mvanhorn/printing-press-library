// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// coverage: for a given company (name or slug), show who you already know
// there. Crosses Happenstance friends and LinkedIn search_people scoped to
// the company.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sourceFlag string

	cmd := &cobra.Command{
		Use:   "coverage <company>",
		Short: "Show who you know at a company (LinkedIn + Happenstance)",
		Long: `Cross-source "who do I know at X" query.

Runs the following in parallel (source flag permitting):

  1. Happenstance: filter /api/friends/list by friends whose current
     company / affiliation mentions the target.
  2. LinkedIn: search_people scoped to the company name.

Results are deduped across sources and ranked:
  Happenstance friend  >  LinkedIn 1st-degree  >  LinkedIn search hit.

LinkedIn's MCP does not distinguish 1st-degree vs 2nd-degree in search
results, so every LinkedIn hit is tagged "linkedin_search" unless we have
additional signal (matches a Happenstance friend → upgraded).`,
		Example: `  contact-goat-pp-cli coverage stripe
  contact-goat-pp-cli coverage "OpenAI" --limit 10 --json
  contact-goat-pp-cli coverage airbnb --source hp`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			company := args[0]
			sources := parseSourceFlag(sourceFlag)
			ctx, cancel := signalCtx(cmd.Context())
			defer cancel()

			var results []flagshipPerson
			var friends []flagshipPerson

			// Source 1: Happenstance friends whose company field matches.
			if sources["hp"] {
				c, err := flags.newClientRequireCookies("happenstance")
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance unavailable: %v\n", err)
				} else {
					all, ferr := fetchHappenstanceFriends(c)
					if ferr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetch friends: %v\n", ferr)
					} else {
						friends = all
						for _, f := range all {
							if matchesCompany(f, company) {
								f.Sources = []string{"hp_friend"}
								f.Relationship = "happenstance_friend"
								f.Rationale = fmt.Sprintf("Happenstance friend at %s (%d connections)", f.Company, f.ConnectionCount)
								results = append(results, f)
							}
						}
					}
				}
			}

			// Source 2: LinkedIn search_people scoped to the company.
			if sources["li"] {
				hits, err := fetchLinkedInSearchPeople(ctx, company, "", 25)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: LinkedIn search failed: %v\n", err)
				} else {
					for _, h := range hits {
						h.Sources = []string{"li_search"}
						h.Relationship = "linkedin_search"
						if matchesCompany(h, company) {
							h.Rationale = fmt.Sprintf("LinkedIn search hit at %s", h.Company)
						} else {
							h.Rationale = "LinkedIn search hit"
						}
						results = append(results, h)
					}
				}
			}

			// Upgrade LinkedIn hits that match a Happenstance friend → 1deg proxy.
			results = hydrateMutualHints(results, friends)
			for i := range results {
				p := &results[i]
				if containsSource(p.Sources, "hp_friend") {
					p.Relationship = "happenstance_friend"
				} else if containsSource(p.Sources, "li_1deg") {
					p.Relationship = "linkedin_1deg"
				}
				p.Score = scoreForRelationship(p.Relationship) + sourceStrength(firstSource(p.Sources)) + connectionBonus(p.ConnectionCount)
			}
			results = mergePeople(results)
			rankPeople(results)
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			persistPeople(results)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := map[string]any{
					"company":   company,
					"results":   results,
					"count":     len(results),
					"sources":   sourcesSummary(sources),
					"timestamp": nowISO(),
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return printCoverageTable(cmd, company, results)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max people to return")
	cmd.Flags().StringVar(&sourceFlag, "source", "both", "Sources: li | hp | both")
	return cmd
}

func matchesCompany(p flagshipPerson, query string) bool {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return false
	}
	return strings.Contains(strings.ToLower(p.Company), q) || strings.Contains(strings.ToLower(p.Title), q)
}

func containsSource(tags []string, want string) bool {
	for _, t := range tags {
		if t == want {
			return true
		}
	}
	return false
}

func firstSource(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return tags[0]
}

// scoreForRelationship is a tiered score for the coverage relationship column.
// Ensures Happenstance friends come first in ranking.
func scoreForRelationship(rel string) float64 {
	switch rel {
	case "happenstance_friend":
		return 10.0
	case "linkedin_1deg":
		return 7.0
	case "linkedin_2deg":
		return 3.0
	}
	return 1.0
}

func printCoverageTable(cmd *cobra.Command, company string, people []flagshipPerson) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Coverage at %s — %d known\n\n", company, len(people))
	if len(people) == 0 {
		fmt.Fprintln(w, "nobody known at this company yet. try `prospect` to fan out.")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("RANK")+"\t"+bold("NAME")+"\t"+bold("TITLE")+"\t"+bold("RELATIONSHIP")+"\t"+bold("URL"))
	for i, p := range people {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\n",
			i+1, truncate(p.Name, 32), truncate(p.Title, 32), p.Relationship, truncate(p.LinkedInURL, 60))
	}
	return tw.Flush()
}
