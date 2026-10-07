// Hand-authored tests for multipart.go (PATCH multipart-attachment-upload).

package client

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/project-management/clickup/internal/config"
)

func newTestClient(t *testing.T, baseURL string) *Client {
	t.Helper()
	cfg := &config.Config{BaseURL: baseURL, ClickupAuthorizationToken: "pk_test_token_1234"}
	c := New(cfg, 5*time.Second, 0)
	c.cacheDir = t.TempDir()
	return c
}

func writeTemp(t *testing.T, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStatUploadFile(t *testing.T) {
	p := writeTemp(t, "diagram.png", []byte("\x89PNG\r\n\x1a\n rest"))
	f, err := StatUploadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if f.Name != "diagram.png" || f.ContentType != "image/png" || f.Size != 13 {
		t.Fatalf("unexpected upload file: %+v", f)
	}

	// No known extension: falls back to sniffing the content.
	noext := writeTemp(t, "blob", []byte("%PDF-1.7 hello"))
	f, err = StatUploadFile(noext)
	if err != nil {
		t.Fatal(err)
	}
	if f.ContentType != "application/pdf" {
		t.Fatalf("sniffed content type = %q, want application/pdf", f.ContentType)
	}

	if _, err := StatUploadFile(filepath.Join(t.TempDir(), "missing.txt")); err == nil {
		t.Fatal("expected error for missing file")
	}
	if _, err := StatUploadFile(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected not-a-regular-file error, got %v", err)
	}
}

// A file with a known extension is still opened, so an unreadable file is
// rejected during the checks rather than mid-way through a multi-file upload.
func TestStatUploadFileRejectsUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-000 files")
	}
	p := writeTemp(t, "locked.pdf", []byte("%PDF-1.4"))
	if err := os.Chmod(p, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := StatUploadFile(p); err == nil {
		t.Fatal("expected error for unreadable file")
	}
}

func TestPostMultipartRequestShape(t *testing.T) {
	content := []byte("%PDF-1.4\nbinary\x00\x01\x02 bytes")
	p := writeTemp(t, "report.pdf", content)
	upload, err := StatUploadFile(p)
	if err != nil {
		t.Fatal(err)
	}

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		if r.URL.Path != "/v2/task/PROJ-123/attachment" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("custom_task_ids"); got != "true" {
			t.Errorf("custom_task_ids = %q", got)
		}
		if got := r.URL.Query().Get("team_id"); got != "1234567" {
			t.Errorf("team_id = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "pk_test_token_1234" {
			t.Errorf("Authorization = %q", got)
		}
		if len(r.TransferEncoding) > 0 {
			t.Errorf("TransferEncoding = %v, want an exact Content-Length", r.TransferEncoding)
		}
		raw, _ := io.ReadAll(r.Body)
		if r.ContentLength != int64(len(raw)) {
			t.Errorf("ContentLength = %d, body = %d bytes", r.ContentLength, len(raw))
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		mediaType, mp, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" || mp["boundary"] == "" {
			t.Errorf("Content-Type = %q (%v)", r.Header.Get("Content-Type"), err)
			return
		}
		mr := multipart.NewReader(r.Body, mp["boundary"])
		part, err := mr.NextPart()
		if err != nil {
			t.Errorf("reading part: %v", err)
			return
		}
		if part.FormName() != "attachment" {
			t.Errorf("field name = %q", part.FormName())
		}
		if part.FileName() != "report.pdf" {
			t.Errorf("filename = %q", part.FileName())
		}
		if ct := part.Header.Get("Content-Type"); ct != "application/pdf" {
			t.Errorf("part Content-Type = %q", ct)
		}
		got, _ := io.ReadAll(part)
		if !bytes.Equal(got, content) {
			t.Errorf("file bytes differ: %q", got)
		}
		if _, err := mr.NextPart(); err != io.EOF {
			t.Errorf("expected exactly one part, got err=%v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"abc.pdf","title":"report.pdf","url":"https://example.test/abc.pdf"}`))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	data, status, err := c.PostMultipart("/v2/task/PROJ-123/attachment",
		map[string]string{"custom_task_ids": "true", "team_id": "1234567"}, "attachment", upload, nil)
	if err != nil {
		t.Fatal(err)
	}
	if status != 200 || !strings.Contains(string(data), `"title":"report.pdf"`) {
		t.Fatalf("status=%d data=%s", status, data)
	}
	if calls != 1 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestPostMultipartExtraFormFields(t *testing.T) {
	p := writeTemp(t, "notes.md", []byte("# hi"))
	upload, _ := StatUploadFile(p)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("parse: %v", err)
			return
		}
		if got := r.FormValue("filename"); got != "renamed.md" {
			t.Errorf("filename field = %q", got)
		}
		if fh := r.MultipartForm.File["attachment"]; len(fh) != 1 || fh[0].Filename != "notes.md" {
			t.Errorf("attachment part = %+v", fh)
		}
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	if _, _, err := c.PostMultipart("/v3/workspaces/1/attachments/x/attachments", nil, "attachment", upload, map[string]string{"filename": "renamed.md"}); err != nil {
		t.Fatal(err)
	}
}

func TestPostMultipartNoRetryOn5xx(t *testing.T) {
	p := writeTemp(t, "a.txt", []byte("a"))
	upload, _ := StatUploadFile(p)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	_, status, err := c.PostMultipart("/v2/task/x/attachment", nil, "attachment", upload, nil)
	if err == nil || status != http.StatusBadGateway {
		t.Fatalf("expected 502 error, got status=%d err=%v", status, err)
	}
	if calls != 1 {
		t.Fatalf("5xx must not be retried; calls = %d", calls)
	}
}

func TestPostMultipartRetriesOn429(t *testing.T) {
	p := writeTemp(t, "a.txt", []byte("retry-body"))
	upload, _ := StatUploadFile(p)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Every attempt must carry the whole file, not a drained stream.
		_, mp, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
		part, err := multipart.NewReader(r.Body, mp["boundary"]).NextPart()
		if err != nil {
			t.Errorf("attempt %d: reading part: %v", atomic.LoadInt32(&calls)+1, err)
		} else if got, _ := io.ReadAll(part); string(got) != "retry-body" {
			t.Errorf("attempt %d: file bytes = %q", atomic.LoadInt32(&calls)+1, got)
		}
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Write([]byte(`{"id":"ok"}`))
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	if _, _, err := c.PostMultipart("/v2/task/x/attachment", nil, "attachment", upload, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
}

func TestPostMultipartDryRunSendsNothing(t *testing.T) {
	p := writeTemp(t, "a.txt", []byte("a"))
	upload, _ := StatUploadFile(p)
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL)
	c.DryRun = true
	data, _, err := c.PostMultipart("/v2/task/x/attachment", nil, "attachment", upload, nil)
	if err != nil || !strings.Contains(string(data), "dry_run") {
		t.Fatalf("data=%s err=%v", data, err)
	}
	if calls != 0 {
		t.Fatalf("dry run sent %d requests", calls)
	}
}
