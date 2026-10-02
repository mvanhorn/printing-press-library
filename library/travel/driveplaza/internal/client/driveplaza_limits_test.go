package client

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

type limitReplay func(*http.Request) (*http.Response, error)

func (f limitReplay) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestDrivePlazaGeneratedClientBodyBound(t *testing.T) {
	for _, tt := range []struct {
		size    int
		allowed bool
	}{{0, true}, {100, true}, {drivePlazaResponseLimit, true}, {drivePlazaResponseLimit + 1, false}} {
		c := &Client{HTTPClient: &http.Client{Transport: limitReplay(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, ContentLength: -1, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", tt.size)))}, nil
		})}}
		ApplyDrivePlazaLimits(c)
		req, _ := http.NewRequest("GET", "https://en.driveplaza.com", nil)
		resp, e := c.HTTPClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		_, e = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if (e == nil) != tt.allowed {
			t.Fatalf("size %d allowed=%v error=%v", tt.size, tt.allowed, e)
		}
		if !c.NoCache {
			t.Fatal("reference responses could use stale cache")
		}
	}
}

func TestDrivePlazaDecodedBodyBound(t *testing.T) {
	for _, tt := range []struct {
		size    int
		allowed bool
	}{{100, true}, {drivePlazaResponseLimit, true}, {drivePlazaResponseLimit + 1, false}} {
		var compressed bytes.Buffer
		w := gzip.NewWriter(&compressed)
		_, _ = w.Write([]byte(strings.Repeat("x", tt.size)))
		_ = w.Close()
		c := &Client{HTTPClient: &http.Client{Transport: limitReplay(func(r *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, ContentLength: int64(compressed.Len()), Header: http.Header{"Content-Encoding": {"gzip"}}, Body: io.NopCloser(bytes.NewReader(compressed.Bytes()))}, nil
		})}}
		ApplyDrivePlazaLimits(c)
		req, _ := http.NewRequest("GET", "https://en.driveplaza.com", nil)
		resp, e := c.HTTPClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		body, e := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if (e == nil) != tt.allowed {
			t.Fatalf("decoded size %d allowed=%v error=%v", tt.size, tt.allowed, e)
		}
		if tt.allowed && len(body) != tt.size {
			t.Fatal("decoded payload truncated")
		}
		if resp.Header.Get("Content-Encoding") != "" {
			t.Fatal("generated decoder would inflate again")
		}
	}
}
