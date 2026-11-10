package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/platform"
)

func TestPostMultipartRequestAndRetry(t *testing.T) {
	for _, tt := range []struct {
		name     string
		filename string
		retry    bool
	}{
		{"extension", `image "quoted".PNG`, false},
		{"sniff and retry", "image", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			contents := []byte("\x89PNG\r\n\x1a\n\x00\xffmultipart file bytes\x00")
			filename := filepath.Join(t.TempDir(), tt.filename)
			if err := os.WriteFile(filename, contents, 0o600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(filename)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			fields := map[string]string{"name": "Test image", "hidden": "false"}
			var calls atomic.Int32
			var firstBody []byte
			var firstContentType string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempt := calls.Add(1)
				if r.Method != http.MethodPost || r.URL.RequestURI() != "/api/image-upload" {
					t.Errorf("request = %s %s, want POST /api/image-upload", r.Method, r.URL.RequestURI())
				}
				if got := r.Header.Get("Authorization"); got != "Klaviyo-API-Key test-key" {
					t.Errorf("Authorization = %q", got)
				}
				if got := r.Header.Get("revision"); got != "2026-10-15" {
					t.Errorf("revision = %q", got)
				}
				if got := r.Header.Get("User-Agent"); got != "klaviyo-pp-cli/2026-10-15" {
					t.Errorf("User-Agent = %q", got)
				}
				if tt.retry && r.Header.Get("Idempotency-Key") != "test-upload" {
					t.Error("retry lost Idempotency-Key")
				}
				contentType := r.Header.Get("Content-Type")
				mediaType, params, err := mime.ParseMediaType(contentType)
				if err != nil || mediaType != "multipart/form-data" || params["boundary"] == "" {
					t.Errorf("Content-Type = %q, error = %v", contentType, err)
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
				}
				if attempt == 1 {
					firstBody, firstContentType = body, contentType
				} else if !bytes.Equal(body, firstBody) || contentType != firstContentType {
					t.Error("retry did not resend the entire identical multipart body and boundary")
				}
				r.Body = io.NopCloser(bytes.NewReader(body))
				if err := r.ParseMultipartForm(1 << 20); err != nil {
					t.Errorf("parse multipart: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				defer r.MultipartForm.RemoveAll()
				if want := map[string][]string{"name": {fields["name"]}, "hidden": {"false"}}; !reflect.DeepEqual(r.MultipartForm.Value, want) {
					t.Errorf("fields = %#v, want %#v", r.MultipartForm.Value, want)
				}
				if len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
					t.Errorf("file fields = %#v, want one file", r.MultipartForm.File)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				upload, header, err := r.FormFile("file")
				if err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				defer upload.Close()
				got, err := io.ReadAll(upload)
				if err != nil || !bytes.Equal(got, contents) {
					t.Errorf("file bytes = %q, error = %v", got, err)
				}
				if header.Filename != tt.filename || header.Header.Get("Content-Type") != "image/png" {
					t.Errorf("file metadata = %#v", header)
				}
				if tt.retry && attempt == 1 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusCreated)
				fmt.Fprint(w, `{"data":{"id":"image-1"}}`)
			}))
			defer server.Close()
			c := New(&config.Config{BaseURL: server.URL, KlaviyoApiKey: "test-key"}, 5*time.Second, 0)
			c.cacheDir = ""
			headers := map[string]string{"content-type": "must be replaced"}
			if tt.retry {
				// Preserve the existing POST safety rule: replay requires a key and a budget.
				headers["Idempotency-Key"] = "test-upload"
				c.platformBudgets = map[string]platform.EndpointBudget{
					"*": {Class: "*", MaxAttempts: 2, RetryBudget: time.Second},
				}
			}
			data, status, err := c.PostMultipartWithHeaders(context.Background(), "/api/image-upload", fields, map[string]string{"file": file.Name()}, headers)
			if err != nil || status != http.StatusCreated || string(data) != `{"data":{"id":"image-1"}}` {
				t.Fatalf("PostMultipart = %s, %d, %v", data, status, err)
			}
			wantCalls := int32(1)
			if tt.retry {
				wantCalls = 2
			}
			if calls.Load() != wantCalls {
				t.Errorf("calls = %d, want %d", calls.Load(), wantCalls)
			}
			if headers["content-type"] != "must be replaced" {
				t.Error("caller headers were mutated")
			}
		})
	}
}

func TestPostMultipartPreservesAPIErrorAndUnsafeWritePolicy(t *testing.T) {
	for _, status := range []int{http.StatusBadRequest, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			file, err := os.CreateTemp(t.TempDir(), "*.png")
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(status)
				fmt.Fprint(w, `{"errors":[{"detail":"rejected"}]}`)
			}))
			defer server.Close()
			c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.platformBudgets = map[string]platform.EndpointBudget{"*": {Class: "*", MaxAttempts: 2, RetryBudget: time.Second}}
			_, gotStatus, err := c.PostMultipartWithHeaders(context.Background(), "/api/image-upload", nil, map[string]string{"file": file.Name()}, nil)
			var apiErr *APIError
			want := fmt.Sprintf(`POST /api/image-upload returned HTTP %d: {"errors":[{"detail":"rejected"}]}`, status)
			if !errors.As(err, &apiErr) || apiErr.Error() != want || gotStatus != status {
				t.Fatalf("status = %d, error = %v, want APIError %q", gotStatus, err, want)
			}
			if calls.Load() != 1 {
				t.Errorf("unsafe POST calls = %d, want 1", calls.Load())
			}
		})
	}
}

func TestPostMultipartDryRunDoesNotReadFile(t *testing.T) {
	// A write-only file with no extension makes both sniffing and copying fail if attempted.
	file, err := os.OpenFile(filepath.Join(t.TempDir(), "image"), os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	const contents = "file contents must not appear in the preview"
	if _, err := file.WriteString(contents); err != nil {
		t.Fatal(err)
	}
	const baseURL = "https://example.invalid"
	c := New(&config.Config{BaseURL: baseURL}, time.Second, 0)
	c.DryRun = true
	c.HTTPClient = nil // Any attempt to send a request would panic.
	stderr, err := os.CreateTemp(t.TempDir(), "preview")
	if err != nil {
		t.Fatal(err)
	}
	defer stderr.Close()
	previousStderr := os.Stderr
	os.Stderr = stderr
	defer func() { os.Stderr = previousStderr }()
	data, status, err := c.PostMultipartWithHeaders(context.Background(), "/api/image-upload", map[string]string{"name": "Preview", "hidden": "false"}, map[string]string{"file": file.Name()}, nil)
	if err != nil || status != 0 || string(data) != `{"dry_run": true}` {
		t.Fatalf("dry run = %s, %d, %v", data, status, err)
	}
	preview, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"POST " + baseURL + "/api/image-upload", "Content-Type: multipart/form-data",
		fmt.Sprintf(`"path": %q`, file.Name()), `"filename": "image"`, fmt.Sprintf(`"size": %d`, len(contents)),
		`"name": "Preview"`, `"hidden": "false"`, "(dry run - no request sent)",
	} {
		if !strings.Contains(string(preview), want) {
			t.Errorf("preview %q missing %q", preview, want)
		}
	}
	if strings.Contains(string(preview), contents) {
		t.Error("dry run exposed file contents")
	}
}
