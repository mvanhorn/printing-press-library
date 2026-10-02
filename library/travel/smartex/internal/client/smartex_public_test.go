package client

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/config"
)

type publicTestTransport func(*http.Request) (*http.Response, error)

func (f publicTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSmartEXReferenceBoundsPlainAndCompressedBodies(t *testing.T) {
	for _, encoding := range []string{"", "gzip", "deflate"} {
		for _, size := range []int{SmartEXPublicBodyLimit, SmartEXPublicBodyLimit + 1, 2 << 20} {
			t.Run(encoding+"_"+http.StatusText(size), func(t *testing.T) {
				raw := []byte(strings.Repeat("x", size))
				wire := raw
				if encoding != "" {
					var buf bytes.Buffer
					var compressor io.WriteCloser
					if encoding == "gzip" {
						compressor = gzip.NewWriter(&buf)
					} else {
						compressor = zlib.NewWriter(&buf)
					}
					_, _ = compressor.Write(raw)
					_ = compressor.Close()
					wire = buf.Bytes()
				}
				c := New(&config.Config{BaseURL: "https://smart-ex.jp"}, time.Second, 2)
				c.NoCache = true
				c.HTTPClient.Transport = smartEXPublicTransport{next: publicTestTransport(func(r *http.Request) (*http.Response, error) {
					return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}, "Content-Encoding": {encoding}}, Body: io.NopCloser(bytes.NewReader(wire)), Request: r}, nil
				})}
				got, err := c.GetWithHeaders(context.Background(), "/en/product/plan/service/", nil, map[string]string{HTMLResponseHeader: "true"})
				if size == SmartEXPublicBodyLimit {
					if err != nil || len(got) != size {
						t.Fatalf("at-cap response rejected: %d %v", len(got), err)
					}
				} else if !errors.Is(err, ErrSmartEXPublicBodyTooLarge) {
					t.Fatalf("oversized %s response not rejected: %d %v", encoding, len(got), err)
				}
			})
		}
	}
}

func TestSmartEXReferenceRateCeilingIncludesAutoDisabledAndHighOverrides(t *testing.T) {
	for _, rate := range []float64{-1, 0, 100, 1, math.NaN(), math.Inf(1)} {
		c := New(&config.Config{}, time.Second, rate)
		for i := 0; i < 20; i++ {
			c.limiter.OnSuccess()
		}
		c.limiter.ObserveHeaders(1000, time.Now().Add(time.Second))
		want := 2.0
		if rate == 1 {
			want = 1
		}
		if got := c.RateLimit(); math.IsNaN(got) || got > want || got <= 0 {
			t.Fatalf("rate %v bypassed ceiling %v: %v", rate, want, got)
		}
	}
}

func TestSmartEXWireCapPrecedesAutomaticGoGzipDecompression(t *testing.T) {
	var wire bytes.Buffer
	for i := 0; i < 10; i++ {
		member := gzip.NewWriter(&wire)
		member.Extra = bytes.Repeat([]byte("x"), 65535)
		_, _ = member.Write([]byte("tiny-data"))
		_ = member.Close()
	}
	if wire.Len() <= SmartEXPublicBodyLimit {
		t.Fatal("fixture does not exceed wire cap")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept-Encoding") != "gzip, deflate" {
			t.Errorf("inner auto-decompression not prevented: %s", r.Header.Get("Accept-Encoding"))
		}
		w.Header().Set("Content-Encoding", "gzip")
		_, _ = w.Write(wire.Bytes())
	}))
	defer srv.Close()
	c := New(&config.Config{BaseURL: srv.URL}, time.Second, 2)
	c.NoCache = true
	// Exercise net/http's transparent gzip path, rather than the generated
	// Chrome HTTP/2 transport, which cannot connect to this HTTP fixture.
	c.HTTPClient.Transport = srv.Client().Transport
	applySmartEXPublicLimits(c, 2)
	_, err := c.GetWithHeaders(context.Background(), "/", nil, map[string]string{HTMLResponseHeader: "true"})
	if !errors.Is(err, ErrSmartEXPublicBodyTooLarge) {
		t.Fatalf("oversized wire body accepted after transparent gzip: %v", err)
	}
}

func TestSmartEXTransportPreservesExplicitEncodingAndInputRequest(t *testing.T) {
	for _, explicit := range []string{"", "gzip", "identity"} {
		req, _ := http.NewRequest("GET", "https://smart-ex.jp/", nil)
		if explicit != "" {
			req.Header.Set("Accept-Encoding", explicit)
		}
		transport := smartEXPublicTransport{next: publicTestTransport(func(r *http.Request) (*http.Response, error) {
			want := explicit
			if want == "" {
				want = "gzip, deflate"
			}
			if r.Header.Get("Accept-Encoding") != want {
				t.Fatalf("explicit encoding changed: %s", r.Header.Get("Accept-Encoding"))
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("small")), Request: r}, nil
		})}
		resp, err := transport.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if req.Header.Get("Accept-Encoding") != explicit {
			t.Fatal("RoundTrip mutated caller request")
		}
	}
}
