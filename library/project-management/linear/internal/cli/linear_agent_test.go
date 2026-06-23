package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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

func TestSimilarEmptyQueryReturnsUsageEnvelope(t *testing.T) {
	out, err := executeRootForTestWithRenderedError("similar", "", "--db", "/dev/null/linear.db", "--agent")
	if err == nil {
		t.Fatalf("similar with empty query succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	var envelope struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("usage error output is not JSON: %v\n%s", err, out)
	}
	if envelope.Code != 2 || envelope.Type != "usage" || !strings.Contains(envelope.Error, "search query cannot be empty") {
		t.Fatalf("usage error envelope = %+v, want code=2 type=usage with empty-query message; output=%s", envelope, out)
	}
}

func TestIssuesSearchAliasUsesSimilarSearchEngine(t *testing.T) {
	t.Parallel()
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.UpsertTeam("team-symph", json.RawMessage(`{"id":"team-symph","key":"SYMPH","name":"Symphony"}`)); err != nil {
		t.Fatalf("UpsertTeam: %v", err)
	}
	if err := db.UpsertIssue("issue-symph", "SYMPH-689", "Kimi replay temp directories cleanup", json.RawMessage(`{"id":"issue-symph","identifier":"SYMPH-689","title":"Kimi replay temp directories cleanup","description":"artifactContract exit code 2","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph"}`)); err != nil {
		t.Fatalf("UpsertIssue: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	out, err := executeRootForTest("issues", "search", "Kimi", "replay", "temp", "directories", "cleanup", "--team", "SYMPH", "--limit", "10", "--db", dbPath, "--agent", "--data-source", "local", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				StalePolicy string `json:"stale_policy"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issues search output is not JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-689" || got.Results[0]["title"] != "Kimi replay temp directories cleanup" {
		t.Fatalf("unexpected issues search results: %s", out)
	}
	if got.Meta.Freshness.StalePolicy != "allow" {
		t.Fatalf("issues search test DB should use stale-local policy via --data-source local, got %+v", got.Meta.Freshness)
	}
}

func TestIssuesSearchMissingQueryReturnsAgentUsageEnvelope(t *testing.T) {
	out, err := executeRootForTestWithRenderedError("issues", "search", "--agent")
	if err == nil {
		t.Fatalf("issues search without query succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	var envelope struct {
		Code  int    `json:"code"`
		Error string `json:"error"`
		Type  string `json:"type"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("usage error output is not JSON: %v\n%s", err, out)
	}
	if envelope.Code != 2 || envelope.Type != "usage" || !strings.Contains(envelope.Error, "linear-pp-cli similar") {
		t.Fatalf("usage error envelope = %+v, want code=2 type=usage with similar hint; output=%s", envelope, out)
	}
}

func TestIssuesSearchRefreshesStaleCacheBeforeSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)
	var issuesQueries int32
	srv := newIssueSearchRefreshServer(t, &issuesQueries, http.StatusOK, 0)
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "search", "Fresh", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search refresh failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Source    string `json:"source"`
			Freshness struct {
				StalePolicy   string `json:"stale_policy"`
				Refreshed     bool   `json:"refreshed"`
				RefreshReason string `json:"refresh_reason"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issues search output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-999" {
		t.Fatalf("unexpected refreshed search results: %+v\n%s", got.Results, out)
	}
	if got.Meta.Source != "local" || got.Meta.Freshness.StalePolicy != "refresh" || !got.Meta.Freshness.Refreshed || got.Meta.Freshness.RefreshReason != "stale" {
		t.Fatalf("unexpected freshness metadata: %+v\n%s", got.Meta, out)
	}
	if atomic.LoadInt32(&issuesQueries) != 1 {
		t.Fatalf("issues refresh queries = %d, want 1", issuesQueries)
	}
}

func TestIssuesSearchFreshCacheSkipsRefresh(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedFreshIssueSearchStore(t, dbPath)
	var apiCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&apiCalls, 1)
		http.Error(w, "fresh cache should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "search", "Fresh", "local", "--team", "SYMPH", "--db", dbPath, "--agent", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search fresh cache failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				Refreshed       bool   `json:"refreshed"`
				RefreshReason   string `json:"refresh_reason"`
				LocalIssueCount int    `json:"local_issue_count"`
				Unsynced        bool   `json:"unsynced"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("fresh cache output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-FRESH" {
		t.Fatalf("unexpected fresh-cache results: %+v\n%s", got.Results, out)
	}
	if atomic.LoadInt32(&apiCalls) != 0 {
		t.Fatalf("fresh cache API calls = %d, want 0", apiCalls)
	}
	if got.Meta.Freshness.Refreshed || got.Meta.Freshness.RefreshReason != "" || got.Meta.Freshness.LocalIssueCount != 1 || got.Meta.Freshness.Unsynced {
		t.Fatalf("unexpected fresh-cache metadata: %+v\n%s", got.Meta.Freshness, out)
	}
}

func TestIssuesSearchRefreshesAllIssueAndLabelPages(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)
	var issuesQueries int32
	var labelQueries int32
	srv := newIssueSearchMultiPageRefreshServer(t, &issuesQueries, &labelQueries)
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "search", "Fresh", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search multi-page refresh failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				Refreshed       bool   `json:"refreshed"`
				RefreshedBy     string `json:"refreshed_by"`
				LocalIssueCount int    `json:"local_issue_count"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("multi-page output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-999" {
		t.Fatalf("unexpected multi-page results: %+v\n%s", got.Results, out)
	}
	if atomic.LoadInt32(&issuesQueries) != 2 || atomic.LoadInt32(&labelQueries) != 2 {
		t.Fatalf("page counts issues=%d labels=%d, want 2/2", issuesQueries, labelQueries)
	}
	if !got.Meta.Freshness.Refreshed || got.Meta.Freshness.RefreshedBy != "self" || got.Meta.Freshness.LocalIssueCount != 2 {
		t.Fatalf("unexpected multi-page freshness metadata: %+v\n%s", got.Meta.Freshness, out)
	}
}

func TestIssuesSearchReclaimsStaleRefreshLock(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)
	lockPath := dbPath + ".issues-search-sync.lock"
	staleCreatedAt := time.Now().UTC().Add(-(issueSearchRefreshLockTimeout + time.Second)).Format(time.RFC3339)
	if err := os.WriteFile(lockPath, []byte(fmt.Sprintf("pid=%d\ncreated_at=%s\n", os.Getpid(), staleCreatedAt)), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	var issuesQueries int32
	srv := newIssueSearchRefreshServer(t, &issuesQueries, http.StatusOK, 0)
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("issues", "search", "Fresh", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search with stale lock failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				LockContended bool `json:"lock_contended"`
				LockReclaimed bool `json:"lock_reclaimed"`
				Refreshed     bool `json:"refreshed"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("issues search output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-999" {
		t.Fatalf("unexpected refreshed search results after lock reclaim: %+v\n%s", got.Results, out)
	}
	if !got.Meta.Freshness.LockContended || !got.Meta.Freshness.LockReclaimed || !got.Meta.Freshness.Refreshed {
		t.Fatalf("unexpected lock freshness metadata: %+v\n%s", got.Meta.Freshness, out)
	}
	if _, err := os.Stat(lockPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("refresh lock after command: err=%v, want not exist", err)
	}
}

func TestIssuesSearchRefreshFailureReturnsTypedError(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)
	var issuesQueries int32
	srv := newIssueSearchRefreshServer(t, &issuesQueries, http.StatusInternalServerError, 0)
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithRenderedError("issues", "search", "Old", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent")
	if err == nil {
		t.Fatalf("issues search with failed refresh succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 5 {
		t.Fatalf("ExitCode() = %d, want 5; err=%v\n%s", got, err, out)
	}
	var envelope struct {
		Code  int    `json:"code"`
		Type  string `json:"type"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("refresh failure output is not JSON: %v\n%s", err, out)
	}
	if envelope.Code != 5 || envelope.Type != "api" || !strings.Contains(envelope.Error, "--data-source local") {
		t.Fatalf("unexpected refresh failure envelope: %+v\n%s", envelope, out)
	}
}

func TestIssuesSearchDataSourceLocalAllowsStaleCache(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)

	out, err := executeRootForTest("issues", "search", "Old", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent", "--data-source", "local", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search --data-source local failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				StalePolicy   string `json:"stale_policy"`
				Refreshed     bool   `json:"refreshed"`
				RefreshReason string `json:"refresh_reason"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("local stale output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-OLD" {
		t.Fatalf("unexpected stale-local results: %+v\n%s", got.Results, out)
	}
	if got.Meta.Freshness.StalePolicy != "allow" || got.Meta.Freshness.Refreshed || got.Meta.Freshness.RefreshReason != "user_requested_local" {
		t.Fatalf("unexpected stale-local metadata: %+v\n%s", got.Meta.Freshness, out)
	}
}

func TestIssuesSearchMaxAgeZeroDisablesFreshnessGate(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)

	out, err := executeRootForTest("issues", "search", "Old", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent", "--max-age", "0", "--select", "identifier,title")
	if err != nil {
		t.Fatalf("issues search --max-age 0 failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				StalePolicy   string `json:"stale_policy"`
				Refreshed     bool   `json:"refreshed"`
				RefreshReason string `json:"refresh_reason"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("max-age zero output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-OLD" {
		t.Fatalf("unexpected max-age zero results: %+v\n%s", got.Results, out)
	}
	if got.Meta.Freshness.StalePolicy != "allow" || got.Meta.Freshness.Refreshed || got.Meta.Freshness.RefreshReason != "freshness_gate_disabled" {
		t.Fatalf("unexpected max-age zero metadata: %+v\n%s", got.Meta.Freshness, out)
	}
}

func TestIssuesSearchMaxAgeZeroMarksUnsyncedStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	out, err := executeRootForTest("issues", "search", "missing", "duplicate", "--db", dbPath, "--agent", "--max-age", "0")
	if err != nil {
		t.Fatalf("issues search --max-age 0 empty store failed: %v\n%s", err, out)
	}
	var got struct {
		Results []map[string]any `json:"results"`
		Meta    struct {
			Freshness struct {
				RefreshReason   string `json:"refresh_reason"`
				LocalIssueCount int    `json:"local_issue_count"`
				Unsynced        bool   `json:"unsynced"`
			} `json:"freshness"`
		} `json:"meta"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("max-age zero empty-store output is not provenance JSON: %v\n%s", err, out)
	}
	if len(got.Results) != 0 {
		t.Fatalf("empty store returned results: %+v\n%s", got.Results, out)
	}
	if got.Meta.Freshness.RefreshReason != "freshness_gate_disabled" || got.Meta.Freshness.LocalIssueCount != 0 || !got.Meta.Freshness.Unsynced {
		t.Fatalf("unexpected max-age zero empty-store metadata: %+v\n%s", got.Meta.Freshness, out)
	}
}

func TestIssuesSearchConcurrentRefreshCoalesces(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	seedStaleIssueSearchStore(t, dbPath)
	var issuesQueries int32
	srv := newIssueSearchRefreshServer(t, &issuesQueries, http.StatusOK, 150*time.Millisecond)
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	var wg sync.WaitGroup
	errs := make(chan string, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out, err := executeRootForTest("issues", "search", "Fresh", "duplicate", "--team", "SYMPH", "--db", dbPath, "--agent")
			if err != nil {
				errs <- fmt.Sprintf("%v\n%s", err, out)
				return
			}
			var got struct {
				Results []map[string]any `json:"results"`
			}
			if err := json.Unmarshal([]byte(out), &got); err != nil || len(got.Results) != 1 || got.Results[0]["identifier"] != "SYMPH-999" {
				errs <- fmt.Sprintf("bad output: %v\n%s", err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatal(msg)
	}
	if got := atomic.LoadInt32(&issuesQueries); got != 1 {
		t.Fatalf("issues refresh queries = %d, want 1", got)
	}
}

func TestIssueSearchRefreshMetadataMarksExternalSync(t *testing.T) {
	freshness := issueSearchFreshness{
		PreviousSyncedAt: "2026-06-19T14:00:00Z",
		SyncedAt:         "2026-06-19T14:01:00Z",
	}

	applyIssueSearchRefreshMetadata(&freshness, false, false)

	if !freshness.Refreshed || freshness.RefreshedBy != "external" {
		t.Fatalf("unexpected external refresh metadata: %+v", freshness)
	}
	if len(freshness.RefreshResources) != 0 {
		t.Fatalf("external refresh should not claim resources refreshed by this process: %+v", freshness.RefreshResources)
	}
}

func seedStaleIssueSearchStore(t *testing.T, dbPath string) {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertTeam("team-symph", json.RawMessage(`{"id":"team-symph","key":"SYMPH","name":"Symphony"}`)); err != nil {
		t.Fatalf("UpsertTeam: %v", err)
	}
	if err := db.UpsertIssue("issue-old", "SYMPH-OLD", "Old duplicate", json.RawMessage(`{"id":"issue-old","identifier":"SYMPH-OLD","title":"Old duplicate","description":"stale local row","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph"}`)); err != nil {
		t.Fatalf("UpsertIssue: %v", err)
	}
	if err := db.UpdateSyncCursor("issues", "", 1); err != nil {
		t.Fatalf("UpdateSyncCursor: %v", err)
	}
	if _, err := db.DB().Exec(`UPDATE sync_state SET last_synced_at = datetime('now', '-2 hours') WHERE resource_type = 'issues'`); err != nil {
		t.Fatalf("age sync_state: %v", err)
	}
}

func seedFreshIssueSearchStore(t *testing.T, dbPath string) {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.UpsertTeam("team-symph", json.RawMessage(`{"id":"team-symph","key":"SYMPH","name":"Symphony"}`)); err != nil {
		t.Fatalf("UpsertTeam: %v", err)
	}
	if err := db.UpsertIssue("issue-fresh", "SYMPH-FRESH", "Fresh local duplicate", json.RawMessage(`{"id":"issue-fresh","identifier":"SYMPH-FRESH","title":"Fresh local duplicate","description":"fresh local row","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph"}`)); err != nil {
		t.Fatalf("UpsertIssue: %v", err)
	}
	if err := db.UpdateSyncCursor("issues", "", 1); err != nil {
		t.Fatalf("UpdateSyncCursor: %v", err)
	}
}

func newIssueSearchRefreshServer(t *testing.T, issuesQueries *int32, status int, issueDelay time.Duration) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if status != http.StatusOK {
			http.Error(w, "upstream unavailable", status)
			return
		}
		switch {
		case strings.Contains(req.Query, "workflowStates"):
			fmt.Fprint(w, `{"data":{"workflowStates":{"nodes":[]}}}`)
		case strings.Contains(req.Query, "issueLabels"):
			fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
		case strings.Contains(req.Query, "issues("):
			atomic.AddInt32(issuesQueries, 1)
			if issueDelay > 0 {
				time.Sleep(issueDelay)
			}
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"issue-fresh","identifier":"SYMPH-999","title":"Fresh duplicate","description":"fresh remote row","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph","state":{"name":"Backlog","type":"backlog"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
		case strings.Contains(req.Query, "teams"):
			fmt.Fprint(w, `{"data":{"teams":{"nodes":[{"id":"team-symph","key":"SYMPH","name":"Symphony"}]}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
}

func newIssueSearchMultiPageRefreshServer(t *testing.T, issuesQueries *int32, labelQueries *int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "workflowStates"):
			fmt.Fprint(w, `{"data":{"workflowStates":{"nodes":[]}}}`)
		case strings.Contains(req.Query, "issueLabels"):
			atomic.AddInt32(labelQueries, 1)
			after, _ := req.Variables["after"].(string)
			if after == "" {
				fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-1","name":"bug","color":"#111","team":{"id":"team-symph","key":"SYMPH","name":"Symphony"}}],"pageInfo":{"hasNextPage":true,"endCursor":"label-page-2"}}}}`)
				return
			}
			if after != "label-page-2" {
				t.Errorf("issueLabels after cursor = %q, want label-page-2", after)
				http.Error(w, "unexpected label cursor", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-2","name":"duplicate","color":"#222","team":{"id":"team-symph","key":"SYMPH","name":"Symphony"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
		case strings.Contains(req.Query, "issues("):
			atomic.AddInt32(issuesQueries, 1)
			after, _ := req.Variables["after"].(string)
			if after == "" {
				fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"issue-page-1","identifier":"SYMPH-998","title":"Fresh page one","description":"first remote page","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph","state":{"name":"Backlog","type":"backlog"}}],"pageInfo":{"hasNextPage":true,"endCursor":"issue-page-2"}}}}`)
				return
			}
			if after != "issue-page-2" {
				t.Errorf("issues after cursor = %q, want issue-page-2", after)
				http.Error(w, "unexpected issue cursor", http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"issue-fresh","identifier":"SYMPH-999","title":"Fresh duplicate","description":"fresh remote row","team":{"id":"team-symph","key":"SYMPH"},"teamId":"team-symph","state":{"name":"Backlog","type":"backlog"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
		case strings.Contains(req.Query, "teams"):
			fmt.Fprint(w, `{"data":{"teams":{"nodes":[{"id":"team-symph","key":"SYMPH","name":"Symphony"}]}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
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
	var seenAfter string
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
			seenAfter, _ = req.Variables["after"].(string)
			fmt.Fprint(w, `{"data":{"issue":{"id":"issue-uuid","identifier":"MOB-99","title":"Issue","comments":{"nodes":[{"id":"comment-1","body":"full comment body","createdAt":"2026-06-09T00:00:00Z","updatedAt":"2026-06-09T00:00:00Z","user":{"id":"user-1","name":"eric"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("comments", "list", "--issue", "MOB-99", "--after", "cursor-1", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("comments list failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "full comment body") {
		t.Fatalf("agent output stripped comment body: %s", out)
	}
	if seenAfter != "cursor-1" {
		t.Fatalf("comments list after cursor = %q, want cursor-1", seenAfter)
	}
}

func TestDocumentsListSendsAfterCursor(t *testing.T) {
	var seenAfter string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "documents(first") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		seenAfter, _ = req.Variables["after"].(string)
		fmt.Fprint(w, `{"data":{"documents":{"nodes":[{"id":"doc-1","title":"Runbook","slugId":"runbook-f7f48ab36080","url":"https://linear.app/acme/document/runbook-f7f48ab36080"}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("documents", "list", "--after", "cursor-1", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("documents list failed: %v\n%s", err, out)
	}
	if seenAfter != "cursor-1" {
		t.Fatalf("documents list after cursor = %q, want cursor-1", seenAfter)
	}
}

func TestDocumentsListTeamKeyFilter(t *testing.T) {
	var seenFilter map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "documents(first") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		seenFilter, _ = req.Variables["filter"].(map[string]any)
		fmt.Fprint(w, `{"data":{"documents":{"nodes":[{"id":"doc-1","title":"Runbook","slugId":"runbook-f7f48ab36080","url":"https://linear.app/acme/document/runbook-f7f48ab36080","team":{"id":"team-symph","key":"SYMPH","name":"Symphony"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTest("documents", "list", "--team", "SYMPH", "--agent", "--data-source", "live")
	if err != nil {
		t.Fatalf("documents list failed: %v\n%s", err, out)
	}
	teamFilter, _ := seenFilter["team"].(map[string]any)
	keyFilter, _ := teamFilter["key"].(map[string]any)
	if keyFilter == nil || keyFilter["eqIgnoreCase"] != "SYMPH" {
		t.Fatalf("documents list team filter = %#v, want key eqIgnoreCase SYMPH", teamFilter)
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

	out, err = executeRootForTest("labels", "list", "--team", "SYMPH", "--agent", "--data-source", "local", "--db", dbPath, "--select", "name,team.key", "--limit", "2")
	if err != nil {
		t.Fatalf("labels list local with limit failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"pipeline-halt"`) {
		t.Fatalf("local labels applied limit before team filter: %s", out)
	}

	t.Setenv("LINEAR_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("LINEAR_API_KEY", "test-token")
	out, err = executeRootForTest("labels", "list", "--team", "SYMPH", "--agent", "--data-source", "auto", "--db", dbPath, "--select", "name,team.key", "--limit", "2")
	if err != nil {
		t.Fatalf("labels list auto fallback failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"pipeline-halt"`) || !strings.Contains(out, `"api_unreachable"`) {
		t.Fatalf("labels list auto did not fall back to local labels: %s", out)
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
		case strings.Contains(req.Query, "issueLabels(filter"):
			fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-hsui","name":"area:protocols","color":"#333","team":{"id":"team-hsui","key":"HSUI","name":"HS UI"}}]}}}`)
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
			fmt.Fprint(w, `{"data":{"commentCreate":{"success":false,"comment":null}}}`)
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
	if !strings.Contains(err.Error(), assetURL) {
		t.Fatalf("uploaded asset URL was not surfaced; err=%v\n%s", err, out)
	}
	// SilenceErrors moved error printing from cobra to finalizeError; assert
	// the agent-mode envelope still carries the asset URL to the user.
	var envelope bytes.Buffer
	finalizeError(&rootFlags{agent: true, asJSON: true}, nil, &envelope, io.Discard, err)
	if !strings.Contains(envelope.String(), assetURL) {
		t.Fatalf("agent error envelope dropped the asset URL: %s", envelope.String())
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

func TestIssuesCreateDryRunWithParentDoesNotCallAPI(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithStdout("issues", "create",
		"--title", "Child",
		"--team", "MOB",
		"--parent", "MOB-123",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--dry-run",
		"--agent")
	if err != nil {
		t.Fatalf("issues create --parent dry-run failed: %v\n%s", err, out)
	}
	if calls != 0 {
		t.Fatalf("dry-run made %d API calls; output:\n%s", calls, out)
	}
	var preview struct {
		Event string `json:"event"`
		Input struct {
			ParentID string `json:"parentId"`
		} `json:"input"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatalf("dry-run output is not JSON: %v\n%s", err, out)
	}
	if preview.Event != "would_create_issue" || preview.Input.ParentID != "MOB-123" {
		t.Fatalf("dry-run output missing parent preview: %+v\n%s", preview, out)
	}
}

func TestIssuesCreateDryRunWithBadParentValidatesLocally(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithRenderedError("issues", "create",
		"--title", "Child",
		"--team", "MOB",
		"--parent", "bad-format",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--dry-run",
		"--agent")
	if err == nil {
		t.Fatalf("issues create --parent bad-format --dry-run succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(out, `"type":"usage"`) || !strings.Contains(out, "--parent expects an issue identifier") {
		t.Fatalf("bad parent dry-run did not render usage envelope:\n%s", out)
	}
	if calls != 0 {
		t.Fatalf("dry-run made %d API calls; output:\n%s", calls, out)
	}
}

func TestIssuesCreateWithParentResolvesIdentifierBeforeMutation(t *testing.T) {
	const teamID = "00000000-0000-0000-0000-000000000001"
	var sawParentLookup bool
	var seenParentID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		switch {
		case strings.Contains(req.Query, "issues(filter"):
			sawParentLookup = true
			fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"parent-uuid"}]}}}`)
		case strings.Contains(req.Query, "issueCreate"):
			input, _ := req.Variables["input"].(map[string]any)
			seenParentID, _ = input["parentId"].(string)
			fmt.Fprint(w, `{"data":{"issueCreate":{"success":true,"issue":{"id":"child-uuid","identifier":"MOB-124","title":"Child","description":"","url":"https://linear.app/issue/MOB-124","priority":0,"createdAt":"2026-06-18T00:00:00Z","updatedAt":"2026-06-18T00:00:00Z","team":{"id":"00000000-0000-0000-0000-000000000001","key":"MOB"},"state":{"id":"state-1","name":"Todo","type":"unstarted"},"parent":{"id":"parent-uuid","identifier":"MOB-123","title":"Parent"}}}}}`)
		default:
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithStdout("issues", "create",
		"--title", "Child",
		"--team", teamID,
		"--parent", "MOB-123",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--agent",
		"--data-source", "live")
	if err != nil {
		t.Fatalf("issues create --parent failed: %v\n%s", err, out)
	}
	if !sawParentLookup {
		t.Fatalf("parent identifier lookup was not performed")
	}
	if seenParentID != "parent-uuid" {
		t.Fatalf("issueCreate parentId = %q, want parent-uuid", seenParentID)
	}
	var created struct {
		Event    string `json:"event"`
		ParentID string `json:"parentId"`
		Parent   *struct {
			ID         string `json:"id"`
			Identifier string `json:"identifier"`
			Title      string `json:"title"`
		} `json:"parent"`
	}
	if err := json.Unmarshal([]byte(out), &created); err != nil {
		t.Fatalf("issue_created output is not JSON: %v\n%s", err, out)
	}
	if created.Event != "issue_created" || created.ParentID != "parent-uuid" || created.Parent == nil || created.Parent.Identifier != "MOB-123" {
		t.Fatalf("issue_created output missing parent details: %+v\n%s", created, out)
	}
}

func TestIssuesCreateWithParentUUIDSkipsIdentifierLookup(t *testing.T) {
	const teamID = "00000000-0000-0000-0000-000000000001"
	const parentID = "00000000-0000-0000-0000-000000000123"
	var seenParentID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if strings.Contains(req.Query, "issues(filter") {
			t.Errorf("uuid parent should not trigger identifier lookup")
			http.Error(w, "unexpected parent lookup", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueCreate") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		input, _ := req.Variables["input"].(map[string]any)
		seenParentID, _ = input["parentId"].(string)
		fmt.Fprint(w, `{"data":{"issueCreate":{"success":true,"issue":{"id":"child-uuid","identifier":"MOB-124","title":"Child","description":"","url":"https://linear.app/issue/MOB-124","priority":0,"createdAt":"2026-06-18T00:00:00Z","updatedAt":"2026-06-18T00:00:00Z","team":{"id":"00000000-0000-0000-0000-000000000001","key":"MOB"},"state":{"id":"state-1","name":"Todo","type":"unstarted"},"parent":{"id":"00000000-0000-0000-0000-000000000123","identifier":"MOB-123","title":"Parent"}}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithStdout("issues", "create",
		"--title", "Child",
		"--team", teamID,
		"--parent", parentID,
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--agent",
		"--data-source", "live")
	if err != nil {
		t.Fatalf("issues create --parent uuid failed: %v\n%s", err, out)
	}
	if seenParentID != parentID {
		t.Fatalf("issueCreate parentId = %q, want %s", seenParentID, parentID)
	}
}

func TestIssuesEditParentAndNoParent(t *testing.T) {
	t.Run("set parent", func(t *testing.T) {
		var seenIssueID string
		var seenParentID string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req client.GraphQLRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode request: %v", err)
				http.Error(w, "bad request", http.StatusBadRequest)
				return
			}
			switch {
			case strings.Contains(req.Query, "issues(filter"):
				number, _ := req.Variables["number"].(float64)
				switch number {
				case 124:
					fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"child-uuid"}]}}}`)
				case 123:
					fmt.Fprint(w, `{"data":{"issues":{"nodes":[{"id":"parent-uuid"}]}}}`)
				default:
					t.Errorf("unexpected issue lookup number: %v", number)
					http.Error(w, "unexpected issue lookup", http.StatusBadRequest)
				}
			case strings.Contains(req.Query, "issueUpdate"):
				seenIssueID, _ = req.Variables["id"].(string)
				input, _ := req.Variables["input"].(map[string]any)
				seenParentID, _ = input["parentId"].(string)
				fmt.Fprint(w, `{"data":{"issueUpdate":{"success":true,"issue":{"id":"child-uuid","identifier":"MOB-124","title":"Child","description":"","url":"https://linear.app/issue/MOB-124","priority":0,"estimate":0,"dueDate":null,"createdAt":"2026-06-18T00:00:00Z","updatedAt":"2026-06-18T00:00:00Z","state":{"id":"state-1","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"MOB","name":"Mobilyze"},"project":null,"assignee":null,"parent":{"id":"parent-uuid","identifier":"MOB-123","title":"Parent"},"children":{"nodes":[]}}}}}`)
			default:
				t.Errorf("unexpected query: %s", req.Query)
				http.Error(w, "unexpected query", http.StatusBadRequest)
			}
		}))
		t.Cleanup(srv.Close)
		t.Setenv("LINEAR_BASE_URL", srv.URL)
		t.Setenv("LINEAR_API_KEY", "test-token")

		out, err := executeRootForTest("issues", "edit", "MOB-124", "--parent", "MOB-123", "--agent", "--data-source", "live")
		if err != nil {
			t.Fatalf("issues edit --parent failed: %v\n%s", err, out)
		}
		if seenIssueID != "child-uuid" {
			t.Fatalf("issueUpdate id = %q, want child-uuid", seenIssueID)
		}
		if seenParentID != "parent-uuid" {
			t.Fatalf("issueUpdate parentId = %q, want parent-uuid", seenParentID)
		}
	})

	t.Run("clear parent", func(t *testing.T) {
		const childID = "00000000-0000-0000-0000-000000000124"
		parentIDSeen := false
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
			input, _ := req.Variables["input"].(map[string]any)
			value, ok := input["parentId"]
			parentIDSeen = ok && value == nil
			fmt.Fprint(w, `{"data":{"issueUpdate":{"success":true,"issue":{"id":"00000000-0000-0000-0000-000000000124","identifier":"MOB-124","title":"Child","description":"","url":"https://linear.app/issue/MOB-124","priority":0,"estimate":0,"dueDate":null,"createdAt":"2026-06-18T00:00:00Z","updatedAt":"2026-06-18T00:00:00Z","state":{"id":"state-1","name":"Todo","type":"unstarted"},"team":{"id":"team-1","key":"MOB","name":"Mobilyze"},"project":null,"assignee":null,"parent":null,"children":{"nodes":[]}}}}}`)
		}))
		t.Cleanup(srv.Close)
		t.Setenv("LINEAR_BASE_URL", srv.URL)
		t.Setenv("LINEAR_API_KEY", "test-token")

		out, err := executeRootForTest("issues", "edit", childID, "--no-parent", "--agent", "--data-source", "live")
		if err != nil {
			t.Fatalf("issues edit --no-parent failed: %v\n%s", err, out)
		}
		if !parentIDSeen {
			t.Fatalf("issueUpdate did not send parentId:null")
		}
	})
}

func TestIssuesEditDryRunWithParentOptionsDoesNotCallAPI(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithStdout("issues", "edit",
		"MOB-124",
		"--parent", "MOB-123",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--dry-run",
		"--agent")
	if err != nil {
		t.Fatalf("issues edit --parent dry-run failed: %v\n%s", err, out)
	}
	var parentPreview struct {
		Event string `json:"event"`
		Input struct {
			ParentID string `json:"parentId"`
		} `json:"input"`
	}
	if err := json.Unmarshal([]byte(out), &parentPreview); err != nil {
		t.Fatalf("parent dry-run output is not JSON: %v\n%s", err, out)
	}
	if parentPreview.Event != "would_update_issue" || parentPreview.Input.ParentID != "MOB-123" {
		t.Fatalf("parent dry-run output missing parent preview: %+v\n%s", parentPreview, out)
	}

	out, err = executeRootForTestWithStdout("issues", "edit",
		"MOB-124",
		"--no-parent",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--dry-run",
		"--agent")
	if err != nil {
		t.Fatalf("issues edit --no-parent dry-run failed: %v\n%s", err, out)
	}
	var clearPreview struct {
		Event string         `json:"event"`
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal([]byte(out), &clearPreview); err != nil {
		t.Fatalf("clear dry-run output is not JSON: %v\n%s", err, out)
	}
	value, ok := clearPreview.Input["parentId"]
	if clearPreview.Event != "would_update_issue" || !ok || value != nil {
		t.Fatalf("clear dry-run output missing parentId:null preview: %+v\n%s", clearPreview, out)
	}
	if calls != 0 {
		t.Fatalf("dry-runs made %d API calls", calls)
	}
}

func TestIssuesEditDryRunWithBadParentValidatesLocally(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		http.Error(w, "dry-run should not call API", http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err := executeRootForTestWithRenderedError("issues", "edit",
		"MOB-124",
		"--parent", "bad-format",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--dry-run",
		"--agent")
	if err == nil {
		t.Fatalf("issues edit --parent bad-format --dry-run succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(out, `"type":"usage"`) || !strings.Contains(out, "--parent expects an issue identifier") {
		t.Fatalf("bad parent dry-run did not render usage envelope:\n%s", out)
	}
	if calls != 0 {
		t.Fatalf("dry-run made %d API calls; output:\n%s", calls, out)
	}
}

func TestIssuesEditParentFlagsAreMutuallyExclusive(t *testing.T) {
	out, err := executeRootForTestWithRenderedError("issues", "edit",
		"MOB-124",
		"--parent", "MOB-123",
		"--no-parent",
		"--agent",
		"--data-source", "live")
	if err == nil {
		t.Fatalf("issues edit --parent --no-parent succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(out, `"type":"usage"`) || !strings.Contains(out, "pass either --parent or --no-parent") {
		t.Fatalf("mutual exclusion did not render usage envelope:\n%s", out)
	}
}

func TestIssueParentResolutionErrorsAreTyped(t *testing.T) {
	out, err := executeRootForTestWithRenderedError("issues", "create",
		"--title", "Child",
		"--team", "00000000-0000-0000-0000-000000000001",
		"--parent", "not-an-issue-ref",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--agent",
		"--data-source", "live")
	if err == nil {
		t.Fatalf("bad parent reference succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 2 {
		t.Fatalf("ExitCode() = %d, want 2; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(out, `"type":"usage"`) {
		t.Fatalf("bad parent did not render usage envelope:\n%s", out)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issues(filter") {
			t.Errorf("unexpected query: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"data":{"issues":{"nodes":[]}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	out, err = executeRootForTestWithRenderedError("issues", "create",
		"--title", "Child",
		"--team", "00000000-0000-0000-0000-000000000001",
		"--parent", "MOB-404",
		"--db", filepath.Join(t.TempDir(), "linear.db"),
		"--agent",
		"--data-source", "live")
	if err == nil {
		t.Fatalf("missing parent succeeded unexpectedly:\n%s", out)
	}
	if got := ExitCode(err); got != 3 {
		t.Fatalf("ExitCode() = %d, want 3; err=%v\n%s", got, err, out)
	}
	if !strings.Contains(out, `"type":"not_found"`) {
		t.Fatalf("missing parent did not render not_found envelope:\n%s", out)
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
		if !strings.Contains(req.Query, "issueLabels(filter") {
			t.Errorf("unexpected query before media upload: %s", req.Query)
			http.Error(w, "unexpected query", http.StatusBadRequest)
			return
		}
		fmt.Fprint(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-1","name":"area:protocols","color":"#333","team":{"id":"team-hsui","key":"HSUI","name":"HS UI"}}]}}}`)
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

// These helpers temporarily replace process stdout; do not use them in tests
// that call t.Parallel.
func executeRootForTestWithStdout(args ...string) (string, error) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	stdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		return "", err
	}
	os.Stdout = w
	cmdErr := cmd.Execute()
	_ = w.Close()
	os.Stdout = stdout
	rendered, readErr := io.ReadAll(r)
	_ = r.Close()
	if readErr != nil {
		return out.String(), readErr
	}
	return out.String() + string(rendered), cmdErr
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
