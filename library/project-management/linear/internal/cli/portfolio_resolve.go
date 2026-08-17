package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
	"github.com/spf13/cobra"
)

type portfolioProjectRef struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	URL        string   `json:"url,omitempty"`
	State      string   `json:"state,omitempty"`
	Team       refKey   `json:"team,omitempty"`
	Initiative refName  `json:"initiative,omitempty"`
	Teams      []refKey `json:"-"`
}

type portfolioInitiativeRef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	URL    string `json:"url,omitempty"`
	Status string `json:"status,omitempty"`
}

type refKey struct {
	ID   string `json:"id,omitempty"`
	Key  string `json:"key,omitempty"`
	Name string `json:"name,omitempty"`
}

type refName struct {
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

type pageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

func newPortfolioLookupClient(flags *rootFlags) (*client.Client, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	// Mutation dry-runs still need live reads so previews can show the UUID
	// that Linear would receive without sending the write mutation.
	c.DryRun = false
	return c, nil
}

func newProjectsListCmd(flags *rootFlags) *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List Linear projects",
		Example: `  linear-pp-cli projects list --agent --select id,name,team.key,state,url
  linear-pp-cli projects list --team SYMPH --agent --select id,name,team.key,initiative.name,url`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, err := searchProjectsLive(c, "", team)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key, name, or UUID")
	return cmd
}

func newProjectsSearchCmd(flags *rootFlags) *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Linear projects by name",
		Example: `  linear-pp-cli projects search "Autonomous Backlog Manager & Dispatch Governance" --team SYMPH --agent --select id,name,team.key,url
  linear-pp-cli projects resolve "Autonomous Backlog Manager & Dispatch Governance" --team SYMPH --agent`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, err := searchProjectsLive(c, strings.Join(args, " "), team)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key, name, or UUID")
	return cmd
}

func newProjectsResolveCmd(flags *rootFlags) *cobra.Command {
	var team string
	cmd := &cobra.Command{
		Use:   "resolve <name>",
		Short: "Resolve one Linear project name to its UUID",
		Example: `  linear-pp-cli projects resolve "Autonomous Backlog Manager & Dispatch Governance" --team SYMPH --agent --select id,name,url
  linear-pp-cli issues edit SYMPH-795 --project-name "Autonomous Backlog Manager & Dispatch Governance" --dry-run --agent`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			project, err := resolveProjectNameLive(c, strings.Join(args, " "), team, flags)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), project, flags)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key, name, or UUID")
	return cmd
}

func newInitiativesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List Linear initiatives",
		Example: `  linear-pp-cli initiatives list --agent --select id,name,status,url`,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, err := searchInitiativesLive(c, "")
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	return cmd
}

func newInitiativesSearchCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search Linear initiatives by name",
		Example: `  linear-pp-cli initiatives search "Backlog Governance" --agent --select id,name,status,url
  linear-pp-cli initiatives resolve "Backlog Governance" --agent`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			results, err := searchInitiativesLive(c, strings.Join(args, " "))
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), results, flags)
		},
	}
	return cmd
}

func newInitiativesResolveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resolve <name>",
		Short:   "Resolve one Linear initiative name to its UUID",
		Example: `  linear-pp-cli initiatives resolve "Backlog Governance" --agent --select id,name,status,url`,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			initiative, err := resolveInitiativeNameLive(c, strings.Join(args, " "), flags)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), initiative, flags)
		},
	}
	return cmd
}

func searchProjectsLive(c graphqlQueryer, query, team string) ([]portfolioProjectRef, error) {
	const gql = `query($first: Int!, $after: String, $filter: ProjectFilter) {
		projects(first: $first, after: $after, filter: $filter) {
			nodes {
				id name state url
				teams { nodes { id key name } }
				initiatives(first: 5) { nodes { id name } }
			}
			pageInfo { hasNextPage endCursor }
		}
	}`
	needle := normalizePortfolioName(query)
	teamNeedle := normalizePortfolioName(team)
	filter := portfolioNameContainsFilter(query)
	var out []portfolioProjectRef
	var after any
	for {
		var resp struct {
			Projects struct {
				Nodes []struct {
					ID    string `json:"id"`
					Name  string `json:"name"`
					State string `json:"state"`
					URL   string `json:"url"`
					Teams struct {
						Nodes []refKey `json:"nodes"`
					} `json:"teams"`
					Initiatives struct {
						Nodes []refName `json:"nodes"`
					} `json:"initiatives"`
				} `json:"nodes"`
				PageInfo pageInfo `json:"pageInfo"`
			} `json:"projects"`
		}
		vars := map[string]any{"first": 100, "after": after}
		if filter != nil {
			vars["filter"] = filter
		}
		if err := c.QueryInto(gql, vars, &resp); err != nil {
			return nil, err
		}
		for _, p := range resp.Projects.Nodes {
			if !portfolioNameMatches(p.Name, needle) {
				continue
			}
			teamRef, ok := matchingProjectTeam(p.Teams.Nodes, teamNeedle)
			if !ok {
				continue
			}
			ref := portfolioProjectRef{ID: p.ID, Name: p.Name, State: p.State, URL: p.URL, Team: teamRef, Teams: p.Teams.Nodes}
			if len(p.Initiatives.Nodes) > 0 {
				ref.Initiative = p.Initiatives.Nodes[0]
			}
			out = append(out, ref)
		}
		if !resp.Projects.PageInfo.HasNextPage || resp.Projects.PageInfo.EndCursor == "" {
			break
		}
		after = resp.Projects.PageInfo.EndCursor
	}
	sort.SliceStable(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Name, out[j].Name) {
			return out[i].ID < out[j].ID
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func searchInitiativesLive(c graphqlQueryer, query string) ([]portfolioInitiativeRef, error) {
	const gql = `query($first: Int!, $after: String, $filter: InitiativeFilter) {
		initiatives(first: $first, after: $after, filter: $filter) {
			nodes {
				id name status url
			}
			pageInfo { hasNextPage endCursor }
		}
	}`
	needle := normalizePortfolioName(query)
	filter := portfolioNameContainsFilter(query)
	var out []portfolioInitiativeRef
	var after any
	for {
		var resp struct {
			Initiatives struct {
				Nodes []struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					Status string `json:"status"`
					URL    string `json:"url"`
				} `json:"nodes"`
				PageInfo pageInfo `json:"pageInfo"`
			} `json:"initiatives"`
		}
		vars := map[string]any{"first": 100, "after": after}
		if filter != nil {
			vars["filter"] = filter
		}
		if err := c.QueryInto(gql, vars, &resp); err != nil {
			return nil, err
		}
		for _, initiative := range resp.Initiatives.Nodes {
			if !portfolioNameMatches(initiative.Name, needle) {
				continue
			}
			ref := portfolioInitiativeRef{ID: initiative.ID, Name: initiative.Name, Status: initiative.Status, URL: initiative.URL}
			out = append(out, ref)
		}
		if !resp.Initiatives.PageInfo.HasNextPage || resp.Initiatives.PageInfo.EndCursor == "" {
			break
		}
		after = resp.Initiatives.PageInfo.EndCursor
	}
	sort.SliceStable(out, func(i, j int) bool {
		if strings.EqualFold(out[i].Name, out[j].Name) {
			return out[i].ID < out[j].ID
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func resolveProjectNameLive(c graphqlQueryer, name, team string, flags *rootFlags) (portfolioProjectRef, error) {
	matches, err := searchProjectsLive(c, name, team)
	if err != nil {
		return portfolioProjectRef{}, classifyLiveReadError(err, flags)
	}
	exact := exactProjectMatches(matches, name)
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return portfolioProjectRef{}, portfolioResolveErr(flags, "project", name, exact, true)
	}
	return portfolioProjectRef{}, portfolioResolveErr(flags, "project", name, matches, false)
}

func resolveProjectNameForWriteLive(c graphqlQueryer, name, preferredTeam string, flags *rootFlags) (portfolioProjectRef, error) {
	matches, err := searchProjectsLive(c, name, "")
	if err != nil {
		return portfolioProjectRef{}, classifyLiveReadError(err, flags)
	}
	exact := exactProjectMatches(matches, name)
	if preferredTeam != "" {
		teamExact := filterProjectRefsByTeam(exact, preferredTeam)
		if len(teamExact) == 1 {
			return teamExact[0], nil
		}
		if len(teamExact) > 1 {
			return portfolioProjectRef{}, portfolioResolveErr(flags, "project", name, teamExact, true)
		}
		teamCandidates := filterProjectRefsByTeam(matches, preferredTeam)
		if len(teamCandidates) > 0 {
			return portfolioProjectRef{}, portfolioTeamResolveErr(flags, "project", name, preferredTeam, teamCandidates)
		}
		candidates := exact
		if len(candidates) == 0 {
			candidates = matches
		}
		return portfolioProjectRef{}, portfolioTeamResolveErr(flags, "project", name, preferredTeam, candidates)
	}
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return portfolioProjectRef{}, portfolioResolveErr(flags, "project", name, exact, true)
	}
	return portfolioProjectRef{}, portfolioResolveErr(flags, "project", name, matches, false)
}

func resolveInitiativeNameLive(c graphqlQueryer, name string, flags *rootFlags) (portfolioInitiativeRef, error) {
	matches, err := searchInitiativesLive(c, name)
	if err != nil {
		return portfolioInitiativeRef{}, classifyLiveReadError(err, flags)
	}
	exact := exactInitiativeMatches(matches, name)
	if len(exact) == 1 {
		return exact[0], nil
	}
	if len(exact) > 1 {
		return portfolioInitiativeRef{}, portfolioResolveErr(flags, "initiative", name, exact, true)
	}
	return portfolioInitiativeRef{}, portfolioResolveErr(flags, "initiative", name, matches, false)
}

func resolveProjectFlag(c graphqlQueryer, projectID, projectName, team string, flags *rootFlags) (string, error) {
	if projectID != "" && projectName != "" {
		return "", usageErr(fmt.Errorf("pass either --project <uuid> or --project-name <name>, not both"))
	}
	if projectID != "" {
		if !store.IsUUID(projectID) {
			return "", portfolioUUIDUsageErr(flags, "--project", projectID, "use --project-name to resolve a project by name")
		}
		return projectID, nil
	}
	if projectName == "" {
		return "", nil
	}
	if c == nil {
		return "", fmt.Errorf("internal error: --project-name resolution requires a live Linear client")
	}
	project, err := resolveProjectNameForWriteLive(c, projectName, team, flags)
	if err != nil {
		return "", err
	}
	return project.ID, nil
}

func portfolioUUIDUsageErr(flags *rootFlags, flag, value, hint string) error {
	err := usageErr(fmt.Errorf("%s expects a UUID, got %q; %s", flag, value, hint))
	if flags != nil && flags.asJSON {
		flags.errorWritten = true
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"error": err.Error(),
			"code":  2,
			"type":  "usage",
			"flag":  flag,
			"value": value,
			"hint":  hint,
		})
	}
	return err
}

func portfolioResolveErr[T any](flags *rootFlags, kind, name string, candidates []T, ambiguous bool) error {
	message := fmt.Sprintf("%s %q not found", kind, name)
	if ambiguous {
		message = fmt.Sprintf("%s %q is ambiguous", kind, name)
	}
	err := usageErr(fmt.Errorf("%s", message))
	if flags != nil && flags.asJSON {
		flags.errorWritten = true
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"error":      message,
			"code":       2,
			"type":       "usage",
			"kind":       kind,
			"name":       name,
			"candidates": candidates,
		})
	}
	return err
}

func portfolioTeamResolveErr[T any](flags *rootFlags, kind, name, team string, candidates []T) error {
	message := fmt.Sprintf("%s %q not found in team %s", kind, name, team)
	err := usageErr(fmt.Errorf("%s", message))
	if flags != nil && flags.asJSON {
		flags.errorWritten = true
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"error":      message,
			"code":       2,
			"type":       "usage",
			"kind":       kind,
			"name":       name,
			"team":       team,
			"reason":     "not_found_in_team",
			"candidates": candidates,
		})
	}
	return err
}

func exactProjectMatches(matches []portfolioProjectRef, name string) []portfolioProjectRef {
	needle := normalizePortfolioName(name)
	var exact []portfolioProjectRef
	for _, m := range matches {
		if normalizePortfolioName(m.Name) == needle {
			exact = append(exact, m)
		}
	}
	return exact
}

func exactInitiativeMatches(matches []portfolioInitiativeRef, name string) []portfolioInitiativeRef {
	needle := normalizePortfolioName(name)
	var exact []portfolioInitiativeRef
	for _, m := range matches {
		if normalizePortfolioName(m.Name) == needle {
			exact = append(exact, m)
		}
	}
	return exact
}

func filterProjectRefsByTeam(matches []portfolioProjectRef, team string) []portfolioProjectRef {
	teamNeedle := normalizePortfolioName(team)
	var filtered []portfolioProjectRef
	for _, m := range matches {
		if teamRef, ok := matchingProjectTeam(m.Teams, teamNeedle); ok {
			m.Team = teamRef
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func matchingProjectTeam(teams []refKey, teamNeedle string) (refKey, bool) {
	if teamNeedle == "" {
		if len(teams) == 0 {
			return refKey{}, true
		}
		return teams[0], true
	}
	for _, team := range teams {
		if normalizePortfolioName(team.ID) == teamNeedle || normalizePortfolioName(team.Key) == teamNeedle || normalizePortfolioName(team.Name) == teamNeedle {
			return team, true
		}
	}
	return refKey{}, false
}

func portfolioNameMatches(name, needle string) bool {
	if needle == "" {
		return true
	}
	return strings.Contains(normalizePortfolioName(name), needle)
}

func normalizePortfolioName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

func portfolioNameContainsFilter(query string) map[string]any {
	terms := strings.Fields(normalizePortfolioName(query))
	if len(terms) == 0 {
		return nil
	}
	if len(terms) == 1 {
		return portfolioNameTermFilter(terms[0])
	}
	filters := make([]map[string]any, 0, len(terms))
	for _, term := range terms {
		filters = append(filters, portfolioNameTermFilter(term))
	}
	return map[string]any{"and": filters}
}

func portfolioNameTermFilter(term string) map[string]any {
	return map[string]any{
		"name": map[string]any{
			"containsIgnoreCase": term,
		},
	}
}
