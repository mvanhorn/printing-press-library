package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
)

func TestIssueReadsExposeLabels(t *testing.T) {
	const (
		issueID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		issue   = `{"id":"` + issueID + `","identifier":"MOB-1447","title":"Labels","priority":2,"estimate":3,"dueDate":"2026-08-30","updatedAt":"2026-08-21T00:00:00Z","url":"https://linear.app/issue/MOB-1447","state":{"id":"state-1","name":"In Progress","type":"started"},"team":{"id":"team-mob","key":"MOB","name":"Mobilyze"},"labels":{"nodes":[{"id":"label-bug","name":"kind:bug","color":"#f00"}]}}`
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "labels { nodes { id name color } }") {
			t.Errorf("issue query omitted labels: %s", req.Query)
		}
		switch {
		case strings.Contains(req.Query, "issue(id:"):
			fmt.Fprintf(w, `{"data":{"issue":%s}}`, issue)
		case strings.Contains(req.Query, "issues(filter:"):
			fmt.Fprintf(w, `{"data":{"issues":{"nodes":[%s]}}}`, issue)
		default:
			fmt.Fprintf(w, `{"data":{"issues":{"nodes":[%s],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`, issue)
		}
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	for _, ref := range []string{"MOB-1447", issueID} {
		out, err := executeRootForTest("issues", ref, "--agent", "--data-source", "live", "--select", "identifier,labels.nodes.id,labels.nodes.name")
		if err != nil {
			t.Fatalf("issues %s failed: %v\n%s", ref, err, out)
		}
		assertIssueLabelOutput(t, out)
	}

	dbPath := issueMultiTestDB(t)
	out, err := executeRootForTest("issues", "list", "--agent", "--data-source", "live", "--state", "all", "--db", dbPath, "--select", "identifier,labels.nodes.id,labels.nodes.name")
	if err != nil {
		t.Fatalf("live issues list failed: %v\n%s", err, out)
	}
	assertIssueListLabelOutput(t, out)

	out, err = executeRootForTest("issues", "list", "--agent", "--data-source", "local", "--state", "all", "--db", dbPath, "--select", "identifier,labels.nodes.id,labels.nodes.name")
	if err != nil {
		t.Fatalf("local issues list failed: %v\n%s", err, out)
	}
	assertIssueListLabelOutput(t, out)

	out, err = executeRootForTest("issues", "list", "--agent", "--data-source", "live", "--state", "all", "--db", dbPath)
	if err != nil {
		t.Fatalf("unselected issues list failed: %v\n%s", err, out)
	}
	want := `[
  {
    "id": "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
    "identifier": "MOB-1447",
    "title": "Labels",
    "priority": 2,
    "estimate": 3,
    "dueDate": "2026-08-30",
    "state": {
      "name": "In Progress",
      "type": "started"
    },
    "team": {
      "id": "team-mob",
      "key": "MOB"
    },
    "labels": {
      "nodes": [
        {
          "id": "label-bug",
          "name": "kind:bug",
          "color": "#f00"
        }
      ]
    },
    "updatedAt": "2026-08-21T00:00:00Z",
    "url": "https://linear.app/issue/MOB-1447"
  }
]
`
	if out != want {
		t.Fatalf("unselected issues list contract changed:\n got: %s\nwant: %s", out, want)
	}
}

func TestIssueCreateVerifiesReturnedLabels(t *testing.T) {
	const teamID = "00000000-0000-0000-0000-000000000001"
	for _, tt := range []struct {
		name     string
		observed string
		wantErr  bool
		dbFails  bool
	}{
		{name: "UUID and exact name in any response order", observed: `[{"id":"label-kind-bug","name":"kind:bug","color":"#f00"},{"id":"label-global","name":"source:user-report","color":"#00f"}]`},
		{name: "success response omits requested UUID", observed: `[{"id":"label-kind-bug","name":"kind:bug","color":"#f00"}]`, wantErr: true},
		{name: "mismatch exposes identity when persistence fails", observed: `[{"id":"label-kind-bug","name":"kind:bug","color":"#f00"}]`, wantErr: true, dbFails: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req client.GraphQLRequest
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				switch {
				case strings.Contains(req.Query, "issueLabels(first"):
					fmt.Fprintf(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-kind-bug","name":"kind:bug","team":{"id":%q,"key":"MOB","name":"Mobilyze"}}],"pageInfo":{"hasNextPage":false,"endCursor":""}}}}`, teamID)
				case strings.Contains(req.Query, "issueLabels(filter"):
					fmt.Fprintf(w, `{"data":{"issueLabels":{"nodes":[{"id":"label-global","name":"source:user-report","color":"#00f","team":null},{"id":"label-kind-bug","name":"kind:bug","color":"#f00","team":{"id":%q,"key":"MOB","name":"Mobilyze"}}]}}}`, teamID)
				case strings.Contains(req.Query, "issueCreate"):
					if !strings.Contains(req.Query, "labels { nodes { id name color } }") {
						t.Errorf("issueCreate response omitted labels: %s", req.Query)
					}
					input, ok := req.Variables["input"].(map[string]any)
					if !ok || fmt.Sprint(input["labelIds"]) != "[label-global label-kind-bug]" {
						t.Fatalf("issueCreate labelIds = %#v, want UUID plus resolved name", input["labelIds"])
					}
					fmt.Fprintf(w, `{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-200","identifier":"MOB-200","title":"Labels","url":"https://linear.app/issue/MOB-200","team":{"id":%q,"key":"MOB"},"state":{"id":"state-1","name":"Todo","type":"unstarted"},"labels":{"nodes":%s}}}}}`, teamID, tt.observed)
				default:
					t.Fatalf("unexpected query: %s", req.Query)
				}
			}))
			t.Cleanup(srv.Close)
			t.Setenv("LINEAR_BASE_URL", srv.URL)
			t.Setenv("LINEAR_API_KEY", "test-token")

			dbPath := filepath.Join(t.TempDir(), "linear.db")
			execute := executeRootForTest
			if tt.dbFails {
				dbPath = filepath.Join(t.TempDir(), "missing", "linear.db")
				execute = executeRootForTestWithRenderedError
			}
			out, err := execute("issues", "create", "--title", "Labels", "--team", teamID, "--label", "label-global", "--label-name", "kind:bug", "--session", "mob-1447-test", "--db", dbPath, "--agent", "--data-source", "live")
			if tt.wantErr {
				if err == nil || ExitCode(err) != 5 || !strings.Contains(err.Error(), "MOB-200 (issue-200)") || !strings.Contains(err.Error(), "label-global") || !strings.Contains(err.Error(), "label-kind-bug") {
					t.Fatalf("label mismatch error = %v (code %d), want created issue and requested/observed IDs; output=%s", err, ExitCode(err), out)
				}
				if tt.dbFails {
					var envelope struct {
						Code         int    `json:"code"`
						Type         string `json:"type"`
						CreatedIssue struct {
							ID         string `json:"id"`
							Identifier string `json:"identifier"`
							URL        string `json:"url"`
						} `json:"created_issue"`
					}
					if decodeErr := json.Unmarshal([]byte(out), &envelope); decodeErr != nil || envelope.Code != 5 || envelope.Type != "api" || envelope.CreatedIssue.ID != "issue-200" || envelope.CreatedIssue.Identifier != "MOB-200" || envelope.CreatedIssue.URL != "https://linear.app/issue/MOB-200" {
						t.Fatalf("mismatch recovery envelope = %+v, decode error = %v, output=%s", envelope, decodeErr, out)
					}
					return
				}
				local, localErr := executeRootForTest("issues", "MOB-200", "--agent", "--data-source", "local", "--db", dbPath, "--select", "identifier,labels.nodes.id")
				if localErr != nil || !strings.Contains(local, "label-kind-bug") {
					t.Fatalf("mismatched issue missing from local write-back: err=%v output=%s", localErr, local)
				}
				fixtures, fixtureErr := executeRootForTestWithStdout("pp-test", "list", "--session", "mob-1447-test", "--db", dbPath, "--agent")
				if fixtureErr != nil || !strings.Contains(fixtures, "MOB-200") {
					t.Fatalf("mismatched issue missing from fixture ledger: err=%v output=%s", fixtureErr, fixtures)
				}
				return
			}
			if err != nil {
				t.Fatalf("issues create with labels failed: %v\n%s", err, out)
			}
			local, err := executeRootForTest("issues", "MOB-200", "--agent", "--data-source", "local", "--db", dbPath, "--select", "identifier,labels.nodes.id")
			if err != nil || !strings.Contains(local, "label-global") || !strings.Contains(local, "label-kind-bug") {
				t.Fatalf("created labels missing from local write-back: err=%v output=%s", err, local)
			}
		})
	}
}

func TestIssueEditWriteBackRetainsLabels(t *testing.T) {
	const issueID = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !strings.Contains(req.Query, "issueUpdate") || !strings.Contains(req.Query, "labels { nodes { id name color } }") {
			t.Fatalf("issueUpdate response omitted labels: %s", req.Query)
		}
		fmt.Fprint(w, `{"data":{"issueUpdate":{"success":true,"issue":{"id":"`+issueID+`","identifier":"MOB-300","title":"Updated","team":{"id":"team-mob","key":"MOB"},"state":{"id":"state-1","name":"Todo","type":"unstarted"},"labels":{"nodes":[{"id":"label-bug","name":"kind:bug","color":"#f00"}]}}}}}`)
	}))
	t.Cleanup(srv.Close)
	t.Setenv("LINEAR_BASE_URL", srv.URL)
	t.Setenv("LINEAR_API_KEY", "test-token")

	dbPath := filepath.Join(t.TempDir(), "linear.db")
	if _, err := executeRootForTest("issues", "edit", issueID, "--title", "Updated", "--db", dbPath, "--agent", "--data-source", "live"); err != nil {
		t.Fatalf("issues edit failed: %v", err)
	}
	local, err := executeRootForTest("issues", "MOB-300", "--agent", "--data-source", "local", "--db", dbPath, "--select", "identifier,labels.nodes.id,labels.nodes.name")
	if err != nil || !strings.Contains(local, "label-bug") || !strings.Contains(local, "kind:bug") {
		t.Fatalf("edited labels missing from local write-back: err=%v output=%s", err, local)
	}
}

func assertIssueLabelOutput(t *testing.T, out string) {
	t.Helper()
	var payload struct {
		Results struct {
			Identifier string `json:"identifier"`
			Labels     struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"nodes"`
			} `json:"labels"`
		} `json:"results"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil || payload.Results.Identifier != "MOB-1447" || len(payload.Results.Labels.Nodes) != 1 || payload.Results.Labels.Nodes[0].ID != "label-bug" || payload.Results.Labels.Nodes[0].Name != "kind:bug" {
		t.Fatalf("issue labels missing from output: err=%v output=%s", err, out)
	}
}

func assertIssueListLabelOutput(t *testing.T, out string) {
	t.Helper()
	var rows []struct {
		Identifier string `json:"identifier"`
		Labels     struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"labels"`
	}
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("issue-list output is not JSON: err=%v output=%s", err, out)
	}
	for _, row := range rows {
		if row.Identifier == "MOB-1447" && len(row.Labels.Nodes) == 1 && row.Labels.Nodes[0].ID == "label-bug" && row.Labels.Nodes[0].Name == "kind:bug" {
			return
		}
	}
	t.Fatalf("issue-list labels missing from output: %s", out)
}
