// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/config"
)

type slackUploadCapture struct {
	mu        sync.Mutex
	hits      int
	filename  string
	length    string
	partName  string
	partBytes []byte
}

func (c *slackUploadCapture) Hits() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits
}

func (c *slackUploadCapture) note(n int) {
	c.mu.Lock()
	c.hits += n
	c.mu.Unlock()
}

func slackUploadStub(t *testing.T) (*httptest.Server, *slackUploadCapture) {
	t.Helper()
	got := &slackUploadCapture{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.note(1)
		switch {
		case strings.Contains(r.URL.Path, "files.getUploadURLExternal"):
			got.mu.Lock()
			got.filename = r.URL.Query().Get("filename")
			got.length = r.URL.Query().Get("length")
			got.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "file_id": "F00000000",
				"upload_url": srv.URL + "/upload-target",
			})
		case strings.Contains(r.URL.Path, "upload-target"):
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			form, err := multipart.NewReader(bytes.NewReader(body), params["boundary"]).ReadForm(1 << 20)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			defer form.RemoveAll()
			files := form.File["file"]
			if len(files) != 1 {
				http.Error(w, "expected one file part", http.StatusBadRequest)
				return
			}
			fh, err := files[0].Open()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			part, err := io.ReadAll(fh)
			_ = fh.Close()
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			got.mu.Lock()
			got.partName = files[0].Filename
			got.partBytes = part
			got.mu.Unlock()
			fmt.Fprint(w, "OK")
		case strings.Contains(r.URL.Path, "files.completeUploadExternal"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "files": []map[string]any{{"id": "F00000000"}},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func slackUploadClient(baseURL string) *client.Client {
	c := client.New(&config.Config{BaseURL: baseURL, SlackUserToken: "xoxp-not-real"}, 0, 0)
	c.NoCache = true
	return c
}

func TestCodeOrchUploadExternal_RejectsPlainPathWithoutRoot(t *testing.T) {
	t.Setenv(slackMCPUploadRootEnvVar, "")
	secret := []byte("SECRET-NOT-UPLOADED")
	path := filepath.Join(t.TempDir(), "secret.bin")
	if err := os.WriteFile(path, secret, 0o600); err != nil {
		t.Fatal(err)
	}
	srv, got := slackUploadStub(t)
	res, err := codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"file": path,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("plain path was accepted without the upload root")
	}
	text := mcpTextContent(t, res)
	if !strings.Contains(text, slackMCPUploadRootEnvVar) {
		t.Fatalf("error %q does not name %s", text, slackMCPUploadRootEnvVar)
	}
	if got.Hits() != 0 {
		t.Fatalf("rejected path still made %d server requests", got.Hits())
	}

	t.Setenv(slackMCPUploadRootEnvVar, "${SLACK_MCP_UPLOAD_ROOT}")
	res, err = codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"file": path,
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || !res.IsError || !strings.Contains(mcpTextContent(t, res), slackMCPUploadRootEnvVar) {
		t.Fatalf("unresolved placeholder was treated as an upload root: %#v", res)
	}
	if got.Hits() != 0 {
		t.Fatalf("placeholder root still made %d server requests", got.Hits())
	}

	res, err = codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"file":    path,
		"content": "also inline",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || !res.IsError || !strings.Contains(mcpTextContent(t, res), "mutually exclusive") {
		t.Fatalf("file plus content = %#v, want mutual exclusion", res)
	}
	if got.Hits() != 0 {
		t.Fatalf("mutually exclusive inputs still made %d server requests", got.Hits())
	}
}

func TestCodeOrchUploadExternal_UploadsContentBase64(t *testing.T) {
	t.Setenv(slackMCPUploadRootEnvVar, "")
	srv, got := slackUploadStub(t)
	payload := []byte("HELLO")
	res, err := codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"content_base64": base64.StdEncoding.EncodeToString(payload),
		"filename":       "note.txt",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("base64 upload failed: %s", mcpTextContent(t, res))
	}
	got.mu.Lock()
	defer got.mu.Unlock()
	if got.filename != "note.txt" || got.partName != "note.txt" {
		t.Fatalf("filename query=%q part=%q, want note.txt", got.filename, got.partName)
	}
	if got.length != fmt.Sprint(len(payload)) {
		t.Fatalf("reserved length = %q, want %d", got.length, len(payload))
	}
	if !bytes.Equal(got.partBytes, payload) {
		t.Fatalf("uploaded bytes = %q, want %q", got.partBytes, payload)
	}
}

func TestCodeOrchUploadExternal_UploadsInlineContent(t *testing.T) {
	t.Setenv(slackMCPUploadRootEnvVar, "")
	srv, got := slackUploadStub(t)
	res, err := codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"content":  "hello inline",
		"filename": "note.txt",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("inline content upload failed: %s", mcpTextContent(t, res))
	}
	got.mu.Lock()
	defer got.mu.Unlock()
	if got.partName != "note.txt" || string(got.partBytes) != "hello inline" {
		t.Fatalf("uploaded name=%q bytes=%q", got.partName, got.partBytes)
	}
}

func TestCodeOrchUploadExternal_PathInsideRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.bin"), []byte("INSIDE"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(slackMCPUploadRootEnvVar, root)
	srv, got := slackUploadStub(t)
	res, err := codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"file": "ok.bin",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("in-root path rejected: %s", mcpTextContent(t, res))
	}
	got.mu.Lock()
	if got.partName != "ok.bin" || string(got.partBytes) != "INSIDE" {
		t.Fatalf("uploaded name=%q bytes=%q", got.partName, got.partBytes)
	}
	got.mu.Unlock()

	parent := t.TempDir()
	relRoot := filepath.Join(parent, "uploads")
	if err := os.Mkdir(relRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	absFile := filepath.Join(relRoot, "a.bin")
	if err := os.WriteFile(absFile, []byte("ABS"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(parent)
	t.Setenv(slackMCPUploadRootEnvVar, "uploads")
	res, err = codeOrchUploadExternal(context.Background(), slackUploadClient(srv.URL), map[string]any{
		"file":     absFile,
		"filename": "renamed.txt",
	})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("absolute path inside a relative root rejected: %s", mcpTextContent(t, res))
	}
	got.mu.Lock()
	defer got.mu.Unlock()
	if got.partName != "renamed.txt" || string(got.partBytes) != "ABS" {
		t.Fatalf("uploaded name=%q bytes=%q", got.partName, got.partBytes)
	}
}

func TestCodeOrchUploadExternal_RefusesEscapeAndSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.bin"), []byte("INSIDE"), 0o600); err != nil {
		t.Fatal(err)
	}
	outsideDir := t.TempDir()
	outside := filepath.Join(outsideDir, "secret.bin")
	secret := []byte("SECRET-NOT-UPLOADED")
	if err := os.WriteFile(outside, secret, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(slackMCPUploadRootEnvVar, root)
	srv, got := slackUploadStub(t)
	c := slackUploadClient(srv.URL)

	for _, path := range []string{"../escape", outside} {
		res, err := codeOrchUploadExternal(context.Background(), c, map[string]any{"file": path})
		if err != nil {
			t.Fatalf("handler error for %q: %v", path, err)
		}
		if res == nil || !res.IsError {
			t.Fatalf("path %q was accepted", path)
		}
	}
	if got.Hits() != 0 {
		t.Fatalf("escaping paths made %d server requests", got.Hits())
	}

	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}
	res, err := codeOrchUploadExternal(context.Background(), c, map[string]any{"file": "link"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatal("symlink out of the upload root was accepted")
	}
	if got.Hits() != 0 {
		t.Fatalf("symlink upload made %d server requests", got.Hits())
	}
	if bytes.Contains(got.partBytes, secret) {
		t.Fatal("symlink target was uploaded")
	}

	if err := os.Mkdir(filepath.Join(root, "adir"), 0o700); err != nil {
		t.Fatal(err)
	}
	res, err = codeOrchUploadExternal(context.Background(), c, map[string]any{"file": "adir"})
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if res == nil || !res.IsError || !strings.Contains(mcpTextContent(t, res), "not a regular file") {
		t.Fatalf("directory result = %#v", res)
	}
	if got.Hits() != 0 {
		t.Fatalf("directory upload made %d server requests", got.Hits())
	}
}

func TestFilesUploadSummaryDescribesMCPInputs(t *testing.T) {
	var summary string
	for _, ep := range codeOrchEndpoints {
		if ep.ID == "files.upload" {
			summary = ep.Summary
			break
		}
	}
	if summary == "" {
		t.Fatal("files.upload endpoint not registered")
	}
	for _, want := range []string{"content_base64", "SLACK_MCP_UPLOAD_ROOT", "inline text"} {
		if !strings.Contains(summary, want) {
			t.Fatalf("summary %q does not mention %s", summary, want)
		}
	}
	if strings.Contains(summary, "path on the host") {
		t.Fatalf("summary still offers an unrestricted host path: %s", summary)
	}
}
