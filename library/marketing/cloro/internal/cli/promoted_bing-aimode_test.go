// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(bing-aimode-endpoint): tests for the hand-written bing-aimode command.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/cloro/internal/cliutil/testenv"
)

// bingAimodeFixture is a live Bing AI Mode response from 2026-11-07,
// trimmed to two sources and two citation pills.
const bingAimodeFixture = `{"success":true,"result":{
"text":"The adidas Supernova Solution 3 is the best overall choice for flat feet thanks to its centered ride and stability features.",
"sources":[
 {"label":"7 Best Running Shoes For Flat Feet in 2026 - RunRepeat","url":"https://runrepeat.com/guides/best-flat-feet-running-shoes","description":"Find out the top picks for flat-footed runners based on testing, analysis, and reviews.","position":1},
 {"label":"5 Best Running Shoes for Flat Feet in 2026","url":"https://flatfeetrunning.com/best-running-shoes-for-flat-feet","position":2}],
"citationPills":[
 {"label":"7 Best Running Shoes For Flat Feet in 2026 - RunRepeat","citationPillId":1,"url":"https://runrepeat.com/guides/best-flat-feet-running-shoes","domain":"runrepeat.com","position":1},
 {"label":"5 Best Running Shoes for Flat Feet in 2026","citationPillId":2,"url":"https://flatfeetrunning.com/best-running-shoes-for-flat-feet","domain":"flatfeetrunning.com","position":2}],
"markdown":"The **adidas Supernova Solution 3** is the best overall choice for flat feet."}}`

// TestBingAimodeSendsBodyToEndpoint checks the path and the request body,
// and that the response reaches the JSON output.
func TestBingAimodeSendsBodyToEndpoint(t *testing.T) {
	testenv.Isolate(t)
	var gotPath string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.Method + " " + r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &gotBody)
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, bingAimodeFixture)
	}))
	defer srv.Close()
	t.Setenv("CLORO_BASE_URL", srv.URL)
	t.Setenv("CLORO_API_KEY", "test-key")

	cmd := RootCmd()
	cmd.SetArgs([]string{"bing-aimode", "--prompt", "best running shoes for flat feet",
		"--country", "US", "--state", "CA", "--include-markdown", "--json"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bing-aimode error = %v\nstderr: %s", err, errOut.String())
	}

	if gotPath != "POST /v1/monitor/bing/aimode" {
		t.Fatalf("request = %q, want POST /v1/monitor/bing/aimode", gotPath)
	}
	if gotBody["prompt"] != "best running shoes for flat feet" || gotBody["country"] != "US" || gotBody["state"] != "CA" {
		t.Errorf("body = %v", gotBody)
	}
	include, _ := gotBody["include"].(map[string]any)
	if include["markdown"] != true {
		t.Errorf("include = %v, want markdown=true", include)
	}
	if _, ok := include["html"]; ok {
		t.Errorf("include = %v, html must not be sent when the flag is not set", include)
	}
	for _, want := range []string{"citationPills", "flatfeetrunning.com", "Supernova Solution 3"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, out.String())
		}
	}
}

func TestBingAimodeRequiresPrompt(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("CLORO_API_KEY", "test-key")
	cmd := RootCmd()
	cmd.SetArgs([]string{"bing-aimode", "--country", "US", "--json"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "prompt") {
		t.Fatalf("err = %v, want a missing --prompt error", err)
	}
}

// TestCitedReadsBingAimodeSources checks that cited runs Bing AI Mode only
// when --engines names it, and reads its sources.
func TestCitedReadsBingAimodeSources(t *testing.T) {
	if strings.Contains(citedDefaultEngines, "bing-aimode") {
		t.Fatalf("default engines = %q, Bing AI Mode must be opt-in", citedDefaultEngines)
	}
	engines, err := parseCitedEngines("bing-aimode")
	if err != nil {
		t.Fatal(err)
	}
	if len(engines) != 1 || engines[0].Path != "/v1/monitor/bing/aimode" {
		t.Fatalf("engines = %+v", engines)
	}
	sources, err := engines[0].Sources(json.RawMessage(bingAimodeFixture))
	if err != nil {
		t.Fatal(err)
	}
	res := matchCited("bing-aimode", sources, "flatfeetrunning.com")
	if !res.Cited || res.Sources != 2 || len(res.Positions) != 1 || res.Positions[0] != 2 {
		t.Fatalf("res = %+v, want cited at position 2 of 2 sources", res)
	}
}
