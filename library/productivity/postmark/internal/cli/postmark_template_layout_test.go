// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/postmark/internal/cliutil/testenv"
)

// ---------------------------------------------------------------------------
// Shared httptest harness for command tests that talk to a fake Postmark.
// ---------------------------------------------------------------------------

type fakeRequest struct {
	Method string
	Path   string
	Query  string
	Body   string
}

func (r fakeRequest) key() string { return r.Method + " " + r.Path }

type fakeRoute func(w http.ResponseWriter, r *http.Request, body []byte)

// newFakePostmarkServer serves routes keyed by "METHOD /path" and records every
// request. Unknown routes fail the test.
func newFakePostmarkServer(t *testing.T, routes map[string]fakeRoute) (*httptest.Server, func() []fakeRequest) {
	t.Helper()
	var mu sync.Mutex
	var reqs []fakeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		reqs = append(reqs, fakeRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.RawQuery, Body: string(body)})
		mu.Unlock()
		route, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s %s?%s", r.Method, r.URL.Path, r.URL.RawQuery)
			w.WriteHeader(http.StatusTeapot)
			return
		}
		route(w, r, body)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []fakeRequest {
		mu.Lock()
		defer mu.Unlock()
		return append([]fakeRequest(nil), reqs...)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func jsonRoute(v any) fakeRoute {
	return func(w http.ResponseWriter, _ *http.Request, _ []byte) { writeJSON(w, http.StatusOK, v) }
}

// runWithServerToken runs the CLI against baseURL with a server token, no
// harness env, and no rate limiting.
func runWithServerToken(t *testing.T, baseURL string, args ...string) (string, string, error) {
	t.Helper()
	testenv.Isolate(t)
	for k, v := range map[string]string{
		"POSTMARK_BASE_URL":                baseURL,
		"POSTMARK_SERVER_TOKEN":            "server-test-token",
		"POSTMARK_ACCOUNT_TOKEN":           "",
		"POSTMARK_SERVER":                  "",
		"PRINTING_PRESS_VERIFY":            "",
		"PRINTING_PRESS_VERIFY_LIVE_HTTP":  "",
		"PRINTING_PRESS_DOGFOOD":           "",
		"POSTMARK_NO_LEARN":                "1",
		"PRINTING_PRESS_CLIENT_PROFILE":    "",
		"POSTMARK_USER_AGENT":              "",
		"PRINTING_PRESS_VERIFY_MOCK_STATE": "",
	} {
		t.Setenv(k, v)
	}
	cmd := RootCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(append(append([]string{}, args...), "--rate-limit", "0", "--no-learn"))
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func mutatingRequests(reqs []fakeRequest) []string {
	out := make([]string, 0)
	for _, r := range reqs {
		if r.Method == http.MethodGet || r.Path == "/templates/validate" {
			continue
		}
		out = append(out, r.key())
	}
	return out
}

// ---------------------------------------------------------------------------
// Layout helper tests.
// ---------------------------------------------------------------------------

func TestTemplateContentDiff(t *testing.T) {
	base := templateContent{Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>x</p>", TextBody: "x", TemplateType: templateTypeStandard, LayoutTemplate: "base"}
	layout := templateContent{Name: "Base", Alias: "base", HtmlBody: "{{{ @content }}}", TemplateType: templateTypeLayout}
	cases := []struct {
		name   string
		remote templateContent
		local  func(templateContent) templateContent
		want   []string
	}{
		{"identical", base, func(c templateContent) templateContent { return c }, []string{}},
		{"subject", base, func(c templateContent) templateContent { c.Subject = "Hello"; return c }, []string{"subject"}},
		{"html and layout", base, func(c templateContent) templateContent { c.HtmlBody = "<p>y</p>"; c.LayoutTemplate = ""; return c }, []string{"html", "layout"}},
		{"name and text", base, func(c templateContent) templateContent { c.Name = "W"; c.TextBody = ""; return c }, []string{"name", "text"}},
		{"layout ignores subject", layout, func(c templateContent) templateContent { c.Subject = "ignored"; return c }, []string{}},
		{"layout ignores layout field", layout, func(c templateContent) templateContent { c.LayoutTemplate = "other"; return c }, []string{}},
		{"layout html", layout, func(c templateContent) templateContent { c.HtmlBody = "changed {{{ @content }}}"; return c }, []string{"html"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := templateContentDiff(tc.remote, tc.local(tc.remote))
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("diff = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSafeTemplateAlias(t *testing.T) {
	cases := []struct {
		alias, ttype string
		want         bool
	}{
		{"password-reset", templateTypeStandard, true},
		{"welcome_v2.1", templateTypeStandard, true},
		{"", templateTypeStandard, false},
		{"..", templateTypeStandard, false},
		{"../escape", templateTypeStandard, false},
		{"a/b", templateTypeStandard, false},
		{`a\b`, templateTypeStandard, false},
		{"_layouts", templateTypeStandard, false},
		{"_layouts", templateTypeLayout, true},
	}
	for _, tc := range cases {
		if got := safeTemplateAlias(tc.alias, tc.ttype); got != tc.want {
			t.Errorf("safeTemplateAlias(%q, %s) = %v, want %v", tc.alias, tc.ttype, got, tc.want)
		}
	}
}

func TestMarshalTemplateMetaMatchesOfficialShape(t *testing.T) {
	std := templateContent{Name: "Welcome <b>", Alias: "welcome", Subject: "Hi {{name}}", TemplateType: templateTypeStandard, LayoutTemplate: "base"}
	got, err := marshalTemplateMeta(metaForContent(std, json.RawMessage(`{"name":"name_Value"}`)))
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"Name\": \"Welcome <b>\",\n  \"Alias\": \"welcome\",\n  \"Subject\": \"Hi {{name}}\",\n  \"TemplateType\": \"Standard\",\n  \"LayoutTemplate\": \"base\",\n  \"TestRenderModel\": {\n    \"name\": \"name_Value\"\n  }\n}"
	if string(got) != want {
		t.Fatalf("meta.json =\n%s\nwant\n%s", got, want)
	}
	layout := templateContent{Name: "Base", Alias: "base", TemplateType: templateTypeLayout, LayoutTemplate: "ignored", Subject: ""}
	got, err = marshalTemplateMeta(metaForContent(layout, nil))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "Subject") || strings.Contains(string(got), "LayoutTemplate") || strings.Contains(string(got), "TestRenderModel") {
		t.Fatalf("layout meta.json carries Standard-only fields: %s", got)
	}
}

func TestWriteAndReadTemplateRoundTrip(t *testing.T) {
	root := t.TempDir()
	std := templateContent{Name: "Welcome", Alias: "welcome", Subject: "Hi", HtmlBody: "<p>Hi</p>", TextBody: "Hi\r\n", TemplateType: templateTypeStandard, LayoutTemplate: "base"}
	layout := templateContent{Name: "Base", Alias: "base", HtmlBody: "{{{ @content }}}", TemplateType: templateTypeLayout}
	res, err := writeTemplateFiles(root, std, json.RawMessage(`{"a":1}`))
	if err != nil || !res.Changed {
		t.Fatalf("first write = %+v, %v", res, err)
	}
	if _, err := writeTemplateFiles(root, layout, nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"welcome/content.html", "welcome/content.txt", "welcome/meta.json", "_layouts/base/content.html", "_layouts/base/meta.json"} {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "_layouts/base/content.txt")); !os.IsNotExist(err) {
		t.Errorf("layout without text body should not write content.txt")
	}
	res, err = writeTemplateFiles(root, std, json.RawMessage(`{"a":1}`))
	if err != nil || res.Changed {
		t.Fatalf("identical rewrite should be a no-op, got %+v, %v", res, err)
	}
	local, problems, err := readLocalTemplates(root)
	if err != nil || len(problems) != 0 {
		t.Fatalf("read = %v, problems %v", err, problems)
	}
	if len(local) != 2 || local[0].Meta.Alias != "base" || local[1].Meta.Alias != "welcome" {
		t.Fatalf("expected layouts first then alias order, got %+v", local)
	}
	if diff := templateContentDiff(std, local[1].content()); len(diff) != 0 {
		t.Fatalf("round trip changed fields: %v", diff)
	}
	if !local[1].hasTestModel() {
		t.Fatalf("TestRenderModel lost on round trip")
	}
	// A body that disappears on the server removes the stale local file.
	std.TextBody = ""
	if _, err := writeTemplateFiles(root, std, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "welcome/content.txt")); !os.IsNotExist(err) {
		t.Fatalf("stale content.txt should be removed")
	}
}

func TestReadLocalTemplatesReportsProblemsAndInfersLayoutType(t *testing.T) {
	root := t.TempDir()
	mustWrite := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mustWrite("broken/meta.json", "{not json")
	mustWrite("_layouts/base/meta.json", `{"Name":"Base","Alias":"base"}`)
	mustWrite("_layouts/base/content.html", "{{{ @content }}}")
	mustWrite(".git/meta.json", `{"Name":"ignored","Alias":"ignored"}`)
	mustWrite("welcome/meta.json", `{"Name":"W","Alias":"welcome","TemplateType":"standard","TestRenderModel":null}`)
	local, problems, err := readLocalTemplates(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0].Reason, "invalid meta.json") {
		t.Fatalf("problems = %+v", problems)
	}
	if len(local) != 2 {
		t.Fatalf("expected base and welcome (hidden dirs skipped), got %+v", local)
	}
	if local[0].Meta.TemplateType != templateTypeLayout {
		t.Errorf("layout type not inferred from _layouts: %q", local[0].Meta.TemplateType)
	}
	if local[1].Meta.TemplateType != templateTypeStandard || local[1].hasTestModel() {
		t.Errorf("welcome = %+v", local[1].Meta)
	}
	if _, _, err := readLocalTemplates(filepath.Join(root, "missing")); err == nil {
		t.Errorf("missing root should error")
	}
}
