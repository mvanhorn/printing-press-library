// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/nap-camp/internal/config"
)

type boundedTestBody struct {
	remaining, read int
	closed          bool
}

func (b *boundedTestBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if n > b.remaining {
		n = b.remaining
	}
	for i := 0; i < n; i++ {
		p[i] = ' '
	}
	b.remaining -= n
	b.read += n
	return n, nil
}
func (b *boundedTestBody) Close() error { b.closed = true; return nil }

type boundedTestTransport func(*http.Request) (*http.Response, error)

func (f boundedTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestReadResponseBodyBoundary(t *testing.T) {
	for _, size := range []int{0, 128, maxResponseBodyBytes, maxResponseBodyBytes + 1, maxResponseBodyBytes * 8} {
		source := &boundedTestBody{remaining: size}
		got, err := readResponseBody(source)
		if size > maxResponseBodyBytes {
			if !errors.Is(err, ErrResponseBodyTooLarge) || got != nil {
				t.Fatalf("size %d returned partial success: len=%d err=%v", size, len(got), err)
			}
			if source.read != maxResponseBodyBytes+1 {
				t.Fatalf("read %d bytes, want cap+1", source.read)
			}
		} else if err != nil || len(got) != size {
			t.Fatalf("size %d: len=%d err=%v", size, len(got), err)
		}
	}
}

func TestOversizedRawSourceResponseFailsAndClosesBody(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusTooManyRequests, http.StatusServiceUnavailable} {
		for _, announcedLength := range []int64{-1, 1, maxResponseBodyBytes * 8} {
			source := &boundedTestBody{remaining: maxResponseBodyBytes * 8}
			calls := 0
			c := New(&config.Config{BaseURL: "https://api.example.test"}, time.Second, 0)
			c.HTTPClient = &http.Client{Transport: boundedTestTransport(func(req *http.Request) (*http.Response, error) {
				calls++
				return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": {"application/json"}}, Body: source, ContentLength: announcedLength, Request: req}, nil
			})}
			got, err := c.GetNoCache(context.Background(), "/api/master", nil)
			if !errors.Is(err, ErrResponseBodyTooLarge) || got != nil || !source.closed || calls != 1 {
				t.Fatalf("status %d length %d: got=%d err=%v closed=%v requests=%d", status, announcedLength, len(got), err, source.closed, calls)
			}
			if source.read > maxResponseBodyBytes+1 || !strings.Contains(err.Error(), "Nap Camp GET /api/master") {
				t.Fatalf("unbounded read or missing provider context: read=%d err=%v", source.read, err)
			}
		}
	}
}

func TestExplicitGzipInflationUsesTransportBound(t *testing.T) {
	var zipped bytes.Buffer
	zw := gzip.NewWriter(&zipped)
	if _, err := zw.Write(bytes.Repeat([]byte(" "), maxResponseBodyBytes+1)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	source := io.NopCloser(bytes.NewReader(zipped.Bytes()))
	c := New(&config.Config{BaseURL: "https://api.example.test"}, time.Second, 0)
	c.HTTPClient = &http.Client{Transport: boundedTestTransport(func(req *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}, "Content-Encoding": {"gzip"}}, Body: source, Request: req}, nil
	})}
	got, err := c.GetNoCache(context.Background(), "/api/master", nil)
	if !errors.Is(err, ErrDecodedBodyTooLarge) || got != nil || !strings.Contains(err.Error(), "Nap Camp GET /api/master") {
		t.Fatalf("inflated response returned data or lost context: got=%d err=%v", len(got), err)
	}
	for _, size := range []int{maxResponseBodyBytes, maxResponseBodyBytes + 1} {
		var buf bytes.Buffer
		writer := gzip.NewWriter(&buf)
		if _, err := writer.Write(bytes.Repeat([]byte(" "), size)); err != nil {
			t.Fatal(err)
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		decoded, err := decodeContentEncoding("gzip", buf.Bytes())
		if size == maxResponseBodyBytes && (err != nil || len(decoded) != size) {
			t.Fatalf("exact inflated cap failed: %v", err)
		}
		if size > maxResponseBodyBytes && (!errors.Is(err, ErrDecodedBodyTooLarge) || decoded != nil) {
			t.Fatalf("inflation cap accepted partial output: %v", err)
		}
	}
}

func TestExactResponseLimitIsCompleteValidJSON(t *testing.T) {
	payload := []byte("[]" + strings.Repeat(" ", maxResponseBodyBytes-2))
	for _, compressed := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			if compressed {
				w.Header().Set("Content-Encoding", "gzip")
				writer := gzip.NewWriter(w)
				_, _ = writer.Write(payload)
				_ = writer.Close()
				return
			}
			_, _ = w.Write(payload)
		}))
		c := New(&config.Config{BaseURL: server.URL}, time.Second, 0)
		got, err := c.GetNoCache(context.Background(), "/api/master", nil)
		server.Close()
		if err != nil || !bytes.Equal(got, payload) {
			t.Fatalf("exact-limit body compressed=%v: got=%d err=%v", compressed, len(got), err)
		}
	}
}

func TestCompressionVariantsKeepExactLimitAndRejectOverflow(t *testing.T) {
	for _, codec := range []string{"gzip", "x-gzip", "zlib", "raw-deflate", "gzip,deflate"} {
		for _, size := range []int{maxResponseBodyBytes, maxResponseBodyBytes + 1} {
			plain := bytes.Repeat([]byte(" "), size)
			var encoded bytes.Buffer
			header := codec
			switch codec {
			case "gzip", "x-gzip":
				writer := gzip.NewWriter(&encoded)
				_, _ = writer.Write(plain)
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			case "zlib", "raw-deflate":
				header = "deflate"
				var writer io.WriteCloser
				if codec == "zlib" {
					writer = zlib.NewWriter(&encoded)
				} else {
					writer, _ = flate.NewWriter(&encoded, flate.DefaultCompression)
				}
				_, _ = writer.Write(plain)
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
			case "gzip,deflate":
				var inner bytes.Buffer
				zipped := gzip.NewWriter(&inner)
				_, _ = zipped.Write(plain)
				if err := zipped.Close(); err != nil {
					t.Fatal(err)
				}
				deflated := zlib.NewWriter(&encoded)
				_, _ = deflated.Write(inner.Bytes())
				if err := deflated.Close(); err != nil {
					t.Fatal(err)
				}
			}
			got, err := decodeContentEncoding(header, encoded.Bytes())
			if size == maxResponseBodyBytes && (err != nil || len(got) != size) {
				t.Fatalf("%s exact cap failed: got=%d err=%v", codec, len(got), err)
			}
			if size > maxResponseBodyBytes && (!errors.Is(err, ErrDecodedBodyTooLarge) || got != nil) {
				t.Fatalf("%s oversized decoding returned data: got=%d err=%v", codec, len(got), err)
			}
		}
	}
}
