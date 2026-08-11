package cli

// favorites_write.go closes GAP-043: favorites were read-only, so the sidebar a
// human curates by hand could be read but never written. The leaves are
// registered on the existing promoted `favorites` command in
// promoted_favorites.go, so `favorites <id>` keeps working.
//
// FavoriteCreateInput is one of the inputs that drifted live-only (the local
// schema copy is missing aiConversationId, initiativeLabelId, pipelineTab, and
// releaseNoteId), so every target flag below was taken from the live inventory
// rather than from the vendored schema.

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// favoriteTarget binds one --flag to the FavoriteCreateInput field it sets.
// Exactly one target may be set per favoriteCreate call: Linear derives
// Favorite.type from whichever id was supplied.
type favoriteTarget struct {
	flag  string
	field string
	help  string
	uuid  bool
	value string
}

// favoriteTargets is the full set of entity targets this CLI can favorite.
// Every field name was verified against FavoriteCreateInput in
// api-inventory.json.
func favoriteTargets() []favoriteTarget {
	return []favoriteTarget{
		{flag: "issue", field: "issueId", help: "Favorite an issue by identifier or UUID"},
		{flag: "project", field: "projectId", help: "Favorite a project by UUID", uuid: true},
		{flag: "document", field: "documentId", help: "Favorite a document by UUID", uuid: true},
		{flag: "label", field: "labelId", help: "Favorite an issue label by UUID", uuid: true},
		{flag: "project-label", field: "projectLabelId", help: "Favorite a project label by UUID", uuid: true},
		{flag: "initiative", field: "initiativeId", help: "Favorite an initiative by UUID", uuid: true},
		{flag: "initiative-label", field: "initiativeLabelId", help: "Favorite an initiative label by UUID", uuid: true},
		{flag: "custom-view", field: "customViewId", help: "Favorite a custom view by UUID", uuid: true},
		{flag: "cycle", field: "cycleId", help: "Favorite a cycle by UUID", uuid: true},
		{flag: "user", field: "userId", help: "Favorite a user by UUID", uuid: true},
		{flag: "team", field: "teamId", help: "Favorite a team by key or UUID"},
		{flag: "facet", field: "facetId", help: "Favorite a facet by UUID", uuid: true},
		{flag: "customer", field: "customerId", help: "Favorite a customer by UUID", uuid: true},
		{flag: "dashboard", field: "dashboardId", help: "Favorite a dashboard by UUID", uuid: true},
		{flag: "pull-request", field: "pullRequestId", help: "Favorite a pull request by UUID", uuid: true},
		{flag: "release", field: "releaseId", help: "Favorite a release by UUID", uuid: true},
		{flag: "release-pipeline", field: "releasePipelineId", help: "Favorite a release pipeline by UUID", uuid: true},
		{flag: "release-note", field: "releaseNoteId", help: "Favorite a release note by UUID", uuid: true},
		{flag: "ai-conversation", field: "aiConversationId", help: "Favorite an AI conversation by UUID", uuid: true},
	}
}

func newFavoritesCreateCmd(flags *rootFlags) *cobra.Command {
	targets := favoriteTargets()
	var predefinedViewType, predefinedViewTeam string
	var projectTab, initiativeTab, pipelineTab string
	var folderName, parent string
	var sortOrder float64
	var dbPath string
	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"add"},
		Short:   "Favorite one Linear entity",
		Long: `Favorite exactly one entity via the favoriteCreate mutation. Linear derives the
favorite's type from whichever target id was supplied, so passing two targets is
a usage error.

--issue accepts an issue identifier or UUID and --team accepts a team key or
UUID. Every other target is a UUID, which the read leaves of this CLI already
print.

The tab flags refine a target rather than select one: --project-tab pairs with
--project, --initiative-tab with --initiative, and --pipeline-tab with
--release-pipeline. --predefined-view-type with --predefined-view-team favorites
one of Linear's built-in team views instead of an entity.`,
		Example: `  linear-pp-cli favorites create --issue ENG-123 --agent
  linear-pp-cli favorites create --project <project-uuid> --folder-name "Portfolio" --agent
  linear-pp-cli favorites create --issue ENG-123 --dry-run --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			selected := make([]favoriteTarget, 0, 1)
			for i := range targets {
				if targets[i].value != "" {
					selected = append(selected, targets[i])
				}
			}
			hasPredefined := predefinedViewType != ""
			if len(selected) == 0 && !hasPredefined {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass exactly one target: %s, or --predefined-view-type", favoriteTargetFlagList(targets)))
			}
			if len(selected) > 1 {
				names := make([]string, 0, len(selected))
				for _, target := range selected {
					names = append(names, "--"+target.flag)
				}
				return usageErr(fmt.Errorf("pass exactly one favorite target, got %s", strings.Join(names, " and ")))
			}
			if len(selected) == 1 && hasPredefined {
				return usageErr(fmt.Errorf("--predefined-view-type favorites a built-in view, so it cannot be combined with --%s", selected[0].flag))
			}

			input := map[string]any{}
			var issueRef string
			var pendingTeam string
			for _, target := range selected {
				if target.uuid && !store.IsUUID(target.value) {
					return usageErr(fmt.Errorf("--%s expects a UUID, got %q", target.flag, target.value))
				}
				switch target.flag {
				case "issue":
					issueRef = target.value
					input["issueId"] = target.value
				case "team":
					teamIDs, unresolved := wbResolveTeamsLocal(dbPath, []string{target.value})
					input["teamId"] = teamIDs[0]
					if len(unresolved) > 0 {
						pendingTeam = teamIDs[0]
					}
				default:
					input[target.field] = target.value
				}
			}
			if hasPredefined {
				input["predefinedViewType"] = predefinedViewType
				if predefinedViewTeam != "" {
					teamIDs, unresolved := wbResolveTeamsLocal(dbPath, []string{predefinedViewTeam})
					input["predefinedViewTeamId"] = teamIDs[0]
					if len(unresolved) > 0 {
						pendingTeam = teamIDs[0]
					}
				}
			}
			if projectTab != "" {
				input["projectTab"] = projectTab
			}
			if initiativeTab != "" {
				input["initiativeTab"] = initiativeTab
			}
			if pipelineTab != "" {
				input["pipelineTab"] = pipelineTab
			}
			if folderName != "" {
				input["folderName"] = folderName
			}
			if parent != "" {
				if !store.IsUUID(parent) {
					return usageErr(fmt.Errorf("--parent expects a favorite UUID, got %q", parent))
				}
				input["parentId"] = parent
			}
			if cmd.Flags().Changed("sort-order") {
				input["sortOrder"] = sortOrder
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_create_favorite", "favoriteCreate", map[string]any{"input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if issueRef != "" && !store.IsUUID(issueRef) {
				issueID, err := resolveIssueID(c, issueRef)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["issueId"] = issueID
			}
			if pendingTeam != "" {
				resolved, err := wbResolveTeamsLive(c, []string{pendingTeam})
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				if _, ok := input["predefinedViewTeamId"]; ok {
					input["predefinedViewTeamId"] = resolved[0]
				} else {
					input["teamId"] = resolved[0]
				}
			}
			resp, err := c.Mutate(client.FavoriteCreateMutation, map[string]any{"input": input})
			if err != nil {
				return wbClassifyCreateError("favoriteCreate", err, flags)
			}
			favorite, err := extractMutationObject(resp, "favoriteCreate", "favorite")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, favorite, "favorites")
		},
	}
	for i := range targets {
		cmd.Flags().StringVar(&targets[i].value, targets[i].flag, "", targets[i].help)
	}
	cmd.Flags().StringVar(&predefinedViewType, "predefined-view-type", "", "Favorite one of Linear's built-in views by its type string")
	cmd.Flags().StringVar(&predefinedViewTeam, "predefined-view-team", "", "Team key or UUID the predefined view belongs to")
	cmd.Flags().StringVar(&projectTab, "project-tab", "", "ProjectTab value to open when the project favorite is clicked")
	cmd.Flags().StringVar(&initiativeTab, "initiative-tab", "", "InitiativeTab value to open when the initiative favorite is clicked")
	cmd.Flags().StringVar(&pipelineTab, "pipeline-tab", "", "PipelineTab value to open when the release pipeline favorite is clicked")
	cmd.Flags().StringVar(&folderName, "folder-name", "", "Group the favorite under this sidebar folder")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent favorite UUID (nests this favorite inside a folder favorite)")
	cmd.Flags().Float64Var(&sortOrder, "sort-order", 0, "Explicit sort order in the sidebar")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path used to resolve team keys offline")
	return cmd
}

// favoriteTargetFlagList renders the target flags for the usage error, sorted so
// the message is stable.
func favoriteTargetFlagList(targets []favoriteTarget) string {
	names := make([]string, 0, len(targets))
	for _, target := range targets {
		names = append(names, "--"+target.flag)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func newFavoritesUpdateCmd(flags *rootFlags) *cobra.Command {
	var folderName, parent string
	var sortOrder float64
	cmd := &cobra.Command{
		Use:     "update <favorite-id>",
		Aliases: []string{"edit"},
		Short:   "Reorder or refolder a favorite",
		Long: `Edit a favorite via the favoriteUpdate mutation. FavoriteUpdateInput carries
only sortOrder, parentId, and folderName, so a favorite's target cannot be
changed: delete it and create the new one instead.

At least one field flag is required.`,
		Example: `  linear-pp-cli favorites update <favorite-uuid> --sort-order 1.5 --agent
  linear-pp-cli favorites update <favorite-uuid> --folder-name "Portfolio" --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<favorite-id> is required"))
			}
			favoriteID := args[0]
			if !store.IsUUID(favoriteID) {
				return usageErr(fmt.Errorf("<favorite-id> expects a favorite UUID, got %q", favoriteID))
			}
			input := map[string]any{}
			if cmd.Flags().Changed("folder-name") {
				input["folderName"] = folderName
			}
			if parent != "" {
				if !store.IsUUID(parent) {
					return usageErr(fmt.Errorf("--parent expects a favorite UUID, got %q", parent))
				}
				input["parentId"] = parent
			}
			if cmd.Flags().Changed("sort-order") {
				input["sortOrder"] = sortOrder
			}
			if len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("pass at least one field to update (--sort-order, --parent, --folder-name)"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_favorite", "favoriteUpdate", map[string]any{"id": favoriteID, "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.FavoriteUpdateMutation, map[string]any{"id": favoriteID, "input": input})
			if err != nil {
				return classifyMutationError("favoriteUpdate", err, flags, nil)
			}
			favorite, err := extractMutationObject(resp, "favoriteUpdate", "favorite")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, favorite, "favorites")
		},
	}
	cmd.Flags().StringVar(&folderName, "folder-name", "", "Move the favorite into this sidebar folder (empty string clears it)")
	cmd.Flags().StringVar(&parent, "parent", "", "Parent favorite UUID")
	cmd.Flags().Float64Var(&sortOrder, "sort-order", 0, "Explicit sort order in the sidebar")
	return cmd
}

func newFavoritesDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "delete <favorite-id>",
		Aliases: []string{"remove"},
		Short:   "Unfavorite an entity",
		Long: `Delete a favorite via the favoriteDelete mutation. The favorited entity itself
is untouched.

Deletion is confirmed interactively unless --yes is passed. --agent implies
--yes. With --ignore-missing an already-deleted favorite exits 0 as a no-op.`,
		Example: `  linear-pp-cli favorites delete <favorite-uuid> --yes --agent
  linear-pp-cli favorites delete <favorite-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<favorite-id> is required"))
			}
			favoriteID := args[0]
			if !store.IsUUID(favoriteID) {
				return usageErr(fmt.Errorf("<favorite-id> expects a favorite UUID, got %q", favoriteID))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_delete_favorite", "favoriteDelete", map[string]any{"id": favoriteID})
			}
			if err := wbConfirm(cmd, flags, fmt.Sprintf("Delete favorite %s", favoriteID)); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.FavoriteDeleteMutation, map[string]any{"id": favoriteID})
			if err != nil {
				return wbClassifyDeleteError("favoriteDelete", err, flags)
			}
			id, err := wbDecodeDeletePayload(resp, "favoriteDelete")
			if err != nil {
				return err
			}
			return wbRenderMutationEvent(cmd, flags, "favorite_deleted", map[string]any{"id": firstNonEmpty(id, favoriteID)},
				fmt.Sprintf("Deleted favorite %s", firstNonEmpty(id, favoriteID)))
		},
	}
	return cmd
}
