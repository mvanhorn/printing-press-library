package cli

// initiatives_write.go closes the initiative half of GAP-041: the `initiatives`
// family could be read, searched, resolved and rolled up, but an initiative
// could never be created, edited, archived, deleted, or linked to a project
// without dropping to raw GraphQL. Initiatives are the live replacement for the
// deprecated roadmaps the CLI still wraps, so a read-only initiative surface
// left the portfolio layer of the workspace effectively frozen.
//
// The leaves are registered on the existing `initiatives` parent in
// linear_groups.go.
//
// Scope note: initiative labels, initiative relations and initiative updates
// are separate families in the same gap row and are not shipped here.

import (
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// initiativeStatusValues are the InitiativeStatus enum members. They are schema
// vocabulary, not workspace data, so hard-coding them is safe across
// workspaces and lets a wrong value exit 2 before a round trip.
var initiativeStatusValues = []string{"Proposed", "Planned", "Active", "Completed", "Canceled"}

// normalizeInitiativeStatus accepts the enum in any case and returns the exact
// spelling Linear requires, because GraphQL enum values are case sensitive.
func normalizeInitiativeStatus(value string) (string, error) {
	for _, candidate := range initiativeStatusValues {
		if strings.EqualFold(candidate, value) {
			return candidate, nil
		}
	}
	return "", usageErr(fmt.Errorf("invalid --status %q: must be one of %s", value, strings.Join(initiativeStatusValues, ", ")))
}

// initiativeWriteFlags carries the InitiativeCreateInput and
// InitiativeUpdateInput fields this CLI exposes. --description is named for
// consistency with projects and issues, but it is sent as the `content` input
// field: InitiativeCreateInput.description and InitiativeUpdateInput.description
// are accepted and silently dropped by Linear (live probe goal-cov-r2,
// 2026-08-11: create and update both returned success with description null on
// an independent re-read, while the same write through `content` persisted).
type initiativeWriteFlags struct {
	name        string
	description string
	owner       string
	targetDate  string
	status      string
}

func (i *initiativeWriteFlags) bind(cmd *cobra.Command) {
	cmd.Flags().StringVar(&i.name, "name", "", "Initiative name")
	cmd.Flags().StringVar(&i.description, "description", "", "Initiative description. Written to the initiative content body because Linear silently drops the API description field")
	cmd.Flags().StringVar(&i.owner, "owner", "", "Initiative owner user UUID")
	cmd.Flags().StringVar(&i.targetDate, "target-date", "", "Estimated completion date as YYYY-MM-DD")
	cmd.Flags().StringVar(&i.status, "status", "", "Initiative status: Proposed, Planned, Active, Completed, or Canceled")
}

func (i *initiativeWriteFlags) input(cmd *cobra.Command) (map[string]any, error) {
	input := map[string]any{}
	setOptionalString(input, "name", i.name)
	// Deliberately keyed to `content`, not `description`. See the type comment:
	// the description input field is a dead write on both initiative mutations.
	setChangedString(cmd, input, "description", "content", i.description)
	if i.owner != "" {
		if !store.IsUUID(i.owner) {
			return nil, usageErr(fmt.Errorf("--owner expects a user UUID, got %q", i.owner))
		}
		input["ownerId"] = i.owner
	}
	if i.targetDate != "" {
		if err := wbCheckTimelessDate("--target-date", i.targetDate); err != nil {
			return nil, err
		}
		input["targetDate"] = i.targetDate
	}
	if i.status != "" {
		status, err := normalizeInitiativeStatus(i.status)
		if err != nil {
			return nil, err
		}
		input["status"] = status
	}
	return input, nil
}

// pwResolveInitiativeID maps the positional initiative reference to a UUID. A
// UUID passes through untouched, anything else is resolved live by exact name
// through the same path `initiatives resolve` uses, so an ambiguous name exits
// with the candidate list instead of mutating the wrong initiative.
func pwResolveInitiativeID(c graphqlQueryer, ref string, flags *rootFlags) (string, error) {
	if store.IsUUID(ref) {
		return ref, nil
	}
	initiative, err := resolveInitiativeNameLive(c, ref, flags)
	if err != nil {
		return "", err
	}
	return initiative.ID, nil
}

func newInitiativesCreateCmd(flags *rootFlags) *cobra.Command {
	write := initiativeWriteFlags{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Linear initiative",
		Long: `Create an initiative via the initiativeCreate mutation. --name is the only
field InitiativeCreateInput requires.

--description is written to the initiative's content body. Linear accepts the
description input field on both initiative mutations and then drops it, so this
CLI sends the text where it survives.

Link projects to the new initiative with 'initiatives link-project'.`,
		Example: `  linear-pp-cli initiatives create --name "Backlog Governance" --status Planned --agent
  linear-pp-cli initiatives create --name "Backlog Governance" --target-date 2026-12-31 --dry-run --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if write.name == "" {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("--name is required"))
			}
			input, err := write.input(cmd)
			if err != nil {
				return err
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_initiative", "initiativeCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.InitiativeCreateMutation, map[string]any{"input": input})
			if err != nil {
				return classifyGraphQLCreateError("initiativeCreate", err, flags)
			}
			initiative, err := extractMutationObject(resp, "initiativeCreate", "initiative")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, initiative, "initiatives")
		},
	}
	write.bind(cmd)
	return cmd
}

func newInitiativesUpdateCmd(flags *rootFlags) *cobra.Command {
	write := initiativeWriteFlags{}
	cmd := &cobra.Command{
		Use:     "update <initiative>",
		Aliases: []string{"edit"},
		Short:   "Update a Linear initiative",
		Long: `Edit an initiative via the initiativeUpdate mutation. At least one field flag
is required.

--description is written to the initiative's content body. Linear accepts the
description input field on both initiative mutations and then drops it, so this
CLI sends the text where it survives.

The initiative reference is a UUID or an exact initiative name, resolved the
same way 'initiatives resolve' resolves it.`,
		Example: `  linear-pp-cli initiatives update <initiative-uuid> --status Active --agent
  linear-pp-cli initiatives update "Backlog Governance" --target-date 2027-03-31 --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<initiative> is required"))
			}
			input, err := write.input(cmd)
			if err != nil {
				return err
			}
			if len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass at least one field to update (--name, --description, --owner, --target-date, --status)"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_initiative", "initiativeUpdate", map[string]any{"initiative": args[0], "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			initiativeID, err := pwResolveInitiativeID(c, args[0], flags)
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.InitiativeUpdateMutation, map[string]any{"id": initiativeID, "input": input})
			if err != nil {
				return classifyGraphQLMutationError("initiativeUpdate", err, flags)
			}
			initiative, err := extractMutationObject(resp, "initiativeUpdate", "initiative")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, initiative, "initiatives")
		},
	}
	write.bind(cmd)
	return cmd
}

func newInitiativesArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <initiative>",
		Short: "Archive a Linear initiative",
		Long: `Archive an initiative via the initiativeArchive mutation. Archiving is
reversible with 'initiatives unarchive', so it is not gated behind a
confirmation. For the irreversible removal see 'initiatives delete'.`,
		Example: `  linear-pp-cli initiatives archive <initiative-uuid> --agent
  linear-pp-cli initiatives archive "Backlog Governance" --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<initiative> is required"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_archive_initiative", "initiativeArchive", map[string]any{"initiative": args[0]})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			initiativeID, err := pwResolveInitiativeID(c, args[0], flags)
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.InitiativeArchiveMutation, map[string]any{"id": initiativeID})
			if err != nil {
				return classifyGraphQLMutationError("initiativeArchive", err, flags)
			}
			initiative, err := extractMutationObject(resp, "initiativeArchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, initiative, "initiatives")
		},
	}
	return cmd
}

func newInitiativesUnarchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unarchive <initiative>",
		Short:   "Restore an archived Linear initiative",
		Long:    `Restore an archived initiative via the initiativeUnarchive mutation.`,
		Example: `  linear-pp-cli initiatives unarchive <initiative-uuid> --agent`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<initiative> is required"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_unarchive_initiative", "initiativeUnarchive", map[string]any{"initiative": args[0]})
			}
			// An archived initiative is not returned by the name search, so the
			// unarchive path takes the UUID as given rather than resolving it.
			initiativeID := args[0]
			if !store.IsUUID(initiativeID) {
				return usageErr(fmt.Errorf("<initiative> expects an initiative UUID here, got %q: an archived initiative is not resolvable by name", initiativeID))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.InitiativeUnarchiveMutation, map[string]any{"id": initiativeID})
			if err != nil {
				return classifyGraphQLMutationError("initiativeUnarchive", err, flags)
			}
			initiative, err := extractMutationObject(resp, "initiativeUnarchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, initiative, "initiatives")
		},
	}
	return cmd
}

func newInitiativesDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete <initiative>",
		Short: "Delete a Linear initiative",
		Long: `Delete an initiative via the initiativeDelete mutation. Projects linked to the
initiative are not deleted, they lose the initiative association.

The schema has no untrash counterpart for an initiative, so prefer
'initiatives archive' when the initiative may be wanted back. Deletion is
confirmed interactively unless --yes is passed, --agent implies --yes, and with
--ignore-missing an already-deleted initiative exits 0 as a no-op.`,
		Example: `  linear-pp-cli initiatives delete <initiative-uuid> --yes --agent
  linear-pp-cli initiatives delete <initiative-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<initiative> is required"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_initiative", "initiativeDelete", map[string]any{"initiative": args[0]})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			initiativeID, err := pwResolveInitiativeID(c, args[0], flags)
			if err != nil {
				return err
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Delete initiative %s?", initiativeID)); err != nil {
				return err
			}
			resp, err := c.Mutate(client.InitiativeDeleteMutation, map[string]any{"id": initiativeID})
			if err != nil {
				return classifyGraphQLMutationError("initiativeDelete", err, flags)
			}
			id, err := extractDeletedEntityID(resp, "initiativeDelete")
			if err != nil {
				return err
			}
			return renderMutationEvent(cmd, flags, "initiative_deleted", map[string]any{"id": firstNonEmpty(id, initiativeID)})
		},
	}
	return cmd
}

func newInitiativesLinkProjectCmd(flags *rootFlags) *cobra.Command {
	var project, projectName string
	var sortOrder float64
	cmd := &cobra.Command{
		Use:   "link-project <initiative>",
		Short: "Link a project to a Linear initiative",
		Long: `Link a project to an initiative via the initiativeToProjectCreate mutation.
InitiativeToProjectCreateInput requires both endpoints, so pass the initiative
positionally and the project with --project or --project-name.

Reverse it with 'initiatives unlink-project'.`,
		Example: `  linear-pp-cli initiatives link-project <initiative-uuid> --project <project-uuid> --agent
  linear-pp-cli initiatives link-project "Backlog Governance" --project-name "Q4 Cleanup" --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (project == "" && projectName == "") {
				if dryRunOK(flags) {
					return nil
				}
				if len(args) == 0 {
					return usageErr(fmt.Errorf("<initiative> is required"))
				}
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name>"))
			}
			if flags.dryRun {
				dryInput := map[string]any{
					"initiativeId": args[0],
					"projectId":    firstNonEmpty(project, projectName),
				}
				if cmd.Flags().Changed("sort-order") {
					dryInput["sortOrder"] = sortOrder
				}
				return renderMutationDryRun(cmd, flags, "would_link_initiative_project", "initiativeToProjectCreate", map[string]any{"input": dryInput})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			initiativeID, err := pwResolveInitiativeID(c, args[0], flags)
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return err
			}
			if projectID == "" {
				return usageErr(fmt.Errorf("could not resolve project; pass --project <uuid> or --project-name <name>"))
			}
			input := map[string]any{"initiativeId": initiativeID, "projectId": projectID}
			if cmd.Flags().Changed("sort-order") {
				input["sortOrder"] = sortOrder
			}
			resp, err := c.Mutate(client.InitiativeToProjectCreateMutation, map[string]any{"input": input})
			if err != nil {
				return classifyGraphQLCreateError("initiativeToProjectCreate", err, flags)
			}
			link, err := extractMutationObject(resp, "initiativeToProjectCreate", "initiativeToProject")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, link, "initiative-to-projects")
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Resolve the project by exact name")
	cmd.Flags().Float64Var(&sortOrder, "sort-order", 0, "Explicit sort order for the project within the initiative")
	return cmd
}

func newInitiativesUnlinkProjectCmd(flags *rootFlags) *cobra.Command {
	var project, projectName string
	cmd := &cobra.Command{
		Use:   "unlink-project <initiative>",
		Short: "Unlink a project from a Linear initiative",
		Long: `Remove a project from an initiative via the initiativeToProjectDelete
mutation. That mutation takes the id of the link row rather than the pair of
endpoints, so the link is looked up first through the project's own
initiativeToProjects connection.

Neither the project nor the initiative is deleted. The unlink is confirmed
interactively unless --yes is passed, --agent implies --yes, and with
--ignore-missing an absent link exits 0 as a no-op.`,
		Example: `  linear-pp-cli initiatives unlink-project <initiative-uuid> --project <project-uuid> --yes --agent
  linear-pp-cli initiatives unlink-project "Backlog Governance" --project-name "Q4 Cleanup" --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 || (project == "" && projectName == "") {
				if dryRunOK(flags) {
					return nil
				}
				if len(args) == 0 {
					return usageErr(fmt.Errorf("<initiative> is required"))
				}
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name>"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_unlink_initiative_project", "initiativeToProjectDelete", map[string]any{
					"initiative": args[0],
					"project":    firstNonEmpty(project, projectName),
				})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			initiativeID, err := pwResolveInitiativeID(c, args[0], flags)
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return err
			}
			if projectID == "" {
				return usageErr(fmt.Errorf("could not resolve project; pass --project <uuid> or --project-name <name>"))
			}
			linkID, err := pwFindInitiativeProjectLink(c, projectID, initiativeID)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			if linkID == "" {
				if flags != nil && flags.ignoreMissing {
					return writeNoop(flags, "already_deleted", "already deleted (no-op)")
				}
				return notFoundErr(fmt.Errorf("project %s is not linked to initiative %s", projectID, initiativeID))
			}
			if err := confirmMutation(cmd, flags, fmt.Sprintf("Unlink project %s from initiative %s?", projectID, initiativeID)); err != nil {
				return err
			}
			resp, err := c.Mutate(client.InitiativeToProjectDeleteMutation, map[string]any{"id": linkID})
			if err != nil {
				return classifyGraphQLMutationError("initiativeToProjectDelete", err, flags)
			}
			id, err := extractDeletedEntityID(resp, "initiativeToProjectDelete")
			if err != nil {
				return err
			}
			return renderMutationEvent(cmd, flags, "initiative_project_unlinked", map[string]any{
				"id":            firstNonEmpty(id, linkID),
				"initiative_id": initiativeID,
				"project_id":    projectID,
			})
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Resolve the project by exact name")
	return cmd
}

// pwFindInitiativeProjectLink returns the InitiativeToProject id joining
// projectID to initiativeID, or "" when there is no such link. Query
// .initiativeToProjects takes no filter argument, so the scoped path is the
// project's own connection, which is short by construction: a project belongs
// to a handful of initiatives, not to the whole workspace's worth.
func pwFindInitiativeProjectLink(c graphqlQueryer, projectID, initiativeID string) (string, error) {
	var after any
	for {
		var resp struct {
			Project struct {
				ID                   string `json:"id"`
				InitiativeToProjects struct {
					Nodes []struct {
						ID         string `json:"id"`
						Initiative struct {
							ID string `json:"id"`
						} `json:"initiative"`
					} `json:"nodes"`
					PageInfo pageInfo `json:"pageInfo"`
				} `json:"initiativeToProjects"`
			} `json:"project"`
		}
		vars := map[string]any{"id": projectID, "first": 100, "after": after}
		if err := c.QueryInto(client.ProjectInitiativeLinksQuery, vars, &resp); err != nil {
			return "", err
		}
		for _, node := range resp.Project.InitiativeToProjects.Nodes {
			if node.Initiative.ID == initiativeID {
				return node.ID, nil
			}
		}
		if !resp.Project.InitiativeToProjects.PageInfo.HasNextPage || resp.Project.InitiativeToProjects.PageInfo.EndCursor == "" {
			return "", nil
		}
		after = resp.Project.InitiativeToProjects.PageInfo.EndCursor
	}
}
