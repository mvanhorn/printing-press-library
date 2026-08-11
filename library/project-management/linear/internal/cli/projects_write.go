package cli

// projects_write.go closes GAP-028: the `projects` family was read-only, so a
// project could be inspected but never created, edited, trashed, or relabelled
// without dropping to raw GraphQL. The write leaves are registered on the
// existing `projects` parent in linear_groups.go.
//
// The wb* helpers at the bottom of this file are shared by the whole tier-2
// write-families-B surface (projects, milestones, templates, custom views,
// favorites). "wb" is short for write-families-B and keeps these
// package-level names clear of the generated command helpers.

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// projectWriteFlags carries the ProjectCreateInput and ProjectUpdateInput
// fields this CLI exposes. Every field maps one-to-one onto a live,
// non-deprecated input field: the deprecated `state` string is deliberately
// absent, so project status is set through --status with a ProjectStatus UUID.
type projectWriteFlags struct {
	name        string
	description string
	lead        string
	status      string
	startDate   string
	targetDate  string
	icon        string
	color       string
	priority    int
	teams       []string
	labels      []string
	members     []string
}

func (p *projectWriteFlags) bind(cmd *cobra.Command, forCreate bool) {
	cmd.Flags().StringVar(&p.name, "name", "", "Project name")
	cmd.Flags().StringVar(&p.description, "description", "", "Short project summary")
	cmd.Flags().StringVar(&p.lead, "lead", "", "Project lead user UUID")
	cmd.Flags().StringVar(&p.status, "status", "", "Project status UUID (see 'project-statuses <id>'); Linear's deprecated free-text state is not supported")
	cmd.Flags().StringVar(&p.startDate, "start-date", "", "Start date as YYYY-MM-DD")
	cmd.Flags().StringVar(&p.targetDate, "target-date", "", "Target date as YYYY-MM-DD")
	cmd.Flags().StringVar(&p.icon, "icon", "", "Project icon name")
	cmd.Flags().StringVar(&p.color, "color", "", "Project color as a hex string")
	cmd.Flags().IntVar(&p.priority, "priority", 0, "Priority: 1=Urgent, 2=High, 3=Medium, 4=Low (0=None)")
	cmd.Flags().StringSliceVar(&p.members, "member", nil, "Project member user UUID (repeatable)")
	if forCreate {
		cmd.Flags().StringSliceVar(&p.teams, "team", nil, "Owning team key or UUID (repeatable, at least one required)")
		cmd.Flags().StringSliceVar(&p.labels, "label", nil, "Project label UUID to attach at creation (repeatable)")
		return
	}
	cmd.Flags().StringSliceVar(&p.teams, "team", nil, "Replace the project's teams with these team keys or UUIDs (repeatable)")
}

// input builds the mutation input from the flags the caller actually set.
// cmd.Flags().Changed is the authority for --priority so that an explicit
// --priority 0 (None) is distinguishable from an unset flag.
func (p *projectWriteFlags) input(cmd *cobra.Command) (map[string]any, error) {
	input := map[string]any{}
	if p.name != "" {
		input["name"] = p.name
	}
	if cmd.Flags().Changed("description") {
		input["description"] = p.description
	}
	if p.lead != "" {
		if !store.IsUUID(p.lead) {
			return nil, usageErr(fmt.Errorf("--lead expects a user UUID, got %q", p.lead))
		}
		input["leadId"] = p.lead
	}
	if p.status != "" {
		if !store.IsUUID(p.status) {
			return nil, usageErr(fmt.Errorf("--status expects a project status UUID, got %q", p.status))
		}
		input["statusId"] = p.status
	}
	if p.startDate != "" {
		if err := wbCheckTimelessDate("--start-date", p.startDate); err != nil {
			return nil, err
		}
		input["startDate"] = p.startDate
	}
	if p.targetDate != "" {
		if err := wbCheckTimelessDate("--target-date", p.targetDate); err != nil {
			return nil, err
		}
		input["targetDate"] = p.targetDate
	}
	if p.icon != "" {
		input["icon"] = p.icon
	}
	if p.color != "" {
		input["color"] = p.color
	}
	if cmd.Flags().Changed("priority") {
		input["priority"] = p.priority
	}
	if len(p.members) > 0 {
		for _, member := range p.members {
			if !store.IsUUID(member) {
				return nil, usageErr(fmt.Errorf("--member expects user UUIDs, got %q", member))
			}
		}
		input["memberIds"] = p.members
	}
	if len(p.labels) > 0 {
		for _, label := range p.labels {
			if !store.IsUUID(label) {
				return nil, usageErr(fmt.Errorf("--label expects project label UUIDs, got %q", label))
			}
		}
		input["labelIds"] = p.labels
	}
	return input, nil
}

func newProjectsCreateCmd(flags *rootFlags) *cobra.Command {
	write := projectWriteFlags{}
	var dbPath string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Linear project",
		Long: `Create a project via the projectCreate mutation.

--team is repeatable and accepts team keys or team UUIDs. Keys are resolved
against the local store first and against the API second, so this works with or
without a prior sync. Project status is set with --status <project-status-uuid>
because Linear's free-text project state input is deprecated.`,
		Example: `  linear-pp-cli projects create --name "Q3 platform work" --team ENG --agent
  linear-pp-cli projects create --name "Q3 platform work" --team ENG --team OPS --target-date 2026-09-30 --agent
  linear-pp-cli projects create --name "x" --team ENG --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if write.name == "" || len(write.teams) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				if write.name == "" {
					return usageErr(fmt.Errorf("--name is required"))
				}
				return usageErr(fmt.Errorf("--team is required (team key like ENG or team UUID, repeatable)"))
			}
			input, err := write.input(cmd)
			if err != nil {
				return err
			}
			teamIDs, unresolved := wbResolveTeamsLocal(dbPath, write.teams)
			input["teamIds"] = teamIDs
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_project", "projectCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(unresolved) > 0 {
				resolved, err := wbResolveTeamsLive(c, teamIDs)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["teamIds"] = resolved
			}
			resp, err := c.Mutate(client.ProjectCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("projectCreate", err, flags)
			}
			project, err := extractMutationObject(resp, "projectCreate", "project")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, project, "projects")
		},
	}
	write.bind(cmd, true)
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

func newProjectsUpdateCmd(flags *rootFlags) *cobra.Command {
	write := projectWriteFlags{}
	var dbPath string
	cmd := &cobra.Command{
		Use:     "update <project-id>",
		Aliases: []string{"edit"},
		Short:   "Update a Linear project",
		Long: `Edit a project via the projectUpdate mutation. At least one field flag is
required.

--team replaces the project's whole team set, so pass every team the project
should belong to. Project labels are managed with the dedicated
"projects add-label" and "projects remove-label" leaves rather than through this
command, because ProjectUpdateInput.labelIds replaces the label set wholesale.`,
		Example: `  linear-pp-cli projects update <project-uuid> --target-date 2026-10-15 --agent
  linear-pp-cli projects update <project-uuid> --status <project-status-uuid> --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<project-id> is required"))
			}
			projectID := args[0]
			if !store.IsUUID(projectID) {
				return portfolioUUIDUsageErr(flags, "<project-id>", projectID, "use 'projects resolve <name>' to find the UUID")
			}
			input, err := write.input(cmd)
			if err != nil {
				return err
			}
			var unresolved []string
			if len(write.teams) > 0 {
				var teamIDs []string
				teamIDs, unresolved = wbResolveTeamsLocal(dbPath, write.teams)
				input["teamIds"] = teamIDs
			}
			if len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass at least one field to update (--name, --description, --status, --lead, --start-date, --target-date, --icon, --color, --priority, --member, --team)"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_project", "projectUpdate", map[string]any{"id": projectID, "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if len(unresolved) > 0 {
				resolved, err := wbResolveTeamsLive(c, input["teamIds"].([]string))
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["teamIds"] = resolved
			}
			resp, err := c.Mutate(client.ProjectUpdateMutation, map[string]any{"id": projectID, "input": input})
			if err != nil {
				return classifyMutationError("projectUpdate", err, flags, nil)
			}
			project, err := extractMutationObject(resp, "projectUpdate", "project")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, project, "projects")
		},
	}
	write.bind(cmd, false)
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

func newProjectsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <project-id>",
		Short: "Delete (trash) a Linear project",
		Long: `Delete a project via the projectDelete mutation. Linear moves the project to
the trash rather than erasing it, and projectUnarchive restores it.

Deletion is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-deleted project exits 0 as a no-op.`,
		Example: `  linear-pp-cli projects delete <project-uuid> --yes --agent
  linear-pp-cli projects delete <project-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<project-id> is required"))
			}
			projectID := args[0]
			if !store.IsUUID(projectID) {
				return portfolioUUIDUsageErr(flags, "<project-id>", projectID, "use 'projects resolve <name>' to find the UUID")
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_project", "projectDelete", map[string]any{"id": projectID})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Delete project %s", projectID)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ProjectDeleteMutation, map[string]any{"id": projectID})
			if err != nil {
				return wbClassifyDeleteError("projectDelete", err, flags)
			}
			id, err := wbDecodeDeletePayload(resp, "projectDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "project_deleted", map[string]any{"id": firstNonEmpty(id, projectID)},
				fmt.Sprintf("Deleted project %s", firstNonEmpty(id, projectID)))
		},
	}
	return cmd
}

func newProjectsAddLabelCmd(flags *rootFlags) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:     "add-label <project-id>",
		Short:   "Attach a project label to a Linear project",
		Long:    "Call the projectAddLabel mutation for exactly one project label. The project's other labels are untouched.",
		Example: "  linear-pp-cli projects add-label <project-uuid> --label <project-label-uuid> --agent",
		Args:    cobra.MaximumNArgs(1),
		RunE: projectLabelEdgeRunE(flags, &label, projectLabelEdge{
			event:    "would_add_project_label",
			mutation: "projectAddLabel",
			document: client.ProjectAddLabelMutation,
		}),
	}
	cmd.Flags().StringVar(&label, "label", "", "Project label UUID (required)")
	return cmd
}

func newProjectsRemoveLabelCmd(flags *rootFlags) *cobra.Command {
	var label string
	cmd := &cobra.Command{
		Use:     "remove-label <project-id>",
		Short:   "Detach a project label from a Linear project",
		Long:    "Call the projectRemoveLabel mutation for exactly one project label. The project's other labels are untouched.",
		Example: "  linear-pp-cli projects remove-label <project-uuid> --label <project-label-uuid> --agent",
		Args:    cobra.MaximumNArgs(1),
		RunE: projectLabelEdgeRunE(flags, &label, projectLabelEdge{
			event:    "would_remove_project_label",
			mutation: "projectRemoveLabel",
			document: client.ProjectRemoveLabelMutation,
		}),
	}
	cmd.Flags().StringVar(&label, "label", "", "Project label UUID (required)")
	return cmd
}

// projectLabelEdge describes one of the two dedicated label-edge mutations.
// Linear models project label attachment as projectAddLabel and
// projectRemoveLabel rather than as a ProjectUpdateInput field, so a single
// label can be attached or detached without rewriting the label set.
type projectLabelEdge struct {
	event    string
	mutation string
	document string
}

func projectLabelEdgeRunE(flags *rootFlags, label *string, edge projectLabelEdge) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || *label == "" {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return usageErr(fmt.Errorf("<project-id> is required"))
			}
			return usageErr(fmt.Errorf("--label is required (project label UUID)"))
		}
		projectID := args[0]
		if !store.IsUUID(projectID) {
			return portfolioUUIDUsageErr(flags, "<project-id>", projectID, "use 'projects resolve <name>' to find the UUID")
		}
		if !store.IsUUID(*label) {
			return usageErr(fmt.Errorf("--label expects a project label UUID, got %q", *label))
		}
		vars := map[string]any{"id": projectID, "labelId": *label}
		if flags.dryRun {
			return renderMutationDryRun(cmd, flags, edge.event, edge.mutation, vars)
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		resp, err := c.Mutate(edge.document, vars)
		if err != nil {
			return classifyMutationError(edge.mutation, err, flags, nil)
		}
		project, err := extractMutationObject(resp, edge.mutation, "project")
		if err != nil {
			return err
		}
		return renderLiveObject(cmd, flags, project, "projects")
	}
}

// wbResolveTeamsLocal maps team keys to UUIDs using the local store. Values
// that are already UUIDs pass through, and values the store cannot resolve are
// returned in the second result so the caller can retry them live after the
// dry-run gate. This keeps --dry-run free of network calls, exactly as
// issues create does.
func wbResolveTeamsLocal(dbPath string, teams []string) ([]string, []string) {
	resolved := make([]string, 0, len(teams))
	var unresolved []string
	var db *store.Store
	if opened, err := store.Open(resolveDBPath(dbPath)); err == nil {
		db = opened
		defer db.Close()
	}
	for _, team := range teams {
		if store.IsUUID(team) {
			resolved = append(resolved, team)
			continue
		}
		if db != nil {
			if id, ok := resolveTeamID(db, team); ok {
				resolved = append(resolved, id)
				continue
			}
		}
		resolved = append(resolved, team)
		unresolved = append(unresolved, team)
	}
	return resolved, unresolved
}

// wbResolveTeamsLive resolves any remaining team keys through the API.
func wbResolveTeamsLive(c *client.Client, teams []string) ([]string, error) {
	out := make([]string, 0, len(teams))
	for _, team := range teams {
		if store.IsUUID(team) {
			out = append(out, team)
			continue
		}
		id, err := resolveTeamIDLive(c, team)
		if err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// wbCheckTimelessDate validates a Linear TimelessDate argument. Linear rejects
// anything other than YYYY-MM-DD, and catching it here turns a code-5 API
// error into a code-2 usage error the caller can act on.
func wbCheckTimelessDate(flag, value string) error {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return usageErr(fmt.Errorf("%s expects a date as YYYY-MM-DD, got %q", flag, value))
	}
	return nil
}

// wbConfirm gates a destructive mutation. --yes (implied by --agent) skips the
// prompt, --no-input turns a missing --yes into a typed usage error instead of
// a hang, and anything other than y/yes aborts.
func wbConfirm(cmd *cobra.Command, flags *rootFlags, prompt string) error {
	if flags != nil && flags.yes {
		return nil
	}
	if flags != nil && flags.noInput {
		return usageErr(fmt.Errorf("%s needs explicit confirmation: pass --yes (or remove --no-input)", prompt))
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s? [y/N] ", prompt)
	var answer string
	fmt.Fscanln(cmd.InOrStdin(), &answer)
	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		return fmt.Errorf("aborted")
	}
	return nil
}

// wbDecodeDeletePayload reads the delete payloads Linear returns. DeletePayload
// carries entityId, while the archive payloads (projectDelete) carry an entity
// object, so both shapes are accepted and the deleted id is returned.
func wbDecodeDeletePayload(resp json.RawMessage, mutationKey string) (string, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(resp, &root); err != nil {
		return "", fmt.Errorf("parsing %s response: %w", mutationKey, err)
	}
	raw, ok := root[mutationKey]
	if !ok {
		return "", fmt.Errorf("%s response missing %q", mutationKey, mutationKey)
	}
	var payload struct {
		Success  bool   `json:"success"`
		EntityID string `json:"entityId"`
		Entity   *struct {
			ID string `json:"id"`
		} `json:"entity"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return "", fmt.Errorf("parsing %s payload: %w", mutationKey, err)
	}
	if !payload.Success {
		return "", apiErr(fmt.Errorf("Linear reported %s success=false", mutationKey))
	}
	if payload.EntityID != "" {
		return payload.EntityID, nil
	}
	if payload.Entity != nil {
		return payload.Entity.ID, nil
	}
	return "", nil
}

// wbRenderMutationEvent emits the completion line for mutations whose payload
// has no object to render (the deletes). Machine consumers get an event
// object, humans get one prose line.
func wbRenderMutationEvent(cmd *cobra.Command, flags *rootFlags, event string, fields map[string]any, prose string) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		out := map[string]any{"event": event}
		for key, value := range fields {
			out[key] = value
		}
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Fprintln(cmd.OutOrStdout(), prose)
	return nil
}

// wbClassifyCreateError honours --idempotent for creates. Linear answers every
// GraphQL call with HTTP 200 and reports duplicates in the error prose, so the
// HTTP 409 branch in classifyAPIError never fires for a GraphQL create.
func wbClassifyCreateError(operation string, err error, flags *rootFlags) error {
	if err == nil {
		return nil
	}
	if flags != nil && flags.idempotent && wbMentionsDuplicate(err) {
		return writeNoop(flags, "already_exists", "already exists (no-op)")
	}
	return classifyMutationError(operation, err, flags, nil)
}

// wbClassifyDeleteError honours --ignore-missing for deletes, matching the
// classifyDeleteError contract for REST endpoints.
func wbClassifyDeleteError(operation string, err error, flags *rootFlags) error {
	if err == nil {
		return nil
	}
	if flags != nil && flags.ignoreMissing && wbMentionsMissing(err) {
		return writeNoop(flags, "already_deleted", "already deleted (no-op)")
	}
	return classifyMutationError(operation, err, flags, nil)
}

func wbMentionsDuplicate(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate")
}

func wbMentionsMissing(err error) bool {
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "entity not found") || strings.Contains(msg, "not found") || strings.Contains(msg, "could not find")
}

// wbJSONObject reads a JSON object argument from either an inline flag or a
// file. Linear's JSON and JSONObject inputs (templateData, filterData) are too
// unwieldy to pass inline in a shell, so the file form is the ergonomic path
// and the inline form stays available for short payloads.
func wbJSONObject(inlineFlag, inline, fileFlag, file, label string) (map[string]any, bool, error) {
	if inline != "" && file != "" {
		return nil, false, usageErr(fmt.Errorf("pass either --%s or --%s, not both", inlineFlag, fileFlag))
	}
	raw := inline
	if file != "" {
		contents, err := os.ReadFile(file)
		if err != nil {
			return nil, false, usageErr(fmt.Errorf("reading %s from %s: %w", label, file, err))
		}
		raw = string(contents)
	}
	if strings.TrimSpace(raw) == "" {
		return nil, false, nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false, usageErr(fmt.Errorf("%s must be a JSON object: %w", label, err))
	}
	return parsed, true, nil
}
