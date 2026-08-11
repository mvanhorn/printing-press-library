package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
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

// creatableWorkflowStateTypes is the subset of WorkflowState.type values that
// WorkflowStateCreateInput.type accepts. Triage and duplicate states are
// provisioned by Linear itself and cannot be created through the API, and
// WorkflowStateUpdateInput has no type field at all, so a state's type is
// fixed for its lifetime.
var creatableWorkflowStateTypes = map[string]struct{}{
	"backlog":   {},
	"unstarted": {},
	"started":   {},
	"completed": {},
	"canceled":  {},
}

const creatableWorkflowStateTypeList = "backlog, unstarted, started, completed, canceled"

func normalizeCreatableWorkflowStateType(stateType string) (string, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(stateType))
	if _, ok := creatableWorkflowStateTypes[normalizedType]; !ok {
		if _, known := validLinearWorkflowStateTypes[normalizedType]; known {
			return "", usageErr(fmt.Errorf("--type %q cannot be created through the API. Linear provisions those states itself. Creatable types: %s", normalizedType, creatableWorkflowStateTypeList))
		}
		return "", usageErr(fmt.Errorf("--type %q is not a valid Linear workflow state type, creatable types: %s", stateType, creatableWorkflowStateTypeList))
	}
	return normalizedType, nil
}

func newWorkflowStatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "workflow-states",
		Aliases:     []string{"states"},
		Short:       "Manage Linear workflow states: list, create, update, archive",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkflowStatesListCmd(flags))
	cmd.AddCommand(newWorkflowStatesCreateCmd(flags))
	cmd.AddCommand(newWorkflowStatesUpdateCmd(flags))
	cmd.AddCommand(newWorkflowStatesArchiveCmd(flags))
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

// Workflow state write surface (GAP-027). workflowStateCreate,
// workflowStateUpdate, and workflowStateArchive are all live in
// api-inventory.json.

func newWorkflowStatesCreateCmd(flags *rootFlags) *cobra.Command {
	var teamFlag, nameFlag, typeFlag, colorFlag, descFlag, dbPath string
	var positionFlag float64
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a workflow state on a team's board",
		Long: `Create a Linear workflow state via the workflowStateCreate mutation.

--team, --name, --type, and --color are all required by WorkflowStateCreateInput.

--type is restricted to the five creatable categories (` + creatableWorkflowStateTypeList + `).
Triage and duplicate states are provisioned by Linear and cannot be created here,
and no mutation can change a state's type afterwards, so pick it correctly now.`,
		Example: `  linear-pp-cli workflow-states create --team ENG --name "In Review" --type started --color "#4ea7fc" --agent
  linear-pp-cli workflow-states create --team ENG --name "Blocked" --type started --color "#f2994a" --position 3 --dry-run --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(teamFlag) == "" {
				return usageErr(fmt.Errorf("--team is required (team key like ENG or team UUID)"))
			}
			if strings.TrimSpace(nameFlag) == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}
			if strings.TrimSpace(colorFlag) == "" {
				return usageErr(fmt.Errorf("--color is required (WorkflowStateCreateInput.color is non-null), pass a hex string such as #4ea7fc"))
			}
			stateType, err := normalizeCreatableWorkflowStateType(typeFlag)
			if err != nil {
				return err
			}
			buildInput := func(c graphqlQueryer) (map[string]any, error) {
				teamID, err := resolveWriteTeamID(c, dbPath, teamFlag)
				if err != nil {
					return nil, err
				}
				input := map[string]any{
					"teamId": teamID,
					"name":   nameFlag,
					"type":   stateType,
					"color":  colorFlag,
				}
				setOptionalString(input, "description", descFlag)
				if cmd.Flags().Changed("position") {
					input["position"] = positionFlag
				}
				return input, nil
			}
			if flags.dryRun {
				input, err := buildInput(nil)
				if err != nil {
					return err
				}
				return renderMutationDryRun(cmd, flags, "would_create_workflow_state", "workflowStateCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			input, err := buildInput(c)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			resp, err := c.Mutate(client.WorkflowStateCreateMutation, map[string]any{"input": input})
			if err != nil {
				return classifyGraphQLCreateError("workflowStateCreate", err, flags)
			}
			state, err := extractMutationObject(resp, "workflowStateCreate", "workflowState")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, state, "workflow_states")
		},
	}
	cmd.Flags().StringVar(&teamFlag, "team", "", "Team key or UUID that owns the state (required)")
	cmd.Flags().StringVar(&nameFlag, "name", "", "State name (required)")
	cmd.Flags().StringVar(&typeFlag, "type", "", "State category, one of: "+creatableWorkflowStateTypeList+" (required)")
	cmd.Flags().StringVar(&colorFlag, "color", "", "State color as a hex string, e.g. #4ea7fc (required)")
	cmd.Flags().StringVar(&descFlag, "description", "", "State description")
	cmd.Flags().Float64Var(&positionFlag, "position", 0, "Board position, lower sorts earlier")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (for team-key resolution)")
	return cmd
}

// newWorkflowStatesUpdateCmd exposes no --type flag: WorkflowStateUpdateInput
// has no type field, so the category is immutable after creation.
func newWorkflowStatesUpdateCmd(flags *rootFlags) *cobra.Command {
	var nameFlag, colorFlag, descFlag string
	var positionFlag float64
	cmd := &cobra.Command{
		Use:   "update <state-id>",
		Short: "Update a workflow state's name, color, description, or board position",
		Long: `Update a Linear workflow state via the workflowStateUpdate mutation.

There is no --type flag. WorkflowStateUpdateInput exposes only name, color,
description, and position, so a state's category cannot be changed after it is
created. Create a correctly typed state and move the issues instead.`,
		Example: `  linear-pp-cli workflow-states update <state-uuid> --name "In Review" --agent
  linear-pp-cli workflow-states update <state-uuid> --position 2 --dry-run --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := map[string]any{}
			setChangedString(cmd, input, "name", "name", nameFlag)
			setChangedString(cmd, input, "color", "color", colorFlag)
			setChangedString(cmd, input, "description", "description", descFlag)
			if cmd.Flags().Changed("position") {
				input["position"] = positionFlag
			}
			if len(input) == 0 {
				return usageErr(fmt.Errorf("no workflow state fields supplied, pass --name, --color, --description, or --position"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_workflow_state", "workflowStateUpdate", map[string]any{"state": args[0], "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.WorkflowStateUpdateMutation, map[string]any{"id": args[0], "input": input})
			if err != nil {
				return classifyGraphQLMutationError("workflowStateUpdate", err, flags)
			}
			state, err := extractMutationObject(resp, "workflowStateUpdate", "workflowState")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, state, "workflow_states")
		},
	}
	cmd.Flags().StringVar(&nameFlag, "name", "", "New state name")
	cmd.Flags().StringVar(&colorFlag, "color", "", "New state color as a hex string")
	cmd.Flags().StringVar(&descFlag, "description", "", "New state description (pass an empty string to clear)")
	cmd.Flags().Float64Var(&positionFlag, "position", 0, "New board position")
	return cmd
}

func newWorkflowStatesArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <state-id>",
		Short: "Archive a workflow state",
		Long: `Archive a Linear workflow state via the workflowStateArchive mutation.

Linear only archives a state whose issues have all been archived, so move or
archive the issues sitting in it first. Requires confirmation unless --yes is set.`,
		Example: `  linear-pp-cli workflow-states archive <state-uuid> --yes --agent`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_archive_workflow_state", "workflowStateArchive", map[string]any{"input": map[string]any{"id": args[0]}})
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Archive workflow state %s?", args[0])); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.WorkflowStateArchiveMutation, map[string]any{"id": args[0]})
			if err != nil {
				return classifyGraphQLMutationError("workflowStateArchive", err, flags)
			}
			state, err := extractMutationObject(resp, "workflowStateArchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, state, "workflow_states")
		},
	}
	return cmd
}
