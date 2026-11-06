// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/circle/internal/cliutil/testenv"
)

// TestNovelSpacesApplyHelpWires smoke-tests that the spaces apply command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelSpacesApplyHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"spaces", "apply", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("spaces apply --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "apply", "space_groups:\n  - name: Spaces\n    slug: spaces\nspaces:\n  - name: Start Here\n    slug: start-here\n    group: Spaces\n    type: basic\n    is_private: false", "Supported keys are"} {
		if !strings.Contains(help, want) {
			t.Fatalf("spaces apply --help missing %q in output:\n%s", want, help)
		}
	}
}

type spacesApplyTestWriter struct{ calls int }

func (c *spacesApplyTestWriter) Post(_ context.Context, endpoint string, _ any) (json.RawMessage, int, error) {
	c.calls++
	if c.calls == 1 && endpoint == "/space_groups" {
		return json.RawMessage(`{"id":1999918}`), 201, nil
	}
	return nil, 500, fmt.Errorf("injected failure")
}

func (c *spacesApplyTestWriter) Put(context.Context, string, any) (json.RawMessage, int, error) {
	return nil, 500, fmt.Errorf("injected failure")
}

func TestSpacesApplyPartialResult(t *testing.T) {
	file, err := parseSpacesApply([]byte("space_groups: [{name: Spaces}]\nspaces: [{name: Start Here, group: Spaces}]"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := diffSpacesApply(file, nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	err = executeSpacesApply(context.Background(), &spacesApplyTestWriter{}, &result)
	if err == nil || !strings.Contains(err.Error(), "injected failure") {
		t.Fatalf("error=%v", err)
	}
	var out bytes.Buffer
	if err := printJSONFiltered(&out, result, &rootFlags{asJSON: true}); err != nil {
		t.Fatal(err)
	}
	var partial spacesApplyResult
	decoder := json.NewDecoder(&out)
	decoder.UseNumber()
	if err := decoder.Decode(&partial); err != nil {
		t.Fatal(err)
	}
	if partial.Applied || !partial.Plan[0].Applied || fmt.Sprint(partial.Plan[0].ID) != "1999918" || partial.Plan[1].Applied || partial.Plan[1].ID != nil {
		t.Fatalf("bad partial result: %s", &out)
	}
}

func TestSpacesApplyDiff(t *testing.T) {
	groups := []map[string]any{{"id": float64(42), "name": "Spaces", "slug": "spaces"}}
	spaces := []map[string]any{{"id": float64(1000969), "name": "Start Here", "slug": "start-here", "is_private": false, "space_type": "basic", "emoji": "👋", "space_group": map[string]any{"id": float64(42), "name": "Spaces"}}}
	for _, tc := range []struct {
		name, document, action, field, warning, err string
	}{
		{"create", "spaces: [{name: Welcome Home, group: Spaces, type: chat}]", "create", "slug", "", ""},
		{"one field", "spaces: [{name: Start Here, is_private: true}]", "update", "is_private", "", ""},
		{"create emoji", "spaces: [{name: Welcome Home, group: Spaces, emoji: '👋'}]", "create", "emoji", "", ""},
		{"update emoji", "spaces: [{name: Start Here, emoji: '🎉'}]", "update", "emoji", "", ""},
		{"unchanged emoji", "spaces: [{name: Start Here, emoji: '👋'}]", "noop", "", "", ""},
		{"slug only update", "spaces: [{slug: start-here, is_private: true}]", "update", "is_private", "", ""},
		{"new needs name", "spaces: [{slug: new-space, group: Spaces}]", "", "", "", "requires name"},
		{"identical", "spaces: [{name: Start Here, is_private: false, type: basic, group: spaces}]", "noop", "", "", ""},
		{"slug beats name", "spaces: [{name: Renamed, slug: start-here}]", "update", "name", "", ""},
		{"no fallback from slug", "spaces: [{name: Start Here, slug: other, group: Spaces}]", "create", "slug", "", ""},
		{"case insensitive name", "spaces: [{name: start here}]", "update", "name", "", ""},
		{"type warning", "spaces: [{name: Start Here, type: course}]", "noop", "", "type cannot change after creation", ""},
		{"missing group", "spaces: [{name: New Space, group: Missing}]", "", "", "", "unknown group reference"},
		{"new needs group", "spaces: [{name: New Space}]", "", "", "", "requires group"},
		{"existing group", "space_groups: [{name: Spaces, slug: spaces}]", "noop", "", "", ""},
		{"create group", "space_groups: [{name: Courses, slug: courses}]", "create", "name", "", ""},
		{"derive group slug", "space_groups: [{name: New Courses}]", "create", "slug", "", ""},
		{"duplicate space", "spaces: [{name: Start Here}, {name: Start Here}]", "", "", "", "duplicate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parseSpacesApply([]byte(tc.document))
			if err != nil {
				t.Fatal(err)
			}
			result, err := diffSpacesApply(file, groups, spaces, false)
			if tc.err != "" {
				if err == nil || !strings.Contains(err.Error(), tc.err) {
					t.Fatalf("error = %v; want %s", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Plan) != 1 || result.Plan[0].Action != tc.action {
				t.Fatalf("plan = %+v", result.Plan)
			}
			if result.Summary[tc.action] != 1 {
				t.Fatalf("summary = %v", result.Summary)
			}
			if tc.field != "" {
				if _, ok := result.Plan[0].Changes[tc.field]; !ok {
					t.Fatalf("missing %s in %+v", tc.field, result.Plan[0])
				}
			}
			if tc.name == "one field" && len(result.Plan[0].Changes) != 1 {
				t.Fatalf("unexpected changes: %v", result.Plan[0].Changes)
			}
			if tc.name == "create" && result.Plan[0].Changes["slug"].To != "welcome-home" {
				t.Fatal("slug not derived")
			}
			if tc.name == "slug beats name" && result.Plan[0].ID != float64(1000969) {
				t.Fatal("did not match slug")
			}
			if tc.warning != "" && (len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], tc.warning)) {
				t.Fatalf("warnings = %v", result.Warnings)
			}
		})
	}
}

func TestSpacesApplyParseValidation(t *testing.T) {
	for _, tc := range []struct{ document, want string }{
		{"spaces: [{name: Test, secret: true}]", "secret"},
		{"space_groups: [{name: Test, surprise: false}]", "surprise"},
		{"unknown: []", "unknown"},
		{"spaces: [{name: Test, is_private: 'false'}]", "boolean"},
		{"spaces: [{name: Test, group: 42}]", "group"},
		{"spaces: [{name: Test, type: invalid}]", "type"},
		{"spaces: [{name: Test, type: basic event}]", "type"},
		{"spaces: [{name: Test, 'name slug': surprise}]", "name slug"},
		{"spaces: [{}]", "requires name or slug"},
		{"spaces: [{name: ''}]", "nonempty"},
		{"spaces: []\n---\nspaces: []", "single YAML"},
		{"spaces: [{name: Test, name: Other}]", "already defined"},
	} {
		t.Run(tc.document, func(t *testing.T) {
			_, err := parseSpacesApply([]byte(tc.document))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v; want %s", err, tc.want)
			}
		})
	}
}

func TestSpacesApplyRerun(t *testing.T) {
	file, err := parseSpacesApply([]byte(`
space_groups:
  - name: Spaces
    slug: spaces
  - name: Courses
    slug: courses
spaces:
  - name: Start Here
    slug: start-here
    group: spaces
    type: basic
    is_private: true
  - name: Welcome Home
    group: Courses
    type: course
    emoji: "👋"
`))
	if err != nil {
		t.Fatal(err)
	}
	groups := []map[string]any{{"id": float64(1999918), "name": "Spaces", "slug": "spaces"}}
	spaces := []map[string]any{{"id": float64(1000969), "name": "Start Here", "slug": "start-here", "space_type": "basic", "space_group": map[string]any{"id": float64(1999918), "name": "Spaces"}, "is_private": false}}
	result, err := diffSpacesApply(file, groups, spaces, true)
	if err != nil {
		t.Fatal(err)
	}
	created := map[string]any{}
	for i, entry := range result.Plan {
		var row map[string]any
		rows := spaces
		if entry.Kind == "space_group" {
			rows = groups
		}
		if entry.Action == "create" {
			row = map[string]any{"id": float64(100 + i)}
			if entry.Kind == "space_group" {
				groups = append(groups, row)
				created[fmt.Sprintf("new-group:%d", i)] = row["id"]
			} else {
				spaces = append(spaces, row)
			}
		} else {
			for _, existing := range rows {
				if reflect.DeepEqual(existing["id"], entry.ID) {
					row = existing
				}
			}
		}
		if row == nil {
			t.Fatalf("no row for %+v", entry)
		}
		for field, change := range entry.Changes {
			value := change.To
			if ref, ok := value.(string); field == "space_group_id" && ok {
				value = created[ref]
				if value == nil {
					t.Fatalf("unresolved group %s", ref)
				}
			}
			if field == "space_group_id" {
				row["space_group"] = map[string]any{"id": value}
			} else {
				row[field] = value
			}
		}
	}
	second, err := diffSpacesApply(file, groups, spaces, true)
	if err != nil {
		t.Fatal(err)
	}
	if second.Summary["noop"] != 4 || second.Summary["create"] != 0 || second.Summary["update"] != 0 || len(*second.Unmanaged) != 0 {
		t.Fatalf("rerun = %+v", second)
	}
}

func TestSpacesApplyGroupResolutionAndPrune(t *testing.T) {
	groups := []map[string]any{{"id": float64(42), "name": "Spaces", "slug": "spaces"}, {"id": float64(43), "name": "Other", "slug": "other"}}
	file, err := parseSpacesApply([]byte("space_groups: [{name: Renamed, slug: spaces}]\nspaces: [{name: New Space, group: Renamed}]"))
	if err != nil {
		t.Fatal(err)
	}
	result, err := diffSpacesApply(file, groups, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if result.Plan[1].Changes["space_group_id"].To != float64(42) {
		t.Fatalf("group = %v", result.Plan[1].Changes)
	}
	if len(*result.Unmanaged) != 1 || (*result.Unmanaged)[0].Name != "Other" {
		t.Fatalf("unmanaged = %v", result.Unmanaged)
	}
	_, err = spacesApplyGroup("Spaces", append(groups, map[string]any{"id": float64(44), "name": "Spaces"}))
	if err == nil {
		t.Fatal("ambiguous group should fail")
	}
}

func TestSpacesApplyDryRunBeforeFileRead(t *testing.T) {
	cmd := newNovelSpacesApplyCmd(&rootFlags{dryRun: true, asJSON: true})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"/does/not/exist.yaml", "--apply"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"dry_run":true`) {
		t.Fatalf("output = %s", out.String())
	}
}

func TestSpacesApplySlug(t *testing.T) {
	for _, tc := range []struct{ name, slug string }{{"  Start HERE! ", "start-here"}, {"Welcome 👋 home", "welcome-home"}, {"👋", ""}, {"one---two", "one-two"}} {
		if got := spacesApplySlug(tc.name); got != tc.slug {
			t.Errorf("slug(%q) = %q", tc.name, got)
		}
	}
}

func TestSpacesApplyResponseID(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		want           any
	}{
		{"space envelope", `{"success":true,"message":"Space created","space":{"id":1000969}}`, json.Number("1000969")},
		{"group object", `{"id":1999918,"name":"Spaces"}`, json.Number("1999918")},
		{"nested takes precedence", `{"id":1999918,"space":{"id":1000969}}`, json.Number("1000969")},
		{"missing nested id", `{"id":1999918,"space":{}}`, json.Number("1999918")},
		{"null nested id", `{"id":1999918,"space":{"id":null}}`, json.Number("1999918")},
		{"null space", `{"id":1999918,"space":null}`, json.Number("1999918")},
		{"missing id", `{"success":true,"space":{}}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var response map[string]any
			dec := json.NewDecoder(strings.NewReader(tc.response))
			dec.UseNumber()
			if err := dec.Decode(&response); err != nil {
				t.Fatal(err)
			}
			if got := spacesApplyResponseID(response); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("id = %v; want %v", got, tc.want)
			}
		})
	}
}

func TestSpacesApplyIDFormatting(t *testing.T) {
	for _, id := range []any{json.Number("1001059"), 1001059, int64(1001059), float64(1001059)} {
		t.Run(fmt.Sprintf("%T", id), func(t *testing.T) {
			normalized := normalizeSpacesApplyValue(id)
			if got := "/spaces/" + fmt.Sprint(normalized); got != "/spaces/1001059" {
				t.Fatalf("path = %s", got)
			}
			body, err := json.Marshal(map[string]any{"space_group_id": normalized})
			if err != nil || string(body) != `{"space_group_id":1001059}` {
				t.Fatalf("body = %s, error = %v", body, err)
			}
		})
	}
}

func TestSpacesApplyNormalizedComparisons(t *testing.T) {
	for _, tc := range []struct {
		name          string
		live, desired any
		equal         bool
	}{
		{"JSON and YAML integer", json.Number("1999918"), 1999918, true},
		{"different IDs", json.Number("1999918"), 1999919, false},
		{"true", true, true, true},
		{"false", false, false, true},
		{"different booleans", true, false, false},
		{"boolean and integer", false, 0, false},
		{"fraction preserved", json.Number("1.5"), 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := reflect.DeepEqual(normalizeSpacesApplyValue(tc.live), normalizeSpacesApplyValue(tc.desired)); got != tc.equal {
				t.Fatalf("equal = %v; want %v", got, tc.equal)
			}
		})
	}
}

func TestSpacesApplyNumberIDsRerunIsNoop(t *testing.T) {
	file, err := parseSpacesApply([]byte("space_groups: [{name: Courses, hide_members_count: false}]\nspaces: [{name: Welcome, group: Courses, is_private: true}]"))
	if err != nil {
		t.Fatal(err)
	}
	groups := []map[string]any{{"id": json.Number("1999918"), "name": "Courses", "hide_members_count": false}}
	for _, groupID := range []any{json.Number("1999918"), 1999918} {
		spaces := []map[string]any{{"id": json.Number("1001059"), "name": "Welcome", "space_group": map[string]any{"id": groupID}, "is_private": true}}
		result, err := diffSpacesApply(file, groups, spaces, false)
		if err != nil {
			t.Fatal(err)
		}
		if result.Summary["noop"] != 2 || result.Summary["create"] != 0 || result.Summary["update"] != 0 {
			t.Fatalf("rerun = %+v", result)
		}
		for _, entry := range result.Plan {
			if len(entry.Changes) != 0 {
				t.Fatalf("unexpected changes = %+v", entry)
			}
		}
	}
}
