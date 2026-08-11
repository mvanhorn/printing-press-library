package cli

// project_updates_write.go closes GAP-047: a project update could be posted but
// never corrected or removed, so a typo in a status post was permanent and a
// stale update stayed on the project forever.
//
// The removal verb is projectUpdateArchive. projectUpdateDelete exists but is
// deprecated in favour of it, so it is not wired here, and projectUpdateUnarchive
// is the matching restore.
//
// The leaves are registered on the existing `project-updates` parent in
// project_updates.go.

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// validateProjectUpdateHealth checks a --health value against the
// ProjectUpdateHealthType enum. The spelling is exact, matching
// `project-updates create`, so the two verbs never disagree about what is
// valid.
func validateProjectUpdateHealth(health string) error {
	switch health {
	case "onTrack", "atRisk", "offTrack":
		// valid enum values from Linear GraphQL schema
		return nil
	default:
		return usageErr(fmt.Errorf("invalid --health value %q: must be onTrack, atRisk, or offTrack", health))
	}
}

func newProjectUpdatesUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyFlag, bodyFile string
	var bodyStdin bool
	var health string
	cmd := &cobra.Command{
		Use:     "update <project-update-id>",
		Aliases: []string{"edit"},
		Short:   "Edit a posted Linear project update",
		Long: `Edit a project update via the projectUpdateUpdate mutation. At least one of
--body, --body-file, --body-stdin or --health is required.

Use --body-file or --body-stdin for multi-line Markdown so shell metacharacters
stay literal. The deprecated isDiffHidden field is not exposed.`,
		Example: `  linear-pp-cli project-updates update <project-update-uuid> --health atRisk --agent
  linear-pp-cli project-updates update <project-update-uuid> --body-file /tmp/update.md --agent`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<project-update-id> is required"))
			}
			updateID := args[0]
			if !store.IsUUID(updateID) {
				return usageErr(fmt.Errorf("<project-update-id> expects a project update UUID, got %q", updateID))
			}
			if health != "" {
				if err := validateProjectUpdateHealth(health); err != nil {
					return err
				}
			}
			body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "body",
				Inline:     bodyFlag,
				FileFlag:   "body-file",
				File:       bodyFile,
				StdinFlag:  "body-stdin",
				Stdin:      bodyStdin,
				Label:      "body",
			})
			if err != nil {
				return err
			}
			input := map[string]any{}
			if bodySet {
				input["body"] = body
			}
			if health != "" {
				input["health"] = health
			}
			if len(input) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("specify at least one of --body, --body-file, --body-stdin, or --health"))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_update_project_update", "projectUpdateUpdate", map[string]any{"id": updateID, "input": input})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ProjectUpdateUpdateMutation, map[string]any{"id": updateID, "input": input})
			if err != nil {
				return classifyGraphQLMutationError("projectUpdateUpdate", err, flags)
			}
			projectUpdate, err := extractMutationObject(resp, "projectUpdateUpdate", "projectUpdate")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, projectUpdate, "project-updates")
		},
	}
	cmd.Flags().StringVar(&bodyFlag, "body", "", "Project update body markdown")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read project update body markdown from file")
	cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read project update body markdown from stdin")
	cmd.Flags().StringVar(&health, "health", "", "Project health status: onTrack, atRisk, or offTrack")
	return cmd
}

func newProjectUpdatesArchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "archive <project-update-id>",
		Short: "Archive a Linear project update",
		Long: `Archive a project update via the projectUpdateArchive mutation. This is the
removal verb for a project update: Linear deprecated projectUpdateDelete in
favour of it.

Archiving is reversible with 'project-updates unarchive', so it is not gated
behind a confirmation. With --ignore-missing a missing update exits 0 as a
no-op.`,
		Example: `  linear-pp-cli project-updates archive <project-update-uuid> --agent
  linear-pp-cli project-updates archive <project-update-uuid> --dry-run --json`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<project-update-id> is required"))
			}
			updateID := args[0]
			if !store.IsUUID(updateID) {
				return usageErr(fmt.Errorf("<project-update-id> expects a project update UUID, got %q", updateID))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_archive_project_update", "projectUpdateArchive", map[string]any{"id": updateID})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ProjectUpdateArchiveMutation, map[string]any{"id": updateID})
			if err != nil {
				return classifyGraphQLMutationError("projectUpdateArchive", err, flags)
			}
			projectUpdate, err := extractMutationObject(resp, "projectUpdateArchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, projectUpdate, "project-updates")
		},
	}
	return cmd
}

func newProjectUpdatesUnarchiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unarchive <project-update-id>",
		Short:   "Restore an archived Linear project update",
		Long:    `Restore an archived project update via the projectUpdateUnarchive mutation.`,
		Example: `  linear-pp-cli project-updates unarchive <project-update-uuid> --agent`,
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("<project-update-id> is required"))
			}
			updateID := args[0]
			if !store.IsUUID(updateID) {
				return usageErr(fmt.Errorf("<project-update-id> expects a project update UUID, got %q", updateID))
			}
			if flags.dryRun {
				return renderMutationDryRun(cmd, flags, "would_unarchive_project_update", "projectUpdateUnarchive", map[string]any{"id": updateID})
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			resp, err := c.Mutate(client.ProjectUpdateUnarchiveMutation, map[string]any{"id": updateID})
			if err != nil {
				return classifyGraphQLMutationError("projectUpdateUnarchive", err, flags)
			}
			projectUpdate, err := extractMutationObject(resp, "projectUpdateUnarchive", "entity")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, projectUpdate, "project-updates")
		},
	}
	return cmd
}
