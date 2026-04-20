// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// warm-intro: given a target (LinkedIn URL or "first last" name), find
// mutual connections across LinkedIn 1st-degree AND Happenstance friends
// who could introduce you.
//
// LinkedIn MCP does not expose 1st-degree connection lists as a first-class
// call, so this implementation leans on:
//
//   1. The user's Happenstance /api/friends/list — every HP friend is
//      someone the user has already vetted as an introducer.
//   2. LinkedIn `get_sidebar` ("People also viewed") for the target —
//      sidebar entries are not guaranteed-warm, but they are the closest
//      1-hop-ish signal the MCP exposes.
//   3. An optional fan-out `search_people` seeded by the target's company
//      so the user can see LinkedIn employees at the same firm who might
//      vouch.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/client"
)

func newWarmIntroCmd(flags *rootFlags) *cobra.Command {
	var targetType, sourcesFlag string
	var limit int
	var useCached bool

	cmd := &cobra.Command{
		Use:   "warm-intro <target>",
		Short: "Find who in your network can intro you to a target (cross-source)",
		Long: `Find mutual connections across LinkedIn 1st-degree and Happenstance friends
who could introduce you to a target person.

The target can be a LinkedIn URL, a LinkedIn slug, or a "First Last" name.
When a name is given the CLI resolves it via LinkedIn search_people first.

Because LinkedIn's MCP does not expose 1st-degree connection lists, this
command composes warm-intro candidates from three signals:

  1. Happenstance friends you share with the target (strongest signal).
  2. LinkedIn "People also viewed" sidebar for the target.
  3. LinkedIn search scoped to the target's current company.

Results are ranked by a composite score (source strength + Happenstance
connection_count + presence in multiple sources).`,
		Example: `  # Name form — resolves via LinkedIn search_people
  contact-goat-pp-cli warm-intro "Patrick Collison"

  # LinkedIn URL form
  contact-goat-pp-cli warm-intro https://www.linkedin.com/in/satyanadella/

  # JSON output, top 5
  contact-goat-pp-cli warm-intro williamhgates --json --limit 5

  # Only Happenstance friends (LinkedIn disabled)
  contact-goat-pp-cli warm-intro "Brian Chesky" --sources hp`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			sources := parseSourceFlag(sourcesFlag)
			ctx, cancel := signalCtx(cmd.Context())
			defer cancel()

			// Resolve the target.
			resolved, rawLI, err := resolveWarmIntroTarget(ctx, target, targetType)
			if err != nil {
				return err
			}

			var candidates []flagshipPerson
			var friends []flagshipPerson
			sourceErrors := map[string]string{}

			// Source 1: Happenstance graph-search at the target's current
			// company. For each person who comes back:
			//   - 1st-degree hit: the person IS the warm-intro candidate
			//     (you already know them directly).
			//   - 2nd-degree hit: the referrer (your friend who knows them)
			//     is the candidate; they can vouch.
			// We fall through to the old friends-list dump ONLY when the
			// target's company is unknown, so the command degrades
			// gracefully when profile enrichment failed.
			if sources["hp"] {
				c, ferr := flags.newClientRequireCookies("happenstance")
				if ferr != nil {
					sourceErrors["hp"] = ferr.Error()
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance unavailable (%v). Skipping hp source.\n", ferr)
				} else {
					if all, fErr := fetchHappenstanceFriends(c); fErr == nil {
						friends = all
					}
					currentUUID, _ := fetchCurrentUserUUID(c)

					if resolved.Company != "" {
						graphRes, gerr := c.SearchPeopleByCompany(resolved.Company)
						if gerr != nil {
							sourceErrors["hp_graph"] = gerr.Error()
							fmt.Fprintf(cmd.ErrOrStderr(), "warning: Happenstance graph-search for %q: %v\n", resolved.Company, gerr)
						} else {
							candidates = append(candidates, warmIntroCandidatesFromGraph(graphRes.People, currentUUID, resolved)...)
						}
					} else {
						// Fallback: no company data means we can't run a
						// targeted graph-search. Dump HP friends with a
						// weaker, clearly-labeled rationale so the user
						// knows this was the weak-fallback path.
						for _, f := range friends {
							f.Sources = []string{"hp_friend"}
							f.Rationale = fmt.Sprintf("Happenstance friend (weak signal — no target company to match against) (%d connections)", f.ConnectionCount)
							candidates = append(candidates, f)
						}
					}
				}
			}

			// Source 2: LinkedIn sidebar (People Also Viewed).
			if sources["li"] && resolved.LinkedInURL != "" {
				sidebar, serr := fetchLinkedInSidebar(ctx, resolved.LinkedInURL)
				if serr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: LinkedIn sidebar failed: %v\n", serr)
				} else {
					for _, s := range sidebar {
						s.Sources = []string{"li_sidebar"}
						s.Rationale = "LinkedIn \"People also viewed\" for target"
						candidates = append(candidates, s)
					}
				}
			}

			// Source 3: LinkedIn search scoped to target's company.
			if sources["li"] && resolved.Company != "" {
				sameCompany, err := fetchLinkedInSearchPeople(ctx, resolved.Company, "", 25)
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: LinkedIn company search failed: %v\n", err)
				} else {
					for _, p := range sameCompany {
						p.Sources = []string{"li_search"}
						p.Rationale = fmt.Sprintf("LinkedIn employee at %s", resolved.Company)
						candidates = append(candidates, p)
					}
				}
			}

			// Overlap bonus + target filtering + merge.
			candidates = hydrateMutualHints(candidates, friends)
			candidates = filterOutTarget(candidates, resolved)
			candidates = mergePeople(candidates)

			for i := range candidates {
				p := &candidates[i]
				score := 0.0
				for _, tag := range p.Sources {
					score += sourceStrength(tag)
				}
				score += connectionBonus(p.ConnectionCount)
				if len(p.Sources) > 1 {
					score += 2.0 * float64(len(p.Sources)-1)
					p.MutualCount = len(p.Sources)
				}
				p.Score = score
				if p.Rationale == "" || len(p.Sources) > 1 {
					p.Rationale = describeSources(p.Sources, p.ConnectionCount)
				}
			}
			rankPeople(candidates)
			if limit > 0 && len(candidates) > limit {
				candidates = candidates[:limit]
			}

			if useCached {
				persistPeople(candidates)
				persistPeople([]flagshipPerson{resolved})
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				out := map[string]any{
					"target":     resolved,
					"candidates": candidates,
					"meta": map[string]any{
						"sources":      sourcesSummary(sources),
						"candidate_ct": len(candidates),
						"timestamp":    nowISO(),
					},
				}
				if len(sourceErrors) > 0 {
					out["source_errors"] = sourceErrors
				}
				if !flags.compact && len(rawLI) > 0 {
					out["target_linkedin_raw"] = json.RawMessage(rawLI)
				}
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}
			if len(sourceErrors) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "\n%d source(s) errored — candidates may be incomplete:\n", len(sourceErrors))
				for src, msg := range sourceErrors {
					fmt.Fprintf(cmd.ErrOrStderr(), "  %s: %s\n", src, msg)
				}
				fmt.Fprintln(cmd.ErrOrStderr())
			}
			return printWarmIntroTable(cmd, resolved, candidates)
		},
	}
	cmd.Flags().StringVar(&targetType, "target-type", "auto", "Target parse mode: auto | url | name")
	cmd.Flags().IntVar(&limit, "limit", 10, "Max candidates to return")
	cmd.Flags().BoolVar(&useCached, "use-cached", true, "Persist resolved people into the local store for re-use")
	cmd.Flags().StringVar(&sourcesFlag, "sources", "li,hp", "Comma-separated sources to query: li,hp")
	return cmd
}

// resolveWarmIntroTarget resolves a raw argument into a flagshipPerson and
// the LinkedIn profile payload (when fetched). It obeys --target-type to
// override the URL-vs-name heuristic.
func resolveWarmIntroTarget(ctx context.Context, target, mode string) (flagshipPerson, json.RawMessage, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	looksLikeURL := strings.Contains(target, "linkedin.com") || strings.HasPrefix(target, "http")
	useURL := mode == "url" || (mode == "auto" && looksLikeURL)
	useName := mode == "name" || (mode == "auto" && !looksLikeURL && !isSlugLike(target))

	if useURL || (mode == "auto" && isSlugLike(target)) {
		liURL := target
		if !strings.Contains(liURL, "linkedin.com") {
			liURL = "https://www.linkedin.com/in/" + strings.TrimPrefix(target, "@") + "/"
		}
		p, raw, err := fetchLinkedInPerson(ctx, liURL, []string{"experience"})
		if err != nil {
			// Fall back to a synthetic record so we can still fan out by name.
			return flagshipPerson{Name: target, LinkedInURL: liURL, Sources: []string{"input"}}, nil, nil
		}
		return *p, raw, nil
	}

	if useName {
		matches, err := fetchLinkedInSearchPeople(ctx, target, "", 1)
		if err != nil || len(matches) == 0 {
			// Final fallback: synthesize a target with just the name.
			return flagshipPerson{Name: target, Sources: []string{"input"}}, nil, nil
		}
		p := matches[0]
		p.Sources = []string{"input"}
		// Enrich with full profile.
		if p.LinkedInURL != "" {
			if prof, raw, err := fetchLinkedInPerson(ctx, p.LinkedInURL, []string{"experience"}); err == nil {
				prof.Sources = []string{"input"}
				return *prof, raw, nil
			}
		}
		return p, nil, nil
	}

	// Unknown mode — treat as name.
	return flagshipPerson{Name: target, Sources: []string{"input"}}, nil, nil
}

// isSlugLike returns true for bare LinkedIn slugs (alphanumeric + hyphen).
func isSlugLike(s string) bool {
	if s == "" || strings.ContainsAny(s, " \t\n") {
		return false
	}
	if strings.Contains(s, ".") || strings.Contains(s, "/") {
		return false
	}
	return true
}

// warmIntroCandidatesFromGraph turns Happenstance graph-search results
// into concrete warm-intro candidates. The rule:
//
//	1st-degree match -> that person is a candidate (you already know them).
//	2nd-degree match -> the referrer (the friend of yours who knows the
//	                    match) is the candidate, since they can introduce you.
//
// 3rd-degree matches (no referrer chain back to the current user) are
// filtered out — those are public hits with no path.
//
// Each candidate carries a rationale naming the concrete bridge
// (shared employer + the person you share, or directly the target's
// co-worker you already know).
func warmIntroCandidatesFromGraph(people []client.Person, currentUserUUID string, target flagshipPerson) []flagshipPerson {
	out := []flagshipPerson{}
	seenReferrer := map[string]bool{}
	for _, p := range people {
		tier := p.Tier(currentUserUUID)
		switch tier {
		case client.TierFirstDegree:
			// You know this person directly. They are the introducer.
			row := flagshipPerson{
				Name:             p.Name,
				LinkedInURL:      p.LinkedInURL,
				HappenstanceUUID: p.PersonUUID,
				Title:            p.CurrentTitle,
				Company:          p.CurrentCompany,
				Sources:          []string{"hp_graph_1deg"},
				Relationship:     string(tier),
				Rationale:        fmt.Sprintf("Your 1st-degree contact at %s — ask them directly", p.CurrentCompany),
			}
			out = append(out, row)
		case client.TierSecondDegree:
			// The referrer is the warm-intro candidate. Deduplicate by
			// referrer uuid so one friend who knows several target-company
			// people only surfaces once.
			for _, r := range p.Referrers.Referrers {
				if r.ID == "" || r.ID == currentUserUUID {
					continue
				}
				if seenReferrer[r.ID] {
					continue
				}
				seenReferrer[r.ID] = true
				row := flagshipPerson{
					Name:             r.Name,
					HappenstanceUUID: r.ID,
					ImageURL:         r.ImageURL,
					Sources:          []string{"hp_graph_2deg"},
					Relationship:     "intro_via_friend",
					Rationale: fmt.Sprintf(
						"Your friend — knows %s at %s (affinity: %s)",
						p.Name, p.CurrentCompany, fallback(r.AffinityLevel, "unknown"),
					),
				}
				out = append(out, row)
			}
		}
	}
	_ = target // target name isn't used yet; reserved for future dedup pass
	return out
}

func fallback(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func filterOutTarget(list []flagshipPerson, target flagshipPerson) []flagshipPerson {
	out := make([]flagshipPerson, 0, len(list))
	tKey := target.dedupKey()
	for _, p := range list {
		if p.dedupKey() == tKey {
			continue
		}
		out = append(out, p)
	}
	return out
}

// parseSourceFlag takes a CSV like "li,hp" and returns a lookup map.
func parseSourceFlag(csv string) map[string]bool {
	out := map[string]bool{}
	for _, tok := range strings.Split(csv, ",") {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if tok == "" {
			continue
		}
		out[tok] = true
	}
	if len(out) == 0 {
		out["li"] = true
		out["hp"] = true
	}
	return out
}

func sourcesSummary(in map[string]bool) []string {
	out := []string{}
	for k := range in {
		out = append(out, k)
	}
	return out
}

// connectionBonus converts a raw connection_count into a log-ish score.
// 100 → ~2, 10k → ~4, 1M → ~6.
func connectionBonus(n int) float64 {
	if n <= 1 {
		return 0
	}
	return math.Log10(float64(n))
}

func describeSources(tags []string, connectionCount int) string {
	hp := false
	li1 := false
	liSidebar := false
	liSearch := false
	hpGraph1 := false
	hpGraph2 := false
	for _, t := range tags {
		switch t {
		case "hp_friend":
			hp = true
		case "li_1deg":
			li1 = true
		case "li_sidebar":
			liSidebar = true
		case "li_search":
			liSearch = true
		case "hp_graph_1deg":
			hpGraph1 = true
		case "hp_graph_2deg":
			hpGraph2 = true
		}
	}
	parts := []string{}
	if hpGraph1 {
		parts = append(parts, "your direct contact at target company")
	}
	if hpGraph2 {
		parts = append(parts, "knows someone at target company")
	}
	if hp {
		if connectionCount > 0 {
			parts = append(parts, fmt.Sprintf("Happenstance friend (%d connections)", connectionCount))
		} else {
			parts = append(parts, "Happenstance friend")
		}
	}
	if li1 {
		parts = append(parts, "LinkedIn 1st-degree")
	}
	if liSidebar {
		parts = append(parts, "LinkedIn sidebar")
	}
	if liSearch {
		parts = append(parts, "LinkedIn company match")
	}
	if len(parts) == 0 {
		return strings.Join(tags, ", ")
	}
	return strings.Join(parts, "; ")
}

func printWarmIntroTable(cmd *cobra.Command, target flagshipPerson, candidates []flagshipPerson) error {
	w := cmd.OutOrStdout()
	headline := target.Name
	if target.Company != "" {
		headline = fmt.Sprintf("%s (%s)", target.Name, target.Company)
	}
	fmt.Fprintf(w, "Warm-intro candidates for %s\n", headline)
	if target.LinkedInURL != "" {
		fmt.Fprintf(w, "  target: %s\n", target.LinkedInURL)
	}
	if len(candidates) == 0 {
		fmt.Fprintln(w, "\nno warm-intro candidates found. try --sources li,hp or widen your network.")
		return nil
	}
	fmt.Fprintln(w)
	tw := newTabWriter(w)
	fmt.Fprintln(tw, bold("RANK")+"\t"+bold("NAME")+"\t"+bold("TITLE")+"\t"+bold("COMPANY")+"\t"+bold("SOURCES")+"\t"+bold("RATIONALE"))
	for i, p := range candidates {
		sources := strings.Join(p.Sources, ",")
		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\n",
			i+1, truncate(p.Name, 36), truncate(p.Title, 28), truncate(p.Company, 24),
			truncate(sources, 24), truncate(p.Rationale, 60))
	}
	return tw.Flush()
}
