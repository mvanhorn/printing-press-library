package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/name-that-ui/internal/namethatui"
)

func runIdentify(t *testing.T, db string, args ...string) (map[string]any, error) {
	t.Helper()
	var flags rootFlags
	root := newRootCmd(&flags)
	out := &bytes.Buffer{}
	root.SetOut(out)
	root.SetErr(&bytes.Buffer{})
	base := []string{"--json", "--no-learn", "identify", "--db", db}
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

func TestIdentifyTwoStageRemoteEnrichmentAndAmbiguity(t *testing.T) {
	db := seedComponentDB(t)
	var calls []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/search" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		calls = append(calls, body)
		w.Header().Set("Content-Type", "application/json")
		if body["mode"] == "retrieve" {
			local, ok := body["local"].([]any)
			if !ok || len(local) == 0 {
				t.Fatalf("retrieve local candidates = %#v", body["local"])
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"web/combobox#input","score":0.72,"reason":"retrieved"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"source":"semantic","results":[{"id":"web/combobox#input","score":0.99,"reason":"exact"}],"ambiguous":true,"clarification":"choose a platform"}`))
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	got, err := runIdentify(t, db, "searchable choice input")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0]["mode"] != "retrieve" || calls[1]["mode"] != "resolve" {
		t.Fatalf("stages = %#v", calls)
	}
	if calls[0]["q"] != "searchable choice input" || calls[1]["q"] != "searchable choice input" {
		t.Fatalf("query bodies = %#v", calls)
	}
	if candidates, ok := calls[1]["candidates"].([]any); !ok || len(candidates) != 1 || candidates[0] != "web/combobox#input" {
		t.Fatalf("resolve candidates = %#v", calls[1]["candidates"])
	}
	if got["source"] != "semantic" || got["ambiguous"] != true || got["clarification"] != "choose a platform" {
		t.Fatalf("upstream fields were changed: %#v", got)
	}
	results := got["results"].([]any)
	result := results[0].(map[string]any)
	if result["score"] != 0.99 || result["reason"] != "exact" || result["source_url"] != "https://example.test/combobox#input" || result["name"] != "Combobox" || result["platform"] != "web" {
		t.Fatalf("enriched result = %#v", result)
	}
	if result["part"].(map[string]any)["id"] != "input" || result["component"].(map[string]any)["id"] != "web/combobox" {
		t.Fatalf("missing component details: %#v", result)
	}
	if got["meta"].(map[string]any)["data_source"] != "live" {
		t.Fatalf("meta = %#v", got["meta"])
	}
}

func TestIdentifyLocalOnlyNegativeAndStableCollections(t *testing.T) {
	db := seedComponentDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("local-only identify must not call the server")
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	got, err := runIdentify(t, db, "unrelated antigravity deforestation", "--local-only")
	if err != nil {
		t.Fatal(err)
	}
	results, ok := got["results"].([]any)
	if !ok || results == nil || len(results) != 0 {
		t.Fatalf("negative results = %#v", got["results"])
	}
	if got["meta"].(map[string]any)["reason"] != "local_only" {
		t.Fatalf("meta = %#v", got["meta"])
	}

	got, err = runIdentify(t, db, "combo box", "--local-only")
	if err != nil {
		t.Fatal(err)
	}
	first := got["results"].([]any)[0].(map[string]any)
	if reasons, ok := first["reasons"].([]any); !ok || reasons == nil {
		t.Fatalf("reasons must be []: %#v", first["reasons"])
	}
}

func TestIdentifyLiveDoesNotRequireLocalStore(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	var calls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if body["mode"] == "retrieve" {
			if local, ok := body["local"].([]any); !ok || len(local) != 0 {
				t.Fatalf("fresh-install local candidates = %#v", body["local"])
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"web/combobox#input"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"web/combobox#input","score":1}]}`))
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)
	got, err := runIdentify(t, missing, "searchable combo box", "--data-source", "live")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 2 || got["meta"].(map[string]any)["data_source"] != "live" {
		t.Fatalf("live identify = %#v, calls=%d", got, calls)
	}
	result := got["results"].([]any)[0].(map[string]any)
	if result["name"] != "Combobox" || result["platform"] != "web" || result["source_url"] != "https://namethatui.com/web/combobox#input" {
		t.Fatalf("fresh live candidate lacks canonical metadata: %#v", result)
	}
	if part, exists := result["part"]; !exists || part.(map[string]any)["id"] != "input" {
		t.Fatalf("fresh live candidate must retain stable part path: %#v", result)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("live identify opened local db: %v", err)
	}
}

func TestIdentifyAgentEnvelopePreservesLiveSource(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.db")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		w.Header().Set("Content-Type", "application/json")
		if body["mode"] == "retrieve" {
			_, _ = w.Write([]byte(`{"results":[{"id":"web/combobox"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"web/combobox","score":1}]}`))
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	stdout, _, err := runRootArgs(t, "--agent", "--no-learn", "--data-source", "live", "identify", "--db", missing, "searchable combo box")
	if err != nil {
		t.Fatal(err)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(stdout), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope["meta"].(map[string]any)["source"] != "live" {
		t.Fatalf("agent provenance = %#v", envelope)
	}
	if envelope["results"].(map[string]any)["meta"].(map[string]any)["data_source"] != "live" {
		t.Fatalf("result provenance = %#v", envelope)
	}
}

func TestIdentifyLocalRankingPrefersWholeComboboxAlias(t *testing.T) {
	items := []namethatui.Component{
		{ID: "web/combobox", Platform: "web", Slug: "combobox", Name: "Combobox", AKA: []string{"combo box"}, Fuzzy: []string{"searchable combo box"}, SourceURL: "https://namethatui.com/web/combobox"},
		{ID: "mac/search-field", Platform: "mac", Slug: "search-field", Name: "Search Field", Fuzzy: []string{"searchable"}, SourceURL: "https://namethatui.com/mac/search-field"},
		{ID: "web/resize-handle", Platform: "web", Slug: "resize-handle", Name: "Resize Handle", Fuzzy: []string{"box"}, SourceURL: "https://namethatui.com/web/resize-handle"},
	}
	got := identifyLocalCandidates(items, "searchable combo box", 8)
	if len(got) < 3 || got[0].ID != "web/combobox" {
		t.Fatalf("ranking = %#v, want canonical web Combobox first", got)
	}
	if got[0].Score <= got[1].Score || got[0].Score <= got[2].Score {
		t.Fatalf("whole phrase did not dominate isolated token noise: %#v", got)
	}
}

func TestIdentifyLocalRankingNormalizesCompoundSlugAgainstActualShape(t *testing.T) {
	items := []namethatui.Component{
		{ID: "web/combobox", Platform: "web", Slug: "combobox", Name: "Combobox (Autocomplete / Typeahead)", AKA: []string{"autocomplete", "typeahead", "search select", "editable dropdown"}, Fuzzy: []string{"a searchable dropdown field"}, Description: "Typing narrows the choices.", SourceURL: "https://namethatui.com/web/combobox"},
		{ID: "macos/popup-pulldown-combo", Platform: "macos", Slug: "popup-pulldown-combo", Name: "Pop-Up Button vs. Pull-Down Button vs. Combo Box", AKA: []string{"combo box", "menu picker"}, Fuzzy: []string{"text field with a dropdown list attached"}, Description: "A combo box is a text field plus a drop-down list.", SourceURL: "https://namethatui.com/macos/popup-pulldown-combo"},
	}
	got := identifyLocalCandidates(items, "searchable combo box", 8)
	if len(got) < 2 || got[0].ID != "web/combobox" {
		t.Fatalf("compound-slug ranking = %#v, want canonical web/combobox first", got)
	}
}

func TestIdentifyLocalOnlyRanksCanonicalComboboxFromStoredRecord(t *testing.T) {
	db := seedComponentDB(t)
	got, err := runIdentify(t, db, "searchable combo box", "--local-only")
	if err != nil {
		t.Fatal(err)
	}
	results := got["results"].([]any)
	if len(results) == 0 || results[0].(map[string]any)["id"] != "web/combobox" {
		t.Fatalf("stored-record ranking = %#v", results)
	}
}

func TestIdentifyFallbackLiveErrorMissingMirrorAndDryRun(t *testing.T) {
	db := seedComponentDB(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	}))
	defer server.Close()
	t.Setenv("NAME_THAT_UI_BASE_URL", server.URL)

	got, err := runIdentify(t, db, "combo box")
	if err != nil {
		t.Fatal(err)
	}
	meta := got["meta"].(map[string]any)
	if meta["data_source"] != "local" || meta["reason"] != "live_error" || meta["fallback_reason"] == "" {
		t.Fatalf("fallback meta = %#v", meta)
	}

	_, err = runIdentify(t, db, "combo box", "--data-source", "live")
	if err == nil || ExitCode(err) != 5 {
		t.Fatalf("live error = %v (code %d)", err, ExitCode(err))
	}
	_, err = runIdentify(t, filepath.Join(t.TempDir(), "missing.db"), "combo box", "--local-only")
	if err == nil || !strings.Contains(err.Error(), "sync --resources catalog") {
		t.Fatalf("missing mirror error = %v", err)
	}

	got, err = runIdentify(t, filepath.Join(t.TempDir(), "dry.db"), "combo box", "--dry-run")
	if err != nil {
		t.Fatal(err)
	}
	if got["sqlite_opened"] != false || len(got["stages"].([]any)) != 2 {
		t.Fatalf("dry run = %#v", got)
	}
}
