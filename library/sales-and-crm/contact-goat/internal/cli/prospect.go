// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// prospect: fan-out search across LinkedIn + Happenstance + (optionally)
// Deepline. Budget-gated so agents don't accidentally spend credits.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/happenstance/api"
)

func newProspectCmd(flags *rootFlags) *cobra.Command {
	var (
		budget      int
		useDeepline bool
		limit       int
		sortMode    string
		deeplineKey string
		sourceFlag  string
	)

	cmd := &cobra.Command{
		Use:   "prospect <query>",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short: "Fan-out search across LinkedIn, Happenstance, and (opt-in) Deepline",
		Long: `Run a single prospect query across every source simultaneously. Budget-gated:
Deepline credits are only spent when --deepline is set AND --budget allows.

Query supports two shapes:

  1. Free-form string (passed verbatim to LinkedIn search_people).
     Example: "VP engineering fintech"

  2. Key=value pairs separated by commas.
     Recognized keys: title, location, industry, company.
     Example: "title=VP,location=SF,industry=fintech"

Results are deduped across sources and ranked by one of:

  --sort relevance  (default): source strength + Happenstance boost
  --sort network:              Happenstance friend overlap wins ties`,
		Example: `  contact-goat-pp-cli prospect "VP engineering fintech"
  contact-goat-pp-cli prospect "title=VP,location=SF,industry=fintech" --limit 50
  contact-goat-pp-cli prospect "Director Product" --deepline --budget 5 --yes
  contact-goat-pp-cli prospect "CTO" --json --sort network`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]
			parsed := parseProspectQuery(query)
			ctx, cancel := signalCtx(cmd.Context())
			defer cancel()

			var results []flagshipPerson
			var friends []flagshipPerson

			// Step A: LinkedIn (free).
			keywords := parsed["freeform"]
			if keywords == "" {
				parts := []string{}
				for _, k := range []string{"title", "industry", "company"} {
					if v := parsed[k]; v != "" {
						parts = append(parts, v)
					}
				}
				keywords = strings.TrimSpace(strings.Join(parts, " "))
			}
			if keywords == "" {
				return usageErr(fmt.Errorf("prospect: unable to derive keywords from %q; provide a phrase or title=...", query))
			}
			liLimit := limit
			if liLimit <= 0 {
				liLimit = 25
			}
			liHits, err := fetchLinkedInSearchPeople(ctx, keywords, parsed["location"], liLimit)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: LinkedIn search failed: %v\n", err)
			} else {
				for _, p := range liHits {
					p.Sources = []string{"li_search"}
					results = append(results, p)
				}
			}

			// Step B: Happenstance. Routed via SelectSource:
			//   - SourceCookie (explicit --source hp, or auto with quota): walk
			//     the cached friends list (free, local).
			//   - SourceAPI (explicit --source api, or auto when cookie is
			//     unavailable / quota exhausted): run a bearer people-search
			//     against the parsed keywords (paid: 2 credits per search).
			cfg, cfgErr := config.Load(flags.configPath)
			if cfgErr != nil {
				return configErr(cfgErr)
			}
			cookieClient, _ := flags.newClient()
			cookieAvailable := cookieClient != nil && cookieClient.HasCookieAuth()
			remaining := UnknownSearchesRemaining
			if cookieAvailable {
				remaining = FetchSearchesRemaining(cookieClient, cfg, flags.noCache)
			}
			chosen, deferredErr, hardErr := SelectSource(cmd.Context(), sourceFlag, cfg, cookieAvailable, remaining)
			if hardErr != nil {
				// Non-fatal here: prospect still has LinkedIn results to show.
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance unavailable: %v\n", hardErr)
			} else {
				LogDeferredHint(cmd.ErrOrStderr(), deferredErr)
				if chosen == SourceCookie {
					if cookieAvailable {
						friends, err = fetchHappenstanceFriends(cookieClient)
						if err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetch friends: %v\n", err)
						} else {
							for _, f := range friends {
								if prospectMatches(f, parsed) {
									f.Sources = []string{"hp_friend"}
									f.Rationale = fmt.Sprintf("Happenstance friend match (%d connections)", f.ConnectionCount)
									results = append(results, f)
								}
							}
						}
					}
				} else if chosen == SourceAPI {
					if bc, berr := flags.newHappenstanceAPIClient(); berr == nil {
						currentUUID := ""
						if cookieAvailable {
							currentUUID, _ = fetchCurrentUserUUID(cookieClient)
						}
						bres, brerr := BearerSearchAdapter(cmd.Context(), bc, keywords, currentUUID, &api.SearchOptions{IncludeMyConnections: true, IncludeFriendsConnections: true})
						if brerr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance bearer search: %v\n", brerr)
						} else if bres != nil {
							for _, p := range bres.People {
								row := flagshipPerson{
									Name:      p.Name,
									Title:     p.CurrentTitle,
									Company:   p.CurrentCompany,
									Sources:   []string{"hp_api"},
									Rationale: bearerRationale(p.Bridges),
									Score:     bearerScore(p.Bridges, p.Score),
									Bridges:   bridgesToFlagship(p.Bridges),
								}
								results = append(results, row)
							}
						}
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance bearer client: %v\n", berr)
					}
				}
			}

			// Step C: Deepline (credit-priced, opt-in).
			deeplineSpend := 0
			if useDeepline {
				if budget <= 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: --deepline set but --budget=%d; Deepline skipped\n", budget)
				} else {
					key, _ := resolveDeeplineKey(deeplineKey)
					if key == "" {
						fmt.Fprintln(cmd.ErrOrStderr(), "warning: no Deepline API key (set DEEPLINE_API_KEY or pass --deepline-key); Deepline skipped")
					} else if !flags.yes {
						fmt.Fprintln(cmd.ErrOrStderr(), "warning: --deepline requires --yes to spend credits; Deepline skipped")
					} else {
						payload := map[string]any{}
						if v := parsed["title"]; v != "" {
							payload["title"] = v
						}
						if v := parsed["location"]; v != "" {
							payload["location"] = v
						}
						if v := parsed["industry"]; v != "" {
							payload["industry"] = v
						}
						if len(payload) == 0 && keywords != "" {
							payload["title"] = keywords
						}
						if limit > 0 {
							payload["limit"] = limit
						}
						raw, cost, err := deeplineApolloSearch(ctx, key, payload)
						if err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: Deepline apollo-people-search failed: %v\n", err)
						} else {
							deeplineSpend = cost
							budget -= cost
							if budget < 0 {
								fmt.Fprintf(cmd.ErrOrStderr(), "warning: Deepline spent %d credit(s), exceeded budget\n", cost)
							}
							for _, p := range parseDeeplinePeople(raw) {
								p.Sources = []string{"dl_apollo"}
								p.Rationale = "Deepline Apollo match"
								results = append(results, p)
							}
						}
					}
				}
			}

			results = hydrateMutualHints(results, friends)
			results = mergePeople(results)
			for i := range results {
				p := &results[i]
				score := 0.0
				for _, tag := range p.Sources {
					score += sourceStrength(tag)
				}
				score += connectionBonus(p.ConnectionCount)
				if sortMode == "network" && containsSource(p.Sources, "hp_friend") {
					score += 10.0
				}
				if ms := matchStrength(*p, parsed); ms > 0 {
					score += ms
				}
				if p.Rationale == "" {
					p.Rationale = describeSources(p.Sources, p.ConnectionCount)
				}
				p.Score = score
			}
			rankPeople(results)
			if limit > 0 && len(results) > limit {
				results = results[:limit]
			}

			persistPeople(results)

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := map[string]any{
					"query":   query,
					"parsed":  parsed,
					"results": results,
					"count":   len(results),
					"meta": map[string]any{
						"deepline_spent_credits": deeplineSpend,
						"budget_remaining":       budget,
						"sort":                   sortMode,
						"timestamp":              nowISO(),
					},
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			return printProspectTable(cmd, query, results, deeplineSpend)
		},
	}
	cmd.Flags().IntVar(&budget, "budget", 0, "Credit ceiling for Deepline (default 0 = never hit Deepline)")
	cmd.Flags().BoolVar(&useDeepline, "deepline", false, "Also query Deepline Apollo (requires --budget > 0 and --yes)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max results to return")
	cmd.Flags().StringVar(&sortMode, "sort", "relevance", "Sort mode: relevance | network")
	cmd.Flags().StringVar(&deeplineKey, "deepline-key", "", "Deepline API key (default from $DEEPLINE_API_KEY)")
	cmd.Flags().StringVar(&sourceFlag, "source", SourceFlagAuto, "Happenstance auth surface: auto | hp | api. auto = cookie/free quota first, bearer fallback")
	return cmd
}

// parseProspectQuery accepts either a freeform phrase or a key=value CSV.
// Returns a map with keys title, location, industry, company, freeform.
func parseProspectQuery(q string) map[string]string {
	out := map[string]string{}
	q = strings.TrimSpace(q)
	if strings.Contains(q, "=") && strings.Contains(q, ",") {
		for _, tok := range strings.Split(q, ",") {
			kv := strings.SplitN(tok, "=", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.ToLower(strings.TrimSpace(kv[0]))
			v := strings.TrimSpace(kv[1])
			if k == "" || v == "" {
				continue
			}
			out[k] = v
		}
		return out
	}
	if strings.Contains(q, "=") && !strings.Contains(q, " ") {
		// Single key=value pair.
		kv := strings.SplitN(q, "=", 2)
		out[strings.ToLower(strings.TrimSpace(kv[0]))] = strings.TrimSpace(kv[1])
		return out
	}
	out["freeform"] = q
	return out
}

// prospectMatches returns true when a flagshipPerson satisfies the parsed query.
// A freeform phrase matches if the phrase appears in title/company/name.
// Key=value pairs apply their own substring match to the corresponding field.
func prospectMatches(p flagshipPerson, parsed map[string]string) bool {
	if v := parsed["freeform"]; v != "" {
		vl := strings.ToLower(v)
		if strings.Contains(strings.ToLower(p.Title), vl) ||
			strings.Contains(strings.ToLower(p.Company), vl) ||
			strings.Contains(strings.ToLower(p.Name), vl) {
			return true
		}
		return false
	}
	if v := parsed["title"]; v != "" && !strings.Contains(strings.ToLower(p.Title), strings.ToLower(v)) {
		return false
	}
	if v := parsed["company"]; v != "" && !strings.Contains(strings.ToLower(p.Company), strings.ToLower(v)) {
		return false
	}
	if v := parsed["location"]; v != "" && !strings.Contains(strings.ToLower(p.Location), strings.ToLower(v)) {
		return false
	}
	// industry has no obvious field on a friend record; accept if nothing else filtered.
	return true
}

// matchStrength gives a small bonus when fields in parsed match the person.
func matchStrength(p flagshipPerson, parsed map[string]string) float64 {
	score := 0.0
	if v := parsed["title"]; v != "" && strings.Contains(strings.ToLower(p.Title), strings.ToLower(v)) {
		score += 2.0
	}
	if v := parsed["location"]; v != "" && strings.Contains(strings.ToLower(p.Location), strings.ToLower(v)) {
		score += 1.5
	}
	if v := parsed["company"]; v != "" && strings.Contains(strings.ToLower(p.Company), strings.ToLower(v)) {
		score += 1.5
	}
	return score
}

func printProspectTable(cmd *cobra.Command, query string, results []flagshipPerson, creditsSpent int) error {
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Prospect: %s — %d results", query, len(results))
	if creditsSpent > 0 {
		fmt.Fprintf(w, " (Deepline spent %d credit(s))", creditsSpent)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	if len(results) == 0 {
		fmt.Fprintln(w, "no matches. try a broader phrase or add --deepline with --budget.")
		return nil
	}
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("RANK")+"\t"+bold("NAME")+"\t"+bold("TITLE")+"\t"+bold("COMPANY")+"\t"+bold("SOURCES")+"\t"+bold("RATIONALE"))
	for i, p := range results {
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			i+1, truncate(p.Name, 32), truncate(p.Title, 28), truncate(p.Company, 24),
			strings.Join(p.Sources, ","), truncate(p.Rationale, 60))
	}
	return tw.Flush()
}
