// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// coverage: for a given company (name or slug), show who you already know
// there. Crosses the Happenstance graph-search (full 1st + 2nd degree
// network) and LinkedIn search_people scoped to the company name.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sourceFlag string

	cmd := &cobra.Command{
		Use:   "coverage <company>",
		Short: "Show who you know at a company (LinkedIn + Happenstance)",
		Long: `Cross-source "who do I know at X" query.

Runs the following in parallel (source flag permitting):

  1. Happenstance graph-search (/api/search + /api/dynamo): the real
     people-search the web app uses. Surfaces your 1st-degree (synced
     LinkedIn / Gmail contacts) and 2nd-degree (your friends' networks)
     hits at the target company, with referrer rationale.
  2. LinkedIn search_people scoped to the company name: a name-match
     fallback that catches people Happenstance doesn't have synced.

Results are deduped across sources and ranked:
  Happenstance 1st-degree  >  Happenstance 2nd-degree  >  LinkedIn 1st-degree  >  LinkedIn search hit.

Use --source hp or --source li to isolate one side. JSON output gains a
"source_errors" block whenever any upstream call errored, so callers can
distinguish "empty because nobody is there" from "empty because the call
failed".`,
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
			sourceErrors := map[string]string{}

			// Source 1: Happenstance graph-search (the real endpoint that
			// sees your synced LinkedIn connections, not the top-connectors
			// widget). Falls back to the friends/list match so top
			// connectors keep their dedicated label even when they also
			// show up in the graph result.
			if sources["hp"] {
				c, err := flags.newClientRequireCookies("happenstance")
				if err != nil {
					sourceErrors["hp"] = err.Error()
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance unavailable: %v\n", err)
				} else {
					// Fetch the friends/list in parallel for the top-connector
					// tag. Best-effort: errors here are logged but don't
					// block the graph-search result set.
					if all, ferr := fetchHappenstanceFriends(c); ferr == nil {
						friends = all
					} else {
						sourceErrors["hp_friends"] = ferr.Error()
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetch friends (non-fatal): %v\n", ferr)
					}

					// Resolve the current user uuid so graph results can be
					// correctly labeled 1st vs 2nd degree.
					currentUUID, _ := fetchCurrentUserUUID(c)

					graphRes, gerr := c.SearchPeopleByCompany(company)
					if gerr != nil {
						sourceErrors["hp_graph"] = gerr.Error()
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance graph-search: %v\n", gerr)
					} else {
						friendsByUUID := map[string]flagshipPerson{}
						for _, f := range friends {
							if f.HappenstanceUUID != "" {
								friendsByUUID[f.HappenstanceUUID] = f
							}
						}
						for _, p := range graphRes.People {
							row := graphPersonToFlagship(p, currentUUID)
							// Upgrade the row if this person is also in the
							// narrow friends/list: they are a direct
							// connector AND a graph hit.
							if _, ok := friendsByUUID[row.HappenstanceUUID]; ok {
								row.Sources = append(row.Sources, "hp_friend")
								row.Relationship = "happenstance_friend"
							}
							results = append(results, row)
						}
					}
				}
			}

			// Source 2: LinkedIn search_people scoped to the company.
			if sources["li"] {
				hits, err := fetchLinkedInSearchPeople(ctx, company, "", 25)
				if err != nil {
					sourceErrors["li_search"] = err.Error()
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
				if len(sourceErrors) > 0 {
					out["source_errors"] = sourceErrors
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if len(sourceErrors) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "\n%d source(s) errored — results may be incomplete:\n", len(sourceErrors))
				for src, msg := range sourceErrors {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", src, msg)
				}
				fmt.Fprintln(cmd.ErrOrStderr())
			}
			return printCoverageTable(cmd, company, results)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 25, "Max people to return")
	cmd.Flags().StringVar(&sourceFlag, "source", "both", "Sources: li | hp | both")
	return cmd
}

// graphPersonToFlagship converts a Happenstance graph-search result
// into the CLI's normalized flagshipPerson shape. It populates sources
// and relationship based on the referrer chain: when the first
// referrer is the current user, the person is 1st-degree; otherwise
// 2nd-degree; empty referrer chain means 3rd-degree (searchEveryone).
func graphPersonToFlagship(p client.Person, currentUserUUID string) flagshipPerson {
	tier := p.Tier(currentUserUUID)
	sourceTag := "hp_graph_2deg"
	switch tier {
	case client.TierFirstDegree:
		sourceTag = "hp_graph_1deg"
	case client.TierThirdDegree:
		sourceTag = "hp_graph_3deg"
	}

	rationale := fmt.Sprintf("Happenstance graph: %s at %s", string(tier), p.CurrentCompany)
	if len(p.Referrers.Referrers) > 0 && tier == client.TierSecondDegree {
		first := p.Referrers.Referrers[0]
		rationale = fmt.Sprintf("2nd-degree via %s at %s", first.Name, p.CurrentCompany)
	}

	return flagshipPerson{
		Name:             p.Name,
		LinkedInURL:      p.LinkedInURL,
		HappenstanceUUID: p.PersonUUID,
		Title:            p.CurrentTitle,
		Company:          p.CurrentCompany,
		Sources:          []string{sourceTag},
		Relationship:     string(tier),
		Rationale:        rationale,
		Score:            p.Score,
	}
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
// Ranking puts Happenstance friends first (a top-connector AND a graph hit),
// then graph 1st-degree (in your synced network), then LinkedIn 1st-degree,
// then graph 2nd-degree (via a friend), then LinkedIn 2nd-degree, then
// LinkedIn search hits, then graph 3rd-degree (public).
func scoreForRelationship(rel string) float64 {
	switch rel {
	case "happenstance_friend":
		return 10.0
	case string(client.TierFirstDegree): // "1st_degree"
		return 8.0
	case "linkedin_1deg":
		return 7.0
	case string(client.TierSecondDegree): // "2nd_degree"
		return 5.0
	case "linkedin_2deg":
		return 3.0
	case string(client.TierThirdDegree): // "3rd_degree"
		return 1.5
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
