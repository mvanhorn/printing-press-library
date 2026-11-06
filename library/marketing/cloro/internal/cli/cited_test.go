// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/cloro/internal/cliutil/testenv"
)

// TestNovelCitedHelpWires smoke-tests that the cited command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelCitedHelpWires(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"cited", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cited --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "cited"} {
		if !strings.Contains(help, want) {
			t.Fatalf("cited --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestCitedHostMatches(t *testing.T) {
	cases := []struct {
		url  string
		want bool
	}{
		{"https://www.nike.com/t/air-max-90", true},
		{"https://nike.com/", true},
		{"https://shop.nike.com/x#:~:text=flat%20feet", true},
		{"https://notnike.com/", false},
		{"https://nike.com.evil.example/", false},
		{"", false},
		{"not a url", false},
	}
	for _, c := range cases {
		if got := citedHostMatches(c.url, "nike.com"); got != c.want {
			t.Errorf("citedHostMatches(%q) = %v, want %v", c.url, got, c.want)
		}
	}
}

func TestNormalizeCitedDomain(t *testing.T) {
	for in, want := range map[string]string{
		"nike.com":                  "nike.com",
		"https://www.Nike.com/path": "nike.com",
		"  WWW.nike.com ":           "nike.com",
		"":                          "",
	} {
		if got := normalizeCitedDomain(in); got != want {
			t.Errorf("normalizeCitedDomain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseCitedEngines(t *testing.T) {
	engines, err := parseCitedEngines("Gemini, aimode,gemini")
	if err != nil {
		t.Fatal(err)
	}
	if len(engines) != 2 || engines[0].Name != "gemini" || engines[1].Name != "aimode" {
		t.Fatalf("engines = %+v, want gemini then aimode", engines)
	}
	if _, err := parseCitedEngines("chatgpt,unknown"); err == nil {
		t.Fatal("unknown engine: want error")
	}
	if _, err := parseCitedEngines(" , "); err == nil {
		t.Fatal("empty list: want error")
	}
}

func TestMatchCitedUsesIndexWhenPositionMissing(t *testing.T) {
	res := matchCited("gemini", []citedSource{
		{URL: "https://example.org/a"},
		{URL: "https://www.nike.com/b"},
		{Position: 7, URL: "https://nike.com/c"},
	}, "nike.com")
	if !res.Cited || res.Sources != 3 {
		t.Fatalf("res = %+v", res)
	}
	if len(res.Positions) != 2 || res.Positions[0] != 2 || res.Positions[1] != 7 {
		t.Fatalf("positions = %v, want [2 7]", res.Positions)
	}
}

// TestCitedAgainstStubServer runs the command end to end against a stub API:
// two engines answer, one fails. The failed engine must be reported in
// fetch_failures and must not count as "not cited".
func TestCitedAgainstStubServer(t *testing.T) {
	testenv.Isolate(t)
	var mu sync.Mutex
	bodies := map[string]map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		bodies[r.URL.Path] = body
		mu.Unlock()
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("%s: Authorization = %q", r.URL.Path, got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/monitor/aimode":
			_, _ = io.WriteString(w, `{"success":true,"result":{"text":"...","sources":[
				{"position":1,"url":"https://www.runnersworld.com/gear/"},
				{"position":2,"url":"https://www.nike.com/a/flat-feet-shoes"}]}}`)
		case "/v1/monitor/google":
			_, _ = io.WriteString(w, `{"success":true,"result":{"organicResults":[],"aioverview":{"text":"...","sources":[
				{"position":1,"url":"https://www.brooksrunning.com/"}]}}}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"success":false,"error":{"code":"INVALID_REQUEST","message":"bad request"}}`)
		}
	}))
	defer srv.Close()
	t.Setenv("CLORO_BASE_URL", srv.URL)
	t.Setenv("CLORO_API_KEY", "test-key")

	cmd := RootCmd()
	cmd.SetArgs([]string{"cited", "https://www.nike.com", "--prompt", "best running shoes for flat feet",
		"--country", "us", "--engines", "aimode,aioverview,gemini", "--json"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("cited error = %v\nstderr: %s", err, errOut.String())
	}

	var view citedView
	if err := json.Unmarshal(out.Bytes(), &view); err != nil {
		t.Fatalf("output is not a cited view: %v\n%s", err, out.String())
	}
	if view.Domain != "nike.com" || view.Country != "US" {
		t.Fatalf("domain/country = %q/%q", view.Domain, view.Country)
	}
	if len(view.CitedBy) != 1 || view.CitedBy[0] != "aimode" {
		t.Fatalf("cited_by = %v, want [aimode]", view.CitedBy)
	}
	if len(view.Engines) != 2 {
		t.Fatalf("engines = %+v, want aimode and aioverview only", view.Engines)
	}
	for _, e := range view.Engines {
		switch e.Engine {
		case "aimode":
			if !e.Cited || len(e.Positions) != 1 || e.Positions[0] != 2 || e.Sources != 2 {
				t.Errorf("aimode = %+v", e)
			}
		case "aioverview":
			if e.Cited || e.Sources != 1 {
				t.Errorf("aioverview = %+v", e)
			}
		default:
			t.Errorf("unexpected engine row %+v", e)
		}
	}
	if len(view.FetchFailures) != 1 || view.FetchFailures[0].Engine != "gemini" {
		t.Fatalf("fetch_failures = %+v, want gemini", view.FetchFailures)
	}
	if !strings.Contains(errOut.String(), "1 of 3 engines failed") {
		t.Errorf("stderr missing partial-failure warning: %q", errOut.String())
	}

	mu.Lock()
	defer mu.Unlock()
	if got := bodies["/v1/monitor/aimode"]; got["prompt"] != "best running shoes for flat feet" || got["country"] != "US" {
		t.Errorf("aimode body = %v", got)
	}
	google := bodies["/v1/monitor/google"]
	include, _ := google["include"].(map[string]any)
	if google["query"] != "best running shoes for flat feet" || include["aioverview"] != true {
		t.Errorf("google body = %v, want query and include.aioverview=true", google)
	}
}

func TestCitedRequiresPrompt(t *testing.T) {
	testenv.Isolate(t)
	cmd := RootCmd()
	cmd.SetArgs([]string{"cited", "nike.com", "--json"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "--prompt is required") {
		t.Fatalf("err = %v, want --prompt is required", err)
	}
}
