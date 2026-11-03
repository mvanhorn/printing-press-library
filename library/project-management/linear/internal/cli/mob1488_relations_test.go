package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

const (
	relIssueA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	relIssueB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
	relIssueC = "cccccccc-cccc-cccc-cccc-cccccccccccc"
)

var relIssues = map[string]string{relIssueA: "MOB-1", relIssueB: "MOB-2", relIssueC: "MOB-3"}

type relationServer struct {
	mu       sync.Mutex
	requests []client.GraphQLRequest
	respond  func(req client.GraphQLRequest) string
}

func newRelationServer(t *testing.T, respond func(req client.GraphQLRequest) string) *relationServer {
	t.Helper()
	rs := &relationServer{respond: respond}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		rs.mu.Lock()
		rs.requests = append(rs.requests, req)
		rs.mu.Unlock()
		fmt.Fprint(w, rs.respond(req))
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")
	return rs
}

func (rs *relationServer) mutations(name string) []client.GraphQLRequest {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	var out []client.GraphQLRequest
	for _, req := range rs.requests {
		if strings.Contains(req.Query, name+"(") {
			out = append(out, req)
		}
	}
	return out
}

func relNode(id, typ, source, target string) string {
	return fmt.Sprintf(`{"id":%q,"type":%q,"createdAt":"2026-11-03T00:00:00Z","issue":{"id":%q,"identifier":%q,"title":"t"},"relatedIssue":{"id":%q,"identifier":%q,"title":"t"}}`,
		id, typ, source, relIssues[source], target, relIssues[target])
}

func relConnection(named string, nodes []string, next string) string {
	return fmt.Sprintf(`{"data":{"issue":{"id":%q,"identifier":%q,"title":"t","connection":{"nodes":[%s],"pageInfo":{"hasNextPage":%t,"endCursor":%q}}}}}`,
		named, relIssues[named], strings.Join(nodes, ","), next != "", next)
}

func relationTestStore(t *testing.T, seed ...store.IssueRelationRecord) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, rec := range seed {
		if err := db.UpsertIssueRelation(rec); err != nil {
			t.Fatal(err)
		}
	}
	return dbPath
}

func storedRelationIDs(t *testing.T, dbPath, issueID string) string {
	t.Helper()
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.ListIssueRelationsForIssue(issueID)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, raw := range rows {
		var rel struct {
			ID string `json:"id"`
		}
		_ = json.Unmarshal(raw, &rel)
		ids = append(ids, rel.ID)
	}
	return strings.Join(ids, ",")
}

type relationOutput struct {
	Results struct {
		Issue     relationIssue      `json:"issue"`
		Relations []issueRelationRow `json:"relations"`
		Removed   []issueRelationRow `json:"removed"`
	} `json:"results"`
}

func decodeRelationOutput(t *testing.T, out string) relationOutput {
	t.Helper()
	var payload relationOutput
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("relation output is not JSON: %v\n%s", err, out)
	}
	return payload
}

func TestIssueRelationsAddSendsDirectionAndWritesBack(t *testing.T) {
	for _, tt := range []struct {
		flag, wantSource, wantTarget, wantType string
	}{
		{flag: "--blocked-by", wantSource: relIssueA, wantTarget: relIssueB, wantType: "blocks"},
		{flag: "--blocks", wantSource: relIssueB, wantTarget: relIssueA, wantType: "blocks"},
		{flag: "--related", wantSource: relIssueB, wantTarget: relIssueA, wantType: "related"},
	} {
		t.Run(tt.flag, func(t *testing.T) {
			rs := newRelationServer(t, func(req client.GraphQLRequest) string {
				input := req.Variables["input"].(map[string]any)
				node := relNode("rel-1", input["type"].(string), input["issueId"].(string), input["relatedIssueId"].(string))
				return `{"data":{"issueRelationCreate":{"success":true,"issueRelation":` + node + `}}}`
			})
			dbPath := relationTestStore(t)
			out, err := executeRootForTest("issues", "relations", "add", relIssueB, tt.flag, relIssueA, "--db", dbPath, "--agent")
			if err != nil {
				t.Fatalf("add failed: %v\n%s", err, out)
			}
			sent := rs.mutations("issueRelationCreate")
			if len(sent) != 1 {
				t.Fatalf("issueRelationCreate calls = %d, want 1", len(sent))
			}
			input := sent[0].Variables["input"].(map[string]any)
			if input["issueId"] != tt.wantSource || input["relatedIssueId"] != tt.wantTarget || input["type"] != tt.wantType {
				t.Fatalf("issueRelationCreate input = %v, want %s %s %s", input, tt.wantSource, tt.wantType, tt.wantTarget)
			}
			got := decodeRelationOutput(t, out).Results
			if got.Issue.ID != relIssueB || len(got.Relations) != 1 || got.Relations[0].Relation != strings.TrimPrefix(tt.flag, "--") || got.Relations[0].Issue.Identifier != "MOB-1" {
				t.Fatalf("add output = %+v, want MOB-2 %s MOB-1", got, tt.flag)
			}
			if ids := storedRelationIDs(t, dbPath, relIssueB); ids != "rel-1" {
				t.Fatalf("stored relations = %q, want rel-1", ids)
			}
		})
	}
}

func TestIssueRelationsTargetFlagCountIsUsageError(t *testing.T) {
	rs := newRelationServer(t, func(client.GraphQLRequest) string { return `{"data":{}}` })
	for _, args := range [][]string{
		{"issues", "relations", "add", relIssueB},
		{"issues", "relations", "add", relIssueB, "--blocked-by", relIssueA, "--related", relIssueC},
		{"issues", "relations", "remove", relIssueB},
	} {
		if _, err := executeRootForTest(append(args, "--agent")...); ExitCode(err) != 2 {
			t.Errorf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}
	if len(rs.requests) != 0 {
		t.Fatalf("usage errors sent %d requests, want 0", len(rs.requests))
	}
}

func TestIssueRelationsListReadsBothConnectionsToTheEnd(t *testing.T) {
	failSecondInversePage := false
	newRelationServer(t, func(req client.GraphQLRequest) string {
		switch {
		case strings.Contains(req.Query, "connection: relations("):
			return relConnection(relIssueB, []string{relNode("rel-out", "blocks", relIssueB, relIssueC)}, "")
		case req.Variables["after"] == nil:
			return relConnection(relIssueB, []string{relNode("rel-in", "blocks", relIssueA, relIssueB)}, "cursor-1")
		case failSecondInversePage:
			return `{"errors":[{"message":"page failed"}]}`
		default:
			return relConnection(relIssueB, []string{relNode("rel-rel", "related", relIssueC, relIssueB)}, "")
		}
	})

	out, err := executeRootForTest("issues", "relations", "list", relIssueB, "--agent")
	if err != nil {
		t.Fatalf("list failed: %v\n%s", err, out)
	}
	got := decodeRelationOutput(t, out).Results
	var rows []string
	for _, row := range got.Relations {
		rows = append(rows, row.ID+" "+row.Relation+" "+row.Issue.Identifier)
	}
	want := "rel-out blocks MOB-3|rel-in blocked-by MOB-1|rel-rel related MOB-3"
	if got.Issue.Identifier != "MOB-2" || strings.Join(rows, "|") != want {
		t.Fatalf("list rows = %q for %s, want %q", strings.Join(rows, "|"), got.Issue.Identifier, want)
	}

	failSecondInversePage = true
	out, err = executeRootForTest("issues", "relations", "list", relIssueB, "--agent")
	if ExitCode(err) != 5 || strings.Contains(out, "rel-in") {
		t.Fatalf("failed page exit = %d (%v), output = %s; want exit 5 and no partial list", ExitCode(err), err, out)
	}
}

func TestIssueRelationsRemove(t *testing.T) {
	inverse := []string{relNode("rel-related", "related", relIssueA, relIssueB), relNode("rel-block", "blocks", relIssueA, relIssueB)}
	serve := func(nodes []string, failDelete string) func(client.GraphQLRequest) string {
		return func(req client.GraphQLRequest) string {
			switch {
			case strings.Contains(req.Query, "issueRelationDelete"):
				if req.Variables["id"] == failDelete {
					return `{"errors":[{"message":"delete failed"}]}`
				}
				return fmt.Sprintf(`{"data":{"issueRelationDelete":{"success":true,"entityId":%q}}}`, req.Variables["id"])
			case strings.Contains(req.Query, "connection: inverseRelations("):
				return relConnection(relIssueB, nodes, "")
			default:
				return relConnection(relIssueB, nil, "")
			}
		}
	}
	seed := []store.IssueRelationRecord{
		{ID: "rel-related", IssueID: relIssueA, RelatedIssueID: relIssueB, Type: "related", Data: json.RawMessage(relNode("rel-related", "related", relIssueA, relIssueB))},
		{ID: "rel-block", IssueID: relIssueA, RelatedIssueID: relIssueB, Type: "blocks", Data: json.RawMessage(relNode("rel-block", "blocks", relIssueA, relIssueB))},
	}

	t.Run("related matches a relation stored in reverse", func(t *testing.T) {
		rs := newRelationServer(t, serve(inverse, ""))
		dbPath := relationTestStore(t, seed...)
		out, err := executeRootForTest("issues", "relations", "remove", relIssueB, "--related", relIssueA, "--db", dbPath, "--agent")
		if err != nil {
			t.Fatalf("remove failed: %v\n%s", err, out)
		}
		deletes := rs.mutations("issueRelationDelete")
		if len(deletes) != 1 || deletes[0].Variables["id"] != "rel-related" {
			t.Fatalf("deletes = %+v, want rel-related only", deletes)
		}
		if removed := decodeRelationOutput(t, out).Results.Removed; len(removed) != 1 || removed[0].Relation != "related" {
			t.Fatalf("removed = %+v", removed)
		}
		if ids := storedRelationIDs(t, dbPath, relIssueB); ids != "rel-block" {
			t.Fatalf("stored relations = %q, want rel-block", ids)
		}
	})

	t.Run("no match is not found unless ignore-missing", func(t *testing.T) {
		rs := newRelationServer(t, serve(inverse, ""))
		if _, err := executeRootForTest("issues", "relations", "remove", relIssueB, "--blocks", relIssueA, "--agent"); ExitCode(err) != 3 {
			t.Fatalf("missing relation exit = %d (%v), want 3", ExitCode(err), err)
		}
		if _, err := executeRootForTestWithStdout("issues", "relations", "remove", relIssueB, "--blocks", relIssueA, "--ignore-missing", "--agent"); err != nil {
			t.Fatalf("--ignore-missing should be a no-op, got %v", err)
		}
		if deletes := rs.mutations("issueRelationDelete"); len(deletes) != 0 {
			t.Fatalf("missing relation sent %d deletes, want 0", len(deletes))
		}
	})

	t.Run("failure after a partial delete reports what was removed", func(t *testing.T) {
		duplicates := []string{relNode("rel-block", "blocks", relIssueA, relIssueB), relNode("rel-block-2", "blocks", relIssueA, relIssueB)}
		newRelationServer(t, serve(duplicates, "rel-block-2"))
		dbPath := relationTestStore(t, seed...)
		out, err := executeRootForTest("issues", "relations", "remove", relIssueB, "--blocked-by", relIssueA, "--db", dbPath, "--agent")
		if ExitCode(err) != 5 || !strings.Contains(err.Error(), "already removed: rel-block") || strings.Contains(out, `"removed"`) {
			t.Fatalf("partial delete exit = %d (%v), output = %s", ExitCode(err), err, out)
		}
		if ids := storedRelationIDs(t, dbPath, relIssueB); ids != "rel-related" {
			t.Fatalf("stored relations = %q, want rel-related", ids)
		}
	})
}

func TestIssueRelationsStrictAndDryRunSendNoMutation(t *testing.T) {
	rs := newRelationServer(t, func(client.GraphQLRequest) string { return `{"data":{}}` })
	dbPath := filepath.Join(t.TempDir(), "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	for id, identifier := range map[string]string{relIssueA: "MOB-1", relIssueB: "MOB-2"} {
		if err := db.RecordPPFixture(id, identifier, "fixture", "mob-1488-test"); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()

	for _, args := range [][]string{
		{"issues", "relations", "add", "MOB-2", "--blocked-by", "MOB-1488", "--db", dbPath},
		{"issues", "relations", "add", "MOB-2", "--blocked-by", "MOB-1488", "--db", dbPath, "--dry-run"},
		{"issues", "relations", "remove", "MOB-1488", "--related", "mob-1", "--db", dbPath, "--dry-run"},
		{"issues", "relations", "add", "MOB-2", "--blocked-by", "MOB-1", "--db", filepath.Join(t.TempDir(), "absent.db"), "--dry-run"},
	} {
		if _, err := executeRootForTest(append(args, "--trust-mode", "strict", "--agent")...); ExitCode(err) != 2 {
			t.Errorf("%v exit = %d (%v), want 2", args, ExitCode(err), err)
		}
	}

	out, err := executeRootForTest("issues", "relations", "add", "mob-2", "--blocked-by", relIssueA, "--db", dbPath, "--trust-mode", "strict", "--dry-run", "--agent")
	if err != nil {
		t.Fatalf("strict dry-run on ledger issues failed: %v\n%s", err, out)
	}
	var dryRun struct {
		Event string         `json:"event"`
		Input map[string]any `json:"input"`
	}
	if err := json.Unmarshal([]byte(out), &dryRun); err != nil || dryRun.Event != "would_create_issue_relation" || dryRun.Input["issueId"] != relIssueA || dryRun.Input["relatedIssueId"] != "mob-2" || dryRun.Input["type"] != "blocks" {
		t.Fatalf("dry-run output = %s (%v)", out, err)
	}
	if len(rs.requests) != 0 {
		t.Fatalf("strict refusals and dry-runs sent %d requests, want 0", len(rs.requests))
	}
}
