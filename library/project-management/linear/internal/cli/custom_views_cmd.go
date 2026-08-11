package cli

// custom_views_cmd.go closes GAP-042: custom views had no CLI surface at all,
// so the workspace's own saved filters were invisible and could not be created,
// edited, or deleted. A CustomView is the workspace-authored, runtime-resolvable
// saved filter, which is why it is worth reading as well as writing: the filter
// payload lives in filterData (issues), projectFilterData (projects), and
// initiativeFilterData (initiatives).
//
// CustomView{Create,Update}Input.filters is deprecated in favour of filterData
// and is therefore not exposed.

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newCustomViewsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "custom-views",
		Aliases:     []string{"views"},
		Short:       "Linear custom views (saved filters): list, get, create, update, delete",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCustomViewsListCmd(flags))
	cmd.AddCommand(newCustomViewsGetCmd(flags))
	cmd.AddCommand(newCustomViewsCreateCmd(flags))
	cmd.AddCommand(newCustomViewsUpdateCmd(flags))
	cmd.AddCommand(newCustomViewsDeleteCmd(flags))
	return cmd
}

func newCustomViewsListCmd(flags *rootFlags) *cobra.Command {
	var team, name string
	var shared, sharedOnly bool
	var after string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List custom views",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the workspace's custom views with their filter payloads.

--team, --name, and --shared are pushed into CustomViewFilter so the API does
the narrowing. --name matches case-insensitively on any part of the view name.`,
		Example: `  linear-pp-cli custom-views list --agent
  linear-pp-cli custom-views list --team ENG --shared --agent --select id,name,modelName`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if limit <= 0 {
				limit = 50
			}
			filter := map[string]any{}
			if team != "" {
				if store.IsUUID(team) {
					filter["team"] = map[string]any{"id": map[string]any{"eq": team}}
				} else {
					filter["team"] = map[string]any{"key": map[string]any{"eqIgnoreCase": team}}
				}
			}
			if name != "" {
				filter["name"] = map[string]any{"containsIgnoreCase": name}
			}
			if cmd.Flags().Changed("shared") || sharedOnly {
				filter["shared"] = map[string]any{"eq": shared || sharedOnly}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			vars := map[string]any{"first": limit, "after": nil, "filter": filter}
			if after != "" {
				vars["after"] = after
			}
			var resp struct {
				CustomViews struct {
					Nodes    []map[string]any `json:"nodes"`
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
				} `json:"customViews"`
			}
			if err := c.QueryInto(client.CustomViewsQuery, vars, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			out, err := json.Marshal(map[string]any{
				"customViews": resp.CustomViews.Nodes,
				"pageInfo":    resp.CustomViews.PageInfo,
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "custom_views", true)
		},
	}
	cmd.Flags().StringVar(&team, "team", "", "Filter by team key or UUID")
	cmd.Flags().StringVar(&name, "name", "", "Filter by name substring, case-insensitive")
	cmd.Flags().BoolVar(&shared, "shared", false, "Filter on the shared flag (pair with --shared=false for private views)")
	cmd.Flags().BoolVar(&sharedOnly, "shared-only", false, "Alias for --shared=true")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum custom views to return")
	return cmd
}

func newCustomViewsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get <custom-view-id>",
		Short:       "Get one custom view including its filter payload",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `Read one custom view by UUID. modelName says which entity the view targets,
and the matching payload field (filterData, projectFilterData, or
initiativeFilterData) carries the saved filter itself.`,
		Example: `  linear-pp-cli custom-views get <custom-view-uuid> --agent
  linear-pp-cli custom-views get <custom-view-uuid> --agent --select filterData`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<custom-view-id> is required"))
			}
			viewID := args[0]
			if !store.IsUUID(viewID) {
				return usageErr(fmt.Errorf("<custom-view-id> expects a custom view UUID, got %q; run 'custom-views list' to find it", viewID))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var resp struct {
				CustomView json.RawMessage `json:"customView"`
			}
			if err := c.QueryInto(client.CustomViewQuery, map[string]any{"id": viewID}, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			if len(resp.CustomView) == 0 || string(resp.CustomView) == "null" {
				return notFoundErr(fmt.Errorf("custom view %s not found", viewID))
			}
			return renderLiveObject(cmd, flags, resp.CustomView, "custom_views")
		},
	}
	return cmd
}

// customViewWriteFlags carries the CustomView{Create,Update}Input fields this
// CLI exposes. The three filter payloads are mutually complementary rather than
// exclusive, so all three are accepted and passed through as given.
type customViewWriteFlags struct {
	name        string
	description string
	icon        string
	color       string
	owner       string
	project     string
	initiative  string
	team        string
	shared      bool

	filterData            string
	filterDataFile        string
	projectFilterData     string
	projectFilterDataFile string
	initiativeFilterData  string
	initiativeFilterFile  string
}

func (v *customViewWriteFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&v.name, "name", "", "Custom view name")
	cmd.Flags().StringVar(&v.description, "description", "", "Custom view description")
	cmd.Flags().StringVar(&v.icon, "icon", "", "Custom view icon name")
	cmd.Flags().StringVar(&v.color, "color", "", "Custom view color as a hex string")
	cmd.Flags().StringVar(&v.owner, "owner", "", "Owning user UUID")
	cmd.Flags().StringVar(&v.team, "team", "", "Scope the view to this team key or UUID")
	cmd.Flags().StringVar(&v.project, "project", "", "Scope the view to this project UUID")
	cmd.Flags().StringVar(&v.initiative, "initiative", "", "Scope the view to this initiative UUID")
	cmd.Flags().BoolVar(&v.shared, "shared", false, "Share the view with the whole workspace")
	cmd.Flags().StringVar(&v.filterData, "filter-data", "", "Issue filter as an inline JSON object")
	cmd.Flags().StringVar(&v.filterDataFile, "filter-data-file", "", "Read the issue filter JSON from a file")
	cmd.Flags().StringVar(&v.projectFilterData, "project-filter-data", "", "Project filter as an inline JSON object")
	cmd.Flags().StringVar(&v.projectFilterDataFile, "project-filter-data-file", "", "Read the project filter JSON from a file")
	cmd.Flags().StringVar(&v.initiativeFilterData, "initiative-filter-data", "", "Initiative filter as an inline JSON object")
	cmd.Flags().StringVar(&v.initiativeFilterFile, "initiative-filter-data-file", "", "Read the initiative filter JSON from a file")
}

// input builds the mutation input. dbPath resolves --team offline, and the
// returned pending slice names team values that still need a live lookup.
func (v *customViewWriteFlags) input(cmd *cobra.Command, flags *rootFlags, dbPath string) (map[string]any, []string, error) {
	input := map[string]any{}
	if v.name != "" {
		input["name"] = v.name
	}
	if cmd.Flags().Changed("description") {
		input["description"] = v.description
	}
	if v.icon != "" {
		input["icon"] = v.icon
	}
	if v.color != "" {
		input["color"] = v.color
	}
	if v.owner != "" {
		if !store.IsUUID(v.owner) {
			return nil, nil, usageErr(fmt.Errorf("--owner expects a user UUID, got %q", v.owner))
		}
		input["ownerId"] = v.owner
	}
	if v.project != "" {
		if !store.IsUUID(v.project) {
			return nil, nil, portfolioUUIDUsageErr(flags, "--project", v.project, "use 'projects resolve <name>' to find the UUID")
		}
		input["projectId"] = v.project
	}
	if v.initiative != "" {
		if !store.IsUUID(v.initiative) {
			return nil, nil, portfolioUUIDUsageErr(flags, "--initiative", v.initiative, "use 'initiatives resolve <name>' to find the UUID")
		}
		input["initiativeId"] = v.initiative
	}
	if cmd.Flags().Changed("shared") {
		input["shared"] = v.shared
	}
	payloads := []struct {
		key       string
		inlineArg string
		inline    string
		fileArg   string
		file      string
		label     string
	}{
		{"filterData", "filter-data", v.filterData, "filter-data-file", v.filterDataFile, "filterData"},
		{"projectFilterData", "project-filter-data", v.projectFilterData, "project-filter-data-file", v.projectFilterDataFile, "projectFilterData"},
		{"initiativeFilterData", "initiative-filter-data", v.initiativeFilterData, "initiative-filter-data-file", v.initiativeFilterFile, "initiativeFilterData"},
	}
	for _, payload := range payloads {
		parsed, ok, err := wbJSONObject(payload.inlineArg, payload.inline, payload.fileArg, payload.file, payload.label)
		if err != nil {
			return nil, nil, err
		}
		if ok {
			input[payload.key] = parsed
		}
	}
	var pending []string
	if v.team != "" {
		teamIDs, unresolved := wbResolveTeamsLocal(dbPath, []string{v.team})
		input["teamId"] = teamIDs[0]
		pending = unresolved
	}
	return input, pending, nil
}

func newCustomViewsCreateCmd(flags *rootFlags) *cobra.Command {
	write := customViewWriteFlags{}
	var dbPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a custom view",
		Long: `Create a custom view via the customViewCreate mutation. CustomViewCreateInput
requires name, and a view without a filter payload matches everything.

The filter payloads take the same JSON shape as the API's own filter inputs:
--filter-data is an IssueFilter, --project-filter-data a ProjectFilter, and
--initiative-filter-data an InitiativeFilter. Read an existing view with
'custom-views get <id>' to see a working payload. The deprecated filters input
is not supported.`,
		Example: `  linear-pp-cli custom-views create --name "My open bugs" --filter-data-file /tmp/filter.json --agent
  linear-pp-cli custom-views create --name "x" --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if write.name == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("--name is required"))
			}
			input, pending, err := write.input(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_custom_view", "customViewCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(pending) > 0 {
				resolved, err := wbResolveTeamsLive(c, []string{input["teamId"].(string)})
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["teamId"] = resolved[0]
			}
			resp, err := c.Mutate(client.CustomViewCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("customViewCreate", err, flags)
			}
			view, err := extractMutationObject(resp, "customViewCreate", "customView")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, view, "custom_views")
		},
	}
	write.bind(cmd)
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

func newCustomViewsUpdateCmd(flags *rootFlags) *cobra.Command {
	write := customViewWriteFlags{}
	var dbPath string
	cmd := &cobra.Command{
		Use:     "update <custom-view-id>",
		Aliases: []string{"edit"},
		Short:   "Update a custom view",
		Long: `Edit a custom view via the customViewUpdate mutation. At least one field flag
is required, and each filter payload flag replaces that payload wholesale.`,
		Example: `  linear-pp-cli custom-views update <custom-view-uuid> --name "My open bugs" --agent
  linear-pp-cli custom-views update <custom-view-uuid> --filter-data-file /tmp/filter.json --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<custom-view-id> is required"))
			}
			viewID := args[0]
			if !store.IsUUID(viewID) {
				return usageErr(fmt.Errorf("<custom-view-id> expects a custom view UUID, got %q", viewID))
			}
			input, pending, err := write.input(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			if len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass at least one field to update (--name, --description, --filter-data, --project-filter-data, --initiative-filter-data, --icon, --color, --owner, --team, --project, --initiative, --shared)"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_custom_view", "customViewUpdate", map[string]any{"id": viewID, "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(pending) > 0 {
				resolved, err := wbResolveTeamsLive(c, []string{input["teamId"].(string)})
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["teamId"] = resolved[0]
			}
			resp, err := c.Mutate(client.CustomViewUpdateMutation, map[string]any{"id": viewID, "input": input})
			if err != nil {
				return classifyMutationError("customViewUpdate", err, flags, nil)
			}
			view, err := extractMutationObject(resp, "customViewUpdate", "customView")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, view, "custom_views")
		},
	}
	write.bind(cmd)
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

func newCustomViewsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <custom-view-id>",
		Short: "Delete a custom view",
		Long: `Delete a custom view via the customViewDelete mutation.

Deletion is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-deleted view exits 0 as a no-op.`,
		Example: `  linear-pp-cli custom-views delete <custom-view-uuid> --yes --agent
  linear-pp-cli custom-views delete <custom-view-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<custom-view-id> is required"))
			}
			viewID := args[0]
			if !store.IsUUID(viewID) {
				return usageErr(fmt.Errorf("<custom-view-id> expects a custom view UUID, got %q", viewID))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_custom_view", "customViewDelete", map[string]any{"id": viewID})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Delete custom view %s", viewID)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.CustomViewDeleteMutation, map[string]any{"id": viewID})
			if err != nil {
				return wbClassifyDeleteError("customViewDelete", err, flags)
			}
			id, err := wbDecodeDeletePayload(resp, "customViewDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "custom_view_deleted", map[string]any{"id": firstNonEmpty(id, viewID)},
				fmt.Sprintf("Deleted custom view %s", firstNonEmpty(id, viewID)))
		},
	}
	return cmd
}
