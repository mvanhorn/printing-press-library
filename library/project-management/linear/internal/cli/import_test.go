package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/config"
	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestImportableResources(t *testing.T) {
	if !importableResources["issues"] {
		t.Fatal("expected issues to be importable")
	}
	for _, resource := range []string{"projects", "cycles", "comments", "users", ""} {
		if importableResources[resource] {
			t.Errorf("resource %q should not be importable", resource)
		}
	}
}

func TestUnsupportedImportError(t *testing.T) {
	err := unsupportedImportError("projects")
	if err == nil {
		t.Fatal("expected an error")
	}
	for _, want := range []string{"projects", "GraphQL", "linear-pp-cli projects create"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error message missing %q\n got: %s", want, err)
		}
	}
	if strings.Contains(err.Error(), "400") {
		t.Errorf("error should not surface a raw HTTP 400: %s", err)
	}
}

func TestIssueRecordToInput(t *testing.T) {
	tests := []struct {
		name   string
		record map[string]any
		want   map[string]any
	}{
		{
			name:   "team alias maps to teamId",
			record: map[string]any{"title": "x", "team": "ENG"},
			want:   map[string]any{"title": "x", "teamId": "ENG"},
		},
		{
			name:   "explicit teamId wins over team alias",
			record: map[string]any{"title": "x", "teamId": "uuid-1", "team": "ENG"},
			want:   map[string]any{"title": "x", "teamId": "uuid-1"},
		},
		{
			name:   "full record passes through",
			record: map[string]any{"title": "x", "teamId": "uuid", "description": "d", "priority": float64(2), "labelIds": []any{"l1"}},
			want:   map[string]any{"title": "x", "teamId": "uuid", "description": "d", "priority": float64(2), "labelIds": []any{"l1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := make(map[string]any, len(tt.record))
			for key, value := range tt.record {
				original[key] = value
			}
			got, err := issueRecordToInput(tt.record)
			if err != nil {
				t.Fatalf("issueRecordToInput failed: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("issueRecordToInput mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.record, original) {
				t.Errorf("issueRecordToInput mutated the input record\n before: %#v\n  after: %#v", original, tt.record)
			}
		})
	}
}

func TestIssueRecordToInputRejectsMissingRequiredFields(t *testing.T) {
	tests := []struct {
		name   string
		record map[string]any
		field  string
	}{
		{name: "missing title", record: map[string]any{"team": "ENG"}, field: "title"},
		{name: "empty title", record: map[string]any{"title": " ", "team": "ENG"}, field: "title"},
		{name: "missing team", record: map[string]any{"title": "x"}, field: "team"},
		{name: "empty teamId", record: map[string]any{"title": "x", "teamId": ""}, field: "teamId"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := issueRecordToInput(tt.record)
			if err == nil || !strings.Contains(err.Error(), tt.field) {
				t.Fatalf("expected error naming %q, got %v", tt.field, err)
			}
		})
	}
}

func TestImportIssuesUsesMutationAndContinuesAfterFailures(t *testing.T) {
	var mutations atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/graphql" {
			t.Errorf("path = %q, want /graphql", r.URL.Path)
			return nil, fmt.Errorf("wrong path")
		}
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			return nil, err
		}
		if strings.Contains(req.Query, "teams(filter") {
			body := `{"data":{"teams":{"nodes":[{"id":"11111111-1111-1111-1111-111111111111","key":"ENG","name":"Engineering"}]}}}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(body)),
				Request:    r,
			}, nil
		}
		if !strings.Contains(req.Query, "issueCreate") {
			t.Errorf("query did not use issueCreate: %s", req.Query)
			return nil, fmt.Errorf("wrong query")
		}
		input, _ := req.Variables["input"].(map[string]any)
		if input["teamId"] != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("teamId = %#v, want resolved UUID", input["teamId"])
		}
		title, _ := input["title"].(string)
		call := mutations.Add(1)
		body := ""
		if call == 2 {
			body = `{"errors":[{"message":"record rejected"}]}`
		} else {
			body = fmt.Sprintf(`{"data":{"issueCreate":{"success":true,"issue":{"id":"issue-%d","identifier":"ENG-%d","title":%q,"description":"","url":"https://linear.app/issue/%d","priority":0,"createdAt":"2026-07-25T00:00:00Z","updatedAt":"2026-07-25T00:00:00Z","team":{"id":"11111111-1111-1111-1111-111111111111","key":"ENG"},"state":{"id":"state-1","name":"Todo","type":"unstarted"}}}}}`, call, call, title, call)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})

	dir := t.TempDir()
	input := strings.Join([]string{
		`{"title":"first","team":"ENG"}`,
		`not-json`,
		`{"title":"rejected","teamId":"11111111-1111-1111-1111-111111111111"}`,
		`{"title":"third","teamId":"11111111-1111-1111-1111-111111111111"}`,
		`# ignored`,
	}, "\n")
	dbPath := filepath.Join(dir, "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c := client.New(&config.Config{BaseURL: "https://linear.test"}, 0, 0)
	c.HTTPClient = &http.Client{Transport: transport}

	summary, err := importIssues(strings.NewReader(input), c, db, "import-test", false, false, io.Discard)
	if err != nil {
		t.Fatalf("import issues failed: %v", err)
	}
	if summary.Succeeded != 2 || summary.Failed != 2 || summary.Skipped != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if got := mutations.Load(); got != 3 {
		t.Fatalf("issueCreate calls = %d, want 3", got)
	}

	fixtures, err := db.ListPPFixtures("import-test")
	if err != nil {
		t.Fatal(err)
	}
	if len(fixtures) != 2 {
		t.Fatalf("pp_created fixtures = %d, want 2", len(fixtures))
	}
	if _, err := db.GetByID("issues", "issue-3"); err != nil {
		t.Fatalf("successful issue was not written back to local store: %v", err)
	}
}

func TestImportIssuesDryRunSendsNothing(t *testing.T) {
	dir := t.TempDir()
	inputPath := filepath.Join(dir, "issues.jsonl")
	if err := os.WriteFile(inputPath, []byte("{\"title\":\"preview\",\"team\":\"ENG\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(dir, "linear.db")
	db, err := store.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.UpsertTeam("11111111-1111-1111-1111-111111111111", json.RawMessage(`{"id":"11111111-1111-1111-1111-111111111111","key":"ENG","name":"Engineering"}`)); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	out, err := executeRootForTest("import", "issues", "--input", inputPath, "--db", dbPath, "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry-run failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"succeeded": 1`) || !strings.Contains(out, `"dry_run": true`) {
		t.Fatalf("unexpected dry-run envelope: %s", out)
	}
}

func TestImportIssuesDryRunResolvesTeamAndCountsResolutionFailures(t *testing.T) {
	var mutations atomic.Int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var req client.GraphQLRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return nil, err
		}
		if strings.Contains(req.Query, "issueCreate") {
			mutations.Add(1)
			return nil, fmt.Errorf("dry-run sent a mutation")
		}
		key, _ := req.Variables["key"].(string)
		body := `{"data":{"teams":{"nodes":[]}}}`
		if key == "ENG" {
			body = `{"data":{"teams":{"nodes":[{"id":"11111111-1111-1111-1111-111111111111","key":"ENG","name":"Engineering"}]}}}`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(body)),
			Request:    r,
		}, nil
	})
	c := client.New(&config.Config{BaseURL: "https://linear.test"}, 0, 0)
	c.HTTPClient = &http.Client{Transport: transport}
	input := "{\"title\":\"preview\",\"team\":\"ENG\"}\n{\"title\":\"bad\",\"team\":\"MISSING\"}\n{\"team\":\"ENG\"}\n"
	var stderr strings.Builder

	summary, err := importIssues(strings.NewReader(input), c, nil, "", true, false, &stderr)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if summary.Succeeded != 1 || summary.Failed != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if got := mutations.Load(); got != 0 {
		t.Fatalf("dry-run sent %d issueCreate mutations", got)
	}
	for _, want := range []string{
		`would create issue: title="preview" team="11111111-1111-1111-1111-111111111111"`,
		`team "MISSING" not found`,
		`missing required field "title"`,
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("dry-run output missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestImportUnsupportedResourceDoesNotSendRequest(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "projects.jsonl")
	if err := os.WriteFile(inputPath, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := executeRootForTest("import", "projects", "--input", inputPath)
	if err == nil || !strings.Contains(err.Error(), "linear-pp-cli projects create") {
		t.Fatalf("expected actionable unsupported-resource error, got %v", err)
	}
}
