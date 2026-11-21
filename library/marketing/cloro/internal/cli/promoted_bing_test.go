// Copyright 2026 Ricardo Batista and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH(bing-endpoint-is-hand-wired-until-the-spec-lists-it): tests for the hand-written bing command.

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

// bingFixture is a Bing Search response in the shape of the API's BingResult
// type: organicResults and aioverview in Google's shapes.
const bingFixture = `{"success":true,"result":{
"organicResults":[
 {"position":1,"title":"7 Best Running Shoes For Flat Feet in 2026 - RunRepeat","link":"https://runrepeat.com/guides/best-flat-feet-running-shoes","displayedLink":"runrepeat.com","snippet":"Find out the top picks for flat-footed runners."},
 {"position":2,"title":"5 Best Running Shoes for Flat Feet in 2026","link":"https://flatfeetrunning.com/best-running-shoes-for-flat-feet","displayedLink":"flatfeetrunning.com","snippet":"Stability shoes for flat feet."}],
"aioverview":{
 "text":"Stability shoes with a firm midsole suit flat feet.",
 "sources":[
  {"label":"7 Best Running Shoes For Flat Feet in 2026 - RunRepeat","url":"https://runrepeat.com/guides/best-flat-feet-running-shoes","position":1},
  {"label":"5 Best Running Shoes for Flat Feet in 2026","url":"https://flatfeetrunning.com/best-running-shoes-for-flat-feet","position":2}]}}}`

// TestBingSendsBodyToEndpoint checks the path and the request body, and that
// the response reaches the JSON output.
func TestBingSendsBodyToEndpoint(t *testing.T) {
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
		_, _ = io.WriteString(w, bingFixture)
	}))
	defer srv.Close()
	t.Setenv("CLORO_BASE_URL", srv.URL)
	t.Setenv("CLORO_API_KEY", "test-key")

	cmd := RootCmd()
	cmd.SetArgs([]string{"bing", "--query", "best running shoes for flat feet",
		"--country", "US", "--include-aioverview", "--json"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("bing error = %v\nstderr: %s", err, errOut.String())
	}

	if gotPath != "POST /v1/monitor/bing" {
		t.Fatalf("request = %q, want POST /v1/monitor/bing", gotPath)
	}
	if gotBody["query"] != "best running shoes for flat feet" || gotBody["country"] != "US" {
		t.Errorf("body = %v", gotBody)
	}
	include, _ := gotBody["include"].(map[string]any)
	if include["aioverview"] != true {
		t.Errorf("include = %v, want aioverview=true", include)
	}
	if _, ok := include["html"]; ok {
		t.Errorf("include = %v, html must not be sent when the flag is not set", include)
	}
	for _, want := range []string{"organicResults", "aioverview", "flatfeetrunning.com"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output does not contain %q:\n%s", want, out.String())
		}
	}
}

func TestBingRequiresQuery(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("CLORO_API_KEY", "test-key")
	cmd := RootCmd()
	cmd.SetArgs([]string{"bing", "--country", "US", "--json"})
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "query") {
		t.Fatalf("err = %v, want a missing --query error", err)
	}
}

// TestCitedReadsBingAioverviewSources checks that cited runs the Bing AI
// Overview only when --engines names it, sends the prompt as the query, and
// reads the AI Overview sources.
func TestCitedReadsBingAioverviewSources(t *testing.T) {
	if strings.Contains(citedDefaultEngines, "bing-aioverview") {
		t.Fatalf("default engines = %q, the Bing AI Overview must be opt-in", citedDefaultEngines)
	}
	engines, err := parseCitedEngines("bing-aioverview")
	if err != nil {
		t.Fatal(err)
	}
	if len(engines) != 1 || engines[0].Path != "/v1/monitor/bing" {
		t.Fatalf("engines = %+v", engines)
	}
	body := engines[0].Body("best running shoes", "US")
	include, _ := body["include"].(map[string]any)
	if body["query"] != "best running shoes" || body["country"] != "US" || include["aioverview"] != true {
		t.Fatalf("body = %v", body)
	}
	sources, err := engines[0].Sources(json.RawMessage(bingFixture))
	if err != nil {
		t.Fatal(err)
	}
	res := matchCited("bing-aioverview", sources, "flatfeetrunning.com")
	if !res.Cited || res.Sources != 2 || len(res.Positions) != 1 || res.Positions[0] != 2 {
		t.Fatalf("res = %+v, want cited at position 2 of 2 sources", res)
	}
}
