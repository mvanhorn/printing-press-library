package cli

import (
	"fmt"
	"sort"
	"strings"
)

type issueLabelRef struct {
	ID   string  `json:"id"`
	Name string  `json:"name"`
	Team *refKey `json:"team,omitempty"`
}

func searchIssueLabelsLive(c graphqlQueryer, query, team string) ([]issueLabelRef, error) {
	const gql = `query($first: Int!, $after: String, $filter: IssueLabelFilter) {
		issueLabels(first: $first, after: $after, filter: $filter) {
			nodes {
				id name
				team { id key name }
			}
			pageInfo { hasNextPage endCursor }
		}
	}`
	needle := normalizePortfolioName(query)
	teamNeedle := normalizePortfolioName(team)
	filter := portfolioNameContainsFilter(query)
	var out []issueLabelRef
	var after any
	for {
		var resp struct {
			IssueLabels struct {
				Nodes []struct {
					ID   string  `json:"id"`
					Name string  `json:"name"`
					Team *refKey `json:"team"`
				} `json:"nodes"`
				PageInfo pageInfo `json:"pageInfo"`
			} `json:"issueLabels"`
		}
		vars := map[string]any{"first": 100, "after": after}
		if filter != nil {
			vars["filter"] = filter
		}
		if err := c.QueryInto(gql, vars, &resp); err != nil {
			return nil, err
		}
		for _, label := range resp.IssueLabels.Nodes {
			if !portfolioNameMatches(label.Name, needle) {
				continue
			}
			if !matchingIssueLabelTeam(label.Team, teamNeedle) {
				continue
			}
			out = append(out, issueLabelRef{ID: label.ID, Name: label.Name, Team: label.Team})
		}
		if !resp.IssueLabels.PageInfo.HasNextPage || resp.IssueLabels.PageInfo.EndCursor == "" {
			break
		}
		after = resp.IssueLabels.PageInfo.EndCursor
	}
	sort.SliceStable(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Name, out[j].Name) {
			return out[i].ID < out[j].ID
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func resolveLabelNameForWriteLive(c graphqlQueryer, name, team string, flags *rootFlags) (string, error) {
	if c == nil {
		return "", internalLabelResolveClientErr()
	}
	matches, err := searchIssueLabelsLive(c, name, team)
	if err != nil {
		return "", classifyLiveReadError(err, flags)
	}
	exact := exactLabelMatches(matches, name)
	if len(exact) == 1 {
		return exact[0].ID, nil
	}
	if len(exact) > 1 {
		return "", portfolioResolveErr(flags, "label", name, exact, true)
	}
	return "", portfolioResolveErr(flags, "label", name, matches, false)
}

func resolveLabelNamesForWriteLive(c graphqlQueryer, names []string, team string, flags *rootFlags) ([]string, error) {
	if c == nil {
		return nil, internalLabelResolveClientErr()
	}
	resolved := make([]string, 0, len(names))
	seenNames := make(map[string]bool, len(names))
	seenIDs := make(map[string]bool, len(names))
	for _, name := range names {
		nameKey := normalizePortfolioName(name)
		if seenNames[nameKey] {
			continue
		}
		seenNames[nameKey] = true
		id, err := resolveLabelNameForWriteLive(c, name, team, flags)
		if err != nil {
			return nil, err
		}
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" || seenIDs[key] {
			continue
		}
		seenIDs[key] = true
		resolved = append(resolved, id)
	}
	return resolved, nil
}

func exactLabelMatches(matches []issueLabelRef, name string) []issueLabelRef {
	needle := normalizePortfolioName(name)
	var exact []issueLabelRef
	for _, m := range matches {
		if normalizePortfolioName(m.Name) == needle {
			exact = append(exact, m)
		}
	}
	return exact
}

func matchingIssueLabelTeam(team *refKey, teamNeedle string) bool {
	if teamNeedle == "" {
		return true
	}
	if team == nil || (team.ID == "" && team.Key == "" && team.Name == "") {
		return true
	}
	return normalizePortfolioName(team.ID) == teamNeedle || normalizePortfolioName(team.Key) == teamNeedle || normalizePortfolioName(team.Name) == teamNeedle
}

func mergeLabelIDs(labelIDs, resolvedLabelIDs []string) []string {
	merged := make([]string, 0, len(labelIDs)+len(resolvedLabelIDs))
	seen := make(map[string]bool, len(labelIDs)+len(resolvedLabelIDs))
	for _, id := range append(labelIDs, resolvedLabelIDs...) {
		key := strings.ToLower(strings.TrimSpace(id))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		merged = append(merged, id)
	}
	return merged
}

func internalLabelResolveClientErr() error {
	return fmt.Errorf("internal error: --label-name resolution requires a live Linear client")
}
