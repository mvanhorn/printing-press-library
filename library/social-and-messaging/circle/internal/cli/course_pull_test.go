// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
)

// TestNovelCoursePullHelpWires smoke-tests that the course pull command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCoursePullHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"course", "pull", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("course pull --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "course pull <folder>"} {
		if !strings.Contains(help, want) {
			t.Fatalf("course pull --help missing %q in output:\n%s", want, help)
		}
	}
	leaf := newNovelCoursePullCmd(&rootFlags{})
	if leaf.Annotations["pp:method"] != "GET" || leaf.Annotations["mcp:write-positionals"] != "0" || leaf.Annotations["mcp:read-only"] == "true" {
		t.Fatal("course pull must declare its GET while keeping its local file destination write-aware")
	}
	if leaf.Annotations["pp:happy-args"] != "folder=course-pull-out;--space=1001068" {
		t.Fatal("course pull live fixture must target pp-test-course")
	}
}

func TestCourseSlug(t *testing.T) {
	for _, tt := range []struct{ in, want string }{{"Hello, World!", "hello-world"}, {"../../", "untitled"}, {"  Déjà Vu  ", "déjà-vu"}, {"A___B---C", "a-b-c"}, {"", "untitled"}} {
		if got := courseSlug(tt.in); got != tt.want {
			t.Errorf("slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCoursePullRoundTrip(t *testing.T) {
	live := courseLive{Sections: []courseSection{{ID: 7, Name: "Welcome: start here"}, {ID: 8, Name: "Empty"}}, Lessons: []courseLesson{{ID: 42, SectionID: 7, Name: "Hello: world", Status: "draft", BodyHTML: courseTestBody("<p>Hello <strong>world</strong></p>\n")}}}
	live.Lessons = append(live.Lessons,
		courseLesson{ID: 43, SectionID: 7, Name: "Wrapped HTML", Status: "published", BodyHTML: courseTestBody("  <div><h2>Hello</h2>\n<p>Second lesson body.</p></div>\n")},
		courseLesson{ID: 44, SectionID: 7, Name: "Inline HTML", Status: "draft", BodyHTML: courseTestBody("<span>Keep **literal** Markdown &amp; HTML</span>\n")},
	)
	files, manifest, err := coursePullFiles(live, 1000969)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	for _, file := range files {
		if err := writeCourseFile(folder, file.Path, file.Data); err != nil {
			t.Fatal(err)
		}
	}
	if err := saveCourseManifest(folder, manifest); err != nil {
		t.Fatal(err)
	}
	local, err := readCourseFolder(folder)
	if err != nil {
		t.Fatal(err)
	}
	if local.Manifest.SpaceID != 1000969 || len(local.Sections) != 2 || local.Sections[0].Lessons[0].ID != 42 {
		t.Fatalf("local = %+v", local)
	}
	for i, lesson := range local.Sections[0].Lessons {
		if lesson.BodyHTML != *live.Lessons[i].BodyHTML {
			t.Fatalf("HTML changed on read: %q, want %q", lesson.BodyHTML, *live.Lessons[i].BodyHTML)
		}
	}
	p, err := buildCoursePlan(local, live, 1000969, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Plan) != 0 || p.Summary["create"] != 0 || p.Summary["update"] != 0 || p.Summary["noop"] != 3 {
		t.Fatalf("round trip changed content: %+v", p.Plan)
	}
	wantOrder := []courseOrder{{SectionID: 7, LessonIDs: []int64{42, 43, 44}}, {SectionID: 8, LessonIDs: []int64{}}}
	if !reflect.DeepEqual(local.Manifest.Order, wantOrder) {
		t.Fatalf("saved order = %+v, want %+v", local.Manifest.Order, wantOrder)
	}
	// A prefix change must reorder ID-matched lessons without changing their content.
	if err := os.Rename(filepath.Join(folder, "01-welcome-start-here/02-wrapped-html.md"), filepath.Join(folder, "01-welcome-start-here/00-wrapped-html.md")); err != nil {
		t.Fatal(err)
	}
	local, err = readCourseFolder(folder)
	if err != nil {
		t.Fatal(err)
	}
	p, err = buildCoursePlan(local, live, 1000969, false)
	if err != nil {
		t.Fatal(err)
	}
	wantOrder[0].LessonIDs = []int64{43, 42, 44}
	if len(p.Plan) != 1 || p.Plan[0].Action != "reorder" || !reflect.DeepEqual(p.Order, wantOrder) {
		t.Fatalf("prefix change plan = %+v, order = %+v", p.Plan, p.Order)
	}
}

func TestCourseDestination(t *testing.T) {
	folder := t.TempDir()
	if err := checkCourseDestination(folder, false, 1000969); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, "existing"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := checkCourseDestination(folder, false, 1000969); err == nil {
		t.Fatal("accepted non-empty folder")
	}
	if err := checkCourseDestination(folder, true, 1000969); err != nil {
		t.Fatal(err)
	}
	if err := checkCourseDestination(filepath.Join(folder, "new"), false, 1000969); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(folder, "section")); err != nil {
		t.Fatal(err)
	}
	if err := writeCourseFile(folder, "section/lesson.md", []byte("unsafe")); err == nil {
		t.Fatal("wrote through section symlink")
	}
}

func TestCourseDryRunDoesNotReadFolder(t *testing.T) {
	for _, command := range []string{"push", "pull"} {
		t.Run(command, func(t *testing.T) {
			testenv.Isolate(t)
			cmd := RootCmd()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"course", command, "/nonexistent/course-folder", "--dry-run", "--json"})
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			var result struct {
				DryRun  bool `json:"dry_run"`
				Success bool `json:"success"`
			}
			if err := json.Unmarshal(out.Bytes(), &result); err != nil || !result.DryRun || !result.Success {
				t.Fatalf("output = %s", out.String())
			}
		})
	}
}

func TestCoursePullChecksSpace(t *testing.T) {
	for _, tt := range []struct {
		name, manifest string
		bad            bool
	}{
		{"same course", "space_id: 1000969\n", false},
		{"other course", "space_id: 1000970\n", true},
		{"unassigned", "sections: {}\n", false},
		{"invalid manifest", "space_id: [\n", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			folder := t.TempDir()
			if err := os.WriteFile(filepath.Join(folder, "course.yaml"), []byte(tt.manifest), 0600); err != nil {
				t.Fatal(err)
			}
			err := checkCourseDestination(folder, true, 1000969)
			if (err != nil) != tt.bad {
				t.Fatalf("error = %v", err)
			}
		})
	}
}

func TestCoursePullReportsStaleFiles(t *testing.T) {
	folder := t.TempDir()
	for _, name := range []string{"01-old/_section.md", "01-old/01-old.md", "01-old/02-keep.md", "02-removed/_section.md", "02-removed/01-gone.md", "01-old/notes.md", "01-old/01-media.mp4", "notes/01-unrelated.md"} {
		if err := writeCourseFile(folder, name, []byte("keep")); err != nil {
			t.Fatal(err)
		}
	}
	files := []courseDiskFile{{Path: "01-old/_section.md"}, {Path: "01-old/02-keep.md"}}
	stale, err := staleCourseFiles(folder, files)
	want := []string{"01-old/01-old.md", "02-removed/01-gone.md", "02-removed/_section.md"}
	if err != nil || !reflect.DeepEqual(stale, want) {
		t.Fatalf("stale = %v, error = %v", stale, err)
	}
	for _, name := range stale {
		data, err := os.ReadFile(filepath.Join(folder, name))
		if err != nil || string(data) != "keep" {
			t.Fatalf("stale file was changed: %s, %v", name, err)
		}
	}
	stale, err = staleCourseFiles(filepath.Join(folder, "missing"), nil)
	if err != nil || stale == nil || len(stale) != 0 {
		t.Fatalf("new destination: %v, %v", stale, err)
	}
}

func TestCoursePullRecordsBaseline(t *testing.T) {
	body := " <div><p>Hello</p></div>\n"
	live := courseLive{Sections: []courseSection{{ID: 7, Name: "Welcome"}}, Lessons: []courseLesson{{ID: 42, SectionID: 7, Name: "Hello", Status: "published", BodyHTML: &body}}}
	_, manifest, err := coursePullFiles(live, 1000969)
	if err != nil {
		t.Fatal(err)
	}
	ref := manifest.Lessons["01-welcome/01-hello.md"]
	if ref.Status != "published" || len(ref.BodySHA256) != 64 || ref.BodySHA256 != courseBodySHA256("<p>Hello</p>") {
		t.Fatalf("baseline = %+v", ref)
	}
}
