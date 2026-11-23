// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/client"
	"github.com/spf13/cobra"
)

// fetchCourseList reads the community's classroom index.
func fetchCourseList(c *client.Client, community string) (json.RawMessage, error) {
	path := "/_next/data/{buildId}/{community}/classroom.json"
	path = replacePathParam(path, "community", community)
	return c.Get(path, map[string]string{"g": community})
}

// fetchCourse reads one course page, which carries the whole folder/page tree.
func fetchCourse(c *client.Client, community, courseSlug string) (json.RawMessage, error) {
	path := "/_next/data/{buildId}/{community}/classroom/{course_slug}.json"
	path = replacePathParam(path, "community", community)
	path = replacePathParam(path, "course_slug", courseSlug)
	return c.Get(path, map[string]string{"g": community})
}

func newClassroomCoursesCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string

	run := func(cmd *cobra.Command, args []string) error {
		if dryRunOK(flags) {
			return nil
		}
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		community, err := resolveCommunity(flagCommunity, c.Config.TemplateVars)
		if err != nil {
			return err
		}
		raw, err := fetchCourseList(c, community)
		if err != nil {
			return classifyAPIError(err, flags)
		}
		courses, err := courseSummariesFromPayload(raw)
		if err != nil {
			return err
		}
		if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"community": community,
				"count":     len(courses),
				"courses":   courses,
			}, flags)
		}
		for _, course := range courses {
			access := course.Access
			if access == "" {
				access = "-"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-3d %-44s %-12s %s\n", course.Order, truncate(course.Title, 44), access, course.Slug)
		}
		return nil
	}

	cmd := &cobra.Command{
		Use:         "courses",
		Short:       "Courses in the community, normalized (id, slug, title, order, access)",
		Example:     "  skool-pp-cli classroom courses list --community <community-slug> --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        run,
	}
	list := &cobra.Command{
		Use:         "list",
		Short:       "List courses in the community",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        run,
	}
	list.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to SKOOL_COMMUNITY / config template_vars)")
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to SKOOL_COMMUNITY / config template_vars)")
	cmd.AddCommand(list)
	return cmd
}

func newClassroomCourseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "course",
		Short: "A single course and its folder/page tree",
	}
	cmd.AddCommand(newClassroomCourseGetCmd(flags))
	return cmd
}

func newClassroomCourseGetCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagRaw bool

	cmd := &cobra.Command{
		Use:   "get <course-slug-or-id>",
		Short: "Get a course as a normalized tree (course -> folders -> pages)",
		Long: `Get one course as a normalized three-level tree: the course, its folders
(the UI calls them modules or sets), and the pages inside each folder, with
id, title, order, and access at every level.

This is the shape 'classroom push' diffs against. Use --raw for the
untouched Next.js page payload instead.`,
		Example:     "  skool-pp-cli classroom course get reading-the-signals --community <community-slug> --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community, err := resolveCommunity(flagCommunity, c.Config.TemplateVars)
			if err != nil {
				return err
			}
			courseRef := strings.TrimSpace(args[0])
			raw, err := fetchCourse(c, community, courseRef)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flagRaw {
				return printOutputWithFlags(cmd.OutOrStdout(), raw, flags)
			}
			tree, err := courseTreeFromPayload(raw)
			if err != nil {
				return err
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"community": community,
					"course":    tree,
				}, flags)
			}
			printCourseTree(cmd.OutOrStdout(), tree)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to SKOOL_COMMUNITY / config template_vars)")
	cmd.Flags().BoolVar(&flagRaw, "raw", false, "Print the untouched Next.js page payload instead of the normalized tree")
	return cmd
}

func printCourseTree(w io.Writer, root classroomNode) {
	fmt.Fprintf(w, "%s  [%s]\n", root.Title, root.ID)
	for _, folder := range root.Children {
		fmt.Fprintf(w, "  %2d. %s  [%s]\n", folder.Order, folder.Title, folder.ID)
		for _, page := range folder.Children {
			access := page.Access
			if access != "" {
				access = "  (" + access + ")"
			}
			fmt.Fprintf(w, "      %2d. %s  [%s]%s\n", page.Order, page.Title, page.ID, access)
		}
	}
}
