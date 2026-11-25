// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"fmt"
	"io"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/skool/internal/client"
	"github.com/spf13/cobra"
)

// pushRateLimit is the default request rate for a push. A bulk classroom
// upload is the one flow in this CLI that fires many authenticated writes in a
// row, and Skool's write host sheds load with opaque 5xx rather than a
// Retry-After, so the default is deliberately slow and the user raises it
// explicitly if they want to.
const pushRateLimit = 1.0

// classroomCoursesPath is the one classroom write endpoint confirmed to exist
// on Skool's write host; see errClassroomWritesUndiscovered for what is still
// missing and how to capture it.
const classroomCoursesPath = "/courses"

// pushOp is one planned change. A plan is always computed and shown; whether
// it is executed is a separate decision gated on --apply.
type pushOp struct {
	Op       string `json:"op"`
	Course   string `json:"course"`
	Lesson   string `json:"lesson,omitempty"`
	Order    int    `json:"order,omitempty"`
	Access   string `json:"access,omitempty"`
	TargetID string `json:"target_id,omitempty"`
	File     string `json:"file,omitempty"`
	Reason   string `json:"reason"`
}

const (
	opCreateCourse = "create-course"
	opCreatePage   = "create-page"
	opUpdatePage   = "update-page"
)

func newClassroomPushCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagApply bool

	cmd := &cobra.Command{
		Use:   "push <dir>",
		Short: "Plan (and, with --apply, upload) a directory of markdown lessons into the classroom",
		Long: `Read a directory of markdown lesson files, diff them against the live
classroom, and print the ordered plan of courses and pages that would be
created or updated.

One file per lesson, with frontmatter:

    ---
    course: "2. Reading the Signals"
    course_order: 2
    lesson: "2.1 Why the trend beats the day"
    order: 1
    access: open
    ---
    Body in markdown.

Bodies are converted to TipTap JSON (paragraphs, bold, ordered lists, bullet
lists, links, H3 max). Courses and lessons are matched to the live classroom by
title, case- and whitespace-insensitively, so pushing the same bundle twice
updates the same pages instead of creating duplicates.

push never deletes. A page that exists live but is absent from the bundle is
reported as an orphan and left alone, and so is a whole live course that no
lesson in the bundle names. If two live courses, or two pages in one course,
share a title the bundle targets, push stops and asks you to rename one rather
than guess.

--dry-run validates the bundle and stops before reading the live classroom,
so it prints no plan. Run without --dry-run (and without --apply) for the plan.`,
		Example: "  skool-pp-cli classroom push ./lessons --community <community-slug>\n" +
			"  skool-pp-cli classroom push ./lessons --community <community-slug> --json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return nil
				}
				return cmd.Help()
			}
			dir := args[0]

			lessons, err := parseLessonDir(dir)
			if err != nil {
				return err
			}
			if len(lessons) == 0 {
				return usageErr(fmt.Errorf("no .md lesson files found under %s", dir))
			}

			// --dry-run sends nothing, and the plan is a diff against the live
			// classroom, so there is no plan to show: the client would hand
			// back its {"dry_run": true} placeholder and the planner would try
			// to read that as a classroom. Stop here, after the bundle itself
			// has been validated, the way the sibling classroom commands do.
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.ErrOrStderr(), "%d lesson file(s) under %s parsed cleanly.\n(dry run - the live classroom was not read, so no plan was built; re-run without --dry-run to see the plan. push only writes with --apply.)\n", len(lessons), dir)
				return nil
			}

			// A bulk upload is the one flow here that fires many writes in a
			// row; slow it down unless the user asked for something else.
			if !cmd.Flags().Changed("rate-limit") && flags.rateLimit == 0 {
				flags.rateLimit = pushRateLimit
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community, err := resolveCommunity(flagCommunity, c.Config.TemplateVars)
			if err != nil {
				return err
			}

			plan, orphans, err := buildPushPlan(c, community, lessons)
			if err != nil {
				return err
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				envelope := map[string]any{
					"community":    community,
					"dir":          dir,
					"lesson_count": len(lessons),
					"applied":      false,
					"mode":         "plan",
					"plan":         plan,
					"orphans":      orphans,
				}
				if flagApply {
					envelope["mode"] = "apply"
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), envelope, flags); err != nil {
					return err
				}
			} else {
				printPushPlan(cmd.OutOrStdout(), community, dir, plan, orphans)
			}

			if !flagApply {
				return nil
			}
			// Scope check before anything is sent: an apply must not be able
			// to reach members, billing, DMs, or group settings even if a
			// future endpoint table is mis-populated.
			if err := assertClassroomScope(classroomCoursesPath); err != nil {
				return err
			}
			return errClassroomWritesUndiscovered()
		},
	}
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to SKOOL_COMMUNITY / config template_vars)")
	cmd.Flags().BoolVar(&flagApply, "apply", false, "Execute the plan instead of only printing it")
	return cmd
}

// errClassroomWritesUndiscovered is the fail-closed path for --apply.
//
// Only POST /courses is confirmed to exist on Skool's write host: an
// unauthenticated OPTIONS/GET probe of api2.skool.com/courses answers
// `allow: OPTIONS, POST` with CORS `access-control-allow-methods: POST`. Its
// request body, the folder and page calls, the access-setting call, the reorder
// call, and the delete call are all still unknown, and guessing any of them
// would send a malformed authenticated write to a live community. So the plan
// is real and the execution refuses until the endpoints are captured.
func errClassroomWritesUndiscovered() error {
	return apiErr(fmt.Errorf(`--apply cannot run yet: Skool's classroom write endpoints are not in this CLI's spec.

Confirmed so far: POST https://api2.skool.com/courses exists (the host answers
"allow: OPTIONS, POST"). Still unknown: its request body, and the folder-create,
page-create, page-update, reorder, and delete calls.

To capture them, sign in as a community admin, open DevTools -> Network, and in a
throwaway course perform: create course, set access (open, then level unlock),
create folder, create page with a heading + bold + numbered list + bullet list +
link, edit the page, reorder two pages, delete the course. Export the HAR, then:

    cli-printing-press browser-sniff --har <capture>.har --json

The plan above is already correct and safe to review; re-run without --apply to
see it on its own`))
}

// buildPushPlan diffs the bundle against the live classroom. It returns the
// ordered plan plus the live pages and courses that the bundle does not
// mention.
func buildPushPlan(c *client.Client, community string, lessons []lessonFile) ([]pushOp, []string, error) {
	listRaw, err := fetchCourseList(c, community)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the live classroom for %q: %w", community, err)
	}
	liveCourses, err := courseSummariesFromPayload(listRaw)
	if err != nil {
		return nil, nil, err
	}
	return planBundle(lessons, liveCourses, func(course classroomNode) (classroomNode, error) {
		courseRaw, err := fetchCourse(c, community, course.Slug)
		if err != nil {
			return classroomNode{}, err
		}
		return courseTreeFromPayload(courseRaw)
	})
}

// planBundle is the diff itself, with the per-course read injected so the
// matching rules can be tested without a live classroom. fetchTree is called
// once for each live course the bundle targets and never for the others.
func planBundle(lessons []lessonFile, liveCourses []classroomNode, fetchTree func(classroomNode) (classroomNode, error)) ([]pushOp, []string, error) {
	liveByTitle := indexCoursesByTitle(liveCourses)

	var plan []pushOp
	var orphans []string
	inBundle := map[string]bool{}
	// Course order is already settled by parseLessonDir; walk it in that order
	// and keep one group per course.
	for _, group := range groupByCourse(lessons) {
		key := titleKey(group.title)
		inBundle[key] = true
		matches := liveByTitle[key]
		// Two live courses with this title: either could be the target, and
		// updating the wrong one is not recoverable by re-running. Refuse
		// rather than pick.
		if len(matches) > 1 {
			return nil, nil, usageErr(fmt.Errorf("%d live courses are titled %q (ids: %s) — push matches by title and cannot tell which one this bundle means; rename one in Skool and re-run",
				len(matches), group.title, nodeIDs(matches)))
		}
		exists := len(matches) == 1
		livePages := map[string][]classroomNode{}
		if exists {
			tree, err := fetchTree(matches[0])
			if err != nil {
				return nil, nil, fmt.Errorf("reading live course %q: %w", group.title, err)
			}
			livePages = flattenPages(tree)
		} else {
			plan = append(plan, pushOp{
				Op:     opCreateCourse,
				Course: group.title,
				Order:  group.order,
				Access: group.access,
				Reason: "no live course matches this title",
			})
		}

		pageOps, groupOrphans, err := planPagesForCourse(group, livePages, exists)
		if err != nil {
			return nil, nil, err
		}
		plan = append(plan, pageOps...)
		orphans = append(orphans, groupOrphans...)
	}

	// A live course the bundle never mentions is an orphan too. It is reported
	// at course level: reading every such course just to list its pages would
	// spend one request per unrelated course on a plan that does not touch
	// them.
	for _, course := range liveCourses {
		if !inBundle[titleKey(course.Title)] {
			orphans = append(orphans, course.Title+" (whole course; no lesson in the bundle names it)")
		}
	}
	sort.Strings(orphans)
	return plan, orphans, nil
}

// planPagesForCourse is the pure diff for one course: which of its lessons map
// onto an existing live page and which are new, plus the live pages the bundle
// never mentions. Split out from planBundle so the matching rules can be
// tested on their own.
//
// A lesson whose title matches more than one live page is an error rather than
// a guess. Same-titled live pages the bundle does not target are all reported
// as orphans.
func planPagesForCourse(group courseGroup, livePages map[string][]classroomNode, courseExists bool) ([]pushOp, []string, error) {
	ops := make([]pushOp, 0, len(group.lessons))
	matched := map[string]bool{}
	for _, l := range group.lessons {
		key := titleKey(l.Lesson)
		pages := livePages[key]
		if len(pages) > 1 {
			return nil, nil, usageErr(fmt.Errorf("%d live pages in course %q are titled %q (ids: %s) — push matches by title and cannot tell which one %s should update; rename one in Skool and re-run",
				len(pages), group.title, l.Lesson, nodeIDs(pages), l.Path))
		}
		if len(pages) == 1 {
			matched[key] = true
			ops = append(ops, pushOp{
				Op:       opUpdatePage,
				Course:   group.title,
				Lesson:   l.Lesson,
				Order:    l.Order,
				Access:   l.Access,
				TargetID: pages[0].ID,
				File:     l.Path,
				Reason:   "live page with the same title",
			})
			continue
		}
		reason := "no live page matches this title"
		if !courseExists {
			reason = "course is being created"
		}
		ops = append(ops, pushOp{
			Op:     opCreatePage,
			Course: group.title,
			Lesson: l.Lesson,
			Order:  l.Order,
			Access: l.Access,
			File:   l.Path,
			Reason: reason,
		})
	}
	var orphans []string
	for key, pages := range livePages {
		if matched[key] {
			continue
		}
		for _, page := range pages {
			orphans = append(orphans, group.title+" / "+page.Title)
		}
	}
	sort.Strings(orphans)
	return ops, orphans, nil
}

type courseGroup struct {
	title   string
	order   int
	access  string
	lessons []lessonFile
}

// groupByCourse collects lessons per course, preserving the order established
// by parseLessonDir. The course's order and access come from its first lesson
// file, which is the file the user would expect to carry them.
func groupByCourse(lessons []lessonFile) []courseGroup {
	var groups []courseGroup
	index := map[string]int{}
	for _, l := range lessons {
		key := titleKey(l.Course)
		if pos, ok := index[key]; ok {
			groups[pos].lessons = append(groups[pos].lessons, l)
			continue
		}
		index[key] = len(groups)
		groups = append(groups, courseGroup{
			title:   l.Course,
			order:   l.CourseOrder,
			access:  l.Access,
			lessons: []lessonFile{l},
		})
	}
	return groups
}

func printPushPlan(w io.Writer, community, dir string, plan []pushOp, orphans []string) {
	fmt.Fprintf(w, "plan for %s from %s (%d operations)\n\n", community, dir, len(plan))
	for _, op := range plan {
		target := op.Course
		if op.Lesson != "" {
			target = op.Course + " / " + op.Lesson
		}
		fmt.Fprintf(w, "  %-14s %-60s %s\n", op.Op, truncate(target, 60), op.Reason)
	}
	if len(orphans) > 0 {
		fmt.Fprintf(w, "\n%d live page(s) or course(s) not in the bundle (left alone; push never deletes):\n", len(orphans))
		for _, o := range orphans {
			fmt.Fprintf(w, "  %s\n", o)
		}
	}
	fmt.Fprintf(w, "\nnothing was sent. re-run with --apply to execute.\n")
}
