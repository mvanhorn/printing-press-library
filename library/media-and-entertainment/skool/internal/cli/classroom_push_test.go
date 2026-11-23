// Copyright 2026 Zain Haseeb and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
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
	live := map[string]classroomNode{
		// Casing and inner whitespace differ from the bundle on purpose.
		titleKey("2.1   why the TREND beats the day"): {ID: "page-1", Title: "2.1 Why the trend beats the day", Kind: classroomKindPage},
		titleKey("2.9 Retired lesson"):                {ID: "page-9", Title: "2.9 Retired lesson", Kind: classroomKindPage},
	}

	ops, orphans := planPagesForCourse(group, live, true)
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
	ops, orphans := planPagesForCourse(group, map[string]classroomNode{}, false)
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
		`{"pageProps":{"courses":[]}}`,
		`{"pageProps":{"members":[{"id":"m1","name":"Someone"}]}}`,
	} {
		if _, err := courseSummariesFromPayload(json.RawMessage(raw)); err == nil {
			t.Errorf("courseSummariesFromPayload(%s) = nil error, want one", raw)
		}
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
	if pages[titleKey("  why   THE trend ")].ID != "p1" {
		t.Errorf("lookup by normalized title failed: %+v", pages)
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
