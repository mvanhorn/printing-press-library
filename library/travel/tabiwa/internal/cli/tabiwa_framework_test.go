package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/tabiwa/internal/store"
)

func runTabiwaRootSearch(t *testing.T, args ...string) (map[string]any, error, string) {
	t.Helper()
	cmd := RootCmd()
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	var value map[string]any
	if err == nil {
		if e := json.Unmarshal(out.Bytes(), &value); e != nil {
			t.Fatalf("decode search: %v: %s", e, out.String())
		}
	}
	return value, err, out.String()
}
func TestTabiwaRootSearchLocalModesPreserveFTSTypeDBLimit(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"response":[{"id":"J0000900","name":"unrelated provider row"}]}`))
	}))
	defer server.Close()
	t.Setenv("TABIWA_BASE_URL", server.URL)
	path := filepath.Join(t.TempDir(), "custom-geography.db")
	db, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"id":"8","name":"福井県"}`, `{"id":"9","name":"福井観光"}`} {
		if err = db.UpsertGeography(json.RawMessage(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"auto", "local"} {
		result, e, _ := runTabiwaRootSearch(t, "search", "福井", "--data-source", mode, "--db", path, "--type", "geography", "--limit", "1", "--json", "--no-learn")
		if e != nil {
			t.Fatal(e)
		}
		rows, ok := result["results"].([]any)
		if !ok || len(rows) != 1 {
			t.Fatalf("limit/type/local FTS lost: %#v", result)
		}
		meta := result["meta"].(map[string]any)
		if meta["source"] != "local" {
			t.Fatalf("not local provenance: %#v", meta)
		}
	}
	result, e, _ := runTabiwaRootSearch(t, "search", "zz_tabiwa_absent_phrase", "--db", path, "--type", "geography", "--json", "--no-learn")
	if e != nil {
		t.Fatal(e)
	}
	if rows := result["results"].([]any); len(rows) != 0 {
		t.Fatalf("false text matches: %#v", result)
	}
	if calls != 0 {
		t.Fatalf("local root search issued %d provider requests", calls)
	}
}
func TestTabiwaRootSearchRejectsLiveWithoutProviderCall(t *testing.T) {
	testenv.Isolate(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Write([]byte(`{"response":[{"id":"J0000900","name":"unrelated"}]}`))
	}))
	defer server.Close()
	t.Setenv("TABIWA_BASE_URL", server.URL)
	_, err, _ := runTabiwaRootSearch(t, "search", "立山", "--data-source", "live", "--json", "--no-learn")
	if err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "catalog search") {
		t.Fatalf("expected actionable unsupported live: %v", err)
	}
	if calls != 0 {
		t.Fatalf("unsupported live still called provider: %d", calls)
	}
}
