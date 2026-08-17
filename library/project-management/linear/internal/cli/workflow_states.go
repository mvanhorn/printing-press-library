package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// workflowStateRow is the projection rendered by `workflow-states list`. It
// mirrors the node shape WorkflowStatesQuery syncs into the local store, so
// live and local reads produce identical output fields.
type workflowStateRow struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Color    string  `json:"color,omitempty"`
	Position float64 `json:"position"`
	Team     struct {
		ID   string `json:"id"`
		Key  string `json:"key"`
		Name string `json:"name"`
	} `json:"team"`
}

var validLinearWorkflowStateTypes = map[string]struct{}{
	"triage":    {},
	"backlog":   {},
	"unstarted": {},
	"started":   {},
	"completed": {},
	"canceled":  {},
	"duplicate": {},
}

const validLinearWorkflowStateTypeList = "triage, backlog, unstarted, started, completed, canceled, duplicate"

func normalizeWorkflowStateType(stateType string) (string, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(stateType))
	if _, ok := validLinearWorkflowStateTypes[normalizedType]; !ok {
		return "", usageErr(fmt.Errorf("--state-type %q is not a valid Linear workflow state type; valid types: %s", stateType, validLinearWorkflowStateTypeList))
	}
	return normalizedType, nil
}

func newWorkflowStatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "workflow-states",
		Aliases:     []string{"states"},
		Short:       "List Linear workflow states (the UUIDs 'issues edit --state' needs)",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowStatesListCmd(flags))
	return cmd
}

func newWorkflowStatesListCmd(flags *rootFlags) *cobra.Command {
	var team, dbPath string
	cmd := &cobra.Command{
		Use:         "list",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Short:       "List workflow states, optionally filtered by team",
		Long: `List workflow states with their UUIDs, names, and types.

Use this before 'issues edit <issue> --state <state-uuid>' to find the UUID for
a target state. --team accepts a team key (e.g. SYMPH) or a team UUID.

With --data-source live (or auto, the default), states are fetched from the
Linear GraphQL API. With --data-source local, states are read from the synced
workflow_states table; run 'linear-pp-cli sync' first.`,
		Example: `  linear-pp-cli workflow-states list --team SYMPH --agent --select id,name,type
  linear-pp-cli states list --team SYMPH --agent
  linear-pp-cli workflow-states list --agent --select id,name,type,team.key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkflowStatesList(cmd, flags, resolveDBPath(dbPath), team)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key or UUID")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func runWorkflowStatesList(cmd *cobra.Command, flags *rootFlags, dbPath, team string) error {
	var raw []json.RawMessage
	var prov DataProvenance

	db, openErr := openStoreAt(dbPath)
	if db != nil {
		defer db.Close()
	}

	fetchLocal := func() ([]json.RawMessage, error) {
		if openErr != nil {
			return nil, fmt.Errorf("opening local database: %w\nRun 'linear-pp-cli sync' first", openErr)
		}
		if db == nil {
			return nil, fmt.Errorf("no local data. Run 'linear-pp-cli sync' first")
		}
		teamID := team
		if team != "" && !store.IsUUID(team) {
			var err error
			teamID, err = resolveTeamFilter(db, team)
			if err != nil {
				return nil, err
			}
		}
		return db.ListWorkflowStates(teamID)
	}

	switch flags.dataSource {
	case "local":
		rows, err := fetchLocal()
		if err != nil {
			return err
		}
		raw = rows
		prov = localProvenance(db, "workflow_states", "user_requested")
	default: // live and auto: live first, auto falls back to local on network error
		rows, err := fetchWorkflowStatesLive(flags, team)
		if err != nil {
			if flags.dataSource == "live" || !isNetworkError(err) {
				return classifyLiveReadError(err, flags)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "  (live API unreachable — falling back to local store)")
			rows, err = fetchLocal()
			if err != nil {
				return err
			}
			raw = rows
			prov = localProvenance(db, "workflow_states", "api_unreachable")
			break
		}
		raw = rows
		prov = DataProvenance{Source: "live", ResourceType: "workflow_states", Reason: "user_requested"}
		// Write-through so a follow-up --data-source local read is fresh.
		if db != nil {
			for _, n := range rows {
				var meta struct {
					ID string `json:"id"`
				}
				if json.Unmarshal(n, &meta) == nil && meta.ID != "" {
					_ = db.UpsertWorkflowState(meta.ID, n)
				}
			}
		}
	}

	rows := make([]workflowStateRow, 0, len(raw))
	for _, r := range raw {
		var row workflowStateRow
		if err := json.Unmarshal(r, &row); err != nil {
			continue
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Team.Key != rows[j].Team.Key {
			return rows[i].Team.Key < rows[j].Team.Key
		}
		return rows[i].Position < rows[j].Position
	})

	prov = attachFreshness(prov, flags)
	printProvenance(cmd, len(rows), prov)
	return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
}

// resolveWorkflowState maps a --state-name or --state-type value to the
// matching workflow state UUID within the issue's team. Exactly one of name or
// stateType must be non-empty. Zero matches is a not-found error; multiple
// matches (common for --state-type when a team has several states of one
// type) is a usage error listing the candidates so the agent can retry with
// --state-name or --state.
func resolveWorkflowState(c graphqlQueryer, team issueTeamInfo, name, stateType string) (string, error) {
	teamFilter := map[string]any{}
	switch {
	case team.ID != "":
		teamFilter["id"] = map[string]any{"eq": team.ID}
	case team.Key != "":
		teamFilter["key"] = map[string]any{"eqIgnoreCase": team.Key}
	default:
		return "", fmt.Errorf("cannot resolve workflow state: issue team is empty")
	}
	filter := map[string]any{"team": teamFilter}
	selector := ""
	switch {
	case name != "":
		filter["name"] = map[string]any{"eqIgnoreCase": name}
		selector = fmt.Sprintf("--state-name %q", name)
	case stateType != "":
		normalizedType, err := normalizeWorkflowStateType(stateType)
		if err != nil {
			return "", err
		}
		filter["type"] = map[string]any{"eq": normalizedType}
		selector = fmt.Sprintf("--state-type %q", normalizedType)
	default:
		return "", fmt.Errorf("cannot resolve workflow state: no name or type given")
	}
	const query = `query($filter: WorkflowStateFilter) {
		workflowStates(first: 50, filter: $filter) {
			nodes { id name type }
		}
	}`
	var resp struct {
		WorkflowStates struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := c.QueryInto(query, map[string]any{"filter": filter}, &resp); err != nil {
		return "", err
	}
	nodes := resp.WorkflowStates.Nodes
	teamLabel := issueTeamName(team)
	switch len(nodes) {
	case 0:
		return "", notFoundErr(fmt.Errorf("no workflow state matching %s in team %s; run 'linear-pp-cli workflow-states list --team %s' to see valid states", selector, teamLabel, teamLabel))
	case 1:
		return nodes[0].ID, nil
	default:
		candidates := make([]string, 0, len(nodes))
		for _, n := range nodes {
			candidates = append(candidates, fmt.Sprintf("%q (%s, %s)", n.Name, n.Type, n.ID))
		}
		return "", usageErr(fmt.Errorf("%s is ambiguous in team %s: matches %s; pass --state-name with the exact name or --state with the UUID", selector, teamLabel, strings.Join(candidates, ", ")))
	}
}

// fetchWorkflowStatesLive queries workflowStates via GraphQL, filtered by team
// key or UUID when provided. Linear's workflowStates filter accepts a nested
// TeamFilter, so team keys resolve server-side without a local sync. Results
// are paginated via pageInfo.endCursor — a single capped page would silently
// truncate the state list on workspaces with many teams, which is exactly the
// failure mode this command exists to eliminate.
func fetchWorkflowStatesLive(flags *rootFlags, team string) ([]json.RawMessage, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	filter := map[string]any{}
	if team != "" {
		if store.IsUUID(team) {
			filter["team"] = map[string]any{"id": map[string]any{"eq": team}}
		} else {
			filter["team"] = map[string]any{"key": map[string]any{"eqIgnoreCase": team}}
		}
	}
	const query = `query($filter: WorkflowStateFilter, $after: String) {
		workflowStates(first: 250, filter: $filter, after: $after) {
			nodes {
				id name type color position
				team { id name key }
			}
			pageInfo { hasNextPage endCursor }
		}
	}`
	var all []json.RawMessage
	cursor := ""
	for {
		vars := map[string]any{"filter": filter}
		if cursor != "" {
			vars["after"] = cursor
		}
		var resp struct {
			WorkflowStates struct {
				Nodes    []json.RawMessage `json:"nodes"`
				PageInfo struct {
					HasNextPage bool   `json:"hasNextPage"`
					EndCursor   string `json:"endCursor"`
				} `json:"pageInfo"`
			} `json:"workflowStates"`
		}
		if err := c.QueryInto(query, vars, &resp); err != nil {
			return nil, err
		}
		all = append(all, resp.WorkflowStates.Nodes...)
		if !resp.WorkflowStates.PageInfo.HasNextPage || resp.WorkflowStates.PageInfo.EndCursor == "" {
			return all, nil
		}
		cursor = resp.WorkflowStates.PageInfo.EndCursor
	}
}
