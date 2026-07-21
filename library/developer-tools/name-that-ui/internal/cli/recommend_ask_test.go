package cli

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/store"
)

func shellWords(t *testing.T, command string) []string {
	t.Helper()
	result, err := exec.Command("sh", "-c", "set -- "+command+"; printf '<%s>\\n' \"$@\"").Output()
	if err != nil {
		t.Fatalf("execute suggested command %q: %v", command, err)
	}
	lines := strings.Split(strings.TrimSpace(string(result)), "\n")
	words := make([]string, 0, len(lines))
	for _, line := range lines {
		words = append(words, strings.TrimSuffix(strings.TrimPrefix(line, "<"), ">"))
	}
	return words
}

func TestAskSuggestedCommandsShellQuoteMultiwordValues(t *testing.T) {
	tests := []struct {
		intent    string
		framework string
		want      []string
	}{
		{"compare disclosure panel vs segmented control", "", []string{"component", "compare", "disclosure panel", "segmented control"}},
		{"what do you call a searchable combo box", "", []string{"identify", "what do you call a searchable combo box"}},
		{"recommend a searchable combo box", "Swift UI", []string{"recommend", "recommend a searchable combo box", "--framework", "Swift UI"}},
	}
	for _, tc := range tests {
		t.Run(tc.intent, func(t *testing.T) {
			route := lexicalAskRoute(tc.intent, tc.framework)
			if got := shellWords(t, route.SuggestedCommand); strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Fatalf("words = %#v, want %#v (command %q)", got, tc.want, route.SuggestedCommand)
			}
		})
	}
}

func seedRecommendAskDB(t *testing.T) string {
	t.Helper()
	path := seedComponentDB(t)
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	styles := []namethatui.Style{
		{ID: "glassmorphism", Slug: "glassmorphism", Name: "Glassmorphism", SourceURL: "https://example.test/glassmorphism", Signals: []namethatui.Signal{{ID: "frosted", Name: "Frosted translucency", Description: "Translucent frosted panels blur the background."}}, Sections: []namethatui.Section{}},
	}
	for _, style := range styles {
		raw, _ := json.Marshal(style)
		if err := db.Upsert("style_details", style.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.SaveSyncState("style_details", "", len(styles)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func runRecommendAsk(t *testing.T, db, command string, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	base := []string{"--json", "--no-learn", command, "--db", db}
	root.SetArgs(append(base, args...))
	err := root.Execute()
	result := map[string]any{}
	if out.Len() > 0 {
		if decodeErr := json.Unmarshal(out.Bytes(), &result); decodeErr != nil {
			t.Fatalf("invalid JSON %q: %v", out.String(), decodeErr)
		}
	}
	return result, err
}

func TestRecommendLocalEvidenceFrameworkAmbiguityAndDryRun(t *testing.T) {
	db := seedRecommendAskDB(t)
	got, err := runRecommendAsk(t, db, "recommend", "searchable combo box", "--framework", "React", "--limit", "2")
	if err != nil {
		t.Fatal(err)
	}
	choice := got["choice"].(map[string]any)
	if choice["id"] != "web/combobox" || len(choice["evidence"].([]any)) == 0 {
		t.Fatalf("recommendation = %#v", got)
	}
	if apis := choice["framework_apis"].([]any); len(apis) != 1 || apis[0].(map[string]any)["framework"] != "React" {
		t.Fatalf("framework APIs = %#v", apis)
	}
	if watchOuts := choice["watch_outs"].([]any); len(watchOuts) != 1 || watchOuts[0] != "debug upstream" {
		t.Fatalf("watch-outs must be verbatim upstream evidence: %#v", watchOuts)
	}
	if provenance := got["provenance"].(map[string]any); provenance["data_source"] != "local" || got["freshness"] == nil {
		t.Fatalf("provenance/freshness = %#v", got)
	}
	got, err = runRecommendAsk(t, db, "recommend", "search")
	if err != nil || got["ambiguous"] != true || len(got["alternatives"].([]any)) == 0 {
		t.Fatalf("close scores must remain ambiguous: %#v err=%v", got, err)
	}
	got, err = runRecommendAsk(t, db, "recommend", "search", "--limit", "1")
	if err != nil || got["ambiguous"] != true {
		t.Fatalf("limit must not erase close-score ambiguity: %#v err=%v", got, err)
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	_, err = runRecommendAsk(t, missing, "recommend", "combo box")
	if err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing catalog hint error = %v", err)
	}
	got, err = runRecommendAsk(t, missing, "recommend", "combo box", "--dry-run")
	if err != nil || got["sqlite_opened"] != false || got["alternatives"] == nil || got["source_urls"] == nil {
		t.Fatalf("dry run = %#v err=%v", got, err)
	}
}

func TestAskLexicalRoutesCandidatesHintsAndLowConfidence(t *testing.T) {
	db := seedRecommendAskDB(t)
	cases := []struct {
		intent string
		route  string
		check  func(t *testing.T, got map[string]any)
	}{
		{"what do you call a searchable combo box", "identify", func(t *testing.T, got map[string]any) {
			if len(got["candidates"].([]any)) == 0 {
				t.Fatal(got)
			}
		}},
		{"recommend a searchable combo box", "recommend", func(t *testing.T, got map[string]any) {
			if len(got["candidates"].([]any)) == 0 {
				t.Fatal(got)
			}
		}},
		{"compare combobox vs select", "component_compare", func(t *testing.T, got map[string]any) {
			if len(got["parsed_terms"].([]any)) != 2 {
				t.Fatal(got)
			}
		}},
		{"translate combobox to SwiftUI", "translate", func(t *testing.T, got map[string]any) {
			if len(got["parsed_terms"].([]any)) != 2 {
				t.Fatal(got)
			}
		}},
		{"identify a frosted visual style", "style_identify", func(t *testing.T, got map[string]any) {
			if len(got["style_candidates"].([]any)) == 0 {
				t.Fatal(got)
			}
		}},
		{"hmm", "identify", func(t *testing.T, got map[string]any) {
			if got["confidence"] != "low" {
				t.Fatal(got)
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.route, func(t *testing.T) {
			got, err := runRecommendAsk(t, db, "ask", tc.intent)
			if err != nil || got["route"] != tc.route || got["suggested_command"] == "" {
				t.Fatalf("ask route = %#v err=%v", got, err)
			}
			tc.check(t, got)
		})
	}
	missing := filepath.Join(t.TempDir(), "missing.db")
	_, err := runRecommendAsk(t, missing, "ask", "identify a visual style")
	if err == nil || !strings.Contains(err.Error(), "sync --resources styles") {
		t.Fatalf("missing style hint error = %v", err)
	}
	got, err := runRecommendAsk(t, missing, "ask", "compare combobox vs select", "--dry-run")
	if err != nil || got["sqlite_opened"] != false || got["candidates"] == nil || got["style_candidates"] == nil || got["parsed_terms"] == nil {
		t.Fatalf("ask dry run = %#v err=%v", got, err)
	}
}

func TestAskAgentOutputUsesBoundedCandidateSummaries(t *testing.T) {
	dbPath := seedRecommendAskDB(t)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 120; i++ {
		component := namethatui.Component{
			ID:       "web/verbose-" + strings.Repeat("x", 3) + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)),
			Platform: "web", Slug: "verbose", Name: "Verbose candidate", AKA: []string{"combo box"},
			Description: strings.Repeat("source-backed searchable combo box detail ", 300),
			Prompt:      strings.Repeat("prompt ", 300), DebugPrompt: strings.Repeat("debug ", 300),
			SourceURL: "https://example.test/verbose",
		}
		raw, _ := json.Marshal(component)
		if err := db.Upsert("components", component.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runRootArgs(t, "--agent", "--no-learn", "ask", "--db", dbPath, "recommend a searchable combo box")
	if err != nil {
		t.Fatal(err)
	}
	if len(stdout) > 64<<10 {
		t.Fatalf("agent ask output = %d bytes, want bounded summary", len(stdout))
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope["results"].(map[string]any)
	if result["suggested_command"] == "" || result["candidate_detail_hint"] == "" {
		t.Fatalf("agent summary lost routing/follow-up evidence: %#v", result)
	}
	if result["candidate_query"] != "searchable combo box" {
		t.Fatalf("candidate query = %#v", result["candidate_query"])
	}
	if candidates := result["candidates"].([]any); len(candidates) > askCandidateSummaryLimit {
		t.Fatalf("candidate summaries = %d, want <= %d", len(candidates), askCandidateSummaryLimit)
	}
	candidate := result["candidates"].([]any)[0].(map[string]any)
	for _, key := range []string{"id", "name", "platform", "source_url"} {
		if candidate[key] == "" {
			t.Fatalf("candidate summary lost %s: %#v", key, candidate)
		}
	}
	if _, found := candidate["description"]; found {
		t.Fatalf("agent summary retained full candidate record: %#v", candidate)
	}
}

func TestAskExtractsSubjectBeforeRanking(t *testing.T) {
	dbPath := seedRecommendAskDB(t)
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		component := namethatui.Component{ID: "web/routing-noise-" + string(rune('a'+i)), Platform: "web", Slug: "routing-noise", Name: "Routing Noise", Description: "recommend source-backed guidance", SourceURL: "https://example.test/noise"}
		raw, _ := json.Marshal(component)
		if err := db.Upsert("components", component.ID, raw); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runRootArgs(t, "--agent", "--no-learn", "ask", "--db", dbPath, "recommend a searchable combo box")
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	result := envelope["results"].(map[string]any)
	if result["candidate_query"] != "searchable combo box" {
		t.Fatalf("candidate query = %#v", result)
	}
	candidates := result["candidates"].([]any)
	if len(candidates) == 0 || candidates[0].(map[string]any)["id"] != "web/combobox" {
		t.Fatalf("routing language polluted ranking: %#v", candidates)
	}
}
