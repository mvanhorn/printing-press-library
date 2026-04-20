// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// coverage: for a given company (name or slug), show who you already know
// there. Crosses the Happenstance graph-search (full 1st + 2nd degree
// network) and LinkedIn search_people scoped to the company name.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/config"
)

func newCoverageCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var sourceFlag string
	var pollTimeoutSec int

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
  contact-goat-pp-cli coverage airbnb --source hp
  contact-goat-pp-cli coverage disney --poll-timeout 300`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			company := args[0]
			sources := parseSourceFlag(sourceFlag)
			ctx, cancel := signalCtx(cmd.Context())
			defer cancel()

			var results []flagshipPerson
			var friends []flagshipPerson
			sourceErrors := map[string]string{}

			// Source 1: Happenstance graph-search. Routed through SelectSource
			// (auto-prefers cookie / free quota; falls back to bearer / paid
			// credits when cookie is unavailable, exhausted, or 429s
			// mid-flight). Explicit --source hp / --source api force the
			// respective surface; --source both fans out cookie+LinkedIn
			// with bearer used only on cookie 429.
			if sources[SourceFlagCookie] || sources[SourceFlagAPI] {
				graphPeople, hpErrs := runCoverageHappenstance(cmd, flags, company, pollTimeoutSec, sources)
				for k, v := range hpErrs {
					sourceErrors[k] = v
				}
				if graphPeople != nil {
					// Fetch friends/list for the top-connector tag — only
					// when the cookie surface is actually live, since the
					// bearer surface has no equivalent endpoint.
					if cookieClient, ferr := flags.newClient(); ferr == nil && cookieClient.HasCookieAuth() {
						if all, fErr := fetchHappenstanceFriends(cookieClient); fErr == nil {
							friends = all
						} else {
							sourceErrors["hp_friends"] = fErr.Error()
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: fetch friends (non-fatal): %v\n", fErr)
						}
					}
					friendsByUUID := map[string]flagshipPerson{}
					for _, f := range friends {
						if f.HappenstanceUUID != "" {
							friendsByUUID[f.HappenstanceUUID] = f
						}
					}
					for _, row := range graphPeople {
						if _, ok := friendsByUUID[row.HappenstanceUUID]; ok {
							row.Sources = append(row.Sources, "hp_friend")
							row.Relationship = "happenstance_friend"
						}
						results = append(results, row)
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
				// Bridge affinity from the bearer API is additive on top of
				// the tier+source composite so a 2nd-degree-via-strong-bridge
				// row can outrank a 2nd-degree-via-weak-bridge row without
				// disturbing the coarse tier ordering. See bridgeAffinityBonus
				// in flagship_helpers.go for the scaling rationale.
				p.Score = scoreForRelationship(p.Relationship) +
					sourceStrength(firstSource(p.Sources)) +
					connectionBonus(p.ConnectionCount) +
					bridgeAffinityBonus(p.Bridges)
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
	cmd.Flags().StringVar(&sourceFlag, "source", "both", "Sources: li | hp | api | both. hp = cookie/free quota; api = bearer/paid credits; both = LinkedIn + auto-routed Happenstance")
	cmd.Flags().IntVar(&pollTimeoutSec, "poll-timeout", 0,
		fmt.Sprintf("Seconds to wait for Happenstance graph-search (0 = use default %ds)",
			int(client.DefaultPollTimeout.Seconds())))
	return cmd
}

// runCoverageHappenstance is the source-aware Happenstance graph-search
// branch of `coverage`. It selects between the cookie surface (free
// quota) and the bearer surface (paid credits) per the SelectSource
// decision tree, runs the chosen surface, and on cookie 429 falls back
// to bearer transparently. Returns:
//
//   - graphPeople: normalized flagshipPerson rows from the chosen surface.
//     Nil when no surface succeeded; the caller treats nil as "skip the
//     friend-overlay step entirely" rather than as an empty result.
//   - errs: per-source error map merged into coverage's source_errors block.
//
// The function takes a *cobra.Command for stderr-routed warnings (so
// output stays consistent with the rest of coverage) and the parsed
// sources map to honor the explicit --source api / --source hp overrides.
func runCoverageHappenstance(cmd *cobra.Command, flags *rootFlags, company string, pollTimeoutSec int, sources map[string]bool) ([]flagshipPerson, map[string]string) {
	errs := map[string]string{}

	// Translate the parsed source flag into the explicit-source string
	// SelectSource expects. Precedence: explicit api > explicit hp >
	// auto-route (when both is set, or only neither is set).
	explicit := ""
	switch {
	case sources[SourceFlagAPI] && !sources[SourceFlagCookie]:
		explicit = SourceFlagAPI
	case sources[SourceFlagCookie] && !sources[SourceFlagAPI]:
		explicit = SourceFlagCookie
	}

	cfg, cfgErr := config.Load(flags.configPath)
	if cfgErr != nil {
		errs["hp"] = cfgErr.Error()
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: load config: %v\n", cfgErr)
		return nil, errs
	}

	// Probe the cookie quota cache best-effort; SelectSource handles
	// UnknownSearchesRemaining gracefully (proceeds cookie-first; the
	// retry wrapper handles 429 fallback).
	cookieClient, _ := flags.newClient()
	cookieAvailable := cookieClient != nil && cookieClient.HasCookieAuth()
	remaining := UnknownSearchesRemaining
	if cookieAvailable {
		remaining = FetchSearchesRemaining(cookieClient, cfg, flags.noCache)
	}

	chosen, deferredErr, hardErr := SelectSource(cmd.Context(), explicit, cfg, cookieAvailable, remaining)
	if hardErr != nil {
		errs["hp"] = hardErr.Error()
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance unavailable: %v\n", hardErr)
		return nil, errs
	}
	LogDeferredHint(cmd.ErrOrStderr(), deferredErr)

	// Cookie runner reuses the existing SearchPeopleByCompanyWithOptions
	// path so the rich /api/dynamo schema (referrers, current user uuid)
	// keeps flowing through downstream renderers unchanged.
	currentUUID := ""
	cookieRun := CookieRunner(nil)
	if cookieAvailable {
		currentUUID, _ = fetchCurrentUserUUID(cookieClient)
		var hpOpts *client.SearchPeopleOptions
		if pollTimeoutSec > 0 {
			hpOpts = &client.SearchPeopleOptions{
				IncludeMyConnections: true,
				IncludeMyFriends:     true,
				PollTimeout:          time.Duration(pollTimeoutSec) * time.Second,
			}
		}
		cookieRun = func() (*client.PeopleSearchResult, error) {
			return cookieClient.SearchPeopleByCompanyWithOptions(company, hpOpts)
		}
	}

	// Bearer runner is constructed only when a key is configured.
	// SelectSource may have already chosen SourceCookie even when an
	// API key is present (free quota first), so we still try to build
	// the bearer client so the cookie-then-bearer fallback wrapper has
	// somewhere to land on a 429.
	var bearerRun BearerRunner
	if bc, berr := flags.newHappenstanceAPIClient(); berr == nil {
		bearerRun = func() (*client.PeopleSearchResult, error) {
			return BearerSearchAdapter(cmd.Context(), bc, "people at "+company, currentUUID, nil)
		}
	}

	out, runErr := ExecuteWithSourceFallback(cmd.Context(), chosen, cookieRun, bearerRun, cmd.ErrOrStderr())
	if runErr != nil {
		errs["hp_graph"] = runErr.Error()
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance graph-search: %v\n", runErr)
		return nil, errs
	}
	if out.Result == nil {
		return nil, errs
	}

	// Project /api/dynamo Person rows (cookie) or normalized bearer rows
	// into flagshipPerson. The graphPersonToFlagship path uses the
	// referrer chain to label tier; the bearer path uses envelope-level
	// mutuals (dereferenced into p.Bridges by the normalizer) to pick a
	// rationale string, score, and bridge list that callers can act on.
	graph := make([]flagshipPerson, 0, len(out.Result.People))
	for _, p := range out.Result.People {
		row := graphPersonToFlagship(p, currentUUID)
		if out.UsedSource == SourceAPI {
			row.Sources = []string{"hp_api"}
			row.Rationale = bearerRationale(p.Bridges)
			row.Score = bearerScore(p.Bridges, p.Score)
			row.Bridges = bridgesToFlagship(p.Bridges)
			// Relationship is a coarse tier label used by downstream
			// ranking. Promote self-graph to "hp_graph_1deg" parity (the
			// person is literally in the user's own synced contacts);
			// friend-bridged rows fall under the API tag so the renderer
			// still shows "source: api" and so mixed cookie+API result
			// sets sort by actual affinity rather than by tag strength.
			if hasSelfGraphBridge(p.Bridges) {
				row.Relationship = string(client.TierFirstDegree)
			} else {
				row.Relationship = "happenstance_api"
			}
		}
		graph = append(graph, row)
	}

	// Sort the bearer slice by affinity-aware score so zero-affinity
	// hits sink below any row with real graph signal. Cookie-only rows
	// keep the default relative order chosen upstream.
	if out.UsedSource == SourceAPI {
		sort.SliceStable(graph, func(i, j int) bool {
			si := graph[i].Score
			sj := graph[j].Score
			if si != sj {
				return si > sj
			}
			return strings.ToLower(graph[i].Name) < strings.ToLower(graph[j].Name)
		})
	}
	return graph, errs
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
