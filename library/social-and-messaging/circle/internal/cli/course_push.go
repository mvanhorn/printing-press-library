// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCoursePushCmd(flags *rootFlags) *cobra.Command {
	var flagSpace string
	var publish bool
	var force bool

	cmd := &cobra.Command{
		Use:   "push <folder>",
		Short: "Upload a course kept as module folders and markdown lessons to a Circle course space, new lessons as drafts.",
		Long:  "Use this command to upload a local course folder to a Circle course space. Do NOT use this command to download a Circle course to disk; use 'course pull' instead.\n\nSections are numbered folders; lessons are Markdown files with optional YAML frontmatter (name, id, status). course.yaml stores space_id, IDs, and the last synced lesson status and normalized body SHA256. Circle's API does not expose lesson order; push compares folder order with the order recorded in course.yaml. Unlisted items retain their positions. Preview is the default; --yes applies the plan. Live edits are preserved when local content is unchanged. Conflicting local and live edits require --force together with --yes. New lessons default to draft. --publish affects only new lessons; existing lessons require explicit frontmatter status. No content is deleted. Raise --timeout for large courses or media uploads.",
		Example: strings.Trim(`
  circle-pp-cli course push ./course --space 1000969 --json
  circle-pp-cli course push ./course --yes
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:preview-happy-path": "true", "pp:happy-args": "folder=testdata/course-example;--space=1001068"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "course push")
			}
			if len(args) != 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide one course folder"))
			}
			local, err := readCourseFolder(args[0])
			if err != nil {
				return err
			}
			spaceID, err := courseSpaceID(flagSpace, local.Manifest.SpaceID)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if local.Manifest.SpaceID != 0 && local.Manifest.SpaceID != spaceID {
				return usageErr(fmt.Errorf("--space conflicts with course.yaml space_id; use a separate folder for another course"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			live, err := fetchCourse(ctx, c, spaceID)
			if err != nil {
				return err
			}
			plan, err := buildCoursePlan(local, live, spaceID, publish)
			if err != nil {
				return err
			}
			plan.Force = force
			if flags.yes {
				if err := applyCoursePlan(ctx, c, args[0], local.Manifest, &plan); err != nil {
					return err
				}
				plan.Applied = true
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
			}
			for _, step := range plan.Plan {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", step.Action, step.Kind, step.Path, step.Name); err != nil {
					return err
				}
				for _, change := range step.Changes {
					if change.Field == "status" {
						if _, err := fmt.Fprintf(cmd.OutOrStdout(), "  status: %s -> %s\n", change.From, change.To); err != nil {
							return err
						}
					}
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Applied: %t. %d planned actions. Use --yes to apply a preview.\n", plan.Applied, len(plan.Plan))
			return err
		},
	}
	cmd.Flags().StringVar(&flagSpace, "space", "", "Use this course space ID, or the space_id in course.yaml.")
	cmd.Flags().BoolVar(&publish, "publish", false, "Publish new lessons; existing lessons still require explicit frontmatter status.")
	cmd.Flags().BoolVar(&force, "force", false, "Allow conflicting local lesson edits to overwrite live edits when applying with --yes.")
	return cmd
}
