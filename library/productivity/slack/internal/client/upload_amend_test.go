// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/config"
)

// slackStub stands in for both the API host and the external upload host. It
// records the byte count it received so tests can assert what was actually sent.
func slackStub(t *testing.T, gotBytes *int) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "files.getUploadURLExternal"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true, "file_id": "F00000000",
				"upload_url": srv.URL + "/upload-target",
			})
		case strings.Contains(r.URL.Path, "upload-target"):
			n, _ := io.Copy(io.Discard, r.Body)
			if gotBytes != nil {
				*gotBytes = int(n)
			}
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
	return srv
}

func stubClient(baseURL string) *Client {
	c := New(&config.Config{BaseURL: baseURL, SlackUserToken: "xoxp-not-real"}, 0, 0)
	c.NoCache = true
	return c
}

// The reservation in step 1 fixes a length, and Slack rejects a body that
// disagrees with it. Opening and statting one descriptor closes the
// path-vs-handle race, but the contents stay live across the step 1 round trip
// and the copy, so the copy has to be bounded and verified.
func TestUploadExternalSendsExactlyReservedLength(t *testing.T) {
	t.Run("a source that grew is a named error, not a truncated upload", func(t *testing.T) {
		var got int
		srv := slackStub(t, &got)
		c := stubClient(srv.URL)

		// Reserve 5 bytes but hand over 50: mimics a file appended to after stat.
		// Capping it at 5 would store a prefix and report success — a silent wrong
		// answer — so this must fail instead.
		_, err := c.UploadExternal(context.Background(), ExternalUpload{
			Filename: "grown.txt",
			Size:     5,
			Reader:   strings.NewReader(strings.Repeat("x", 50)),
		})
		if err == nil {
			t.Fatal("expected an error when the source is longer than reserved, got success")
		}
		if !strings.Contains(err.Error(), "grew while reading") {
			t.Errorf("expected the source-grew error, got: %v", err)
		}
	})

	t.Run("a source that shrank is a named error, not a short upload", func(t *testing.T) {
		srv := slackStub(t, nil)
		c := stubClient(srv.URL)

		// Reserve 100 bytes but hand over 10: mimics truncation after stat.
		_, err := c.UploadExternal(context.Background(), ExternalUpload{
			Filename: "shrunk.txt",
			Size:     100,
			Reader:   strings.NewReader("tiny"),
		})
		if err == nil {
			t.Fatal("expected an error when the source is shorter than reserved")
		}
		if !strings.Contains(err.Error(), "shrank while reading") {
			t.Errorf("expected the source-changed error, got: %v", err)
		}
	})

	t.Run("an exact-size source succeeds", func(t *testing.T) {
		var got int
		srv := slackStub(t, &got)
		c := stubClient(srv.URL)

		body := "hello world"
		res, err := c.UploadExternal(context.Background(), ExternalUpload{
			Filename: "exact.txt",
			Size:     int64(len(body)),
			Reader:   strings.NewReader(body),
		})
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if res.FileID != "F00000000" {
			t.Errorf("expected the reserved file id, got %q", res.FileID)
		}
		if got == 0 {
			t.Error("expected the upload host to receive a body")
		}
	})
}

// Guards on the inputs, so a caller cannot reserve an upload it can't fulfil.
func TestUploadExternalRejectsBadInput(t *testing.T) {
	c := stubClient("https://slack.com/api")
	for _, tc := range []struct {
		name string
		up   ExternalUpload
	}{
		{"no filename", ExternalUpload{Size: 1, Reader: strings.NewReader("x")}},
		{"zero size", ExternalUpload{Filename: "a.txt", Size: 0, Reader: strings.NewReader("x")}},
		{"negative size", ExternalUpload{Filename: "a.txt", Size: -1, Reader: strings.NewReader("x")}},
		{"nil reader", ExternalUpload{Filename: "a.txt", Size: 1}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := c.UploadExternal(context.Background(), tc.up); err == nil {
				t.Error("expected an error")
			}
		})
	}
}
