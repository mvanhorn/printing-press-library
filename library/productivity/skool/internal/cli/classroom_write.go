// Hand-written classroom write/import commands for skool-pp-cli.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/productivity/skool/internal/classroomwrite"
	"github.com/mvanhorn/printing-press-library/library/productivity/skool/internal/client"
	"github.com/spf13/cobra"
)

func newClassroomWriteCmds(flags *rootFlags) []*cobra.Command {
	return []*cobra.Command{
		newClassroomCreateCourseCmd(flags),
		newClassroomCreateLessonCmd(flags),
		newClassroomInspectCourseCmd(flags),
		newClassroomInspectLessonCmd(flags),
		newClassroomUpdateLessonCmd(flags),
		newClassroomRestoreCoursePrivacyCmd(flags),
		newClassroomBatchNormalizeCmd(flags),
		newClassroomNormalizeLessonCmd(flags),
		newClassroomImportCourseCmd(flags),
		newClassroomVerifyImportCmd(flags),
	}
}

func newCourseRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "course", Short: "Create and inspect Skool courses (api2 write)"}
	create := newClassroomCreateCourseCmd(flags)
	create.Use = "create"
	inspect := newClassroomInspectCourseCmd(flags)
	inspect.Use = "inspect"
	cmd.AddCommand(create)
	cmd.AddCommand(inspect)
	return cmd
}

func newLessonRootCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "lesson", Short: "Create and inspect Skool lessons (api2 write)"}
	create := newClassroomCreateLessonCmd(flags)
	create.Use = "create"
	inspect := newClassroomInspectLessonCmd(flags)
	inspect.Use = "inspect"
	cmd.AddCommand(create)
	cmd.AddCommand(inspect)
	return cmd
}

func newImportCourseTopCmd(flags *rootFlags) *cobra.Command {
	cmd := newClassroomImportCourseCmd(flags)
	cmd.Use = "import-course"
	return cmd
}

func newVerifyImportTopCmd(flags *rootFlags) *cobra.Command {
	cmd := newClassroomVerifyImportCmd(flags)
	cmd.Use = "verify-import"
	return cmd
}

func newClassroomCreateCourseCmd(flags *rootFlags) *cobra.Command {
	var title, desc, confirmCommunity string
	cmd := &cobra.Command{
		Use:   "create-course",
		Short: "Create a private draft course in a community",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			if err := classroomwrite.AssertImportCourseTitle(title); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			rec, err := api.CreateCourse(classroomwrite.AllowedGroupID, title, desc)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "Course title")
	cmd.Flags().StringVar(&desc, "desc", "", "Course description")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate: must equal target community slug")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("confirm-community")
	return cmd
}

func newClassroomCreateLessonCmd(flags *rootFlags) *cobra.Command {
	var courseID, title, mdFile, confirmCommunity string
	cmd := &cobra.Command{
		Use:   "create-lesson",
		Short: "Create a private draft lesson (module) in a course",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			rawBody, err := os.ReadFile(mdFile)
			if err != nil {
				return err
			}
			text, err := classroomwrite.StripFrontmatter(string(rawBody))
			if err != nil {
				return err
			}
			desc, err := classroomwrite.MarkdownToLessonDesc(text)
			if err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			rec, err := api.CreateLesson(classroomwrite.AllowedGroupID, courseID, title, desc)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
		},
	}
	cmd.Flags().StringVar(&courseID, "course-id", "", "Parent course id")
	cmd.Flags().StringVar(&title, "title", "", "Lesson title")
	cmd.Flags().StringVar(&mdFile, "md-file", "", "Markdown body file")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate: must equal target community slug")
	_ = cmd.MarkFlagRequired("course-id")
	_ = cmd.MarkFlagRequired("title")
	_ = cmd.MarkFlagRequired("md-file")
	_ = cmd.MarkFlagRequired("confirm-community")
	return cmd
}

func newClassroomInspectCourseCmd(flags *rootFlags) *cobra.Command {
	var courseID string
	cmd := &cobra.Command{
		Use:   "inspect-course",
		Short: "Inspect a course and its lessons from api2",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			tree, err := api.GetCourseTree(courseID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), tree, flags)
		},
	}
	cmd.Flags().StringVar(&courseID, "course-id", "", "Course id")
	_ = cmd.MarkFlagRequired("course-id")
	return cmd
}

func newClassroomInspectLessonCmd(flags *rootFlags) *cobra.Command {
	var lessonID, courseID string
	cmd := &cobra.Command{
		Use:   "inspect-lesson",
		Short: "Inspect a lesson (module) record from api2",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			rec, err := api.GetLesson(courseID, lessonID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), rec, flags)
		},
	}
	cmd.Flags().StringVar(&lessonID, "lesson-id", "", "Lesson id")
	cmd.Flags().StringVar(&courseID, "course-id", "", "Parent course id")
	_ = cmd.MarkFlagRequired("lesson-id")
	_ = cmd.MarkFlagRequired("course-id")
	return cmd
}

func newClassroomRestoreCoursePrivacyCmd(flags *rootFlags) *cobra.Command {
	var courseID, community, confirmCommunity string
	var privacy int
	cmd := &cobra.Command{
		Use:   "restore-course-privacy",
		Short: "Restore top-level course privacy via flat api2 PUT (read-then-write)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if community != "" && community != classroomwrite.AllowedCommunitySlug {
				return fmt.Errorf("community slug mismatch: got %q want %q", community, classroomwrite.AllowedCommunitySlug)
			}
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			protected := []string{"Start Here", "ChatGPT for Real Estate", "AI Images & Personal Branding", "Social Media & Facebook for Realtors"}
			beforeSnap, err := api.SnapshotCoursesByTitle(classroomwrite.AllowedGroupID, protected)
			if err != nil {
				return err
			}
			result, err := api.UpdateCoursePrivacy(courseID, privacy)
			if err != nil {
				return err
			}
			afterSnap, err := api.SnapshotCoursesByTitle(classroomwrite.AllowedGroupID, protected)
			if err != nil {
				return err
			}
			if err := classroomwrite.AssertCoursesUnchanged(beforeSnap, afterSnap); err != nil {
				return err
			}
			out := map[string]any{
				"course_update": result,
				"protected_courses_unchanged": true,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&community, "community", "", "Target community slug")
	cmd.Flags().StringVar(&courseID, "course-id", "", "Top-level course id")
	cmd.Flags().IntVar(&privacy, "privacy", classroomwrite.CoursePrivacyLevelUnlock, "Target metadata.privacy value")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate")
	_ = cmd.MarkFlagRequired("community")
	_ = cmd.MarkFlagRequired("course-id")
	_ = cmd.MarkFlagRequired("confirm-community")
	return cmd
}

func newClassroomBatchNormalizeCmd(flags *rootFlags) *cobra.Command {
	var community, confirmCommunity string
	var preset0304 bool
	var draftOnly bool
	cmd := &cobra.Command{
		Use:   "batch-normalize-lessons",
		Short: "Normalize remaining preset 03/04 lessons using flat lesson PUT",
		RunE: func(cmd *cobra.Command, args []string) error {
			if community != "" && community != classroomwrite.AllowedCommunitySlug {
				return fmt.Errorf("community slug mismatch: got %q want %q", community, classroomwrite.AllowedCommunitySlug)
			}
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			if !draftOnly {
				return fmt.Errorf("refusing write: pass --draft-only")
			}
			if !preset0304 {
				return usageErr(fmt.Errorf("--preset-0304 is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			for _, courseID := range []string{"f8269e6e1d00425e831ba15ad2627855", "9e3d3af2079a47bf9bf07782bd3cdfa1"} {
				tree, err := api.GetCourseTree(courseID)
				if err != nil {
					return err
				}
				if err := classroomwrite.AssertDraftPrivate(tree.Course.State, tree.Course.Metadata.Privacy); err != nil {
					return fmt.Errorf("batch preflight failed for course %s: %w", courseID, err)
				}
			}
			targets, err := classroomwrite.ResolveBatchTargets(api, classroomwrite.RemainingBatchTargets0304())
			if err != nil {
				return err
			}
			report, err := api.BatchNormalizeLessons(targets)
			if err != nil {
				out := map[string]any{"report": report, "error": err.Error()}
				_ = printJSONFiltered(cmd.OutOrStdout(), out, flags)
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), report, flags)
		},
	}
	cmd.Flags().StringVar(&community, "community", "", "Target community slug")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate")
	cmd.Flags().BoolVar(&preset0304, "preset-0304", false, "Normalize remaining lessons in Courses 03 and 04")
	cmd.Flags().BoolVar(&draftOnly, "draft-only", false, "Required safety gate")
	_ = cmd.MarkFlagRequired("community")
	_ = cmd.MarkFlagRequired("confirm-community")
	_ = cmd.MarkFlagRequired("draft-only")
	return cmd
}

func newClassroomUpdateLessonCmd(flags *rootFlags) *cobra.Command {
	var lessonID, courseID, contentFile, community, confirmCommunity string
	var draftOnly bool
	cmd := &cobra.Command{
		Use:   "update-lesson",
		Short: "Update a draft lesson body from local markdown (flat api2 PUT, draft only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if community != "" && community != classroomwrite.AllowedCommunitySlug {
				return fmt.Errorf("community slug mismatch: got %q want %q", community, classroomwrite.AllowedCommunitySlug)
			}
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			if !draftOnly {
				return fmt.Errorf("refusing write: pass --draft-only to acknowledge draft-only lesson updates")
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			rawBody, err := os.ReadFile(contentFile)
			if err != nil {
				return err
			}
			text, err := classroomwrite.StripFrontmatter(string(rawBody))
			if err != nil {
				return err
			}
			desc, err := classroomwrite.MarkdownToLessonDesc(text)
			if err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			result, err := api.UpdateLesson(courseID, lessonID, classroomwrite.UpdateLessonOptions{Desc: desc})
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&community, "community", "", "Target community slug (must match approved import community)")
	cmd.Flags().StringVar(&courseID, "course-id", "", "Parent course id")
	cmd.Flags().StringVar(&lessonID, "lesson-id", "", "Lesson id")
	cmd.Flags().StringVar(&contentFile, "content-file", "", "Local markdown source file")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate: must equal target community slug")
	cmd.Flags().BoolVar(&draftOnly, "draft-only", false, "Required safety gate: only update draft lessons in draft private courses")
	_ = cmd.MarkFlagRequired("community")
	_ = cmd.MarkFlagRequired("course-id")
	_ = cmd.MarkFlagRequired("lesson-id")
	_ = cmd.MarkFlagRequired("content-file")
	_ = cmd.MarkFlagRequired("confirm-community")
	_ = cmd.MarkFlagRequired("draft-only")
	return cmd
}

func newClassroomNormalizeLessonCmd(flags *rootFlags) *cobra.Command {
	var lessonID, courseID, mdFile, confirmCommunity string
	cmd := &cobra.Command{
		Use:   "normalize-lesson",
		Short: "Reformat a lesson body from local markdown (TipTap fix, draft only)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			rawBody, err := os.ReadFile(mdFile)
			if err != nil {
				return err
			}
			text, err := classroomwrite.StripFrontmatter(string(rawBody))
			if err != nil {
				return err
			}
			desc, err := classroomwrite.MarkdownToLessonDesc(text)
			if err != nil {
				return err
			}
			api := classroomwrite.NewAPI(c)
			result, err := api.UpdateLessonBody(courseID, lessonID, desc)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&lessonID, "lesson-id", "", "Lesson id")
	cmd.Flags().StringVar(&courseID, "course-id", "", "Parent course id")
	cmd.Flags().StringVar(&mdFile, "md-file", "", "Local markdown source file")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate: must equal target community slug")
	_ = cmd.MarkFlagRequired("lesson-id")
	_ = cmd.MarkFlagRequired("course-id")
	_ = cmd.MarkFlagRequired("md-file")
	_ = cmd.MarkFlagRequired("confirm-community")
	return cmd
}

func newClassroomImportCourseCmd(flags *rootFlags) *cobra.Command {
	var dir, confirmCommunity string
	var preset0304, preset0507, preset0812, preset1317 bool
	cmd := &cobra.Command{
		Use:   "import-course",
		Short: "Import one or more local course directories as private drafts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			var plans []*classroomwrite.LocalCoursePlan
			if preset1317 {
				plans, err = classroomwrite.LoadPreset1317Plans()
			} else if preset0812 {
				plans, err = classroomwrite.LoadPreset0812Plans()
			} else if preset0507 {
				plans, err = classroomwrite.LoadPreset0507Plans()
			} else if preset0304 {
				plans, err = classroomwrite.LoadPreset0304Plans()
			} else if dir != "" {
				plan, loadErr := classroomwrite.LoadLocalCoursePlan(dir)
				if loadErr != nil {
					return loadErr
				}
				plans = []*classroomwrite.LocalCoursePlan{plan}
			} else {
				return usageErr(fmt.Errorf("specify --dir, --preset-0304, --preset-0507, --preset-0812, or --preset-1317"))
			}
			readAPI := classroomwrite.NewAPI(readClientForDryRun(c, flags.dryRun))
			existing, err := readAPI.ListCourses(classroomwrite.AllowedGroupID)
			if err != nil {
				return err
			}
			importer := &classroomwrite.Importer{API: readAPI}
			counters := &classroomwrite.ImportCounters{}
			results := make([]*classroomwrite.ImportResult, 0, len(plans))
			for _, plan := range plans {
				var res *classroomwrite.ImportResult
				var impErr error
				if flags.dryRun {
					res, impErr = importer.DryRunPlan(plan, existing, counters)
					if impErr == nil {
						previewImportWrites(os.Stderr, plan, res)
					}
				} else {
					writeAPI := classroomwrite.NewAPI(c)
					importer.API = writeAPI
					res, impErr = importer.ImportPlan(plan, existing, counters)
				}
				if impErr != nil {
					return impErr
				}
				results = append(results, res)
			}
			out := map[string]any{
				"dry_run":          flags.dryRun,
				"community":        classroomwrite.AllowedCommunitySlug,
				"group_id":         classroomwrite.AllowedGroupID,
				"courses_created":  counters.CoursesCreated,
				"lessons_created":  counters.LessonsCreated,
				"results":          results,
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Local course directory containing course.meta.json")
	cmd.Flags().BoolVar(&preset0304, "preset-0304", false, "Import approved Courses 03 and 04 from skool-content")
	cmd.Flags().BoolVar(&preset0507, "preset-0507", false, "Import approved Courses 05, 06, and 07 from skool-content")
	cmd.Flags().BoolVar(&preset0812, "preset-0812", false, "Import approved Courses 08, 09, 10, 11, and 12 from skool-content")
	cmd.Flags().BoolVar(&preset1317, "preset-1317", false, "Import approved Courses 13, 14, 15, 16, and 17 from skool-content")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate: must equal target community slug")
	_ = cmd.MarkFlagRequired("confirm-community")
	return cmd
}

func newClassroomVerifyImportCmd(flags *rootFlags) *cobra.Command {
	var confirmCommunity string
	var preset0304, preset0507, preset0812, preset1317 bool
	cmd := &cobra.Command{
		Use:   "verify-import",
		Short: "Verify expected courses exist remotely as private drafts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := classroomwrite.RequireCommunityConfirmation(confirmCommunity == classroomwrite.AllowedCommunitySlug); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if err := assertLiveCommunity(c, flags); err != nil {
				return err
			}
			var titles []string
			if preset1317 {
				titles = []string{"AI Lead Follow-Up & Client Conversations", "Open House Marketing & Lead Conversion", "Listing Marketing System", "Buyer & Seller Consultations With AI", "Database, Past Clients & Referral Marketing"}
			} else if preset0812 {
				titles = []string{"Gamma & AI Presentations", "Verified Market Updates & Data Content", "YouTube, Reels & Video Distribution", "Local SEO & Google Business Profile", "AI Productivity & Daily Realtor Workflows"}
			} else if preset0507 {
				titles = []string{"Canva AI & Content Creation", "Gemini & Creative AI Tools", "Real Estate Prompt Library"}
			} else if preset0304 {
				titles = []string{"CapCut & Video for Real Estate", "Social Media & Facebook for Realtors"}
			} else {
				return usageErr(fmt.Errorf("--preset-0304, --preset-0507, --preset-0812, or --preset-1317 is required for verify-import"))
			}
			api := classroomwrite.NewAPI(c)
			courses, err := api.ListCourses(classroomwrite.AllowedGroupID)
			if err != nil {
				return err
			}
			report := make([]map[string]any, 0, len(titles))
			ok := true
			for _, title := range titles {
				rec := classroomwrite.FindCourseByTitle(courses, title)
				item := map[string]any{"title": title, "found": rec != nil}
				if rec == nil {
					ok = false
				} else {
					item["id"] = rec.ID
					item["state"] = rec.State
					item["privacy"] = rec.Metadata.Privacy
					if rec.State != 1 || rec.Metadata.Privacy != 1 {
						ok = false
						item["status"] = "invalid_visibility"
					} else {
						item["status"] = "ok"
					}
					tree, treeErr := api.GetCourseTree(rec.ID)
					if treeErr == nil {
						item["lesson_count"] = len(tree.Children)
						if len(tree.Children) != 8 {
							ok = false
							item["status"] = "lesson_count_mismatch"
						}
					}
				}
				report = append(report, item)
			}
			out := map[string]any{"ok": ok, "courses": report}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().BoolVar(&preset0304, "preset-0304", false, "Verify approved Courses 03 and 04")
	cmd.Flags().BoolVar(&preset0507, "preset-0507", false, "Verify approved Courses 05, 06, and 07")
	cmd.Flags().BoolVar(&preset0812, "preset-0812", false, "Verify approved Courses 08, 09, 10, 11, and 12")
	cmd.Flags().BoolVar(&preset1317, "preset-1317", false, "Verify approved Courses 13, 14, 15, 16, and 17")
	cmd.Flags().StringVar(&confirmCommunity, "confirm-community", "", "Required safety gate: must equal target community slug")
	_ = cmd.MarkFlagRequired("confirm-community")
	return cmd
}

func readClientForDryRun(c *client.Client, dryRun bool) *client.Client {
	if !dryRun {
		return c
	}
	cp := *c
	cp.DryRun = false
	return &cp
}

func assertLiveCommunity(c *client.Client, flags *rootFlags) error {
	rc := readClientForDryRun(c, flags != nil && flags.dryRun)
	path := "/_next/data/{buildId}/" + classroomwrite.AllowedCommunitySlug + "/about.json"
	params := map[string]string{"g": classroomwrite.AllowedCommunitySlug}
	raw, err := rc.Get(path, params)
	if err != nil {
		return err
	}
	var env struct {
		PageProps struct {
			CurrentGroup struct {
				ID       string `json:"id"`
				Name     string `json:"name"`
				Metadata struct {
					DisplayName string `json:"displayName"`
				} `json:"metadata"`
			} `json:"currentGroup"`
		} `json:"pageProps"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("parsing community about page: %w", err)
	}
	id := classroomwrite.CommunityIdentity{
		Slug:        env.PageProps.CurrentGroup.Name,
		GroupID:     env.PageProps.CurrentGroup.ID,
		DisplayName: env.PageProps.CurrentGroup.Metadata.DisplayName,
	}
	if err := classroomwrite.ValidateCommunityIdentity(id); err != nil {
		return err
	}
	if flags != nil && flags.dryRun {
		fmt.Fprintln(os.Stderr, "community identity verified (dry-run reads enabled)")
	}
	return nil
}

func previewImportWrites(w *os.File, plan *classroomwrite.LocalCoursePlan, result *classroomwrite.ImportResult) {
	if result.Action == "would_create" {
		body := classroomwrite.BuildCreateCourseBody(classroomwrite.AllowedGroupID, plan.Title, plan.Description)
		enc, _ := json.MarshalIndent(body, "  ", "  ")
		fmt.Fprintf(w, "POST https://api2.skool.com/courses (course %q)\n  %s\n", plan.Title, enc)
	}
	courseID := result.CourseID
	if courseID == "(dry-run)" {
		courseID = "<new-course-id>"
	}
	for _, lesson := range result.Lessons {
		if lesson.Action != "would_create" {
			continue
		}
		var body string
		for _, src := range plan.Lessons {
			if src.Title == lesson.Title {
				desc, err := classroomwrite.MarkdownToLessonDesc(src.Body)
				if err != nil {
					continue
				}
				b, _ := json.Marshal(classroomwrite.BuildCreateLessonBody(classroomwrite.AllowedGroupID, courseID, lesson.Title, desc))
				body = string(b)
				break
			}
		}
		fmt.Fprintf(w, "POST https://api2.skool.com/courses (lesson %q)\n  %s\n", lesson.Title, body)
	}
}
