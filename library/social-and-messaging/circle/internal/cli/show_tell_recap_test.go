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
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
)

// TestNovelShowTellRecapHelpWires smoke-tests that the show-tell recap command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelShowTellRecapHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"show-tell", "recap", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("show-tell recap --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "show-tell recap <event-id>"} {
		if !strings.Contains(help, want) {
			t.Fatalf("show-tell recap --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestShowTellMarkdown(t *testing.T) {
	event := circleEvent{Name: "Show & Tell", StartsAt: time.Date(2026, 11, 5, 18, 0, 0, 0, time.UTC)}
	for _, tc := range []struct {
		name, transcript string
		hasRoom          bool
		want             string
	}{
		{"recording absent", "", false, "No live room recording found"},
		{"transcript absent", "", true, "No transcript available"},
		{"transcript present", "Ada: Hello", true, "Ada: Hello"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := showTellMarkdown(event, time.Hour, []circleAttendee{{Name: "Ada"}, {Name: "Bo"}}, tc.transcript, tc.hasRoom)
			for _, part := range []string{"# Show & Tell recap", "2026-11-05T18:00:00Z", "Duration: 1h0m0s", "Attendees: 2", "- Ada\n- Bo", "## Transcript", tc.want} {
				if !strings.Contains(got, part) {
					t.Fatalf("markdown missing %q: %s", part, got)
				}
			}
		})
	}
}

func TestShowTellPostBody(t *testing.T) {
	for _, tc := range []struct {
		name, markdown, want string
	}{
		{"empty", "\n  \n", `[]`},
		{"headings", "# Recap\n## Transcript\n###### Detail", `[
			{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Recap"}]},
			{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Transcript"}]},
			{"type":"heading","attrs":{"level":6},"content":[{"type":"text","text":"Detail"}]}
		]`},
		{"paragraphs", "Ada: Hello\nBo: Hi\n\nNext topic", `[
			{"type":"paragraph","content":[{"type":"text","text":"Ada: Hello Bo: Hi"}]},
			{"type":"paragraph","content":[{"type":"text","text":"Next topic"}]}
		]`},
		{"bullet markers", "- Ada\n* Bo\n+ Cy", `[
			{"type":"bulletList","content":[
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Ada"}]}]},
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Bo"}]}]},
				{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Cy"}]}]}
			]}
		]`},
		{"adjacent blocks and CRLF", "# Recap\r\nAttendees: 1\r\n- Ada\r\n## Transcript\r\nHello", `[
			{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Recap"}]},
			{"type":"paragraph","content":[{"type":"text","text":"Attendees: 1"}]},
			{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Ada"}]}]}]},
			{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Transcript"}]},
			{"type":"paragraph","content":[{"type":"text","text":"Hello"}]}
		]`},
		{"literal text", "#hashtag\n####### text\n<hello> & goodbye", `[
			{"type":"paragraph","content":[{"type":"text","text":"#hashtag ####### text <hello> & goodbye"}]}
		]`},
		{"separate lists", "- Ada\n\n- Bo", `[
			{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Ada"}]}]}]},
			{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"Bo"}]}]}]}
		]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := showTellPostBody("1000969", "Show & Tell", tc.markdown)
			if body["status"] != "draft" || body["skip_notifications"] != true || body["space_id"] != json.Number("1000969") || body["name"] != "Show & Tell recap" {
				t.Fatalf("unsafe post: %v", body)
			}
			data, err := json.Marshal(body["tiptap_body"])
			if err != nil {
				t.Fatal(err)
			}
			var got, want any
			if err := json.Unmarshal(data, &got); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(`{"body":{"type":"doc","content":`+tc.want+`}}`), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("tiptap = %s; want content %s", data, tc.want)
			}
		})
	}
}

func TestShowTellPostID(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      json.Number
		bad       bool
	}{
		{"nested", `{"message":"Post created","post":{"id":30000925}}`, "30000925", false},
		{"top level", `{"id":30000925}`, "30000925", false},
		{"nested takes precedence", `{"id":30000924,"post":{"id":30000925}}`, "30000925", false},
		{"empty post fallback", `{"id":30000925,"post":{}}`, "30000925", false},
		{"null post fallback", `{"id":30000925,"post":null}`, "30000925", false},
		{"missing ID", `{"message":"Post created","post":{}}`, "", true},
		{"invalid nested ID", `{"post":{"id":0}}`, "", true},
		{"invalid top level ID", `{"id":-1}`, "", true},
		{"API failure", `{"success":false,"message":"Rejected","post":{"id":30000925}}`, "", true},
		{"malformed response", `{`, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := showTellPostID(json.RawMessage(tc.raw))
			if (err != nil) != tc.bad || got != tc.want {
				t.Fatalf("showTellPostID() = %q, %v; want %q, error=%v", got, err, tc.want, tc.bad)
			}
		})
	}
}

func TestShowTellValidationAndDryRun(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
		bad  bool
		want string
	}{
		{"schedule help", []string{"show-tell", "schedule"}, false, "Usage:"},
		{"recap help", []string{"show-tell", "recap"}, false, "Usage:"},
		{"schedule missing", []string{"show-tell", "schedule", "--template=3859021"}, true, ""},
		{"recap missing", []string{"show-tell", "recap", "--room=482719"}, true, ""},
		{"post missing space", []string{"show-tell", "recap", "3859021", "--post"}, true, ""},
		{"non-day cadence", []string{"show-tell", "schedule", "--template=3859021", "--every=36h", "--until=2027-01-17"}, true, ""},
		{"invalid until", []string{"show-tell", "schedule", "--template=3859021", "--every=2w", "--until=tomorrow"}, true, ""},
		{"invalid id", []string{"show-tell", "recap", "../3859021"}, true, ""},
		{"schedule dry run", []string{"show-tell", "schedule", "--dry-run", "--json"}, false, `"dry_run":true`},
		{"recap dry run", []string{"show-tell", "recap", "--dry-run", "--json", "--post", "--yes"}, false, `"dry_run":true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testenv.Isolate(t)
			cmd := RootCmd()
			cmd.SetArgs(tc.args)
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			err := cmd.Execute()
			if (err != nil) != tc.bad {
				t.Fatalf("error=%v, output=%s", err, &out)
			}
			if tc.want != "" && !strings.Contains(out.String(), tc.want) {
				t.Fatalf("output missing %q: %s", tc.want, &out)
			}
		})
	}
}

func TestShowTellRecapDryRunDoesNotTouchOutput(t *testing.T) {
	testenv.Isolate(t)
	file := filepath.Join(t.TempDir(), "recap.md")
	if err := os.WriteFile(file, []byte("keep this"), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := RootCmd()
	cmd.SetArgs([]string{"show-tell", "recap", "3859021", "--out", file, "--dry-run"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "keep this" {
		t.Fatal("dry-run overwrote the recap")
	}
}

func TestWriteShowTellRecap(t *testing.T) {
	for _, kind := range []string{"new", "existing", "symlink", "dangling symlink"} {
		t.Run(kind, func(t *testing.T) {
			dir := t.TempDir()
			file, target := filepath.Join(dir, "recap.md"), filepath.Join(dir, "target.md")
			if kind == "existing" {
				if err := os.WriteFile(file, []byte("old"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if strings.Contains(kind, "symlink") {
				if kind == "symlink" {
					if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
						t.Fatal(err)
					}
				}
				if err := os.Symlink(target, file); err != nil {
					t.Fatal(err)
				}
			}
			err := writeShowTellRecap(file, []byte("new recap"))
			if strings.Contains(kind, "symlink") {
				if err == nil {
					t.Fatal("overwrote symlink")
				}
				info, err := os.Lstat(file)
				if err != nil || info.Mode()&os.ModeSymlink == 0 {
					t.Fatalf("symlink changed: %v", err)
				}
				data, err := os.ReadFile(target)
				if kind == "symlink" && (err != nil || string(data) != "keep") {
					t.Fatalf("target changed: %s, %v", data, err)
				}
				if kind == "dangling symlink" && !os.IsNotExist(err) {
					t.Fatalf("created target: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(file)
			if err != nil || string(data) != "new recap" {
				t.Fatalf("file=%s, error=%v", data, err)
			}
			info, err := os.Stat(file)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("permissions: %v, %v", info, err)
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Fatalf("temporary files left behind: %v, %v", files, err)
			}
		})
	}
}

func TestShowTellRecapPreviewOmitsID(t *testing.T) {
	data, err := json.Marshal(showTellRecapPost{SpaceID: "1000969", Status: "draft"})
	if err != nil || strings.Contains(string(data), `"id"`) {
		t.Fatalf("preview ID: %s, %v", data, err)
	}
}
