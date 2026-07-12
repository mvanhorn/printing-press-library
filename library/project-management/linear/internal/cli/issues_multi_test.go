package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

func TestParseIssueIdentifiers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		raw     string
		want    []string
		wantErr bool
	}{
		{name: "single", raw: "MOB-1", want: []string{"MOB-1"}},
		{name: "ordered and trimmed", raw: "MOB-2, MOB-1", want: []string{"MOB-2", "MOB-1"}},
		{name: "deduplicated case-insensitively", raw: "MOB-2,mob-2,MOB-1", want: []string{"MOB-2", "MOB-1"}},
		{name: "empty middle", raw: "MOB-1,,MOB-2", wantErr: true},
		{name: "empty trailing", raw: "MOB-1,", wantErr: true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseIssueIdentifiers(tt.raw)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseIssueIdentifiers(%q) error = %v, wantErr %v", tt.raw, err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseIssueIdentifiers(%q) = %#v, want %#v", tt.raw, got, tt.want)
			}
		})
	}
}

func TestIssuesGlobalOutputFlagPlacement(t *testing.T) {
	dbPath := issueMultiTestDB(t)
	placements := [][]string{
		{"--json", "issues", "MOB-1", "--data-source", "local", "--db", dbPath},
		{"issues", "--json", "MOB-1", "--data-source", "local", "--db", dbPath},
		{"issues", "MOB-1", "--json", "--data-source", "local", "--db", dbPath},
		{"--agent", "issues", "MOB-1", "--data-source", "local", "--db", dbPath},
		{"issues", "--agent", "MOB-1", "--data-source", "local", "--db", dbPath},
		{"issues", "MOB-1", "--agent", "--data-source", "local", "--db", dbPath},
	}
	for _, args := range placements {
		out, err := executeRootForTest(args...)
		if err != nil {
			t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, out)
		}
		var payload struct {
			Results struct {
				Identifier string `json:"identifier"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(out), &payload); err != nil || payload.Results.Identifier != "MOB-1" {
			t.Fatalf("%s returned unexpected payload: err=%v output=%s", strings.Join(args, " "), err, out)
		}
	}
}

func TestIssuesCommaSeparatedReadPreservesOrderAndSingleContract(t *testing.T) {
	dbPath := issueMultiTestDB(t)

	out, err := executeRootForTest("issues", "MOB-2, MOB-1,MOB-2", "--agent", "--data-source", "local", "--db", dbPath, "--select", "identifier,title")
	if err != nil {
		t.Fatalf("multi issue read failed: %v\n%s", err, out)
	}
	var multi struct {
		Results []struct {
			Identifier string `json:"identifier"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &multi); err != nil {
		t.Fatalf("multi output is not JSON: %v\n%s", err, out)
	}
	if got := []string{multi.Results[0].Identifier, multi.Results[1].Identifier}; !reflect.DeepEqual(got, []string{"MOB-2", "MOB-1"}) {
		t.Fatalf("multi identifiers = %v, want caller order without duplicates", got)
	}

	out, err = executeRootForTest("issues", "MOB-1,MOB-1", "--agent", "--data-source", "local", "--db", dbPath, "--select", "identifier")
	if err != nil {
		t.Fatalf("deduplicated comma-list read failed: %v\n%s", err, out)
	}
	var deduplicated struct {
		Results []struct {
			Identifier string `json:"identifier"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &deduplicated); err != nil || len(deduplicated.Results) != 1 || deduplicated.Results[0].Identifier != "MOB-1" {
		t.Fatalf("comma-list contract changed after deduplication: err=%v output=%s", err, out)
	}

	out, err = executeRootForTest("issues", "MOB-1", "--agent", "--data-source", "local", "--db", dbPath, "--select", "identifier")
	if err != nil {
		t.Fatalf("single issue read failed: %v\n%s", err, out)
	}
	var single struct {
		Results struct {
			Identifier string `json:"identifier"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &single); err != nil || single.Results.Identifier != "MOB-1" {
		t.Fatalf("single issue contract changed: err=%v output=%s", err, out)
	}
}

func TestIssuesCommaSeparatedReadFailsClosedOnMissingIssue(t *testing.T) {
	dbPath := issueMultiTestDB(t)
	out, err := executeRootForTest("issues", "MOB-1,MOB-404", "--agent", "--data-source", "local", "--db", dbPath)
	if err == nil {
		t.Fatalf("multi issue read unexpectedly succeeded: %s", out)
	}
	if ExitCode(err) != 3 {
		t.Fatalf("missing multi issue exit code = %d, want 3: %v", ExitCode(err), err)
	}
	if strings.Contains(out, "MOB-1") {
		t.Fatalf("partial issue data leaked before failure: %s", out)
	}
}

func TestIssuesCommaSeparatedLiveReadDeduplicatesAndFailsClosed(t *testing.T) {
	calls := map[int]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		number := int(req.Variables["number"].(float64))
		calls[number]++
		if number == 404 {
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[]}}}`)
			return
		}
		fmt.Fprintf(w, `{"data":{"issues":{"nodes":[{"id":"issue-%d","identifier":"MOB-%d","title":"Issue %d"}]}}}`, number, number, number)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "MOB-2,MOB-1,MOB-2", "--agent", "--data-source", "live", "--select", "identifier")
	if err != nil {
		t.Fatalf("live multi issue read failed: %v\n%s", err, out)
	}
	var payload struct {
		Results []struct {
			Identifier string `json:"identifier"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("live multi output is not JSON: %v\n%s", err, out)
	}
	if got := []string{payload.Results[0].Identifier, payload.Results[1].Identifier}; !reflect.DeepEqual(got, []string{"MOB-2", "MOB-1"}) {
		t.Fatalf("live identifiers = %v, want caller order", got)
	}
	if calls[2] != 1 || calls[1] != 1 {
		t.Fatalf("duplicate identifiers triggered duplicate API work: calls=%v", calls)
	}

	out, err = executeRootForTest("issues", "MOB-1,MOB-404", "--agent", "--data-source", "live")
	if err == nil || ExitCode(err) != 3 {
		t.Fatalf("partial live read error = %v (code %d), want not-found code 3; output=%s", err, ExitCode(err), out)
	}
	if strings.Contains(out, "MOB-1") {
		t.Fatalf("partial live issue data leaked before failure: %s", out)
	}
}

func issueMultiTestDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, issue := range []struct {
		id, identifier, title string
	}{
		{id: "issue-1", identifier: "MOB-1", title: "First"},
		{id: "issue-2", identifier: "MOB-2", title: "Second"},
	} {
		raw, err := json.Marshal(map[string]any{"id": issue.id, "identifier": issue.identifier, "title": issue.title})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.UpsertIssue(issue.id, issue.identifier, issue.title, raw); err != nil {
			t.Fatal(err)
		}
	}
	return dbPath
}
