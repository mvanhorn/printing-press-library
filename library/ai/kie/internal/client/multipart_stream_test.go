// Copyright 2026 Kelvin Cushman and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/kie/internal/config"
)

func TestStreamMultipartBodyUsesPipeAndPreservesPayload(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(filePath, []byte("streamed-file-content"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	reader, contentType, _ := streamMultipartBody(multipartRequestBody{
		Fields:     map[string]string{"model": "example"},
		FileFields: map[string]string{"media": filePath},
	})
	defer reader.Close()
	if _, ok := reader.(*io.PipeReader); !ok {
		t.Fatalf("multipart reader type = %T, want *io.PipeReader", reader)
	}

	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	parts := multipart.NewReader(reader, params["boundary"])

	field, err := parts.NextPart()
	if err != nil {
		t.Fatalf("read field part: %v", err)
	}
	fieldData, err := io.ReadAll(field)
	if err != nil {
		t.Fatalf("read field: %v", err)
	}
	if field.FormName() != "model" || string(fieldData) != "example" {
		t.Fatalf("field = (%q, %q)", field.FormName(), fieldData)
	}

	file, err := parts.NextPart()
	if err != nil {
		t.Fatalf("read file part: %v", err)
	}
	fileData, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if file.FormName() != "media" || file.FileName() != "sample.bin" || string(fileData) != "streamed-file-content" {
		t.Fatalf("file part = field %q filename %q data %q", file.FormName(), file.FileName(), fileData)
	}
}

func TestValidateMultipartBodyRejectsMissingFile(t *testing.T) {
	err := validateMultipartBody(multipartRequestBody{FileFields: map[string]string{"media": filepath.Join(t.TempDir(), "missing.bin")}})
	if err == nil {
		t.Fatal("validateMultipartBody accepted missing file")
	}
}

func TestMultipartUploadStreamsThroughClient(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "sample.bin")
	if err := os.WriteFile(filePath, []byte("streamed-file-content"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength >= 0 {
			http.Error(w, "upload was buffered", http.StatusBadRequest)
			return
		}
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		parts := multipart.NewReader(r.Body, params["boundary"])
		part, err := parts.NextPart()
		if err != nil || part.FormName() != "media" {
			http.Error(w, "missing media part", http.StatusBadRequest)
			return
		}
		data, err := io.ReadAll(part)
		if err != nil || string(data) != "streamed-file-content" {
			http.Error(w, "wrong media content", http.StatusBadRequest)
			return
		}
		if _, err := parts.NextPart(); err != io.EOF {
			http.Error(w, "multipart body did not end cleanly", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
	c.HTTPClient = server.Client()
	c.NoCache = true
	if _, _, err := c.doRead(context.Background(), http.MethodPatch, "/upload", nil,
		multipartRequestBody{FileFields: map[string]string{"media": filePath}}, nil); err != nil {
		t.Fatalf("streamed upload failed: %v", err)
	}
}

func TestMultipartUploadReplaysBodyAfterRedirect(t *testing.T) {
	for _, redirectStatus := range []int{http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		t.Run(http.StatusText(redirectStatus), func(t *testing.T) {
			filePath := filepath.Join(t.TempDir(), "sample.bin")
			if err := os.WriteFile(filePath, []byte("redirected-file-content"), 0o600); err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/redirect" {
					http.Redirect(w, r, "/upload", redirectStatus)
					return
				}
				if r.URL.Path != "/upload" || r.Method != http.MethodPatch {
					http.Error(w, "unexpected upload request", http.StatusBadRequest)
					return
				}
				_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
				if err != nil {
					http.Error(w, "invalid content type", http.StatusBadRequest)
					return
				}
				parts := multipart.NewReader(r.Body, params["boundary"])
				part, err := parts.NextPart()
				if err != nil || part.FormName() != "media" {
					http.Error(w, "missing media part", http.StatusBadRequest)
					return
				}
				data, err := io.ReadAll(part)
				if err != nil || string(data) != "redirected-file-content" {
					http.Error(w, "wrong media content", http.StatusBadRequest)
					return
				}
				if _, err := parts.NextPart(); err != io.EOF {
					http.Error(w, "multipart body did not end cleanly", http.StatusBadRequest)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"arrived":true}`))
			}))
			defer server.Close()

			c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
			c.HTTPClient = server.Client()
			c.NoCache = true
			body, status, err := c.doRead(context.Background(), http.MethodPatch, "/redirect", nil,
				multipartRequestBody{FileFields: map[string]string{"media": filePath}}, nil)
			if err != nil || status != http.StatusOK || !bytes.Contains(body, []byte(`"arrived":true`)) {
				t.Fatalf("redirected upload failed: status %d, error %v, body %s", status, err, body)
			}
		})
	}
}
