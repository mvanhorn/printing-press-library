// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelCoursePullCmd(flags *rootFlags) *cobra.Command {
	var flagSpace string
	var force bool

	cmd := &cobra.Command{
		Use:   "pull <folder>",
		Short: "Download a Circle course space into module folders with one markdown file per lesson.",
		Long:  "Use this command to download a Circle course space into a local folder. Do NOT use this command to upload local lesson changes; use 'course push' instead.\n\nWrites numbered section folders, lesson files with YAML frontmatter, and course.yaml. Lesson bodies retain the original HTML. Media is not downloaded. A non-empty folder requires --force. --dry-run describes the intended write without reading or writing files or contacting Circle.",
		Example: strings.Trim(`
  circle-pp-cli course pull ./course --space 1000969
  circle-pp-cli course pull ./course --space 1000969 --force --json
`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:method": "GET", "mcp:write-positionals": "0", "pp:happy-args": "folder=course-pull-out;--space=1001068"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "course pull (write section folders, lesson files, and course.yaml; media excluded)")
			}
			if len(args) != 1 || flagSpace == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("provide one course folder and --space"))
			}
			spaceID, err := courseSpaceID(flagSpace, 0)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if err := checkCourseDestination(args[0], force, spaceID); err != nil {
				return err
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
			files, manifest, err := coursePullFiles(live, spaceID)
			if err != nil {
				return err
			}
			stale, err := staleCourseFiles(args[0], files)
			if err != nil {
				return err
			}
			if err := os.MkdirAll(args[0], 0755); err != nil {
				return err
			}
			for _, file := range files {
				if err := writeCourseFile(args[0], file.Path, file.Data); err != nil {
					return err
				}
			}
			if err := saveCourseManifest(args[0], manifest); err != nil {
				return err
			}
			result := struct {
				SpaceID    int64    `json:"space_id"`
				Folder     string   `json:"folder"`
				Files      int      `json:"files_written"`
				Note       string   `json:"note"`
				StaleFiles []string `json:"stale_files"`
			}{spaceID, args[0], len(files) + 1, "Circle's API does not expose lesson order; files are numbered in creation order. Lesson bodies retain HTML; media is not downloaded.", stale}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			for _, name := range stale {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Stale file retained: %s\n", name); err != nil {
					return err
				}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d files to %s. %s\n", result.Files, result.Folder, result.Note)
			return err
		},
	}
	cmd.Flags().StringVar(&flagSpace, "space", "", "Download this course space ID.")
	cmd.Flags().BoolVar(&force, "force", false, "Allow overwriting files in a non-empty destination folder.")
	return cmd
}
