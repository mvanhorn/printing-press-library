// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
)

// TestNovelCoursePushHelpWires smoke-tests that the course push command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCoursePushHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"course", "push", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("course push --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "course push <folder>"} {
		if !strings.Contains(help, want) {
			t.Fatalf("course push --help missing %q in output:\n%s", want, help)
		}
	}
}

func courseTestBody(s string) *string { return &s }

func TestCourseBodyWhitespaceComparison(t *testing.T) {
	for _, tt := range []struct {
		name, before, after string
		equal               bool
	}{
		{"code indentation", "<pre><code>if ok:\n  run()\n</code></pre>", "<pre><code>if ok:\n    run()\n</code></pre>", false},
		{"inline code", "<p><code>a  b</code></p>", "<p><code>a b</code></p>", false},
		{"pre whitespace", "<pre>  \n</pre>", "<pre> \n</pre>", false},
		{"ordinary whitespace", "<p>Two  words\n here</p>", "<p>Two words here</p>", true},
		{"around code blocks", "<p>Body</p>\n<pre><code>  run()\n</code></pre>\n<p>End</p>", "<p>Body</p><pre><code>  run()\n</code></pre><p>End</p>", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if (normalizeCourseBody(tt.before) == normalizeCourseBody(tt.after)) != tt.equal || (courseBodySHA256(tt.before) == courseBodySHA256(tt.after)) != tt.equal {
				t.Fatal("normalized body/hash lost meaningful whitespace or retained ordinary whitespace")
			}
		})
	}
}

func TestCourseMovedPathReused(t *testing.T) {
	for _, path := range []string{"01-one/new.md", "02-two/new.md"} {
		t.Run(path, func(t *testing.T) {
			folder := t.TempDir()
			old := "01-one/old.md"
			baseline := courseRef{ID: 2, Status: "draft", BodySHA256: courseBodySHA256("Body"), MediaChecksum: "media"}
			manifest := emptyCourseManifest()
			manifest.Lessons[old] = baseline
			manifest.Sections["01-one"] = courseRef{ID: 1}
			manifest.Order = []courseOrder{{SectionID: 1, LessonIDs: []int64{2}}}
			live := courseLive{Sections: []courseSection{{ID: 1, Name: "One"}}, Lessons: []courseLesson{{ID: 2, SectionID: 1, Name: "Moved", Status: "draft", BodyHTML: courseTestBody("Body")}}}
			local := courseFolder{Manifest: manifest, Sections: []courseLocalSection{{ID: 1, Path: "01-one", Name: "One", Lessons: []courseLocalLesson{{ID: 2, Path: path, Name: "Moved", BodyHTML: "Body"}}}}}
			plan, err := buildCoursePlan(local, live, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := plan.Manifest.Lessons[old]; ok || plan.Manifest.Lessons[path] != baseline {
				t.Fatalf("move lost baseline or retained old path: %+v", plan.Manifest)
			}
			if err := applyCoursePlan(context.Background(), nil, folder, manifest, &plan); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Join(folder, "01-one"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(folder, old), []byte("---\nname: Replacement\n---\nNew body"), 0600); err != nil {
				t.Fatal(err)
			}
			next, err := readCourseFolder(folder)
			if err != nil {
				t.Fatal(err)
			}
			plan, err = buildCoursePlan(next, live, 1, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.Plan) == 0 || plan.Plan[0].Action != "create" || plan.Plan[0].Kind != "lesson" || plan.Plan[0].ID == 2 {
				t.Fatalf("replacement plan = %+v", plan.Plan)
			}
		})
	}
}

func TestNormalizeCourseBody(t *testing.T) {
	for _, tt := range []struct{ name, input, want string }{
		{"wrapper", "  <div><p>Second lesson body.</p></div>\n", "<p>Second lesson body.</p>"},
		{"markdown output", "<p>Second lesson body.</p>\n", "<p>Second lesson body.</p>"},
		{"heading and whitespace", "<div>\n<h1>Hello</h1>\n <p>Two  words\n here.</p>\n</div>", "<h2>Hello</h2><p>Two words here.</p>"},
		{"heading attributes", `<h1 id="hello">Hello</h1>`, `<h2 id="hello">Hello</h2>`},
		{"one wrapper", "<div><div><p>Hello</p></div></div>", "<div><p>Hello</p></div>"},
		{"empty", " \n\t", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeCourseBody(tt.input); got != tt.want {
				t.Fatalf("normalize = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCourseLessonChanges(t *testing.T) {
	for _, tt := range []struct {
		name, lessonName, status, body string
		changes                        []string
	}{
		{"normalized noop", "Hello", "draft", "<h1>Hello</h1>\n<p>Two   words</p>\n", nil},
		{"name", "Renamed", "draft", "<h2>Hello</h2><p>Two words</p>", []string{"name"}},
		{"status", "Hello", "published", "<h2>Hello</h2><p>Two words</p>", []string{"status"}},
		{"body", "Hello", "draft", "<p>Changed</p>\n", []string{"body_html"}},
		{"all", "Renamed", "published", "Changed", []string{"name", "status", "body_html"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			live := courseLive{Sections: []courseSection{{ID: 7, Name: "Welcome"}}, Lessons: []courseLesson{{ID: 42, SectionID: 7, Name: "Hello", Status: "draft", BodyHTML: courseTestBody("<div><h2>Hello</h2><p>Two words</p></div>")}}}
			local := courseFolder{Manifest: emptyCourseManifest(), Sections: []courseLocalSection{{ID: 7, Name: "Welcome", Lessons: []courseLocalLesson{{ID: 42, Name: tt.lessonName, Status: tt.status, BodyHTML: tt.body}}}}}
			local.Manifest.Order = []courseOrder{{SectionID: 7, LessonIDs: []int64{42}}}
			p, err := buildCoursePlan(local, live, 1000969, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(tt.changes) == 0 {
				if len(p.Plan) != 0 || p.Summary["noop"] != 1 {
					t.Fatalf("unchanged plan = %+v", p)
				}
				return
			}
			data, err := json.Marshal(p.Plan)
			if err != nil {
				t.Fatal(err)
			}
			var steps []courseStep
			if err := json.Unmarshal(data, &steps); err != nil {
				t.Fatal(err)
			}
			fields := make([]string, 0)
			for _, change := range steps[0].Changes {
				fields = append(fields, change.Field)
				if change.Field == "status" && (change.From != "draft" || change.To != "published") {
					t.Fatalf("status change = %+v", change)
				}
				if change.Field == "body_html" && (change.From != "" || change.To != "") {
					t.Fatal("body leaked into preview")
				}
			}
			if len(steps) != 1 || steps[0].Action != "update" || !reflect.DeepEqual(fields, tt.changes) {
				t.Fatalf("JSON steps = %s", data)
			}
			if len(p.Plan[0].Body) != len(tt.changes) {
				t.Fatalf("body = %+v", p.Plan[0].Body)
			}
			for _, field := range tt.changes {
				if _, ok := p.Plan[0].Body[field]; !ok {
					t.Fatalf("missing body field %s", field)
				}
			}
		})
	}
}

func TestCourseFrontmatter(t *testing.T) {
	for _, tt := range []struct {
		name, input, wantName, wantBody string
		wantID                          int64
		bad                             bool
	}{
		{"plain", "# Hello\n", "", "# Hello\n", 0, false},
		{"yaml", "---\nname: 'Start: here'\nid: 42\nstatus: draft\n---\nHello\n", "Start: here", "Hello\n", 42, false},
		{"crlf", "---\r\nname: Hello\r\n---\r\nBody", "Hello", "Body", 0, false},
		{"unclosed", "---\nname: Missing", "", "", 0, true},
		{"bad status", "---\nstatus: public\n---\n", "", "", 0, true},
		{"bad id", "---\nid: -1\n---\n", "", "", 0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			front, body, err := parseCourseMarkdown([]byte(tt.input))
			if (err != nil) != tt.bad {
				t.Fatalf("error = %v", err)
			}
			if tt.bad {
				return
			}
			if front.Name != tt.wantName || front.ID != tt.wantID || body != tt.wantBody {
				t.Fatalf("got %+v %q", front, body)
			}
		})
	}
}

func TestReadCourseFolder(t *testing.T) {
	folder := t.TempDir()
	files := map[string]string{
		"course.yaml":                  "space_id: 1000969\nsections:\n  02-welcome: {id: 7}\nlessons:\n  02-welcome/02-start-here.md: {id: 8}\n",
		"10-last/01-end.md":            "Done",
		"02-welcome/10-other.md":       "Other",
		"02-welcome/02-start-here.md":  "---\nname: Getting Started\nid: 99\nstatus: published\n---\n# Hello\n",
		"02-welcome/02-start-here.mp4": "media",
		"01-first/_section.md":         "---\nname: Introduction\nid: 6\n---\n",
	}
	for name, data := range files {
		target := filepath.Join(folder, name)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
	}
	local, err := readCourseFolder(folder)
	if err != nil {
		t.Fatal(err)
	}
	if len(local.Sections) != 3 || local.Sections[0].Name != "Introduction" || local.Sections[0].ID != 6 || local.Sections[1].Name != "Welcome" || local.Sections[2].Name != "Last" {
		t.Fatalf("sections = %+v", local.Sections)
	}
	lesson := local.Sections[1].Lessons[0]
	if lesson.ID != 8 || lesson.Name != "Getting Started" || lesson.Status != "published" || lesson.BodyHTML != "<h1>Hello</h1>\n" || lesson.Checksum == "" || lesson.Size != 5 {
		t.Fatalf("lesson = %+v", lesson)
	}
}

func TestCoursePlanMatching(t *testing.T) {
	live := courseLive{Sections: []courseSection{{ID: 1, Name: "Welcome"}}, Lessons: []courseLesson{{ID: 11, SectionID: 1, Name: "Hello", Status: "draft", BodyHTML: courseTestBody("<p>Hello</p>\n")}}}
	for _, tt := range []struct {
		name                                  string
		sectionID, lessonID                   int64
		sectionName, lessonName, status, body string
		publish                               bool
		want                                  []string
		bad                                   bool
	}{
		{"name noop", 0, 0, "Welcome", "Hello", "", "<p>Hello</p>\n", false, []string{}, false},
		{"id rename", 1, 11, "Renamed", "New name", "", "<p>Hello</p>\n", false, []string{"rename", "update"}, false},
		{"body update", 1, 11, "Welcome", "Hello", "", "Changed", false, []string{"update"}, false},
		{"publish preserves existing", 1, 11, "Welcome", "Hello", "", "<p>Hello</p>\n", true, []string{}, false},
		{"explicit publish existing", 1, 11, "Welcome", "Hello", "published", "<p>Hello</p>\n", false, []string{"update"}, false},
		{"new lesson", 1, 0, "Welcome", "New", "", "Body", false, []string{"create", "reorder"}, false},
		{"missing id", 1, 999, "Welcome", "Hello", "", "Body", false, nil, true},
		{"missing section id", 999, 11, "Welcome", "Hello", "", "Body", false, nil, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			local := courseFolder{Manifest: emptyCourseManifest(), Sections: []courseLocalSection{{ID: tt.sectionID, Name: tt.sectionName, Path: "01-welcome", Lessons: []courseLocalLesson{{ID: tt.lessonID, Name: tt.lessonName, Path: "01-welcome/01-hello.md", Status: tt.status, BodyHTML: tt.body}}}}}
			local.Manifest.Order = []courseOrder{{SectionID: 1, LessonIDs: []int64{11}}}
			p, err := buildCoursePlan(local, live, 1000969, tt.publish)
			if (err != nil) != tt.bad {
				t.Fatalf("err = %v", err)
			}
			if tt.bad {
				return
			}
			actions := make([]string, 0)
			for _, step := range p.Plan {
				actions = append(actions, step.Action)
			}
			if !reflect.DeepEqual(actions, tt.want) {
				t.Fatalf("actions = %v, want %v", actions, tt.want)
			}
		})
	}
}

func TestCourseRecordedOrder(t *testing.T) {
	for _, tt := range []struct {
		name     string
		recorded []courseOrder
		want     int
	}{
		{"matches folder despite live creation order", []courseOrder{{SectionID: 7, LessonIDs: []int64{43, 42}}}, 0},
		{"changed folder order", []courseOrder{{SectionID: 7, LessonIDs: []int64{42, 43}}}, 1},
		{"first push", nil, 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			local := courseFolder{Manifest: emptyCourseManifest(), Sections: []courseLocalSection{{ID: 7, Name: "Welcome", Lessons: []courseLocalLesson{{ID: 43, Name: "Second"}, {ID: 42, Name: "First"}}}}}
			local.Manifest.Order = tt.recorded
			if tt.recorded == nil {
				local.Sections[0].Lessons[0], local.Sections[0].Lessons[1] = local.Sections[0].Lessons[1], local.Sections[0].Lessons[0]
			}
			live := courseLive{Sections: []courseSection{{ID: 7, Name: "Welcome"}}, Lessons: []courseLesson{{ID: 42, SectionID: 7, Name: "First"}, {ID: 43, SectionID: 7, Name: "Second"}}}
			p, err := buildCoursePlan(local, live, 1000969, false)
			if err != nil {
				t.Fatal(err)
			}
			if p.Summary["reorder"] != tt.want || len(p.Plan) != tt.want {
				t.Fatalf("plan = %+v, want %d reorder", p.Plan, tt.want)
			}
		})
	}
}

func TestCourseNewLessonStatus(t *testing.T) {
	for _, tt := range []struct {
		status  string
		publish bool
		want    string
	}{{"", false, "draft"}, {"", true, "published"}, {"draft", true, "published"}, {"published", false, "published"}} {
		local := courseFolder{Manifest: emptyCourseManifest(), Sections: []courseLocalSection{{Name: "New", Path: "01-new", Lessons: []courseLocalLesson{{Name: "Hello", Path: "01-new/01-hello.md", Status: tt.status}}}}}
		p, err := buildCoursePlan(local, courseLive{}, 1, tt.publish)
		if err != nil {
			t.Fatal(err)
		}
		if p.Plan[1].Body["status"] != tt.want {
			t.Fatalf("status=%q publish=%t: %+v", tt.status, tt.publish, p.Plan)
		}
	}
}

func TestCourseReorderPreservesUnmanaged(t *testing.T) {
	live := courseLive{Sections: []courseSection{{ID: 1, Name: "One"}, {ID: 2, Name: "Two"}}, Lessons: []courseLesson{{ID: 10, SectionID: 1}, {ID: 11, SectionID: 1}, {ID: 20, SectionID: 2}}}
	local := courseFolder{Manifest: emptyCourseManifest(), Sections: []courseLocalSection{{ID: 1, Name: "One", Path: "01-one", Lessons: []courseLocalLesson{{ID: 11, Name: "Managed", Path: "01-one/01-managed.md"}}}}}
	p, err := buildCoursePlan(local, live, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	want := []courseOrder{{SectionID: 1, LessonIDs: []int64{10, 11}}, {SectionID: 2, LessonIDs: []int64{20}}}
	if !reflect.DeepEqual(p.Order, want) {
		t.Fatalf("order = %+v", p.Order)
	}
	// Moving an ID-matched lesson must not leave it in its original section too.
	local.Sections[0].Lessons[0].ID = 20
	p, err = buildCoursePlan(local, live, 1, false)
	if err != nil {
		t.Fatal(err)
	}
	want = []courseOrder{{SectionID: 1, LessonIDs: []int64{10, 11, 20}}, {SectionID: 2, LessonIDs: []int64{}}}
	if !reflect.DeepEqual(p.Order, want) {
		t.Fatalf("moved order = %+v", p.Order)
	}
}

func TestCourseAmbiguousNames(t *testing.T) {
	for _, live := range []courseLive{
		{Sections: []courseSection{{ID: 1, Name: "One"}, {ID: 2, Name: "One"}}},
		{Sections: []courseSection{{ID: 1, Name: "One"}}, Lessons: []courseLesson{{ID: 10, SectionID: 1, Name: "Hello"}, {ID: 11, SectionID: 1, Name: "Hello"}}},
	} {
		local := courseFolder{Manifest: emptyCourseManifest(), Sections: []courseLocalSection{{Name: "One", Lessons: []courseLocalLesson{{Name: "Hello"}}}}}
		if _, err := buildCoursePlan(local, live, 1, false); err == nil {
			t.Fatal("accepted ambiguous match")
		}
	}
}

func TestCourseMediaPlan(t *testing.T) {
	for _, checksum := range []string{"same", "changed"} {
		manifest := emptyCourseManifest()
		manifest.Lessons["lesson.md"] = courseRef{ID: 2, MediaChecksum: checksum}
		local := courseFolder{Manifest: manifest, Sections: []courseLocalSection{{ID: 1, Name: "One", Lessons: []courseLocalLesson{{ID: 2, Name: "Hello", Path: "lesson.md", MediaPath: "lesson.mp4", Checksum: "same"}}}}}
		live := courseLive{Sections: []courseSection{{ID: 1, Name: "One"}}, Lessons: []courseLesson{{ID: 2, SectionID: 1, Name: "Hello", BodyHTML: courseTestBody("")}}}
		p, err := buildCoursePlan(local, live, 1, false)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		if checksum == "changed" {
			want = 1
		}
		if p.Summary["upload"] != want {
			t.Fatalf("plan = %+v", p.Plan)
		}
	}
}

func TestCourseSyncConflicts(t *testing.T) {
	for _, tt := range []struct {
		name, localBody, localStatus, liveBody, liveStatus, action string
	}{
		{"unchanged", "<p>Original</p>", "draft", "<p>Original</p>", "draft", ""},
		{"normalized", " <div><p>Original</p></div> ", "draft", "<p>Original</p>", "draft", ""},
		{"local body", "<p>Local</p>", "draft", "<p>Original</p>", "draft", "update"},
		{"local status", "<p>Original</p>", "published", "<p>Original</p>", "draft", "update"},
		{"live body", "<p>Original</p>", "draft", "<p>Live</p>", "draft", "live-edited"},
		{"live status", "<p>Original</p>", "draft", "<p>Original</p>", "published", "live-edited"},
		{"live status without frontmatter", "<p>Original</p>", "", "<p>Original</p>", "published", "live-edited"},
		{"both body", "<p>Local</p>", "draft", "<p>Live</p>", "draft", "conflict"},
		{"cross field", "<p>Local</p>", "draft", "<p>Original</p>", "published", "conflict"},
		{"status local body live", "<p>Original</p>", "published", "<p>Live</p>", "draft", "conflict"},
		{"converged", "<p>Same edit</p>", "published", "<p>Same edit</p>", "published", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			manifest := emptyCourseManifest()
			baseline := courseRef{ID: 42, Status: "draft", BodySHA256: courseBodySHA256("<p>Original</p>")}
			// An ID match must retain the baseline even after the file is renamed.
			manifest.Lessons["01-welcome/01-old.md"] = baseline
			manifest.Order = []courseOrder{{SectionID: 7, LessonIDs: []int64{42}}}
			path := "01-welcome/01-new.md"
			local := courseFolder{Manifest: manifest, Sections: []courseLocalSection{{ID: 7, Name: "Welcome", Lessons: []courseLocalLesson{{ID: 42, Path: path, Name: "Hello", Status: tt.localStatus, BodyHTML: tt.localBody}}}}}
			live := courseLive{Sections: []courseSection{{ID: 7, Name: "Welcome"}}, Lessons: []courseLesson{{ID: 42, SectionID: 7, Name: "Hello", Status: tt.liveStatus, BodyHTML: courseTestBody(tt.liveBody)}}}
			p, err := buildCoursePlan(local, live, 1000969, false)
			if err != nil {
				t.Fatal(err)
			}
			if tt.action == "" {
				if len(p.Plan) != 0 || p.Summary["noop"] != 1 {
					t.Fatalf("plan = %+v", p.Plan)
				}
				if p.Manifest.Lessons[path].BodySHA256 != courseBodySHA256(tt.localBody) {
					t.Fatal("current path kept a stale baseline")
				}
				return
			}
			if len(p.Plan) != 1 || p.Plan[0].Action != tt.action {
				t.Fatalf("plan = %+v", p.Plan)
			}
			step := p.Plan[0]
			if tt.action == "conflict" && (!step.LiveChanged || !step.LocalChanged) {
				t.Fatalf("missing conflict sides: %+v", step)
			}
			if p.Manifest.Lessons[path] != baseline {
				t.Fatal("preview advanced sync baseline")
			}
			if tt.action == "live-edited" || tt.action == "conflict" {
				folder := t.TempDir()
				// nil client proves blocked steps never attempt a write.
				err := applyCoursePlan(context.Background(), nil, folder, manifest, &p)
				if tt.action == "conflict" {
					if err == nil || !strings.Contains(err.Error(), "--force --yes") {
						t.Fatalf("conflict error = %v", err)
					}
					if _, err := os.Stat(filepath.Join(folder, "course.yaml")); !os.IsNotExist(err) {
						t.Fatalf("conflict wrote manifest: %v", err)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				saved, err := readCourseFolder(folder)
				if err != nil || saved.Manifest.Lessons[path] != baseline {
					t.Fatalf("blocked baseline changed: %+v, %v", saved.Manifest, err)
				}
			}
		})
	}
}

func TestMergeCourseIDs(t *testing.T) {
	for _, tt := range []struct {
		name                string
		base, desired, want []int64
	}{
		{"partial", []int64{1, 2, 3, 4, 5}, []int64{5, 1}, []int64{5, 2, 3, 4, 1}},
		{"single", []int64{1, 2, 3}, []int64{3}, []int64{1, 2, 3}},
		{"new", []int64{1, 2, 3}, []int64{3, 6, 1}, []int64{3, 2, 6, 1}},
		{"none", []int64{1, 2}, nil, []int64{1, 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := mergeCourseIDs(tt.base, tt.desired); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCoursePartialOrder(t *testing.T) {
	live := courseLive{Sections: []courseSection{{ID: 1}, {ID: 2}, {ID: 3}}, Lessons: []courseLesson{{ID: 10, SectionID: 1}, {ID: 11, SectionID: 1}, {ID: 12, SectionID: 1}, {ID: 30, SectionID: 3}}}
	desired := []courseOrder{{SectionID: 3, LessonIDs: []int64{30}}, {SectionID: 1, LessonIDs: []int64{12, 10}}}
	for _, tt := range []struct {
		name           string
		recorded, want []courseOrder
	}{
		{"live fallback", nil, []courseOrder{{SectionID: 3, LessonIDs: []int64{30}}, {SectionID: 2, LessonIDs: []int64{}}, {SectionID: 1, LessonIDs: []int64{12, 11, 10}}}},
		{"recorded", []courseOrder{{SectionID: 2}, {SectionID: 1, LessonIDs: []int64{11, 10, 12}}, {SectionID: 3, LessonIDs: []int64{30}}}, []courseOrder{{SectionID: 2, LessonIDs: []int64{}}, {SectionID: 3, LessonIDs: []int64{30}}, {SectionID: 1, LessonIDs: []int64{11, 12, 10}}}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeCourseOrder(tt.recorded, live, desired)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("order = %+v, want %+v", got, tt.want)
			}
			data, err := json.Marshal(coursePlan{Order: got})
			if err != nil || !bytes.Contains(data, []byte(`"order":[`)) {
				t.Fatalf("order missing in preview: %s, %v", data, err)
			}
		})
	}
}

func TestCourseUploadError(t *testing.T) {
	for _, tt := range []struct {
		name  string
		cause error
		want  string
	}{
		{"timeout", context.DeadlineExceeded, "timed out; raise --timeout"},
		{"canceled", context.Canceled, "context canceled"},
		{"other", errors.New("connection failed"), "presigned upload failed"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := courseUploadError(&url.Error{Op: "Put", URL: "https://upload.invalid/?signature=private", Err: tt.cause})
			if !strings.Contains(err.Error(), tt.want) || strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "upload.invalid") {
				t.Fatalf("error = %v", err)
			}
			if errors.Is(tt.cause, context.DeadlineExceeded) || errors.Is(tt.cause, context.Canceled) {
				if !errors.Is(err, tt.cause) {
					t.Fatal("lost context error identity")
				}
			}
		})
	}
}

func TestCoursePushRejectsSectionSymlink(t *testing.T) {
	folder := t.TempDir()
	link := filepath.Join(folder, "01-linked-section")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	if _, err := readCourseFolder(folder); err == nil || !strings.Contains(err.Error(), link) {
		t.Fatalf("error = %v", err)
	}
}

func TestCourseStepShouldApply(t *testing.T) {
	for _, tt := range []struct {
		action                   string
		liveChanged, force, want bool
	}{
		{"update", false, false, true},
		{"conflict", true, false, false},
		{"conflict", true, true, true},
		{"live-edited", true, false, false},
		{"live-edited", true, true, false},
		{"upload", true, false, false},
		{"upload", true, true, true},
		{"upload", false, false, true},
	} {
		step := courseStep{Action: tt.action, LiveChanged: tt.liveChanged}
		if got := step.shouldApply(tt.force); got != tt.want {
			t.Errorf("%s liveChanged=%t force=%t: got %t", tt.action, tt.liveChanged, tt.force, got)
		}
	}
}

func TestRecordCourseLessonSync(t *testing.T) {
	manifest := emptyCourseManifest()
	manifest.Lessons["old.md"] = courseRef{ID: 42, MediaChecksum: "media"}
	manifest.Lessons["renamed.md"] = courseRef{ID: 42}
	manifest.Lessons["other.md"] = courseRef{ID: 43, Status: "draft"}
	synced := courseRef{Status: "published", BodySHA256: courseBodySHA256("Updated")}
	recordCourseLessonSync(manifest, 42, synced)
	for _, path := range []string{"old.md", "renamed.md"} {
		ref := manifest.Lessons[path]
		if ref.Status != synced.Status || ref.BodySHA256 != synced.BodySHA256 || ref.ID != 42 {
			t.Fatalf("%s: %+v", path, ref)
		}
	}
	if manifest.Lessons["old.md"].MediaChecksum != "media" || manifest.Lessons["other.md"].Status != "draft" {
		t.Fatal("sync changed unrelated metadata")
	}
}
