package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

func TestIssueRelationsRejectStalledCursor(t *testing.T) {
	for _, connection := range []string{"relations", "inverseRelations"} {
		for _, cursor := range []string{"missing", "null", "empty", "unchanged"} {
			t.Run(connection+"/"+cursor, func(t *testing.T) {
				calls := 0
				newRelationServer(t, func(req client.GraphQLRequest) string {
					if !strings.Contains(req.Query, "connection: "+connection+"(") {
						return relConnection(relIssueB, nil, "")
					}
					calls++
					if cursor == "unchanged" && calls == 1 {
						return relConnection(relIssueB, nil, "cursor-1")
					}
					limit := 1
					if cursor == "unchanged" {
						limit = 2
					}
					if calls > limit {
						return `{"errors":[{"message":"unexpected request after stalled cursor"}]}`
					}
					end := ""
					switch cursor {
					case "null":
						end = `,"endCursor":null`
					case "empty":
						end = `,"endCursor":""`
					case "unchanged":
						end = `,"endCursor":"cursor-1"`
					}
					return fmt.Sprintf(`{"data":{"issue":{"id":%q,"connection":{"nodes":[],"pageInfo":{"hasNextPage":true%s}}}}}`, relIssueB, end)
				})
				out, err := executeRootForTest("issues", "relations", "list", relIssueB, "--agent")
				wantCalls := 1
				if cursor == "unchanged" {
					wantCalls = 2
				}
				if err == nil || !strings.Contains(err.Error(), "cursor") || calls != wantCalls || strings.Contains(out, `"relations"`) {
					t.Fatalf("stalled cursor: calls=%d want=%d, error=%v, output=%s", calls, wantCalls, err, out)
				}
			})
		}
	}
}

func TestIssueRelationsUppercaseUUIDCommands(t *testing.T) {
	for _, command := range []string{"add", "remove"} {
		for _, flag := range []string{"--blocks", "--blocked-by", "--related"} {
			t.Run(command+flag, func(t *testing.T) {
				source, target, typ := relIssueB, relIssueA, "blocks"
				if flag == "--blocked-by" {
					source, target = target, source
				}
				if flag == "--related" {
					typ = "related"
				}
				node := relNode("rel-case", typ, source, target)
				rs := newRelationServer(t, func(req client.GraphQLRequest) string {
					switch {
					case strings.Contains(req.Query, "issueRelationCreate"):
						input := req.Variables["input"].(map[string]any)
						if input["issueId"] != source || input["relatedIssueId"] != target {
							t.Errorf("noncanonical UUID mutation input: %v", input)
						}
						return `{"data":{"issueRelationCreate":{"success":true,"issueRelation":` + node + `}}}`
					case strings.Contains(req.Query, "issueRelationDelete"):
						return `{"data":{"issueRelationDelete":{"success":true,"entityId":"rel-case"}}}`
					case strings.Contains(req.Query, "connection: relations("):
						if source == relIssueB {
							return relConnection(relIssueB, []string{node}, "")
						}
					case strings.Contains(req.Query, "connection: inverseRelations("):
						if target == relIssueB {
							return relConnection(relIssueB, []string{node}, "")
						}
					}
					return relConnection(relIssueB, nil, "")
				})
				dbPath := relationTestStore(t)
				db, err := store.Open(dbPath)
				if err != nil {
					t.Fatal(err)
				}
				for id, identifier := range map[string]string{relIssueA: "MOB-1", relIssueB: "MOB-2"} {
					if err := db.RecordPPFixture(id, identifier, "fixture", "mob-1488-review"); err != nil {
						t.Fatal(err)
					}
				}
				_ = db.Close()
				args := []string{"issues", "relations", command, strings.ToUpper(relIssueB), flag, strings.ToUpper(relIssueA), "--db", dbPath, "--agent"}
				for _, strict := range []bool{false, true} {
					runArgs := append([]string{}, args...)
					if strict {
						runArgs = append(runArgs, "--trust-mode", "strict")
					}
					out, err := executeRootForTest(runArgs...)
					if err != nil {
						t.Errorf("strict=%t: %v\n%s", strict, err, out)
						continue
					}
					got := decodeRelationOutput(t, out).Results
					rows := got.Relations
					if command == "remove" {
						rows = got.Removed
					}
					if got.Issue.ID != relIssueB || len(rows) != 1 || rows[0].Relation != strings.TrimPrefix(flag, "--") || rows[0].Issue.ID != relIssueA {
						t.Errorf("strict=%t: output=%+v", strict, got)
					}
				}
				mutation := "issueRelationCreate"
				if command == "remove" {
					mutation = "issueRelationDelete"
				}
				if got := len(rs.mutations(mutation)); got != 2 {
					t.Errorf("mutation calls=%d, want 2", got)
				}
			})
		}
	}
}

func TestIssueRelationsAddReplacesLocalSnapshot(t *testing.T) {
	node := strings.Replace(relNode("rel-new", "related", relIssueA, relIssueB), `"createdAt":"2026-11-03T00:00:00Z"`, `"createdAt":"2026-11-03T00:00:00Z","updatedAt":"2026-11-03T01:00:00Z","archivedAt":null`, 1)
	newRelationServer(t, func(req client.GraphQLRequest) string {
		if strings.Contains(req.Query, "issueRelationCreate") {
			return `{"data":{"issueRelationCreate":{"success":true,"issueRelation":` + node + `}}}`
		}
		if strings.Contains(req.Query, "connection: inverseRelations(") {
			return relConnection(relIssueB, []string{node}, "")
		}
		return relConnection(relIssueB, nil, "")
	})
	dbPath := relationTestStore(t, store.IssueRelationRecord{ID: "rel-old", IssueID: relIssueA, RelatedIssueID: relIssueB, Type: "blocks", Data: json.RawMessage(relNode("rel-old", "blocks", relIssueA, relIssueB))})
	for i := 0; i < 2; i++ {
		out, err := executeRootForTest("issues", "relations", "add", relIssueB, "--related", relIssueA, "--db", dbPath, "--agent")
		if err != nil {
			t.Fatalf("add: %v\n%s", err, out)
		}
		if ids := storedRelationIDs(t, dbPath, relIssueB); ids != "rel-new" {
			t.Errorf("stored IDs=%q, want only rel-new", ids)
		}
	}
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.ListIssueRelationsForIssue(relIssueB)
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range rows {
		var got map[string]any
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if got["id"] == "rel-new" {
			archived, present := got["archivedAt"]
			if got["type"] != "related" || got["updatedAt"] != "2026-11-03T01:00:00Z" || !present || archived != nil {
				t.Errorf("snapshot lost type or timestamp/null metadata: %s", raw)
			}
		}
	}
}

func TestIssueRelationsStrictRejectsReusedIdentifiers(t *testing.T) {
	for _, command := range []string{"add", "remove"} {
		for _, unowned := range []string{"issue", "other"} {
			t.Run(command+"/"+unowned, func(t *testing.T) {
				issueID, otherID := relIssueB, relIssueA
				if unowned == "issue" {
					issueID = relIssueC
				} else {
					otherID = relIssueC
				}
				node := relNode("rel-reused", "related", issueID, otherID)
				rs := newRelationServer(t, func(req client.GraphQLRequest) string {
					switch {
					case strings.Contains(req.Query, "issues(filter:"):
						id := otherID
						if req.Variables["number"] == float64(2) {
							id = issueID
						}
						return fmt.Sprintf(`{"data":{"issues":{"nodes":[{"id":%q}]}}}`, id)
					case strings.Contains(req.Query, "issueRelationCreate"):
						return `{"data":{"issueRelationCreate":{"success":true,"issueRelation":` + node + `}}}`
					case strings.Contains(req.Query, "issueRelationDelete"):
						return `{"data":{"issueRelationDelete":{"success":true,"entityId":"rel-reused"}}}`
					case strings.Contains(req.Query, "connection: relations("):
						return relConnection(issueID, []string{node}, "")
					default:
						return relConnection(issueID, nil, "")
					}
				})
				dbPath := relationTestStore(t)
				db, err := store.Open(dbPath)
				if err != nil {
					t.Fatal(err)
				}
				for id, identifier := range map[string]string{relIssueA: "MOB-1", relIssueB: "MOB-2"} {
					if err := db.RecordPPFixture(id, identifier, "fixture", "mob-1488-review"); err != nil {
						t.Fatal(err)
					}
				}
				_ = db.Close()
				out, err := executeRootForTest("issues", "relations", command, "MOB-2", "--related", "MOB-1", "--db", dbPath, "--trust-mode", "strict", "--agent")
				if ExitCode(err) != 2 || !strings.Contains(err.Error(), relIssueC) {
					t.Errorf("unowned resolved UUID: exit=%d error=%v output=%s, want exit 2 naming %s", ExitCode(err), err, out, relIssueC)
				}
				if creates, deletes := len(rs.mutations("issueRelationCreate")), len(rs.mutations("issueRelationDelete")); creates != 0 || deletes != 0 {
					t.Errorf("sent %d creates and %d deletes, want zero mutations", creates, deletes)
				}
			})
		}
	}
}
