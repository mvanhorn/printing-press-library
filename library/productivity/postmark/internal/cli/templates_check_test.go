// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

// TestNovelTemplatesCheckHelpWires smoke-tests that the templates check command
// resolves at runtime and renders useful --help output.
func TestNovelTemplatesCheckHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"templates", "check", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("templates check --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "templates check [flags]", "Do NOT use this command to see the rendered output", "Exit codes:"} {
		if !strings.Contains(help, want) {
			t.Fatalf("templates check --help missing %q in output:\n%s", want, help)
		}
	}
	check, _, err := RootCmd().Find([]string{"templates", "check"})
	if err != nil || check.Annotations["pp:typed-exit-codes"] != "0,3" || check.Annotations["mcp:read-only"] != "true" {
		t.Fatalf("templates check annotations = %v, %v", check.Annotations, err)
	}
	if check.LocalFlags().Lookup("server") != nil {
		t.Fatalf("templates check must use the global --server flag, not shadow it")
	}
}

func TestMissingTemplateModelKeys(t *testing.T) {
	cases := []struct {
		name      string
		suggested string
		model     string
		want      []string
	}{
		{"model lacks user", `{"user":{"name":"name_Value"},"company":"ACME"}`, `{"company":"ACME"}`, []string{"user.name"}},
		{"complete model", `{"user":{"name":"Jane"}}`, `{"user":{"name":"Jane"}}`, []string{}},
		{"scalar where object expected", `{"user":{"name":"n","email":"e"}}`, `{"user":"bob"}`, []string{"user.email", "user.name"}},
		{"array element keys", `{"items":[{"title":"t","price":"p"}]}`, `{"items":[{"title":"A"},{"title":"B","price":"1"}]}`, []string{"items[].price"}},
		{"empty array in model is fine", `{"items":[{"title":"t"}]}`, `{"items":[]}`, []string{}},
		{"missing array", `{"tags":["tags_Value"]}`, `{}`, []string{"tags[]"}},
		{"no suggested model", ``, `{"a":1}`, []string{}},
		{"null model", `{"a":"a_Value"}`, `null`, []string{"a"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := missingTemplateModelKeys(json.RawMessage(tc.suggested), json.RawMessage(tc.model))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("missing = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStaticTemplateProblems(t *testing.T) {
	layouts := map[string]bool{"base": true}
	exists := func(a string) bool { return layouts[a] }
	cases := []struct {
		name      string
		content   templateContent
		wantKinds []string
	}{
		{"ok", templateContent{Alias: "w", Subject: "Hi", TemplateType: templateTypeStandard, LayoutTemplate: "base"}, nil},
		{"no alias", templateContent{Subject: "Hi", TemplateType: templateTypeStandard}, []string{checkKindMissingAlias}},
		{"no subject", templateContent{Alias: "w", TemplateType: templateTypeStandard}, []string{checkKindMissingSubject}},
		{"dangling", templateContent{Alias: "w", Subject: "Hi", TemplateType: templateTypeStandard, LayoutTemplate: "gone"}, []string{checkKindDanglingLayout}},
		{"layout needs no subject", templateContent{Alias: "base", TemplateType: templateTypeLayout}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := newTemplateCheckRow(tc.content)
			staticTemplateProblems(&row, tc.content, exists)
			kinds := make([]string, 0)
			for _, p := range row.Problems {
				kinds = append(kinds, p.Kind)
			}
			if len(kinds) != len(tc.wantKinds) || (len(kinds) > 0 && !reflect.DeepEqual(kinds, tc.wantKinds)) {
				t.Fatalf("kinds = %v, want %v", kinds, tc.wantKinds)
			}
			if (len(kinds) > 0) != (row.Status == checkStatusFail) {
				t.Fatalf("status %q inconsistent with problems %v", row.Status, kinds)
			}
		})
	}
}

func TestTemplatesCheckDirReportsMissingModelKey(t *testing.T) {
	srv, requests := newFakePostmarkServer(t, templateRoutes(nil, map[string]any{"user": map[string]any{"name": "name_Value"}}))
	root := t.TempDir()
	welcome := templateContent{Name: "Welcome", Alias: "welcome", Subject: "Hi {{user.name}}", HtmlBody: "<p>{{user.name}} at {{company}}</p>", TemplateType: templateTypeStandard}
	if _, err := writeTemplateFiles(root, welcome, json.RawMessage(`{"company":"ACME"}`)); err != nil {
		t.Fatal(err)
	}
	good := templateContent{Name: "Good", Alias: "good", Subject: "Hi", TextBody: "{{user.name}}", TemplateType: templateTypeStandard}
	if _, err := writeTemplateFiles(root, good, json.RawMessage(`{"user":{"name":"Jane"}}`)); err != nil {
		t.Fatal(err)
	}
	out, stderr, err := runWithServerToken(t, srv.URL, "templates", "check", "--dir", root, "--json")
	if err == nil || ExitCode(err) != templatesCheckFailedExit {
		t.Fatalf("expected exit 3, got %v\n%s\n%s", err, out, stderr)
	}
	var view templateCheckView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("stdout must stay valid JSON on failure: %v\n%s", err, out)
	}
	rows := map[string]templateCheckRow{}
	for _, r := range view.Templates {
		rows[r.Alias] = r
	}
	w := rows["welcome"]
	if w.Status != checkStatusFail || !reflect.DeepEqual(w.MissingModelKeys, []string{"user.name"}) || w.TestModel != checkModelMeta {
		t.Fatalf("welcome row = %+v", w)
	}
	if rows["good"].Status != checkStatusPass {
		t.Fatalf("good row = %+v", rows["good"])
	}
	if view.Total != 2 || view.Failed != 1 || view.Passed != 1 {
		t.Fatalf("totals = %d/%d/%d", view.Total, view.Passed, view.Failed)
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("check is read-only but issued %v", got)
	}
}

func TestTemplatesCheckServerModeDanglingLayoutAndPass(t *testing.T) {
	templates := []templateDetail{
		{TemplateId: 1, Name: "Base", Alias: "base", HtmlBody: "{{{ @content }}}", TemplateType: templateTypeLayout},
		{TemplateId: 2, Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>Hi</p>", TemplateType: templateTypeStandard, LayoutTemplate: "base"},
	}
	srv, _ := newFakePostmarkServer(t, templateRoutes(templates, nil))
	out, _, err := runWithServerToken(t, srv.URL, "templates", "check", "--json")
	if err != nil {
		t.Fatalf("all-pass server check should exit 0: %v\n%s", err, out)
	}

	templates = append(templates, templateDetail{TemplateId: 3, Name: "Orphan", Alias: "orphan", Subject: "Hi", HtmlBody: "<p>x</p>", TemplateType: templateTypeStandard, LayoutTemplate: "gone"})
	srv2, requests := newFakePostmarkServer(t, templateRoutes(templates, nil))
	out, _, err = runWithServerToken(t, srv2.URL, "templates", "check", "--json")
	if ExitCode(err) != templatesCheckFailedExit {
		t.Fatalf("dangling layout should exit 3, got %v", err)
	}
	var view templateCheckView
	_ = json.Unmarshal([]byte(out), &view)
	var orphan templateCheckRow
	for _, r := range view.Templates {
		if r.Alias == "orphan" {
			orphan = r
		}
	}
	if orphan.Status != checkStatusFail || len(orphan.Problems) != 1 || orphan.Problems[0].Kind != checkKindDanglingLayout {
		t.Fatalf("orphan row = %+v", orphan)
	}
	for _, r := range requests() {
		if r.Path == "/templates/validate" && strings.Contains(r.Body, `"gone"`) {
			t.Fatalf("dangling layout must be validated without the missing layout: %s", r.Body)
		}
	}
}

func TestTemplatesCheckTreats422AsTemplateFailure(t *testing.T) {
	templates := []templateDetail{{TemplateId: 2, Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>Hi</p>", TemplateType: templateTypeStandard}}
	routes := templateRoutes(templates, nil)
	routes["POST /templates/validate"] = func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{"ErrorCode": 1122, "Message": "The template body could not be parsed."})
	}
	srv, _ := newFakePostmarkServer(t, routes)
	out, _, err := runWithServerToken(t, srv.URL, "templates", "check", "--json")
	if ExitCode(err) != templatesCheckFailedExit {
		t.Fatalf("422 should fail the template with exit 3, got %v", err)
	}
	var view templateCheckView
	_ = json.Unmarshal([]byte(out), &view)
	if len(view.Templates) != 1 || view.Templates[0].Problems[0].Kind != checkKindValidateFailure || len(view.FetchFailures) != 0 {
		t.Fatalf("view = %+v", view)
	}
}
