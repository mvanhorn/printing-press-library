package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/tabelog/internal/config"
)

type fixtureBodyReader struct{}

func (fixtureBodyReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// Exercise the HTTP boundary, including Go's transparent decompression and
// failure responses, without retaining a giant source fixture in the repo.
func TestClientNonBinaryResponseSizeLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		encoding string
		status   int
	}{
		{"plain", "", http.StatusOK},
		{"identity", "identity", http.StatusOK},
		{"transparent-gzip", "gzip", http.StatusOK},
		{"error-body", "", http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if tc.encoding != "" {
					w.Header().Set("Content-Encoding", tc.encoding)
				}
				w.WriteHeader(tc.status)
				var out io.Writer = w
				if tc.encoding == "gzip" {
					if r.Header.Get("Accept-Encoding") != "gzip" {
						t.Error("client did not request transparent gzip")
					}
					compressed := gzip.NewWriter(w)
					defer compressed.Close()
					out = compressed
				}
				_, _ = io.CopyN(out, fixtureBodyReader{}, int64(maxDecodedBodyBytes)+1)
			}))
			defer server.Close()
			c := New(&config.Config{BaseURL: server.URL}, 5*time.Second, 1000)
			c.NoCache = true
			_, err := c.Get(context.Background(), "/fixture", nil)
			if !errors.Is(err, ErrDecodedBodyTooLarge) {
				t.Fatalf("oversized %s body was not rejected by the size guard: %v", tc.name, err)
			}
		})
	}
}

func TestClientBinaryResponseRetainsLargePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = io.CopyN(w, fixtureBodyReader{}, int64(maxDecodedBodyBytes)+1)
	}))
	defer server.Close()
	c := New(&config.Config{BaseURL: server.URL}, 5*time.Second, 1000)
	c.NoCache = true
	data, err := c.GetWithHeaders(context.Background(), "/fixture", nil, map[string]string{BinaryResponseHeader: "true"})
	if err != nil {
		t.Fatal(err)
	}
	body, _, ok := UnwrapBinaryResponse(data)
	if !ok || len(body) != maxDecodedBodyBytes+1 || bytes.Count(body, []byte{'x'}) != len(body) {
		t.Fatalf("binary payload was capped or damaged: recognized=%t bytes=%d", ok, len(body))
	}
}
