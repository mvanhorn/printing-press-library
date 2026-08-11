package cli

// milestones_write.go closes GAP-029: projectMilestoneCreate, Update, Delete,
// and Move had no CLI surface, and the only milestone read was the at-risk
// analytic plus the by-id promoted leaf, so a project's milestone list was not
// enumerable. The leaves are registered on the existing `milestones` parent in
// milestones_at_risk.go.

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newMilestonesListCmd(flags *rootFlags) *cobra.Command {
	var project, projectName string
	var after string
	var limit int
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List a project's milestones",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Long: `List the milestones of one project, read through Project.projectMilestones so
the project scope is part of the query rather than a client-side filter.

Each row carries the ProjectMilestoneStatus Linear computes (unstarted, next,
overdue, done) and the milestone's own progress, which is what
"milestones at-risk" ranks on.`,
		Example: `  linear-pp-cli milestones list --project <project-uuid> --agent
  linear-pp-cli milestones list --project-name "Q3 platform work" --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" && projectName == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name>"))
			}
			if dryRunOK(flags) {
				return nil
			}
			if limit <= 0 {
				limit = 50
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return err
			}
			vars := map[string]any{"id": projectID, "first": limit, "after": nil}
			if after != "" {
				vars["after"] = after
			}
			var resp struct {
				Project *struct {
					ID                string `json:"id"`
					Name              string `json:"name"`
					ProjectMilestones struct {
						Nodes    []map[string]any `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"projectMilestones"`
				} `json:"project"`
			}
			if err := c.QueryInto(client.ProjectMilestonesForProjectQuery, vars, &resp); err != nil {
				return classifyLiveReadError(err, flags)
			}
			if resp.Project == nil {
				return notFoundErr(fmt.Errorf("project %s not found", projectID))
			}
			out, err := json.Marshal(map[string]any{
				"project":    map[string]any{"id": resp.Project.ID, "name": resp.Project.Name},
				"milestones": resp.Project.ProjectMilestones.Nodes,
				"pageInfo":   resp.Project.ProjectMilestones.PageInfo,
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "project_milestones", true)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Resolve the project by exact name")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum milestones to return")
	return cmd
}

// milestoneWriteFlags carries the ProjectMilestoneCreateInput and
// ProjectMilestoneUpdateInput fields this CLI exposes. descriptionData
// (JSONObject) is intentionally absent: description is the field a caller can
// write by hand, and the two are alternative representations of the same prose.
type milestoneWriteFlags struct {
	name        string
	description string
	targetDate  string
	sortOrder   float64
}

func (m *milestoneWriteFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&m.name, "name", "", "Milestone name")
	cmd.Flags().StringVar(&m.description, "description", "", "Milestone description (markdown)")
	cmd.Flags().StringVar(&m.targetDate, "target-date", "", "Target date as YYYY-MM-DD")
	cmd.Flags().Float64Var(&m.sortOrder, "sort-order", 0, "Explicit sort order within the project")
}

func (m *milestoneWriteFlags) input(cmd *cobra.Command) (map[string]any, error) {
	input := map[string]any{}
	if m.name != "" {
		input["name"] = m.name
	}
	if cmd.Flags().Changed("description") {
		input["description"] = m.description
	}
	if m.targetDate != "" {
		if err := wbCheckTimelessDate("--target-date", m.targetDate); err != nil {
			return nil, err
		}
		input["targetDate"] = m.targetDate
	}
	if cmd.Flags().Changed("sort-order") {
		input["sortOrder"] = m.sortOrder
	}
	return input, nil
}

func newMilestonesCreateCmd(flags *rootFlags) *cobra.Command {
	write := milestoneWriteFlags{}
	var project, projectName string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project milestone",
		Long: `Create a milestone via the projectMilestoneCreate mutation. Both --name and a
project are required, matching ProjectMilestoneCreateInput.`,
		Example: `  linear-pp-cli milestones create --project <project-uuid> --name "Beta cut" --target-date 2026-09-15 --agent
  linear-pp-cli milestones create --project <project-uuid> --name "Beta cut" --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if write.name == "" || (project == "" && projectName == "") {
				if dryRunOK(flags) {
					return nil
				}
				if write.name == "" {
					return usageErr(fmt.Errorf("--name is required"))
				}
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name>"))
			}
			input, err := write.input(cmd)
			if err != nil {
				return err
			}
			if flags.dryRun {
				dryInput := map[string]any{}
				for key, value := range input {
					dryInput[key] = value
				}
				dryInput["projectId"] = firstNonEmpty(project, projectName)
				return renderMutationDryRun(cmd, flags, "would_create_milestone", "projectMilestoneCreate", map[string]any{"input": dryInput})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return err
			}
			input["projectId"] = projectID
			resp, err := c.Mutate(client.ProjectMilestoneCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("projectMilestoneCreate", err, flags)
			}
			milestone, err := extractMutationObject(resp, "projectMilestoneCreate", "projectMilestone")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, milestone, "project_milestones")
		},
	}
	write.bind(cmd)
	cmd.Flags().StringVar(&project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Resolve the project by exact name")
	return cmd
}

func newMilestonesUpdateCmd(flags *rootFlags) *cobra.Command {
	write := milestoneWriteFlags{}
	var project, projectName string
	cmd := &cobra.Command{
		Use:     "update <milestone-id>",
		Aliases: []string{"edit"},
		Short:   "Update a project milestone",
		Long: `Edit a milestone via the projectMilestoneUpdate mutation. At least one field
flag is required.

ProjectMilestoneUpdateInput also accepts projectId, which reparents the
milestone without moving its issues. Use "milestones move" instead when the
milestone's issues should follow it, because projectMilestoneMove is the
mutation that carries the issue and team side effects.`,
		Example: `  linear-pp-cli milestones update <milestone-uuid> --target-date 2026-10-01 --agent
  linear-pp-cli milestones update <milestone-uuid> --name "Beta cut (slipped)" --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<milestone-id> is required"))
			}
			milestoneID := args[0]
			if !store.IsUUID(milestoneID) {
				return usageErr(fmt.Errorf("<milestone-id> expects a milestone UUID, got %q", milestoneID))
			}
			input, err := write.input(cmd)
			if err != nil {
				return err
			}
			if project == "" && projectName == "" && len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass at least one field to update (--name, --description, --target-date, --sort-order, --project)"))
			}
			if flags.dryRun {
				dryInput := map[string]any{}
				for key, value := range input {
					dryInput[key] = value
				}
				if project != "" || projectName != "" {
					dryInput["projectId"] = firstNonEmpty(project, projectName)
				}
				return renderMutationDryRun(cmd, flags, "would_update_milestone", "projectMilestoneUpdate", map[string]any{"id": milestoneID, "input": dryInput})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if project != "" || projectName != "" {
				projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
				if err != nil {
					return err
				}
				input["projectId"] = projectID
			}
			resp, err := c.Mutate(client.ProjectMilestoneUpdateMutation, map[string]any{"id": milestoneID, "input": input})
			if err != nil {
				return classifyMutationError("projectMilestoneUpdate", err, flags, nil)
			}
			milestone, err := extractMutationObject(resp, "projectMilestoneUpdate", "projectMilestone")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, milestone, "project_milestones")
		},
	}
	write.bind(cmd)
	cmd.Flags().StringVar(&project, "project", "", "Reparent the milestone to this project UUID (issues stay put; see 'milestones move')")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Reparent the milestone to the project with this exact name")
	return cmd
}

func newMilestonesDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <milestone-id>",
		Short: "Delete a project milestone",
		Long: `Delete a milestone via the projectMilestoneDelete mutation. Issues assigned to
the milestone stay in the project and lose their milestone association.

Deletion is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-deleted milestone exits 0 as a no-op.`,
		Example: `  linear-pp-cli milestones delete <milestone-uuid> --yes --agent
  linear-pp-cli milestones delete <milestone-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<milestone-id> is required"))
			}
			milestoneID := args[0]
			if !store.IsUUID(milestoneID) {
				return usageErr(fmt.Errorf("<milestone-id> expects a milestone UUID, got %q", milestoneID))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_milestone", "projectMilestoneDelete", map[string]any{"id": milestoneID})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Delete milestone %s", milestoneID)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ProjectMilestoneDeleteMutation, map[string]any{"id": milestoneID})
			if err != nil {
				return wbClassifyDeleteError("projectMilestoneDelete", err, flags)
			}
			id, err := wbDecodeDeletePayload(resp, "projectMilestoneDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "milestone_deleted", map[string]any{"id": firstNonEmpty(id, milestoneID)},
				fmt.Sprintf("Deleted milestone %s", firstNonEmpty(id, milestoneID)))
		},
	}
	return cmd
}

func newMilestonesMoveCmd(flags *rootFlags) *cobra.Command {
	var project, projectName, newIssueTeam string
	var addIssueTeamToProject bool
	var dbPath string
	cmd := &cobra.Command{
		Use:   "move <milestone-id>",
		Short: "Move a project milestone and its issues to another project",
		Long: `Move a milestone via the projectMilestoneMove mutation, which carries the
milestone's issues across with it. ProjectMilestoneMoveInput requires the
destination projectId and additionally accepts newIssueTeamId (the team the
moved issues should belong to) and addIssueTeamToProject (whether that team is
added to the destination project).

The undo inputs on ProjectMilestoneMoveInput are not exposed: they exist for
replaying a prior move's response and have no meaning on a fresh call.`,
		Example: `  linear-pp-cli milestones move <milestone-uuid> --project <project-uuid> --agent
  linear-pp-cli milestones move <milestone-uuid> --project <project-uuid> --new-issue-team ENG --add-issue-team-to-project --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (project == "" && projectName == "") {
				if dryRunOK(flags) {
					return nil
				}
				if len(args) == 0 {
					return usageErr(fmt.Errorf("<milestone-id> is required"))
				}
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name> as the move destination"))
			}
			milestoneID := args[0]
			if !store.IsUUID(milestoneID) {
				return usageErr(fmt.Errorf("<milestone-id> expects a milestone UUID, got %q", milestoneID))
			}
			input := map[string]any{}
			if cmd.Flags().Changed("add-issue-team-to-project") {
				input["addIssueTeamToProject"] = addIssueTeamToProject
			}
			var unresolvedTeams []string
			if newIssueTeam != "" {
				teamIDs, unresolved := wbResolveTeamsLocal(dbPath, []string{newIssueTeam})
				input["newIssueTeamId"] = teamIDs[0]
				unresolvedTeams = unresolved
			}
			if flags.dryRun {
				dryInput := map[string]any{}
				for key, value := range input {
					dryInput[key] = value
				}
				dryInput["projectId"] = firstNonEmpty(project, projectName)
				return renderMutationDryRun(cmd, flags, "would_move_milestone", "projectMilestoneMove", map[string]any{"id": milestoneID, "input": dryInput})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return err
			}
			input["projectId"] = projectID
			if len(unresolvedTeams) > 0 {
				resolved, err := wbResolveTeamsLive(c, []string{input["newIssueTeamId"].(string)})
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["newIssueTeamId"] = resolved[0]
			}
			resp, err := c.Mutate(client.ProjectMilestoneMoveMutation, map[string]any{"id": milestoneID, "input": input})
			if err != nil {
				return classifyMutationError("projectMilestoneMove", err, flags, nil)
			}
			milestone, err := extractMutationObject(resp, "projectMilestoneMove", "projectMilestone")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, milestone, "project_milestones")
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Destination project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Resolve the destination project by exact name")
	cmd.Flags().StringVar(&newIssueTeam, "new-issue-team", "", "Team key or UUID the moved issues should belong to")
	cmd.Flags().BoolVar(&addIssueTeamToProject, "add-issue-team-to-project", false, "Add --new-issue-team to the destination project")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}
