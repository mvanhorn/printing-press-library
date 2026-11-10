// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"bytes"
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/klaviyo/internal/config"
)

type klaviyoRoundTripper func(*http.Request) (*http.Response, error)

func (f klaviyoRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestKlaviyoTransportWriteRecovery(t *testing.T) {
	for _, code := range []int{500, 429} {
		t.Run(http.StatusText(code), func(t *testing.T) {
			calls := 0
			c := New(&config.Config{BaseURL: "https://example.test", KlaviyoApiKey: "synthetic"}, time.Second, 0)
			c.cacheDir = ""
			c.HTTPClient.Transport = klaviyoRoundTripper(func(r *http.Request) (*http.Response, error) {
				calls++
				body, err := io.ReadAll(r.Body)
				if err != nil || string(body) != `{"data":{"type":"campaign"}}` {
					t.Fatalf("body=%s err=%v", body, err)
				}
				if r.Header.Get("revision") != "2026-10-15" {
					t.Fatalf("revision=%q", r.Header.Get("revision"))
				}
				status := code
				if calls > 1 {
					status = 201
				}
				return &http.Response{StatusCode: status, Header: http.Header{"Retry-After": {"0"}}, Body: io.NopCloser(strings.NewReader(`{"data":{"id":"one"}}`)), Request: r}, nil
			})
			_, _, err := c.Post(context.Background(), "/api/campaigns", map[string]any{"data": map[string]any{"type": "campaign"}})
			if code == 500 && (err == nil || calls != 1) {
				t.Fatalf("ambiguous write: calls=%d err=%v", calls, err)
			}
			if code == 429 && (err != nil || calls != 2) {
				t.Fatalf("bounded recovery: calls=%d err=%v", calls, err)
			}
		})
	}
}

func TestKlaviyoTransportMultipartPreservesFileAndHeaders(t *testing.T) {
	contents := []byte("\x89PNG\r\n\x1a\nimage")
	filename := filepath.Join(t.TempDir(), "image")
	if err := os.WriteFile(filename, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	c := New(&config.Config{BaseURL: "https://example.test"}, time.Second, 0)
	c.cacheDir = ""
	c.HTTPClient.Transport = klaviyoRoundTripper(func(r *http.Request) (*http.Response, error) {
		_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil {
			t.Fatal(err)
		}
		reader := multipart.NewReader(r.Body, params["boundary"])
		found := false
		for {
			part, err := reader.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			data, err := io.ReadAll(part)
			if err != nil {
				t.Fatal(err)
			}
			if part.FormName() == "file" {
				found = true
				if part.Header.Get("Content-Type") != "image/png" || !bytes.Equal(data, contents) {
					t.Fatalf("file type=%q bytes=%q", part.Header.Get("Content-Type"), data)
				}
			}
		}
		if !found || r.Header.Get("revision") != "2026-10-15" {
			t.Fatal("missing file or API revision")
		}
		return &http.Response{StatusCode: 201, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"data":{}}`)), Request: r}, nil
	})
	headers := map[string]string{"content-type": "replace-me"}
	if _, _, err := c.PostMultipartWithHeaders(context.Background(), "/api/image-upload", nil, map[string]string{"file": filename}, headers); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(headers, map[string]string{"content-type": "replace-me"}) {
		t.Fatal("caller headers mutated")
	}
}
