// Copyright 2026 Peter Yang and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/internal/client"
	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/substack-notes/internal/config"
)

func TestAttachNoteImagesUploadsAttachesThenPublishes(t *testing.T) {
	t.Parallel()
	imagePath := writeTinyPNG(t)
	var order []string
	var publishBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/image":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode image upload body: %v", err)
			}
			if !strings.HasPrefix(body["image"], "data:image/png;base64,") {
				t.Fatalf("image upload body = %q, want data URL", body["image"])
			}
			_, _ = w.Write([]byte(`{"url":"https://example.com/image.png","width":1,"height":1}`))
		case "/api/v1/comment/attachment":
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode attachment body: %v", err)
			}
			if body["type"] != "image" || body["url"] != "https://example.com/image.png" {
				t.Fatalf("attachment body = %#v", body)
			}
			_, _ = w.Write([]byte(`{"id":"attachment-123"}`))
		case "/api/v1/comment/feed":
			if err := json.NewDecoder(r.Body).Decode(&publishBody); err != nil {
				t.Fatalf("decode publish body: %v", err)
			}
			_, _ = w.Write([]byte(`{"id":1,"status":"published"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	c := client.New(&config.Config{BaseURL: server.URL, SubstackNotesCookieAuth: "session_cookie=test"}, 5*time.Second, 0)
	c.HTTPClient = server.Client()
	body := map[string]any{"bodyJson": proseMirrorDoc("hello"), "replyMinimumRole": "everyone"}
	body, err := attachNoteImages(context.Background(), c, body, []string{imagePath})
	if err != nil {
		t.Fatalf("attachNoteImages() error = %v", err)
	}
	if _, _, err := c.PostWithParams(context.Background(), "/api/v1/comment/feed", map[string]string{}, body); err != nil {
		t.Fatalf("publish error = %v", err)
	}
	if got := strings.Join(order, ","); got != "/api/v1/image,/api/v1/comment/attachment,/api/v1/comment/feed" {
		t.Fatalf("request order = %s", got)
	}
	gotIDs, ok := publishBody["attachmentIds"].([]any)
	if !ok || len(gotIDs) != 1 || gotIDs[0] != "attachment-123" {
		t.Fatalf("publish attachmentIds = %#v", publishBody["attachmentIds"])
	}
}

func TestAttachNoteImagesStopsBeforePublishWhenAttachmentFails(t *testing.T) {
	t.Parallel()
	imagePath := writeTinyPNG(t)
	var order []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, r.URL.Path)
		switch r.URL.Path {
		case "/api/v1/image":
			_, _ = w.Write([]byte(`{"url":"https://example.com/image.png"}`))
		case "/api/v1/comment/attachment":
			http.Error(w, `{"error":"bad attachment"}`, http.StatusBadRequest)
		case "/api/v1/comment/feed":
			t.Fatalf("publish endpoint should not be called after attachment failure")
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	c := client.New(&config.Config{BaseURL: server.URL, SubstackNotesCookieAuth: "session_cookie=test"}, 5*time.Second, 0)
	c.HTTPClient = server.Client()
	_, err := attachNoteImages(context.Background(), c, map[string]any{"bodyJson": proseMirrorDoc("hello")}, []string{imagePath})
	if err == nil {
		t.Fatalf("attachNoteImages() error = nil, want attachment failure")
	}
	if got := strings.Join(order, ","); got != "/api/v1/image,/api/v1/comment/attachment" {
		t.Fatalf("request order = %s", got)
	}
}

func TestImageDataURLRejectsUnsupportedTypeBeforeNetwork(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := imageDataURL(path); err == nil {
		t.Fatalf("imageDataURL() error = nil, want unsupported type")
	}
}

func TestAttachNoteImagesRejectsDryRun(t *testing.T) {
	t.Parallel()
	c := client.New(&config.Config{BaseURL: "https://example.com", SubstackNotesCookieAuth: "session_cookie=test"}, 5*time.Second, 0)
	c.DryRun = true
	_, err := attachNoteImages(context.Background(), c, map[string]any{"bodyJson": proseMirrorDoc("hello")}, []string{"image.png"})
	if err == nil || !strings.Contains(err.Error(), "--dry-run with --image") {
		t.Fatalf("attachNoteImages dry-run error = %v", err)
	}
}

func TestExtractRecentNotesFiltersCardsAndIncludesAttachments(t *testing.T) {
	t.Parallel()
	input := json.RawMessage(`{
	  "items": [
	    {"type":"latest_post","body":"ignore me"},
	    {"comment":{"id":101,"date":"2026-06-06T12:00:00Z","body":"<p>Hello &amp; welcome</p>","canonical_url":"https://example.com/n/one","attachments":[{"id":"att-1","type":"image","url":"https://example.com/img.png","width":10,"height":20}]}},
	    {"comment":{"id":102,"body":""}},
	    {"comment":{"id":103,"text":"Second note"}}
	  ]
	}`)
	notes, err := extractRecentNotes(input, 5)
	if err != nil {
		t.Fatalf("extractRecentNotes() error = %v", err)
	}
	if len(notes) != 2 {
		t.Fatalf("len(notes) = %d, want 2", len(notes))
	}
	if notes[0].Body != "Hello & welcome" {
		t.Fatalf("first body = %q", notes[0].Body)
	}
	if len(notes[0].Attachments) != 1 || notes[0].Attachments[0].ID != "att-1" {
		t.Fatalf("attachments = %#v", notes[0].Attachments)
	}
}

func writeTinyPNG(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tiny.png")
	data := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
