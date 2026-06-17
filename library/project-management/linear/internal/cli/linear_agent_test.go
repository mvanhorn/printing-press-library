package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func TestRenderIssueSelectDescriptionBeatsAgentCompact(t *testing.T) {
	t.Parallel()
	data := json.RawMessage(`{
		"identifier":"SYMPH-310",
		"title":"Follow-up",
		"description":"literal body with $(expansion) and ` + "`backticks`" + `",
		"state":{"name":"Backlog"}
	}`)
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	flags := &rootFlags{asJSON: true, compact: true, selectFields: "identifier,description"}
	if err := renderIssue(cmd, flags, data, DataProvenance{Source: "live", ResourceType: "issues"}); err != nil {
		t.Fatalf("renderIssue: %v", err)
	}
	var got struct {
		Results struct {
			Identifier  string `json:"identifier"`
			Description string `json:"description"`
			Title       string `json:"title"`
		} `json:"results"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if got.Results.Description == "" {
		t.Fatalf("description was stripped under --agent + --select: %s", out.String())
	}
	if got.Results.Title != "" {
		t.Fatalf("unselected title leaked into output: %s", out.String())
	}
}

func TestCommentsAddReadsBodyFileLiterally(t *testing.T) {
	body := "Source body with $(danger), ${vars}, `backticks`, and GraphQL $input: String!\n"
	bodyPath := filepath.Join(t.TempDir(), "comment.md")
	if err := os.WriteFile(bodyPath, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "issues(filter"):
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"issue-uuid"}]}}}`)
		case strings.Contains(req.Query, "commentCreate"):
			input, _ := req.Variables["input"].(map[string]any)
			seenBody, _ = input["body"].(string)
			fmt.Fprint(w, `{"data":{"commentCreate":{"success":true,"comment":{"id":"comment-1","body":"ok","createdAt":"2026-06-09T00:00:00Z","updatedAt":"2026-06-09T00:00:00Z","user":{"id":"user-1","name":"eric","displayName":"eric","email":"e@example.com"},"issue":{"id":"issue-uuid","identifier":"MOB-99","title":"Issue"}}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("comments", "add", "--issue", "MOB-99", "--body-file", bodyPath, "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("comments add failed: %v\n%s", err, out)
	}
	if seenBody != body {
		t.Fatalf("body sent to GraphQL = %q, want literal %q", seenBody, body)
	}
}

func TestCommentsAddReadsBodyStdinLiterally(t *testing.T) {
	body := "stdin body with $(danger), ${vars}, `backticks`, and GraphQL $input: String!\n"
	var seenBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "issues(filter"):
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"issue-uuid"}]}}}`)
		case strings.Contains(req.Query, "commentCreate"):
			input, _ := req.Variables["input"].(map[string]any)
			seenBody, _ = input["body"].(string)
			fmt.Fprint(w, `{"data":{"commentCreate":{"success":true,"comment":{"id":"comment-1","body":"ok","createdAt":"2026-06-09T00:00:00Z","updatedAt":"2026-06-09T00:00:00Z","user":{"id":"user-1","name":"eric","displayName":"eric","email":"e@example.com"},"issue":{"id":"issue-uuid","identifier":"MOB-99","title":"Issue"}}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithInput(body, "comments", "add", "--issue", "MOB-99", "--body-stdin", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("comments add failed: %v\n%s", err, out)
	}
	if seenBody != body {
		t.Fatalf("body sent to GraphQL = %q, want literal %q", seenBody, body)
	}
}

func TestCommentsAddRejectsEmptyBodyStdin(t *testing.T) {
	out, err := executeRootForTestWithInputAndRenderedError("", "comments", "add", "--issue", "MOB-99", "--body-stdin", "--agent")
	if err == nil {
		t.Fatalf("comments add with empty stdin succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	var envelope struct {
		Code int    `json:"code"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("empty stdin error output is not JSON: %v\n%s", err, out)
	}
	if envelope.Code != 2 || envelope.Type != "usage" {
		t.Fatalf("empty stdin envelope = %+v, want code=2 type=usage; output=%s", envelope, out)
	}
}

func TestSimilarAgentOutputsJSON(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	raw := json.RawMessage(`{"id":"issue-1","identifier":"SYMPH-309","title":"Headless follow-ups","description":"body"}`)
	if err := db.UpsertIssue("issue-1", "SYMPH-309", "Headless follow-ups", raw); err != nil {
		t.Fatalf("UpsertIssue: %v", err)
	}

	out, err := executeRootForTest("similar", "SYMPH-309", "--db", dbPath, "--agent")
	if err != nil {
		t.Fatalf("similar --agent failed: %v\n%s", err, out)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("similar --agent output is not JSON: %v\n%s", err, out)
	}
	if len(results) != 1 || results[0]["identifier"] != "SYMPH-309" {
		t.Fatalf("unexpected similar results: %s", out)
	}
}

func TestSimilarTeamFilterUsesLocalTeamKey(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.UpsertTeam("team-symph", json.RawMessage(`{"id":"team-symph","key":"SYMPH","name":"Symphony"}`)); err != nil {
		t.Fatalf("UpsertTeam symph: %v", err)
	}
	if err := db.UpsertTeam("team-mob", json.RawMessage(`{"id":"team-mob","key":"MOB","name":"Mobilyze"}`)); err != nil {
		t.Fatalf("UpsertTeam mob: %v", err)
	}
	if err := db.UpsertIssue("issue-symph", "SYMPH-309", "Pipeline follow-up", json.RawMessage(`{"id":"issue-symph","identifier":"SYMPH-309","title":"Pipeline follow-up","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph"}`)); err != nil {
		t.Fatalf("UpsertIssue symph: %v", err)
	}
	if err := db.UpsertIssue("issue-mob", "MOB-118", "Pipeline follow-up", json.RawMessage(`{"id":"issue-mob","identifier":"MOB-118","title":"Pipeline follow-up","team":{"id":"team-mob","key":"MOB"},"teamId":"team-mob"}`)); err != nil {
		t.Fatalf("UpsertIssue mob: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	out, err := executeRootForTest("similar", "pipeline follow-up", "--team", "SYMPH", "--db", dbPath, "--agent")
	if err != nil {
		t.Fatalf("similar --team failed: %v\n%s", err, out)
	}
	var results []map[string]any
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		t.Fatalf("similar --team output is not JSON: %v\n%s", err, out)
	}
	if len(results) != 1 || results[0]["identifier"] != "SYMPH-309" {
		t.Fatalf("unexpected similar --team results: %s", out)
	}
}

func TestDocumentsCreateRequiresExactlyOneParentBeforeMutation(t *testing.T) {
	out, err := executeRootForTestWithRenderedError("documents", "create", "--title", "Runbook", "--content", "body", "--agent")
	if err == nil {
		t.Fatalf("documents create without parent succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	var envelope struct {
		Code int    `json:"code"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("usage error output is not JSON: %v\n%s", err, out)
	}
	if envelope.Code != 2 || envelope.Type != "usage" {
		t.Fatalf("usage error envelope = %+v, want code=2 type=usage; output=%s", envelope, out)
	}

	out, err = executeRootForTestWithRenderedError("documents", "create", "--title", "Runbook", "--content", "body", "--team", "SYMPH", "--project", "project-1", "--agent")
	if err == nil {
		t.Fatalf("documents create with multiple parents succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
}

func TestDocumentsCreateResolvesTeamKeyBeforeMutation(t *testing.T) {
	var sawTeamLookup bool
	var seenTeamID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "teams(filter"):
			sawTeamLookup = true
			fmt.Fprint(w, `{"data":{"teams":{"nodes":[{"id":"team-symph","key":"SYMPH","name":"Symphony"}]}}}`)
		case strings.Contains(req.Query, "documentCreate"):
			input, _ := req.Variables["input"].(map[string]any)
			seenTeamID, _ = input["teamId"].(string)
			fmt.Fprint(w, `{"data":{"documentCreate":{"success":true,"document":{"id":"doc-1","title":"Runbook","slugId":"runbook-f7f48ab36080","url":"https://linear.app/acme/document/runbook-f7f48ab36080","content":"body","createdAt":"2026-06-12T00:00:00Z","updatedAt":"2026-06-12T00:00:00Z","documentContentId":"content-1","team":{"id":"team-symph","key":"SYMPH","name":"Symphony"}}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("documents", "create", "--title", "Runbook", "--team", "SYMPH", "--content", "body", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("documents create failed: %v\n%s", err, out)
	}
	if !sawTeamLookup {
		t.Fatalf("team key lookup was not performed")
	}
	if seenTeamID != "team-symph" {
		t.Fatalf("documentCreate teamId = %q, want team-symph", seenTeamID)
	}
}

func TestDocumentsEditUUIDTitleDoesNotFetchExistingDocument(t *testing.T) {
	const documentID = "00000000-0000-0000-0000-000000000123"
	var sawUpdate bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "documentUpdate"):
			sawUpdate = true
			if got, _ := req.Variables["id"].(string); got != documentID {
				t.Errorf("documentUpdate id = %q, want %q", got, documentID)
			}
			fmt.Fprint(w, `{"data":{"documentUpdate":{"success":true,"document":{"id":"00000000-0000-0000-0000-000000000123","title":"Updated","slugId":"updated-f7f48ab36080","url":"https://linear.app/acme/document/updated-f7f48ab36080","content":"body","createdAt":"2026-06-12T00:00:00Z","updatedAt":"2026-06-12T00:00:00Z","documentContentId":"content-1"}}}}`)
		case strings.Contains(req.Query, "document(id:") || strings.Contains(req.Query, "documents(filter"):
			t.Errorf("documents edit fetched existing document despite UUID title-only edit: %s", req.Query)
			http.Error(w, "unexpected fetch", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("documents", "edit", documentID, "--title", "Updated", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("documents edit failed: %v\n%s", err, out)
	}
	if !sawUpdate {
		t.Fatalf("documentUpdate was not called")
	}
}

func TestCommentsListKeepsBodiesInAgentMode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "issues(filter"):
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"issue-uuid"}]}}}`)
		case strings.Contains(req.Query, "comments(first"):
			fmt.Fprint(w, `{"data":{"issue":{"id":"issue-uuid","identifier":"MOB-99","title":"Issue","comments":{"nodes":[{"id":"comment-1","body":"full comment body","createdAt":"2026-06-09T00:00:00Z","updatedAt":"2026-06-09T00:00:00Z","user":{"id":"user-1","name":"eric"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("comments", "list", "--issue", "MOB-99", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("comments list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "full comment body") {
		t.Fatalf("agent output stripped comment body: %s", out)
	}
}

func TestPromotedGraphQLReadsUsePost(t *testing.T) {
	var seen []string
	var teamsAfter []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.Path)
		if r.Method != http.MethodPost {
			http.Error(w, "GraphQL must use POST", http.StatusBadRequest)
			return
		}
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "teams(first"):
			after, _ := req.Variables["after"].(string)
			teamsAfter = append(teamsAfter, after)
			if after == "" {
				fmt.Fprint(w, `{"data":{"teams":{"nodes":[{"id":"team-1","key":"SYMPH","name":"Symphony","description":"Team","createdAt":"2026-06-10T00:00:00Z","updatedAt":"2026-06-10T00:00:00Z"}],"pageInfo":{"hasNextPage":true,"endCursor":"cursor-1"}}}}`)
				return
			}
			if after != "cursor-1" {
				t.Errorf("teams after cursor = %q, want cursor-1", after)
				http.Error(w, "unexpected cursor", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"data":{"teams":{"nodes":[{"id":"team-2","key":"MOB","name":"Mobilyze","description":"Team","createdAt":"2026-06-10T00:00:00Z","updatedAt":"2026-06-10T00:00:00Z"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
		case strings.Contains(req.Query, "project(id:"):
			fmt.Fprint(w, `{"data":{"project":{"id":"project-1","name":"Pipeline","state":"backlog","description":"Reserved","teams":{"nodes":[{"id":"team-1","key":"SYMPH","name":"Symphony"}]}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("teams", "--agent", "--data-source", "live", "--select", "id,key,name")
	if err != nil {
		t.Fatalf("teams failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "SYMPH") {
		t.Fatalf("teams output missing result: %s", out)
	}
	if !strings.Contains(out, "MOB") {
		t.Fatalf("teams output missing paginated result: %s", out)
	}
	if strings.Join(teamsAfter, ",") != ",cursor-1" {
		t.Fatalf("teams cursors = %q, want first page then cursor-1", teamsAfter)
	}

	out, err = executeRootForTest("projects", "get", "project-1", "--agent", "--data-source", "live", "--select", "id,name,state")
	if err != nil {
		t.Fatalf("projects get failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Pipeline") {
		t.Fatalf("projects output missing result: %s", out)
	}
	for _, methodPath := range seen {
		if methodPath != "POST /graphql" {
			t.Fatalf("saw %s, want only POST /graphql", methodPath)
		}
	}
}

func TestLabelsListFiltersTeamAndGlobal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueLabels") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[{"id":"global","name":"source:user-report","color":"#111","team":null},{"id":"symph","name":"pipeline-halt","color":"#222","team":{"id":"team-symph","key":"SYMPH","name":"Symphony"}},{"id":"hsui","name":"area:protocols","color":"#333","team":{"id":"team-hsui","key":"HSUI","name":"HS UI"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("labels", "list", "--team", "SYMPH", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("labels list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "pipeline-halt") || !strings.Contains(out, "source:user-report") {
		t.Fatalf("labels list omitted safe labels: %s", out)
	}
	if strings.Contains(out, "area:protocols") {
		t.Fatalf("labels list included another team's label: %s", out)
	}

	out, err = executeRootForTest("labels", "list", "--team", "Symphony", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("labels list by team name failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "pipeline-halt") || strings.Contains(out, "area:protocols") {
		t.Fatalf("labels list by team name returned wrong labels: %s", out)
	}
}

func TestLabelsListUsesLocalIssueLabelTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if err := db.UpsertIssueLabel("global", json.RawMessage(`{"id":"global","name":"source:user-report","color":"#111","team":null}`)); err != nil {
		t.Fatalf("upsert global label: %v", err)
	}
	if err := db.UpsertIssueLabel("symph", json.RawMessage(`{"id":"symph","name":"pipeline-halt","color":"#222","team":{"id":"team-symph","key":"SYMPH","name":"Symphony"}}`)); err != nil {
		t.Fatalf("upsert symph label: %v", err)
	}
	if err := db.UpsertIssueLabel("hsui", json.RawMessage(`{"id":"hsui","name":"area:protocols","color":"#333","team":{"id":"team-hsui","key":"HSUI","name":"HS UI"}}`)); err != nil {
		t.Fatalf("upsert hsui label: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	out, err := executeRootForTest("labels", "list", "--team", "SYMPH", "--agent", "--data-source", "local", "--db", dbPath, "--select", "name,team.key")
	if err != nil {
		t.Fatalf("labels list local failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"source:user-report"`) || !strings.Contains(out, `"pipeline-halt"`) {
		t.Fatalf("local labels omitted safe labels: %s", out)
	}
	if strings.Contains(out, "area:protocols") {
		t.Fatalf("local labels included another team's label: %s", out)
	}
	var envelope struct {
		Meta struct {
			Source string `json:"source"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("local labels output is not JSON: %v\n%s", err, out)
	}
	if envelope.Meta.Source != "local" {
		t.Fatalf("local labels source = %q, want local: %s", envelope.Meta.Source, out)
	}
}

func TestIssueCreateRejectsCrossTeamLabelBeforeMutation(t *testing.T) {
	createCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "issueLabel(id:"):
			fmt.Fprint(w, `{"data":{"issueLabel":{"id":"label-hsui","name":"area:protocols","color":"#333","team":{"id":"team-hsui","key":"HSUI","name":"HS UI"}}}}`)
		case strings.Contains(req.Query, "issueCreate"):
			createCalled = true
			http.Error(w, "issueCreate should not be called", http.StatusInternalServerError)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "create", "--team", "SYMPH", "--title", "Bad label", "--label", "label-hsui", "--agent", "--data-source", "live")
	if err == nil {
		t.Fatalf("issues create succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	if createCalled {
		t.Fatalf("issueCreate mutation was called despite cross-team label")
	}
}

func TestLiveReadCommandsClassifyAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		args       []string
		wantCode   int
	}{
		{
			name:       "comments list auth",
			statusCode: http.StatusUnauthorized,
			args:       []string{"comments", "list", "--issue", "00000000-0000-0000-0000-000000000000", "--agent", "--data-source", "live"},
			wantCode:   4,
		},
		{
			name:       "documents read not found",
			statusCode: http.StatusNotFound,
			args:       []string{"documents", "missing-doc", "--agent", "--data-source", "live"},
			wantCode:   3,
		},
		{
			name:       "documents list rate limit",
			statusCode: http.StatusTooManyRequests,
			args:       []string{"documents", "list", "--agent", "--data-source", "live"},
			wantCode:   7,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(tt.statusCode), tt.statusCode)
			}))
			t.Cleanup(srv.Close)
			t.Setenv("LINEAR_BASE_URL", srv.URL)
			t.Setenv("LINEAR_API_KEY", "test-token")

			out, err := executeRootForTest(tt.args...)
			if err == nil {
				t.Fatalf("command succeeded unexpectedly:\n%s", out)
			}
			if got := ExitCode(err); got != tt.wantCode {
				t.Fatalf("ExitCode() = %d, want %d; err=%v\n%s", got, tt.wantCode, err, out)
			}
		})
	}
}

func TestWriteCommandsClassifyResolverAPIErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		args       []string
		wantCode   int
	}{
		{
			name:       "comments add issue resolver auth",
			statusCode: http.StatusUnauthorized,
			args:       []string{"comments", "add", "--issue", "MOB-99", "--body", "hello", "--agent", "--data-source", "live"},
			wantCode:   4,
		},
		{
			name:       "issues edit resolver rate limit",
			statusCode: http.StatusTooManyRequests,
			args:       []string{"issues", "edit", "MOB-99", "--title", "Updated", "--agent", "--data-source", "live"},
			wantCode:   7,
		},
		{
			name:       "documents create parent resolver auth",
			statusCode: http.StatusUnauthorized,
			args:       []string{"documents", "create", "--title", "Doc", "--issue", "MOB-99", "--content", "body", "--agent", "--data-source", "live"},
			wantCode:   4,
		},
		{
			name:       "documents edit lookup rate limit",
			statusCode: http.StatusTooManyRequests,
			args:       []string{"documents", "edit", "00000000-0000-0000-0000-000000000000", "--title", "Updated", "--agent", "--data-source", "live"},
			wantCode:   7,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, http.StatusText(tt.statusCode), tt.statusCode)
			}))
			t.Cleanup(srv.Close)
			t.Setenv("LINEAR_BASE_URL", srv.URL)
			t.Setenv("LINEAR_API_KEY", "test-token")

			out, err := executeRootForTest(tt.args...)
			if err == nil {
				t.Fatalf("command succeeded unexpectedly:\n%s", out)
			}
			if got := ExitCode(err); got != tt.wantCode {
				t.Fatalf("ExitCode() = %d, want %d; err=%v\n%s", got, tt.wantCode, err, out)
			}
		})
	}
}

func TestIssueCreateClassifiesMutationAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueCreate") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "create", "--team", "00000000-0000-0000-0000-000000000001", "--title", "Mutation failure", "--db", filepath.Join(t.TempDir(), "linear.db"), "--agent", "--data-source", "live")
	if err == nil {
		t.Fatalf("issues create succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 4 {
		t.Fatalf("ExitCode() = %d, want 4; err=%v\n%s", got, err, out)
	}
}

func TestMutationSuccessFalseUsesTypedAPIExitCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueUpdate") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"data":{"issueUpdate":{"success":false,"issue":null}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithRenderedError("issues", "edit", "00000000-0000-0000-0000-000000000000", "--title", "Rejected", "--agent", "--data-source", "live")
	if err == nil {
		t.Fatalf("issues edit succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode() = %d, want 5; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(out, `"code":5`) || !strings.Contains(out, `"type":"api"`) {
		t.Fatalf("agent error envelope did not classify success=false as API error:\n%s", out)
	}

	_, err = extractMutationObject(json.RawMessage(`{"commentCreate":{"success":false,"comment":null}}`), "commentCreate", "comment")
	if err == nil {
		t.Fatal("extractMutationObject succeeded unexpectedly")
	}
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode() = %d, want 5; err=%v", got, err)
	}
}

func TestMutationFailureAfterMediaUploadReportsAssetURL(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "screenshot.png")
	if err := os.WriteFile(mediaPath, []byte("image bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	const assetURL = "https://asset.example/screenshot.png"
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && r.URL.Path == "/upload" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "fileUpload"):
			uploadURL := srv.URL + "/upload"
			if err := json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"fileUpload": map[string]any{
						"success": true,
						"uploadFile": map[string]any{
							"uploadUrl": uploadURL,
							"assetUrl":  assetURL,
							"headers":   []map[string]string{},
						},
					},
				},
			}); err != nil {
				t.Errorf("encode fileUpload response: %v", err)
			}
		case strings.Contains(req.Query, "commentCreate"):
			fmt.Fprint(w, `{"errors":[{"message":"mutation rejected"}]}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithRenderedError("comments", "add", "--project", "project-1", "--body", "body", "--media", mediaPath, "--agent", "--data-source", "live")
	if err == nil {
		t.Fatalf("comments add succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode() = %d, want 5; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(err.Error(), assetURL) || !strings.Contains(out, assetURL) {
		t.Fatalf("uploaded asset URL was not surfaced; err=%v\n%s", err, out)
	}
}

func TestIssuesEditDryRunWithLabelsDoesNotCallAPI(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "edit", "MOB-99", "--label", "label-1", "--dry-run", "--agent")
	if err != nil {
		t.Fatalf("issues edit dry-run failed: %v\n%s", err, out)
	}
	if calls != 0 {
		t.Fatalf("dry-run made %d API calls; output:\n%s", calls, out)
	}
	if !strings.Contains(out, "would_update_issue") || !strings.Contains(out, "label-1") {
		t.Fatalf("dry-run output missing preview details: %s", out)
	}
}

func TestIssuesCreateDryRunWithMediaDoesNotCallAPI(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "create", "--title", "Dry run", "--team", "MOB", "--media", "/tmp/nonexistent-dry-run.png", "--dry-run", "--agent")
	if err != nil {
		t.Fatalf("issues create dry-run failed: %v\n%s", err, out)
	}
	if calls != 0 {
		t.Fatalf("dry-run made %d API calls; output:\n%s", calls, out)
	}
	if !strings.Contains(out, "would_create_issue") || !strings.Contains(out, "/tmp/nonexistent-dry-run.png") {
		t.Fatalf("dry-run output missing preview details: %s", out)
	}
}

func TestIssuesCreateValidatesLabelsBeforeUploadingMedia(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueLabel") {
			t.Errorf("unexpected query before media upload: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"data":{"issueLabel":{"id":"label-1","name":"area:protocols","color":"#333","team":{"id":"team-hsui","key":"HSUI","name":"HS UI"}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "create", "--title", "Bad label", "--team", "MOB", "--label", "label-1", "--media", "/tmp/nonexistent-dry-run.png", "--agent")
	if err == nil {
		t.Fatalf("issues create succeeded unexpectedly:\n%s", out)
	}
	if !strings.Contains(err.Error(), "belongs to team HSUI") && !strings.Contains(out, "belongs to team HSUI") {
		t.Fatalf("error did not come from label validation before media upload: err=%v\n%s", err, out)
	}
	if strings.Contains(err.Error(), "nonexistent-dry-run.png") || strings.Contains(out, "nonexistent-dry-run.png") {
		t.Fatalf("media path was touched before label validation: err=%v\n%s", err, out)
	}
}

func TestCommentsAndDocumentsDryRunDoNotCallAPI(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantEvent string
		wantToken string
	}{
		{
			name:      "comments add",
			args:      []string{"comments", "add", "--issue", "MOB-99", "--media", "/tmp/nonexistent-dry-run.png", "--dry-run", "--agent"},
			wantEvent: "would_create_comment",
			wantToken: "/tmp/nonexistent-dry-run.png",
		},
		{
			name:      "comments edit",
			args:      []string{"comments", "edit", "comment-1", "--media", "/tmp/nonexistent-dry-run.png", "--dry-run", "--agent"},
			wantEvent: "would_update_comment",
			wantToken: "comment-1",
		},
		{
			name:      "documents create",
			args:      []string{"documents", "create", "--title", "Runbook", "--issue", "MOB-99", "--media", "/tmp/nonexistent-dry-run.png", "--dry-run", "--agent"},
			wantEvent: "would_create_document",
			wantToken: "MOB-99",
		},
		{
			name:      "documents edit",
			args:      []string{"documents", "edit", "doc-slug", "--media", "/tmp/nonexistent-dry-run.png", "--dry-run", "--agent"},
			wantEvent: "would_update_document",
			wantToken: "doc-slug",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
			}))
			t.Cleanup(srv.Close)
			t.Setenv("LINEAR_BASE_URL", srv.URL)
			t.Setenv("LINEAR_API_KEY", "test-token")

			out, err := executeRootForTest(tt.args...)
			if err != nil {
				t.Fatalf("%s dry-run failed: %v\n%s", tt.name, err, out)
			}
			if calls != 0 {
				t.Fatalf("%s dry-run made %d API calls; output:\n%s", tt.name, calls, out)
			}
			if !strings.Contains(out, tt.wantEvent) || !strings.Contains(out, tt.wantToken) {
				t.Fatalf("%s dry-run output missing preview details: %s", tt.name, out)
			}
		})
	}
}

func TestIssuesEditPriorityZeroIsSent(t *testing.T) {
	var seenInput map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueUpdate") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		seenInput, _ = req.Variables["input"].(map[string]any)
		fmt.Fprint(w, `{"data":{"issueUpdate":{"success":true,"issue":{"id":"00000000-0000-0000-0000-000000000000","identifier":"MOB-99","title":"Issue","description":"","url":"https://linear.app/issue/MOB-99","priority":0,"state":{"id":"state-1","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"MOB","name":"Mobilyze"}}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "edit", "00000000-0000-0000-0000-000000000000", "--priority", "0", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("issues edit failed: %v\n%s", err, out)
	}
	if _, ok := seenInput["priority"]; !ok {
		t.Fatalf("priority was not sent in issueUpdate input: %#v", seenInput)
	}
	if got := seenInput["priority"]; got != float64(0) {
		t.Fatalf("priority = %#v, want 0", got)
	}
}

func executeRootForTest(args ...string) (string, error) {
	return executeRootForTestWithInput("", args...)
}

func executeRootForTestWithRenderedError(args ...string) (string, error) {
	return executeRootForTestWithInputAndRenderedError("", args...)
}

func executeRootForTestWithInput(input string, args ...string) (string, error) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if input != "" {
		cmd.SetIn(strings.NewReader(input))
	}
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func executeRootForTestWithInputAndRenderedError(input string, args ...string) (string, error) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if input != "" {
		cmd.SetIn(strings.NewReader(input))
	}
	cmd.SetArgs(args)
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	cmdErr := cmd.Execute()
	if cmdErr != nil {
		if isCobraUsageError(cmdErr) {
			cmdErr = usageErr(cmdErr)
		}
		if flags.asJSON && !flags.errorWritten {
			writeCLIErrorEnvelope(&flags, cmdErr, ExitCode(cmdErr))
		}
	}
	_ = w.Close()
	os.Stdout = stdout
	rendered, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		return out.String(), readErr
	}
	return out.String() + string(rendered), cmdErr
}
