// Hand-authored tests for clickup_attachments.go and the multipart-patched
// generated upload commands (PATCH task-attach-and-attachments,
// multipart-attachment-upload).

package cli

import (
	"bytes"
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

type recordedUpload struct {
	Path, Query, Field, Filename, PartType, Auth string
	Body                                         []byte
	Form                                         map[string]string
}

// uploadServer records multipart uploads and answers with a ClickUp-shaped
// attachment object.
func uploadServer(t *testing.T, status int) (*httptest.Server, *[]recordedUpload) {
	t.Helper()
	var mu sync.Mutex
	var got []recordedUpload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := recordedUpload{Path: r.URL.Path, Query: r.URL.RawQuery, Auth: r.Header.Get("Authorization"), Form: map[string]string{}}
		_, mp, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err == nil {
			mr := multipart.NewReader(r.Body, mp["boundary"])
			for {
				part, err := mr.NextPart()
				if err != nil {
					break
				}
				buf := new(bytes.Buffer)
				buf.ReadFrom(part)
				if part.FileName() != "" {
					rec.Field, rec.Filename, rec.PartType, rec.Body = part.FormName(), part.FileName(), part.Header.Get("Content-Type"), buf.Bytes()
				} else {
					rec.Form[part.FormName()] = buf.String()
				}
			}
		}
		mu.Lock()
		got = append(got, rec)
		mu.Unlock()
		w.WriteHeader(status)
		if status < 400 {
			json.NewEncoder(w).Encode(map[string]any{"id": "att-" + rec.Filename, "title": rec.Filename, "url": "https://files.example.test/" + rec.Filename, "extension": strings.TrimPrefix(filepath.Ext(rec.Filename), ".")})
		} else {
			w.Write([]byte(`{"err":"boom"}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func runCLI(t *testing.T, baseURL string, args ...string) (string, error) {
	t.Helper()
	t.Setenv("CLICKUP_CONFIG", filepath.Join(t.TempDir(), "none.toml"))
	t.Setenv("CLICKUP_AUTHORIZATION_TOKEN", "pk_test_token_1234")
	t.Setenv("CLICKUP_BASE_URL", baseURL)
	t.Setenv("HOME", t.TempDir())
	var flags rootFlags
	root := newRootCmd(&flags)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append(args, "--no-cache"))
	err := root.Execute()
	return out.String(), err
}

func tempFile(t *testing.T, name string, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTaskAttachUploadsEachFileAsMultipart(t *testing.T) {
	srv, got := uploadServer(t, 200)
	a := tempFile(t, "notes.md", "# notes")
	b := tempFile(t, "report.pdf", "%PDF-1.4 data")

	out, err := runCLI(t, srv.URL, "task", "attach", "PROJ-123", a, b, "--custom-task-ids", "--team-id", "1234567", "--json")
	if err != nil {
		t.Fatalf("attach: %v\n%s", err, out)
	}
	if len(*got) != 2 {
		t.Fatalf("requests = %d, want 2", len(*got))
	}
	for i, want := range []struct{ name, body, ct string }{{"notes.md", "# notes", "text/markdown"}, {"report.pdf", "%PDF-1.4 data", "application/pdf"}} {
		r := (*got)[i]
		if r.Path != "/v2/task/PROJ-123/attachment" || r.Query != "custom_task_ids=true&team_id=1234567" {
			t.Errorf("req %d path=%s query=%s", i, r.Path, r.Query)
		}
		if r.Field != "attachment" || r.Filename != want.name || string(r.Body) != want.body || !strings.HasPrefix(r.PartType, want.ct) {
			t.Errorf("req %d part = field %q file %q type %q body %q", i, r.Field, r.Filename, r.PartType, r.Body)
		}
		if r.Auth != "pk_test_token_1234" {
			t.Errorf("req %d auth = %q", i, r.Auth)
		}
	}
	var res struct {
		TaskID   string           `json:"task_id"`
		Uploaded []map[string]any `json:"uploaded"`
		Failed   []map[string]any `json:"failed"`
	}
	if err := json.Unmarshal([]byte(out), &res); err != nil {
		t.Fatalf("output not JSON: %v\n%s", err, out)
	}
	if res.TaskID != "PROJ-123" || len(res.Uploaded) != 2 || len(res.Failed) != 0 || res.Uploaded[0]["file"] != a || res.Uploaded[1]["title"] != "report.pdf" {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestTaskAttachDryRunSendsNothing(t *testing.T) {
	srv, got := uploadServer(t, 200)
	a := tempFile(t, "notes.md", "# notes")
	out, err := runCLI(t, srv.URL, "task", "attach", "PROJ-123", a, "--custom-task-ids", "--team-id", "1234567", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if len(*got) != 0 {
		t.Fatalf("dry run sent %d requests", len(*got))
	}
	var preview struct {
		DryRun bool   `json:"dry_run"`
		TaskID string `json:"task_id"`
		URL    string `json:"url"`
		Auth   string `json:"authorization"`
		Files  []struct {
			Name        string `json:"name"`
			ContentType string `json:"content_type"`
			Size        int64  `json:"size"`
		} `json:"files"`
	}
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatalf("dry-run output not JSON: %v\n%s", err, out)
	}
	if !preview.DryRun || preview.TaskID != "PROJ-123" || !strings.HasSuffix(preview.URL, "/v2/task/PROJ-123/attachment?custom_task_ids=true&team_id=1234567") {
		t.Fatalf("preview = %+v", preview)
	}
	if len(preview.Files) != 1 || preview.Files[0].Name != "notes.md" || preview.Files[0].Size != 7 {
		t.Fatalf("preview files = %+v", preview.Files)
	}
	if strings.Contains(preview.Auth, "pk_test") || !strings.HasSuffix(preview.Auth, "1234") {
		t.Fatalf("auth not masked: %q", preview.Auth)
	}
}

func TestTaskAttachMissingFileFailsBeforeAnyRequest(t *testing.T) {
	srv, got := uploadServer(t, 200)
	a := tempFile(t, "notes.md", "# notes")
	_, err := runCLI(t, srv.URL, "task", "attach", "abc123xyz", a, filepath.Join(t.TempDir(), "missing.pdf"), "--json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if len(*got) != 0 {
		t.Fatalf("sent %d requests before validating files", len(*got))
	}
}

func TestTaskAttachReportsFailures(t *testing.T) {
	srv, got := uploadServer(t, 502)
	a := tempFile(t, "notes.md", "# notes")
	out, err := runCLI(t, srv.URL, "task", "attach", "abc123xyz", a, "--json")
	if err == nil {
		t.Fatal("expected non-nil error when an upload fails")
	}
	if len(*got) != 1 {
		t.Fatalf("5xx must not be retried; requests = %d", len(*got))
	}
	if !strings.Contains(out, `"failed"`) || !strings.Contains(out, "notes.md") {
		t.Fatalf("output should list the failure: %s", out)
	}
}

func TestGeneratedUploadCommandsUseMultipart(t *testing.T) {
	srv, got := uploadServer(t, 200)
	a := tempFile(t, "notes.md", "# notes")

	if out, err := runCLI(t, srv.URL, "task", "attachment", "create-task", "PROJ-123", "--file", a, "--custom-task-ids", "--team-id", "1234567", "--json"); err != nil {
		t.Fatalf("create-task: %v\n%s", err, out)
	}
	if out, err := runCLI(t, srv.URL, "workspaces", "attachments", "post-entity", "1234567", "abc123xyz", "--file", a, "--filename", "renamed.md", "--json"); err != nil {
		t.Fatalf("post-entity: %v\n%s", err, out)
	}
	if len(*got) != 2 {
		t.Fatalf("requests = %d", len(*got))
	}
	v2, v3 := (*got)[0], (*got)[1]
	if v2.Path != "/v2/task/PROJ-123/attachment" || v2.Query != "custom_task_ids=true&team_id=1234567" || v2.Field != "attachment" || string(v2.Body) != "# notes" {
		t.Errorf("create-task request = %+v", v2)
	}
	if v3.Path != "/v3/workspaces/1234567/tasks/abc123xyz/attachments" || v3.Field != "attachment" || v3.Form["filename"] != "renamed.md" || string(v3.Body) != "# notes" {
		t.Errorf("post-entity request = %+v", v3)
	}

	if _, err := runCLI(t, srv.URL, "task", "attachment", "create-task", "PROJ-123", "--json"); err == nil {
		t.Error("create-task without --file should fail")
	}
}

// taskFixture mirrors the shape of a real GET /v2/task/{id} response
// (trimmed, with synthetic values). size arrives as a number or a string.
const taskFixture = `{
  "id": "abc123xyz",
  "custom_id": "PROJ-123",
  "attachments": [
    {"id": "a1.md", "title": "decision-register.md", "extension": "md", "size": 4268,
     "total_comments": 0, "resolved_comments": 0, "url": "https://files.example.test/a1.md", "mimetype": "text/markdown"},
    {"id": "f2.pdf", "title": "orientation.pdf", "extension": "pdf", "size": "179637",
     "total_comments": 7, "resolved_comments": 6, "url": "https://files.example.test/f2.pdf", "mimetype": "application/pdf"}
  ]
}`

func TestAttachmentRowsFromTask(t *testing.T) {
	for name, data := range map[string]string{
		"raw":      taskFixture,
		"envelope": `{"meta":{"source":"live"},"results":` + taskFixture + `}`,
	} {
		rows, err := attachmentRowsFromTask(json.RawMessage(data))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(rows) != 2 {
			t.Fatalf("%s: rows = %d", name, len(rows))
		}
		want := attachmentRow{ID: "f2.pdf", Title: "orientation.pdf", Extension: "pdf", Size: 179637, URL: "https://files.example.test/f2.pdf", TotalComments: 7, ResolvedComments: 6, OpenComments: 1}
		if rows[1] != want {
			t.Fatalf("%s: row = %+v, want %+v", name, rows[1], want)
		}
		if rows[0].Size != 4268 || rows[0].OpenComments != 0 {
			t.Fatalf("%s: row0 = %+v", name, rows[0])
		}
	}
}

func TestTaskAttachmentsCommand(t *testing.T) {
	var query string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v2/task/PROJ-123" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		query = r.URL.RawQuery
		w.Write([]byte(taskFixture))
	}))
	defer srv.Close()

	out, err := runCLI(t, srv.URL, "task", "attachments", "PROJ-123", "--custom-task-ids", "--team-id", "1234567", "--agent")
	if err != nil {
		t.Fatalf("attachments: %v", err)
	}
	if query != "custom_task_ids=true&team_id=1234567" {
		t.Errorf("query = %q", query)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("output not JSON array: %v\n%s", err, out)
	}
	// --agent implies --compact; the rows must keep size and comment counts.
	if len(rows) != 2 || rows[1]["open_comments"] != float64(1) || rows[1]["size"] != float64(179637) {
		t.Fatalf("rows = %v", rows)
	}

	out, err = runCLI(t, srv.URL, "task", "attachments", "PROJ-123", "--json", "--select", "title,open_comments")
	if err != nil {
		t.Fatal(err)
	}
	rows = nil
	json.Unmarshal([]byte(out), &rows)
	if len(rows) != 2 || len(rows[0]) != 2 || rows[1]["title"] != "orientation.pdf" {
		t.Fatalf("--select rows = %v", rows)
	}
}

// PATCH(clickup-task-attachments-dry-run-json): --dry-run --json must emit a
// JSON preview on stdout, like the generated read commands, and send nothing.
func TestTaskAttachmentsDryRunJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("dry run sent %s %s", r.Method, r.URL.Path)
	}))
	defer srv.Close()

	out, err := runCLI(t, srv.URL, "task", "attachments", "PROJ-123", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	var preview map[string]any
	if err := json.Unmarshal([]byte(out), &preview); err != nil {
		t.Fatalf("dry-run output not JSON: %v\n%q", err, out)
	}
	if preview["dry_run"] != true || preview["action"] != "task attachments" || preview["method"] != "GET" {
		t.Fatalf("preview = %v", preview)
	}
	if u, _ := preview["url"].(string); !strings.HasSuffix(u, "/v2/task/PROJ-123") {
		t.Fatalf("preview url = %v", preview["url"])
	}
}
