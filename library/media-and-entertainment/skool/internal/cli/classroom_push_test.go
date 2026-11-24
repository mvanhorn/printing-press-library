// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleLesson = `---
course: "2. Reading the Signals"
course_order: 2
lesson: "2.1 Why the trend beats the day"
order: 1
access: open
---
The **trend** is the signal.

1. Load first
2. Then intensity
`

func TestParseLessonFileReadsTheDocumentedFormat(t *testing.T) {
	got, err := parseLessonFile("lessons/2-1.md", sampleLesson)
	if err != nil {
		t.Fatalf("parseLessonFile: %v", err)
	}
	if got.Course != "2. Reading the Signals" {
		t.Errorf("Course = %q", got.Course)
	}
	if got.CourseOrder != 2 {
		t.Errorf("CourseOrder = %d, want 2", got.CourseOrder)
	}
	if got.Lesson != "2.1 Why the trend beats the day" {
		t.Errorf("Lesson = %q", got.Lesson)
	}
	if got.Order != 1 {
		t.Errorf("Order = %d, want 1", got.Order)
	}
	if got.Access != "open" {
		t.Errorf("Access = %q, want open", got.Access)
	}
	if !strings.HasPrefix(got.Body, "The **trend**") {
		t.Errorf("Body = %q, want the markdown after the frontmatter", got.Body)
	}
	if strings.Contains(got.Body, "---") {
		t.Errorf("Body still carries the frontmatter delimiter: %q", got.Body)
	}
}

func TestParseLessonFileRejectsMalformedInput(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"no frontmatter", "Just a body.\n", "missing frontmatter"},
		{"unclosed frontmatter", "---\ncourse: A\nlesson: B\n", "not closed"},
		{"missing course", "---\nlesson: B\n---\nbody\n", "missing 'course'"},
		{"missing lesson", "---\ncourse: A\n---\nbody\n", "missing 'lesson'"},
		{"unknown key", "---\ncourse: A\nlesson: B\nauthor: C\n---\nbody\n", "unknown frontmatter key"},
		{"non-numeric order", "---\ncourse: A\nlesson: B\norder: first\n---\nbody\n", "must be a whole number"},
		{"bad access", "---\ncourse: A\nlesson: B\naccess: everyone\n---\nbody\n", "must be one of"},
		{"bad level", "---\ncourse: A\nlesson: B\naccess: level:zero\n---\nbody\n", "level:<n>"},
		{"empty body", "---\ncourse: A\nlesson: B\n---\n\n", "body is empty"},
		{"frontmatter line without colon", "---\ncourse A\n---\nbody\n", "not 'key: value'"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseLessonFile("x.md", tc.content)
			if err == nil {
				t.Fatalf("parseLessonFile(%q) succeeded, want error containing %q", tc.name, tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestValidateAccessAcceptsOnlyTheThreeShapes(t *testing.T) {
	for _, ok := range []string{"open", "private", "level:1", "level:12"} {
		if err := validateAccess(ok); err != nil {
			t.Errorf("validateAccess(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", "public", "level:", "level:0", "level:-1", "level:x"} {
		if err := validateAccess(bad); err == nil {
			t.Errorf("validateAccess(%q) = nil, want an error", bad)
		}
	}
}

func writeLesson(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func lessonDoc(course string, courseOrder int, lesson string, order int) string {
	return "---\ncourse: \"" + course + "\"\ncourse_order: " + itoa(courseOrder) +
		"\nlesson: \"" + lesson + "\"\norder: " + itoa(order) + "\n---\nbody text\n"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

func TestParseLessonDirSortsByCourseThenLessonOrder(t *testing.T) {
	dir := t.TempDir()
	// Written out of order, and nested, to prove the sort does the work.
	writeLesson(t, dir, "b/second-course-l2.md", lessonDoc("3. Durability", 3, "3.2 Second", 2))
	writeLesson(t, dir, "a/first-course-l2.md", lessonDoc("2. Signals", 2, "2.2 Later", 2))
	writeLesson(t, dir, "a/first-course-l1.md", lessonDoc("2. Signals", 2, "2.1 Earlier", 1))
	writeLesson(t, dir, "b/second-course-l1.md", lessonDoc("3. Durability", 3, "3.1 First", 1))
	writeLesson(t, dir, "notes.txt", "ignored, not markdown")

	lessons, err := parseLessonDir(dir)
	if err != nil {
		t.Fatalf("parseLessonDir: %v", err)
	}
	var got []string
	for _, l := range lessons {
		got = append(got, l.Lesson)
	}
	want := []string{"2.1 Earlier", "2.2 Later", "3.1 First", "3.2 Second"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestParseLessonDirRejectsDuplicateLessons(t *testing.T) {
	dir := t.TempDir()
	writeLesson(t, dir, "one.md", lessonDoc("2. Signals", 2, "2.1 Why", 1))
	// Same lesson title, different casing and spacing — still the same lesson.
	writeLesson(t, dir, "two.md", lessonDoc("2.  signals", 2, "2.1   WHY", 3))

	_, err := parseLessonDir(dir)
	if err == nil {
		t.Fatal("parseLessonDir accepted a duplicate lesson")
	}
	if !strings.Contains(err.Error(), "duplicate lesson") {
		t.Errorf("error = %q, want it to mention a duplicate lesson", err)
	}
}

func TestPlanPagesForCourseIsIdempotentOnTitleMatch(t *testing.T) {
	group := courseGroup{
		title: "2. Reading the Signals",
		order: 2,
		lessons: []lessonFile{
			{Path: "a.md", Course: "2. Reading the Signals", Lesson: "2.1 Why the trend beats the day", Order: 1},
			{Path: "b.md", Course: "2. Reading the Signals", Lesson: "2.2 Brand new lesson", Order: 2},
		},
	}
	live := map[string][]classroomNode{
		// Casing and inner whitespace differ from the bundle on purpose.
		titleKey("2.1   why the TREND beats the day"): {{ID: "page-1", Title: "2.1 Why the trend beats the day", Kind: classroomKindPage}},
		titleKey("2.9 Retired lesson"):                {{ID: "page-9", Title: "2.9 Retired lesson", Kind: classroomKindPage}},
	}

	ops, orphans, err := planPagesForCourse(group, live, true)
	if err != nil {
		t.Fatalf("planPagesForCourse: %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("got %d ops, want 2: %+v", len(ops), ops)
	}
	if ops[0].Op != opUpdatePage || ops[0].TargetID != "page-1" {
		t.Errorf("first op = %+v, want an update of page-1", ops[0])
	}
	if ops[1].Op != opCreatePage {
		t.Errorf("second op = %+v, want a create", ops[1])
	}
	if len(orphans) != 1 || !strings.Contains(orphans[0], "2.9 Retired lesson") {
		t.Errorf("orphans = %v, want the retired lesson reported once", orphans)
	}
}

func TestPlanPagesForCourseMarksEverythingNewWhenTheCourseIsNew(t *testing.T) {
	group := courseGroup{
		title:   "4. Brand New",
		lessons: []lessonFile{{Path: "a.md", Lesson: "4.1 One", Order: 1}},
	}
	ops, orphans, err := planPagesForCourse(group, map[string][]classroomNode{}, false)
	if err != nil {
		t.Fatalf("planPagesForCourse: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != opCreatePage {
		t.Fatalf("ops = %+v, want one create-page", ops)
	}
	if ops[0].Reason != "course is being created" {
		t.Errorf("reason = %q", ops[0].Reason)
	}
	if len(orphans) != 0 {
		t.Errorf("orphans = %v, want none", orphans)
	}
}

func TestApplyFailsClosedUntilWriteEndpointsAreKnown(t *testing.T) {
	err := errClassroomWritesUndiscovered()
	if err == nil {
		t.Fatal("errClassroomWritesUndiscovered returned nil")
	}
	msg := err.Error()
	for _, want := range []string{"--apply cannot run yet", "browser-sniff", "api2.skool.com/courses"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message is missing %q:\n%s", want, msg)
		}
	}
}

func TestCourseTreeFromPayload(t *testing.T) {
	raw := json.RawMessage(`{"pageProps":{"course":{"id":"c1","name":"Reading the Signals",
		"metadata":{"slug":"reading-the-signals","access":"open"},
		"modules":[
			{"id":"m1","name":"Week 1","metadata":{"order":1},"lessons":[
				{"id":"p1","name":"Why the trend beats the day","metadata":{"order":1,"access":"open"}},
				{"id":"p2","name":"Reading load","metadata":{"order":2}}]},
			{"id":"m2","name":"Week 2","metadata":{"order":2},"lessons":[]}]}}}`)
	tree, err := courseTreeFromPayload(raw)
	if err != nil {
		t.Fatalf("courseTreeFromPayload: %v", err)
	}
	if tree.ID != "c1" || tree.Kind != classroomKindCourse || tree.Slug != "reading-the-signals" {
		t.Errorf("root = %+v", tree)
	}
	if tree.Access != "open" {
		t.Errorf("root access = %q, want open", tree.Access)
	}
	if len(tree.Children) != 2 {
		t.Fatalf("got %d folders, want 2", len(tree.Children))
	}
	if tree.Children[0].Kind != classroomKindFolder || tree.Children[0].Order != 1 {
		t.Errorf("first folder = %+v", tree.Children[0])
	}
	pages := tree.Children[0].Children
	if len(pages) != 2 || pages[0].ID != "p1" || pages[0].Kind != classroomKindPage {
		t.Errorf("pages = %+v", pages)
	}
	// Order falls back to position when the payload omits it.
	if pages[1].Order != 2 {
		t.Errorf("second page order = %d, want 2", pages[1].Order)
	}
}

// An unreadable payload must error rather than return an empty tree: an empty
// tree would make push plan a create for every lesson that already exists.
func TestCourseTreeFromPayloadKeepsTheCourseOrder(t *testing.T) {
	raw := json.RawMessage(`{"pageProps":{"course":{"id":"c3","name":"Third",
		"metadata":{"slug":"third","order":3},"modules":[]}}}`)
	tree, err := courseTreeFromPayload(raw)
	if err != nil {
		t.Fatalf("courseTreeFromPayload: %v", err)
	}
	if tree.Order != 3 {
		t.Errorf("course Order = %d, want 3 from metadata.order", tree.Order)
	}

	// No order in the payload: the root stays at zero rather than inventing one.
	raw = json.RawMessage(`{"pageProps":{"course":{"id":"c3","name":"Third","modules":[]}}}`)
	tree, err = courseTreeFromPayload(raw)
	if err != nil {
		t.Fatalf("courseTreeFromPayload: %v", err)
	}
	if tree.Order != 0 {
		t.Errorf("course Order = %d, want 0 when the payload carries none", tree.Order)
	}
}

func TestCourseTreeFromPayloadErrorsOnMissingCourse(t *testing.T) {
	for _, raw := range []string{`{}`, `{"pageProps":{}}`, `{"pageProps":{"course":{}}}`, `{"notFound":true}`} {
		if _, err := courseTreeFromPayload(json.RawMessage(raw)); err == nil {
			t.Errorf("courseTreeFromPayload(%s) = nil error, want one", raw)
		}
	}
}

func TestCourseSummariesFromPayload(t *testing.T) {
	raw := json.RawMessage(`{"pageProps":{"somethingElse":[{"id":"x","name":"not a course"}],
		"courses":[
			{"id":"c2","name":"Second","metadata":{"slug":"second","order":2}},
			{"id":"c1","name":"First","metadata":{"slug":"first","order":1,"access":"level:2"}}]}}`)
	got, err := courseSummariesFromPayload(raw)
	if err != nil {
		t.Fatalf("courseSummariesFromPayload: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d courses, want 2: %+v", len(got), got)
	}
	// Sorted by order, not payload order.
	if got[0].ID != "c1" || got[1].ID != "c2" {
		t.Errorf("order = %+v", got)
	}
	if got[0].Slug != "first" || got[0].Access != "level:2" {
		t.Errorf("first course = %+v", got[0])
	}
}

func TestCourseSummariesFallBackToASlugifiedTitle(t *testing.T) {
	raw := json.RawMessage(`{"pageProps":{"courses":[{"id":"c1","name":"Reading The Signals"}]}}`)
	got, err := courseSummariesFromPayload(raw)
	if err != nil {
		t.Fatalf("courseSummariesFromPayload: %v", err)
	}
	if got[0].Slug != "reading-the-signals" {
		t.Errorf("slug = %q, want the slugified title", got[0].Slug)
	}
}

func TestCourseSummariesErrorWhenTheListIsNotFound(t *testing.T) {
	for _, raw := range []string{
		`{}`,
		`{"pageProps":{}}`,
		`{"pageProps":{"members":[{"id":"m1","name":"Someone"}]}}`,
		// Empty arrays that are not the course list say nothing about courses.
		`{"pageProps":{"courseTags":[]}}`,
		`{"pageProps":{"members":[]}}`,
		`{"courses":[]}`,
		`{"pageProps":{"sidebar":{"courses":[]}}}`,
		// The client's --dry-run placeholder must never read as "no courses".
		`{"dry_run": true}`,
	} {
		if _, err := courseSummariesFromPayload(json.RawMessage(raw)); err == nil {
			t.Errorf("courseSummariesFromPayload(%s) = nil error, want one", raw)
		}
	}
}

func TestCourseSummariesAcceptAnEmptyClassroom(t *testing.T) {
	// An admin who has not created a course yet: a valid, empty classroom.
	for _, raw := range []string{
		`{"pageProps":{"courses":[]}}`,
		`{"pageProps":{"allCourses":[],"members":[{"id":"m1","name":"Someone"}]}}`,
	} {
		got, err := courseSummariesFromPayload(json.RawMessage(raw))
		if err != nil {
			t.Errorf("courseSummariesFromPayload(%s): %v", raw, err)
			continue
		}
		if got == nil || len(got) != 0 {
			t.Errorf("courseSummariesFromPayload(%s) = %#v, want an empty non-nil list", raw, got)
		}
	}
}

func TestCourseSummariesPreferAPopulatedListOverAnEmptyOne(t *testing.T) {
	raw := json.RawMessage(`{"pageProps":{"courses":[],"self":{"enrolledCourses":[{"id":"c1","name":"First"}]}}}`)
	got, err := courseSummariesFromPayload(raw)
	if err != nil {
		t.Fatalf("courseSummariesFromPayload: %v", err)
	}
	if len(got) != 1 || got[0].ID != "c1" {
		t.Errorf("got %+v, want the populated list", got)
	}
}

func noFetch(t *testing.T) func(classroomNode) (classroomNode, error) {
	return func(c classroomNode) (classroomNode, error) {
		t.Errorf("fetched course %q; the plan should not have needed it", c.Title)
		return classroomNode{}, errors.New("unexpected fetch")
	}
}

func TestPlanBundleAgainstAnEmptyClassroomCreatesEverything(t *testing.T) {
	lessons := []lessonFile{
		{Path: "a.md", Course: "1. Start", CourseOrder: 1, Lesson: "1.1 One", Order: 1},
		{Path: "b.md", Course: "1. Start", CourseOrder: 1, Lesson: "1.2 Two", Order: 2},
	}
	plan, orphans, err := planBundle(lessons, []classroomNode{}, noFetch(t))
	if err != nil {
		t.Fatalf("planBundle: %v", err)
	}
	if len(plan) != 3 || plan[0].Op != opCreateCourse || plan[1].Op != opCreatePage || plan[2].Op != opCreatePage {
		t.Errorf("plan = %+v, want create-course then two create-page", plan)
	}
	if len(orphans) != 0 {
		t.Errorf("orphans = %v, want none", orphans)
	}
}

func TestPlanBundleReportsLiveCoursesTheBundleNeverMentions(t *testing.T) {
	lessons := []lessonFile{{Path: "a.md", Course: "1. Start", Lesson: "1.1 One", Order: 1}}
	live := []classroomNode{
		{ID: "c1", Slug: "start", Title: "1. start", Kind: classroomKindCourse},
		{ID: "c2", Slug: "archive", Title: "Old Archive", Kind: classroomKindCourse},
	}
	fetched := 0
	plan, orphans, err := planBundle(lessons, live, func(c classroomNode) (classroomNode, error) {
		fetched++
		if c.ID != "c1" {
			t.Errorf("fetched %q; only the course the bundle targets should be read", c.Title)
		}
		return classroomNode{ID: "c1", Title: "1. Start", Kind: classroomKindCourse, Children: []classroomNode{
			{Kind: classroomKindFolder, Children: []classroomNode{
				{Kind: classroomKindPage, ID: "p1", Title: "1.1 One"},
				{Kind: classroomKindPage, ID: "p2", Title: "1.9 Retired"},
			}},
		}}, nil
	})
	if err != nil {
		t.Fatalf("planBundle: %v", err)
	}
	if fetched != 1 {
		t.Errorf("fetched %d courses, want 1", fetched)
	}
	if len(plan) != 1 || plan[0].Op != opUpdatePage || plan[0].TargetID != "p1" {
		t.Errorf("plan = %+v, want one update of p1", plan)
	}
	if len(orphans) != 2 {
		t.Fatalf("orphans = %v, want the retired page and the unmentioned course", orphans)
	}
	joined := strings.Join(orphans, "\n")
	if !strings.Contains(joined, "1. Start / 1.9 Retired") {
		t.Errorf("orphans = %v, want the retired page", orphans)
	}
	if !strings.Contains(joined, "Old Archive (whole course") {
		t.Errorf("orphans = %v, want the unmentioned course at course level", orphans)
	}
}

func TestPlanBundleRefusesAnAmbiguousCourseTitle(t *testing.T) {
	lessons := []lessonFile{{Path: "a.md", Course: "1. Start", Lesson: "1.1 One", Order: 1}}
	live := []classroomNode{
		{ID: "c1", Slug: "start", Title: "1. Start"},
		{ID: "c2", Slug: "start-2", Title: "1.  START"},
	}
	_, _, err := planBundle(lessons, live, noFetch(t))
	if err == nil {
		t.Fatal("planBundle = nil error, want it to refuse two live courses with one title")
	}
	for _, want := range []string{"c1", "c2", "1. Start"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestPlanBundleIgnoresDuplicateCourseTitlesItDoesNotTarget(t *testing.T) {
	lessons := []lessonFile{{Path: "a.md", Course: "New Course", Lesson: "One", Order: 1}}
	live := []classroomNode{
		{ID: "c1", Slug: "old", Title: "Old"},
		{ID: "c2", Slug: "old-2", Title: "Old"},
	}
	plan, orphans, err := planBundle(lessons, live, noFetch(t))
	if err != nil {
		t.Fatalf("planBundle: %v", err)
	}
	if len(plan) != 2 || plan[0].Op != opCreateCourse {
		t.Errorf("plan = %+v, want create-course + create-page", plan)
	}
	if len(orphans) != 2 {
		t.Errorf("orphans = %v, want both same-titled courses listed", orphans)
	}
}

func TestPlanPagesForCourseRefusesAnAmbiguousPageTitle(t *testing.T) {
	group := courseGroup{
		title:   "1. Start",
		lessons: []lessonFile{{Path: "lessons/a.md", Lesson: "Intro", Order: 1}},
	}
	live := map[string][]classroomNode{
		titleKey("Intro"): {{ID: "p1", Title: "Intro"}, {ID: "p7", Title: "intro"}},
	}
	_, _, err := planPagesForCourse(group, live, true)
	if err == nil {
		t.Fatal("planPagesForCourse = nil error, want it to refuse two live pages with one title")
	}
	for _, want := range []string{"p1", "p7", "Intro", "lessons/a.md"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

func TestPlanPagesForCourseReportsEverySameTitledOrphan(t *testing.T) {
	group := courseGroup{
		title:   "1. Start",
		lessons: []lessonFile{{Path: "a.md", Lesson: "Something else", Order: 1}},
	}
	live := map[string][]classroomNode{
		titleKey("Intro"): {{ID: "p1", Title: "Intro"}, {ID: "p7", Title: "Intro"}},
	}
	ops, orphans, err := planPagesForCourse(group, live, true)
	if err != nil {
		t.Fatalf("planPagesForCourse: %v", err)
	}
	if len(ops) != 1 || ops[0].Op != opCreatePage {
		t.Errorf("ops = %+v, want one create", ops)
	}
	if len(orphans) != 2 {
		t.Errorf("orphans = %v, want both same-titled pages", orphans)
	}
}

func TestFlattenPagesFindsAMovedLesson(t *testing.T) {
	tree := classroomNode{
		Kind: classroomKindCourse,
		Children: []classroomNode{
			{Kind: classroomKindFolder, Title: "Week 1", Children: []classroomNode{
				{Kind: classroomKindPage, ID: "p1", Title: "Why the trend"},
			}},
			{Kind: classroomKindFolder, Title: "Week 2", Children: []classroomNode{
				{Kind: classroomKindPage, ID: "p2", Title: "Reading load"},
			}},
		},
	}
	pages := flattenPages(tree)
	if len(pages) != 2 {
		t.Fatalf("got %d pages, want 2", len(pages))
	}
	if got := pages[titleKey("  why   THE trend ")]; len(got) != 1 || got[0].ID != "p1" {
		t.Errorf("lookup by normalized title failed: %+v", pages)
	}
}

func TestFlattenPagesKeepsEveryPageSharingATitle(t *testing.T) {
	tree := classroomNode{
		Kind: classroomKindCourse,
		Children: []classroomNode{
			{Kind: classroomKindFolder, Title: "Week 1", Children: []classroomNode{
				{Kind: classroomKindPage, ID: "p1", Title: "Intro"},
			}},
			{Kind: classroomKindFolder, Title: "Week 2", Children: []classroomNode{
				{Kind: classroomKindPage, ID: "p2", Title: "INTRO"},
			}},
		},
	}
	got := flattenPages(tree)[titleKey("intro")]
	if len(got) != 2 || got[0].ID != "p1" || got[1].ID != "p2" {
		t.Errorf("pages titled intro = %+v, want p1 then p2", got)
	}
}

func TestAssertClassroomScopeRefusesEverythingElse(t *testing.T) {
	for _, ok := range []string{"/courses", "/courses/abc", "/courses?x=1", "https://api2.skool.com/courses/abc"} {
		if err := assertClassroomScope(ok); err != nil {
			t.Errorf("assertClassroomScope(%q) = %v, want nil", ok, err)
		}
	}
	// The cookie that reaches the classroom also reaches these; a classroom
	// bulk tool must never send them.
	for _, bad := range []string{
		"/group-members/abc",
		"/groups/abc/members/pending",
		"/posts",
		"/notifications",
		"https://api2.skool.com/group-members/abc",
		"/coursesomething",
	} {
		if err := assertClassroomScope(bad); err == nil {
			t.Errorf("assertClassroomScope(%q) = nil, want a refusal", bad)
		}
	}
}

func TestResolveCommunityPrefersTheFlagThenConfig(t *testing.T) {
	got, err := resolveCommunity("from-flag", map[string]string{"community": "from-config"})
	if err != nil || got != "from-flag" {
		t.Errorf("resolveCommunity = (%q, %v), want from-flag", got, err)
	}
	got, err = resolveCommunity("  ", map[string]string{"community": "from-config"})
	if err != nil || got != "from-config" {
		t.Errorf("resolveCommunity = (%q, %v), want from-config", got, err)
	}
	if _, err := resolveCommunity("", map[string]string{}); err == nil {
		t.Error("resolveCommunity with no community anywhere = nil error, want one")
	}
}
