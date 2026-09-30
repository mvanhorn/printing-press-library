// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func localTpl(alias, ttype, name, subject, html, layout string) localTemplate {
	return localTemplate{
		Meta:     templateMeta{Name: name, Alias: alias, Subject: subject, TemplateType: ttype, LayoutTemplate: layout},
		HtmlBody: html,
		Dir:      "/tpl/" + alias,
	}
}

func remoteTpl(alias, ttype, name, subject, html, layout string) templateDetail {
	return templateDetail{Name: name, Alias: alias, Subject: subject, HtmlBody: html, TemplateType: ttype, LayoutTemplate: layout}
}

func planActions(items []templatePushItem) map[string]string {
	out := map[string]string{}
	for _, it := range items {
		out[it.Alias] = it.Action
	}
	return out
}

func TestBuildTemplatePushPlan(t *testing.T) {
	remote := []templateDetail{
		remoteTpl("base", templateTypeLayout, "Base", "", "{{{ @content }}}", ""),
		remoteTpl("shared", templateTypeLayout, "Shared", "", "{{{ @content }}}", ""),
		remoteTpl("welcome", templateTypeStandard, "Welcome", "Hi", "<p>Hi</p>", "shared"),
		remoteTpl("old", templateTypeStandard, "Old", "Old", "<p>old</p>", ""),
		remoteTpl("kind", templateTypeLayout, "Kind", "", "{{{ @content }}}", ""),
		{TemplateId: 9, Name: "No alias", TemplateType: templateTypeStandard, LayoutTemplate: "orphan-layout"},
		remoteTpl("orphan-layout", templateTypeLayout, "Orphan", "", "{{{ @content }}}", ""),
	}
	local := []localTemplate{
		localTpl("base", templateTypeLayout, "Base", "", "changed {{{ @content }}}", ""),
		localTpl("welcome", templateTypeStandard, "Welcome", "Hi", "<p>Hi</p>", "shared"),
		localTpl("newone", templateTypeStandard, "New", "Subject", "<p>n</p>", "base"),
		localTpl("kind", templateTypeStandard, "Kind", "S", "<p>k</p>", ""),
		localTpl("dangling", templateTypeStandard, "D", "S", "<p>d</p>", "missing-layout"),
		localTpl("empty", templateTypeStandard, "E", "S", "", ""),
		localTpl("nosubject", templateTypeStandard, "N", "", "<p>n</p>", ""),
	}
	sortLocalTemplates(local)

	t.Run("without prune", func(t *testing.T) {
		items := buildTemplatePushPlan(local, remote, false)
		want := map[string]string{
			"base": pushActionUpdate, "welcome": pushActionUnchanged, "newone": pushActionCreate,
			"kind": pushActionConflict, "dangling": pushActionBlocked, "empty": pushActionConflict, "nosubject": pushActionConflict,
		}
		if got := planActions(items); !reflect.DeepEqual(got, want) {
			t.Fatalf("actions = %v, want %v", got, want)
		}
		if items[0].Alias != "base" {
			t.Fatalf("layouts must come first, got %s", items[0].Alias)
		}
	})
	t.Run("with prune", func(t *testing.T) {
		items := buildTemplatePushPlan(local, remote, true)
		got := planActions(items)
		if got["old"] != pushActionDelete {
			t.Errorf("old = %q, want delete", got["old"])
		}
		if got["shared"] != pushActionBlocked {
			t.Errorf("shared layout used by welcome = %q, want blocked", got["shared"])
		}
		if got["orphan-layout"] != pushActionBlocked {
			t.Errorf("layout used by an unaliased server template = %q, want blocked", got["orphan-layout"])
		}
		if _, ok := got[""]; ok {
			t.Errorf("unaliased server templates must never be planned")
		}
		last := items[len(items)-1]
		if last.TemplateType != templateTypeLayout {
			t.Errorf("layout deletes must come after standard deletes; last item = %+v", last)
		}
	})
}

func TestValidateLocalAliases(t *testing.T) {
	local := []localTemplate{
		localTpl("a", templateTypeStandard, "A", "S", "x", ""),
		localTpl("", templateTypeStandard, "No alias", "S", "x", ""),
		{Meta: templateMeta{Alias: "dup", TemplateType: templateTypeStandard}, Dir: "/one/dup"},
		{Meta: templateMeta{Alias: "dup", TemplateType: templateTypeStandard}, Dir: "/two/dup"},
	}
	valid, problems := validateLocalAliases(local)
	if len(valid) != 1 || valid[0].Meta.Alias != "a" {
		t.Fatalf("valid = %+v", valid)
	}
	if len(problems) != 3 {
		t.Fatalf("problems = %+v", problems)
	}
}

func TestPullDecision(t *testing.T) {
	remote := templateContent{Name: "W", Alias: "w", Subject: "Hi", HtmlBody: "<p>x</p>", TemplateType: templateTypeStandard}
	same := localTemplate{Meta: templateMeta{Name: "W", Alias: "w", Subject: "Hi", TemplateType: templateTypeStandard, TestRenderModel: json.RawMessage(`{"k":1}`)}, HtmlBody: "<p>x</p>"}
	edited := same
	edited.HtmlBody = "<p>edited</p>"
	cases := []struct {
		name      string
		local     *localTemplate
		localErr  string
		overwrite bool
		wantKeep  bool
		wantSkip  string
	}{
		{"new", nil, "", false, false, ""},
		{"same keeps model", &same, "", false, true, ""},
		{"same overwrite regenerates", &same, "", true, false, ""},
		{"edited skipped", &edited, "", false, false, "local copy differs"},
		{"edited overwrite", &edited, "", true, false, ""},
		{"bad meta skipped", nil, "invalid meta.json", false, false, "could not be read"},
		{"bad meta overwrite", nil, "invalid meta.json", true, false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keep, skip := pullDecision(remote, tc.local, tc.localErr, tc.overwrite)
			if (keep != nil) != tc.wantKeep {
				t.Errorf("keep = %s, want keep %v", keep, tc.wantKeep)
			}
			if tc.wantSkip == "" && skip != "" || tc.wantSkip != "" && !strings.Contains(skip, tc.wantSkip) {
				t.Errorf("skip = %q, want %q", skip, tc.wantSkip)
			}
		})
	}
}

func TestParseModelFlagAndTypeFilter(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "model.json")
	if err := os.WriteFile(p, []byte(`{"user":{"name":"Jane"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	if m, err := parseModelFlag("@model.json"); err != nil || !strings.Contains(string(m), "Jane") {
		t.Errorf("@file = %s, %v", m, err)
	}
	for _, bad := range []string{"@" + p, "@../model.json", "@.."} {
		if _, err := parseModelFlag(bad); err == nil {
			t.Errorf("parseModelFlag(%q) should reject paths outside the working directory", bad)
		}
	}
	if m, err := parseModelFlag(`{"a":1}`); err != nil || string(m) != `{"a":1}` {
		t.Errorf("inline = %s, %v", m, err)
	}
	if _, err := parseModelFlag(`[1,2]`); err == nil {
		t.Errorf("array model should be rejected")
	}
	if m, err := parseModelFlag(""); err != nil || m != nil {
		t.Errorf("empty = %s, %v", m, err)
	}
	for in, want := range map[string]string{"": "All", "ALL": "All", "standard": "Standard", "Layout": "Layout"} {
		if got, err := parseTemplateTypeFilter(in); err != nil || got != want {
			t.Errorf("parseTemplateTypeFilter(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := parseTemplateTypeFilter("bogus"); err == nil {
		t.Errorf("bogus type should error")
	}
}

// templateStore is a fake Postmark template backend for command tests.
func templateRoutes(templates []templateDetail, suggested map[string]any) map[string]fakeRoute {
	routes := map[string]fakeRoute{
		"GET /templates": func(w http.ResponseWriter, r *http.Request, _ []byte) {
			q := r.URL.Query()
			count, _ := strconv.Atoi(q.Get("Count"))
			offset, _ := strconv.Atoi(q.Get("Offset"))
			filtered := make([]templateSummary, 0)
			for _, d := range templates {
				tt := q.Get("TemplateType")
				if tt != "" && tt != "All" && tt != d.TemplateType {
					continue
				}
				filtered = append(filtered, templateSummary{TemplateId: d.TemplateId, Name: d.Name, Alias: d.Alias, TemplateType: d.TemplateType, LayoutTemplate: d.LayoutTemplate})
			}
			end := offset + count
			if end > len(filtered) {
				end = len(filtered)
			}
			page := filtered[min(offset, len(filtered)):end]
			writeJSON(w, http.StatusOK, map[string]any{"TotalCount": len(filtered), "Templates": page})
		},
		"POST /templates/validate": func(w http.ResponseWriter, _ *http.Request, body []byte) {
			var req map[string]any
			_ = json.Unmarshal(body, &req)
			model := map[string]any{}
			for k, v := range suggested {
				model[k] = v
			}
			if m, ok := req["TestRenderModel"].(map[string]any); ok {
				for k, v := range m {
					model[k] = v
				}
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"AllContentIsValid":      true,
				"HtmlBody":               map[string]any{"ContentIsValid": true, "ValidationErrors": []any{}, "RenderedContent": "rendered html"},
				"TextBody":               nil,
				"Subject":                map[string]any{"ContentIsValid": true, "ValidationErrors": []any{}, "RenderedContent": "rendered subject"},
				"SuggestedTemplateModel": model,
			})
		},
	}
	for _, d := range templates {
		if d.Alias == "" {
			continue
		}
		routes["GET /templates/"+d.Alias] = jsonRoute(d)
	}
	return routes
}

func TestTemplatesPullPagesPastFirstPageAndReportsAliasless(t *testing.T) {
	templates := make([]templateDetail, 0)
	for i := 0; i < 101; i++ {
		alias := fmt.Sprintf("t%03d", i)
		templates = append(templates, templateDetail{TemplateId: int64(i + 1), Name: "T " + alias, Alias: alias, Subject: "Hi {{name}}", HtmlBody: "<p>{{name}}</p>", TemplateType: templateTypeStandard})
	}
	templates = append(templates, templateDetail{TemplateId: 500, Name: "Unnamed draft", TemplateType: templateTypeStandard})
	templates = append(templates, templateDetail{TemplateId: 501, Name: "Base", Alias: "base", HtmlBody: "{{{ @content }}}", TemplateType: templateTypeLayout})
	srv, requests := newFakePostmarkServer(t, templateRoutes(templates, map[string]any{"name": "name_Value"}))
	root := filepath.Join(t.TempDir(), "tpl")
	out, stderr, err := runWithServerToken(t, srv.URL, "templates", "pull", root, "--json")
	if err != nil {
		t.Fatalf("pull: %v\n%s\n%s", err, out, stderr)
	}
	var view templatePullView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatalf("parse: %v\n%s", err, out)
	}
	if view.ServerTotal != 103 || view.Listed != 103 || len(view.Pulled) != 102 {
		t.Fatalf("total=%d listed=%d pulled=%d, want 103/103/102", view.ServerTotal, view.Listed, len(view.Pulled))
	}
	if len(view.Skipped) != 1 || view.Skipped[0].TemplateID != 500 || !strings.Contains(view.Skipped[0].Reason, "missing alias") {
		t.Fatalf("skipped = %+v", view.Skipped)
	}
	listCalls := 0
	for _, r := range requests() {
		if r.key() == "GET /templates" {
			listCalls++
		}
	}
	if listCalls != 2 {
		t.Fatalf("expected 2 list pages for 103 templates, got %d", listCalls)
	}
	meta, err := os.ReadFile(filepath.Join(root, "t100", "meta.json"))
	if err != nil || !strings.Contains(string(meta), `"name": "name_Value"`) {
		t.Fatalf("t100 meta.json = %s, %v", meta, err)
	}
	if _, err := os.Stat(filepath.Join(root, "_layouts", "base", "content.html")); err != nil {
		t.Fatalf("layout not written under _layouts: %v", err)
	}

	// A local edit is preserved on re-pull without --overwrite.
	edited := filepath.Join(root, "t000", "content.html")
	if err := os.WriteFile(edited, []byte("<p>local edit</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	out, _, err = runWithServerToken(t, srv.URL, "templates", "pull", root, "--alias", "t000", "--json")
	if err != nil {
		t.Fatalf("re-pull: %v", err)
	}
	view = templatePullView{}
	_ = json.Unmarshal([]byte(out), &view)
	if len(view.Skipped) != 1 || !strings.Contains(view.Skipped[0].Reason, "--overwrite") {
		t.Fatalf("expected the edited template to be skipped, got %+v", view)
	}
	if b, _ := os.ReadFile(edited); string(b) != "<p>local edit</p>" {
		t.Fatalf("local edit clobbered: %s", b)
	}
	out, _, err = runWithServerToken(t, srv.URL, "templates", "pull", root, "--alias", "t000", "--overwrite", "--json")
	if err != nil {
		t.Fatalf("overwrite pull: %v", err)
	}
	view = templatePullView{}
	_ = json.Unmarshal([]byte(out), &view)
	if len(view.Pulled) != 1 || view.Pulled[0].Status != pullStatusUpdated {
		t.Fatalf("overwrite result = %+v", view)
	}
}

func pushFixture() []templateDetail {
	return []templateDetail{
		{TemplateId: 1, Name: "Base", Alias: "base", HtmlBody: "<html>{{{ @content }}}</html>", TemplateType: templateTypeLayout},
		{TemplateId: 2, Name: "Shared", Alias: "shared", HtmlBody: "<div>{{{ @content }}}</div>", TemplateType: templateTypeLayout},
		{TemplateId: 3, Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>Hi</p>", TemplateType: templateTypeStandard, LayoutTemplate: "shared"},
		{TemplateId: 4, Name: "Old", Alias: "old", Subject: "Old", HtmlBody: "<p>old</p>", TemplateType: templateTypeStandard},
	}
}

func pushRoutes(templates []templateDetail) map[string]fakeRoute {
	routes := templateRoutes(templates, nil)
	ok := func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeJSON(w, http.StatusOK, map[string]any{"ErrorCode": 0, "Message": "OK"})
	}
	routes["POST /templates"] = ok
	for _, d := range templates {
		routes["PUT /templates/"+d.Alias] = ok
		routes["DELETE /templates/"+d.Alias] = ok
	}
	return routes
}

func writeLocalFixture(t *testing.T, root string, templates []templateDetail) {
	t.Helper()
	for _, d := range templates {
		if _, err := writeTemplateFiles(root, d.content(), nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTemplatesPushPlanOnlyAndSubjectEdit(t *testing.T) {
	remote := pushFixture()
	srv, requests := newFakePostmarkServer(t, pushRoutes(remote))
	root := t.TempDir()
	writeLocalFixture(t, root, remote)

	out, stderr, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--json")
	if err != nil {
		t.Fatalf("push: %v\n%s", err, stderr)
	}
	var view templatePushView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.Mode != "plan" || view.Summary[pushActionUnchanged] != 4 || view.Writes != 0 {
		t.Fatalf("identical dir should be all unchanged: %+v", view.Summary)
	}

	welcome := remote[2]
	welcome.Subject = "Welcome aboard"
	if _, err := writeTemplateFiles(root, welcome.content(), nil); err != nil {
		t.Fatal(err)
	}
	out, _, err = runWithServerToken(t, srv.URL, "templates", "push", root, "--json")
	if err != nil {
		t.Fatal(err)
	}
	view = templatePushView{}
	_ = json.Unmarshal([]byte(out), &view)
	updates := make([]templatePushItem, 0)
	for _, it := range view.Items {
		if it.Action == pushActionUpdate {
			updates = append(updates, it)
		}
	}
	if len(updates) != 1 || updates[0].Alias != "welcome" || !reflect.DeepEqual(updates[0].Changes, []string{"subject"}) {
		t.Fatalf("expected exactly one subject update, got %+v", updates)
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("plan mode issued writes: %v", got)
	}
}

func TestTemplatesPushYesIssuesExactWrites(t *testing.T) {
	remote := pushFixture()
	srv, requests := newFakePostmarkServer(t, pushRoutes(remote))
	root := t.TempDir()
	local := []templateDetail{
		{Name: "Base", Alias: "base", HtmlBody: "<html class=\"v2\">{{{ @content }}}</html>", TemplateType: templateTypeLayout},
		{Name: "Welcome", Alias: "welcome", Subject: "Welcome aboard", HtmlBody: "<p>Hi</p>", TemplateType: templateTypeStandard, LayoutTemplate: "shared"},
		{Name: "New one", Alias: "newone", Subject: "Fresh", TextBody: "fresh", TemplateType: templateTypeStandard, LayoutTemplate: "base"},
	}
	writeLocalFixture(t, root, local)

	// Without --yes: nothing is written even with --prune.
	if _, _, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--prune", "--json"); err != nil {
		t.Fatal(err)
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("plan mode issued writes: %v", got)
	}

	out, stderr, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--prune", "--yes", "--json")
	if err != nil {
		t.Fatalf("push --yes: %v\n%s\n%s", err, out, stderr)
	}
	want := []string{"PUT /templates/base", "POST /templates", "PUT /templates/welcome", "DELETE /templates/old"}
	if got := mutatingRequests(requests()); !reflect.DeepEqual(got, want) {
		t.Fatalf("writes = %v, want %v", got, want)
	}
	var view templatePushView
	_ = json.Unmarshal([]byte(out), &view)
	if view.Mode != "apply" || view.Writes != 4 || view.Failed != 0 {
		t.Fatalf("apply view = %+v", view)
	}
	for _, it := range view.Items {
		if it.Alias == "shared" && it.Action != pushActionBlocked {
			t.Fatalf("layout still used by welcome must be blocked, got %+v", it)
		}
	}
	for _, r := range requests() {
		switch r.key() {
		case "POST /templates":
			var body map[string]any
			_ = json.Unmarshal([]byte(r.Body), &body)
			if body["Alias"] != "newone" || body["LayoutTemplate"] != "base" || body["TemplateType"] != templateTypeStandard || body["Subject"] != "Fresh" {
				t.Errorf("create body = %v", body)
			}
		case "PUT /templates/welcome":
			var body map[string]any
			_ = json.Unmarshal([]byte(r.Body), &body)
			if body["Subject"] != "Welcome aboard" || body["LayoutTemplate"] != "shared" {
				t.Errorf("update body = %v", body)
			}
		case "PUT /templates/base":
			var body map[string]any
			_ = json.Unmarshal([]byte(r.Body), &body)
			if _, hasSubject := body["Subject"]; hasSubject {
				t.Errorf("layout update must not send Subject: %v", body)
			}
		}
	}
}

func TestTemplatesPushRefusesInvalidLocalWithYes(t *testing.T) {
	remote := pushFixture()
	srv, requests := newFakePostmarkServer(t, pushRoutes(remote))
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "broken"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken", "meta.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, err := runWithServerToken(t, srv.URL, "templates", "push", root, "--yes", "--json")
	if err == nil || ExitCode(err) != 2 {
		t.Fatalf("expected usage error, got %v", err)
	}
	if got := mutatingRequests(requests()); len(got) != 0 {
		t.Fatalf("invalid local dir still pushed: %v", got)
	}
}

func TestTemplatesRenderModelPrecedence(t *testing.T) {
	remote := []templateDetail{{TemplateId: 1, Name: "Welcome", Alias: "welcome", Subject: "Hi {{user.name}}", HtmlBody: "<p>{{user.name}}</p>", TemplateType: templateTypeStandard}}
	srv, requests := newFakePostmarkServer(t, templateRoutes(remote, map[string]any{"user": map[string]any{"name": "name_Value"}}))
	root := t.TempDir()
	if _, err := writeTemplateFiles(root, remote[0].content(), json.RawMessage(`{"company":"ACME"}`)); err != nil {
		t.Fatal(err)
	}
	lastValidateModel := func() string {
		reqs := requests()
		for i := len(reqs) - 1; i >= 0; i-- {
			if reqs[i].Path == "/templates/validate" {
				var body map[string]json.RawMessage
				_ = json.Unmarshal([]byte(reqs[i].Body), &body)
				return string(body["TestRenderModel"])
			}
		}
		return ""
	}
	cases := []struct {
		args       []string
		wantSource string
		wantModel  string
		wantMiss   []string
	}{
		{[]string{"templates", "render", "welcome", "--json"}, renderModelSuggested, "", []string{}},
		{[]string{"templates", "render", "welcome", "--dir", root, "--json"}, renderModelMeta, `{"company":"ACME"}`, []string{"user.name"}},
		{[]string{"templates", "render", "welcome", "--dir", root, "--model", `{"user":{"name":"Jane"}}`, "--json"}, renderModelFlag, `{"user":{"name":"Jane"}}`, []string{}},
	}
	for _, tc := range cases {
		out, stderr, err := runWithServerToken(t, srv.URL, tc.args...)
		if err != nil {
			t.Fatalf("%v: %v\n%s", tc.args, err, stderr)
		}
		var view templateRenderView
		if err := json.Unmarshal([]byte(out), &view); err != nil {
			t.Fatal(err)
		}
		if view.ModelSource != tc.wantSource || lastValidateModel() != tc.wantModel {
			t.Errorf("%v: source %q model sent %q; want %q %q", tc.args, view.ModelSource, lastValidateModel(), tc.wantSource, tc.wantModel)
		}
		if !reflect.DeepEqual(view.MissingModelKeys, tc.wantMiss) {
			t.Errorf("%v: missing = %v, want %v", tc.args, view.MissingModelKeys, tc.wantMiss)
		}
		if view.Subject == nil || *view.Subject != "rendered subject" {
			t.Errorf("%v: rendered subject missing: %+v", tc.args, view.Subject)
		}
	}
	if _, _, err := runWithServerToken(t, srv.URL, "templates", "render", "nope", "--dir", root, "--json"); ExitCode(err) != 3 {
		t.Errorf("unknown alias in --dir should exit 3, got %v", err)
	}
}
