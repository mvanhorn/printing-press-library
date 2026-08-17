// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.

package sculptok

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/sculptok/internal/config"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	cfg := &config.Config{BaseURL: srv.URL, SculptokApiKey: "testkey"}
	return New(cfg, 5*time.Second, 0)
}

func TestBalanceSuccess(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("apikey") != "testkey" {
			t.Errorf("missing apikey header")
		}
		w.Write([]byte(`{"code":0,"msg":"success","data":{"point":42}}`))
	})
	got, err := c.Balance(context.Background())
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if got != 42 {
		t.Fatalf("Balance = %d; want 42", got)
	}
}

func TestEnvelopeErrorBecomesAPIError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		// HTTP 200 with a non-zero code — the SculptOK failure shape.
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"code":10021,"msg":"apikey is invalid","data":null}`))
	})
	_, err := c.Balance(context.Background())
	if err == nil {
		t.Fatal("expected error for code 10021")
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error type = %T; want *APIError", err)
	}
	if apiErr.Code != 10021 {
		t.Fatalf("APIError.Code = %d; want 10021", apiErr.Code)
	}
}

func TestSubmitReturnsPromptID(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s; want POST", r.Method)
		}
		w.Write([]byte(`{"code":0,"data":{"promptId":"abc-123"}}`))
	})
	id, err := c.Submit(context.Background(), "/draw/prompt", map[string]any{"imageUrl": "u"})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if id != "abc-123" {
		t.Fatalf("promptId = %q; want abc-123", id)
	}
}

func TestGetStatusDone(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("uuid"); got != "p1" {
			t.Errorf("uuid = %q; want p1", got)
		}
		w.Write([]byte(`{"code":0,"data":{"status":2,"currentStep":3,"imgRecords":["a","b","c"]}}`))
	})
	s, err := c.GetStatus(context.Background(), "p1")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if !s.Done() {
		t.Fatal("Done() = false; want true (imgRecords present)")
	}
	if len(s.ImgRecords) != 3 {
		t.Fatalf("imgRecords = %d; want 3", len(s.ImgRecords))
	}
}

func TestStatusNotDone(t *testing.T) {
	s := &Status{Status: 1, ImgRecords: nil}
	if s.Done() {
		t.Fatal("Done() = true for empty imgRecords; want false")
	}
}

func TestUploadMultipart(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type = %q; want multipart/form-data", ct)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			t.Errorf("FormFile: %v", err)
		} else {
			f.Close()
		}
		w.Write([]byte(`{"code":0,"data":{"src":"https://cdn/up.png"}}`))
	})
	tmp := filepath.Join(t.TempDir(), "img.png")
	if err := os.WriteFile(tmp, []byte("fakepng"), 0o644); err != nil {
		t.Fatal(err)
	}
	src, err := c.Upload(context.Background(), tmp)
	if err != nil {
		t.Fatalf("Upload: %v", err)
	}
	if src != "https://cdn/up.png" {
		t.Fatalf("src = %q", src)
	}
}

func TestListPage(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"code":0,"data":{"total":2,"list":[{"id":"a"},{"id":"b"}]}}`))
	})
	items, total, err := c.ListPage(context.Background(), "/image/page", 1, 50)
	if err != nil {
		t.Fatalf("ListPage: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("ListPage = %d items, total %d; want 2/2", len(items), total)
	}
}

func TestHasKey(t *testing.T) {
	c := New(&config.Config{BaseURL: "http://x", SculptokApiKey: ""}, time.Second, 0)
	if c.HasKey() {
		t.Fatal("HasKey() = true with empty key")
	}
}
