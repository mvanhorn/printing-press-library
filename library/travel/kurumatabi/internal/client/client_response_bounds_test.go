package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/config"
)

type boundsTransport func(*http.Request) (*http.Response, error)

func (f boundsTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type boundsBody struct {
	read   int
	closed bool
}

func (b *boundsBody) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	b.read += len(p)
	return len(p), nil
}
func (b *boundsBody) Close() error { b.closed = true; return nil }

func TestSourceDocumentResponseBoundBeforeStatusOrRetry(t *testing.T) {
	for _, status := range []int{200, 429, 503} {
		for _, length := range []int64{-1, 1} {
			t.Run(fmt.Sprintf("status%d_length%d", status, length), func(t *testing.T) {
				body := &boundsBody{}
				calls := 0
				c := New(&config.Config{BaseURL: "https://example.test"}, time.Second, 0)
				c.NoCache = true
				c.HTTPClient = &http.Client{Transport: boundsTransport(func(*http.Request) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: status, ContentLength: length, Header: http.Header{"Content-Type": []string{"text/html"}}, Body: body}, nil
				})}
				data, err := c.GetWithHeaders(context.Background(), "/park/search.php", nil, map[string]string{HTMLResponseHeader: "true"})
				if !errors.Is(err, ErrResponseBodyTooLarge) || data != nil {
					t.Fatalf("data=%d err=%v", len(data), err)
				}
				if body.read != maxResponseBodyBytes+1 || !body.closed || calls != 1 {
					t.Fatalf("read=%d closed=%v calls=%d", body.read, body.closed, calls)
				}
				if !strings.Contains(err.Error(), "GET") || !strings.Contains(err.Error(), "/park/search.php") {
					t.Fatalf("missing request context: %v", err)
				}
			})
		}
	}
}

func TestSourceDocumentExactBoundAndBinaryDelivery(t *testing.T) {
	prefix, suffix := `{"text":"}`, `"}`
	exact := []byte(prefix + strings.Repeat("a", maxResponseBodyBytes-len(prefix)-len(suffix)) + suffix)
	for _, tc := range []struct {
		name, contentType string
		body              []byte
		headers           map[string]string
	}{
		{"exact_json", "application/json", exact, nil},
		{"binary_contract", "application/octet-stream", bytes.Repeat([]byte("x"), maxResponseBodyBytes+1), map[string]string{BinaryResponseHeader: "true"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New(&config.Config{BaseURL: "https://example.test"}, time.Second, 0)
			c.NoCache = true
			c.HTTPClient = &http.Client{Transport: boundsTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{tc.contentType}}, Body: io.NopCloser(bytes.NewReader(tc.body))}, nil
			})}
			data, err := c.GetWithHeaders(context.Background(), "/document", nil, tc.headers)
			if err != nil {
				t.Fatal(err)
			}
			if tc.headers != nil {
				raw, ct, ok := UnwrapBinaryResponse(data)
				if !ok || ct != tc.contentType || !bytes.Equal(raw, tc.body) {
					t.Fatal("generic binary delivery changed")
				}
			} else if !bytes.Equal(data, tc.body) {
				t.Fatal("exact bound must be accepted without truncation")
			}
		})
	}
}

func TestHTMLContractCannotBypassBoundWithBinaryContentType(t *testing.T) {
	body := &boundsBody{}
	c := New(&config.Config{BaseURL: "https://example.test"}, time.Second, 0)
	c.NoCache = true
	c.HTTPClient = &http.Client{Transport: boundsTransport(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}, Body: body}, nil
	})}
	data, err := c.GetWithHeaders(context.Background(), "/park/search.php", nil, map[string]string{HTMLResponseHeader: "true"})
	if data != nil || !errors.Is(err, ErrResponseBodyTooLarge) || !body.closed || body.read != maxResponseBodyBytes+1 {
		t.Fatalf("read=%d data=%d err=%v", body.read, len(data), err)
	}
}

func TestSourceDocumentInflatedOutputBound(t *testing.T) {
	raw := bytes.Repeat([]byte("a"), maxResponseBodyBytes+1)
	for _, encoding := range []string{"gzip", "x-gzip", "deflate-zlib", "deflate-raw", "gzip, deflate"} {
		t.Run(encoding, func(t *testing.T) {
			var compressed bytes.Buffer
			var w io.WriteCloser
			var err error
			wire := encoding
			switch encoding {
			case "gzip", "x-gzip", "gzip, deflate":
				w = gzip.NewWriter(&compressed)
			case "deflate-zlib":
				w = zlib.NewWriter(&compressed)
				wire = "deflate"
			case "deflate-raw":
				w, err = flate.NewWriter(&compressed, flate.DefaultCompression)
				wire = "deflate"
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write(raw); err != nil {
				t.Fatal(err)
			}
			if err = w.Close(); err != nil {
				t.Fatal(err)
			}
			if encoding == "gzip, deflate" {
				inner := append([]byte(nil), compressed.Bytes()...)
				compressed.Reset()
				outer := zlib.NewWriter(&compressed)
				if _, err := outer.Write(inner); err != nil {
					t.Fatal(err)
				}
				if err := outer.Close(); err != nil {
					t.Fatal(err)
				}
			}
			c := New(&config.Config{BaseURL: "https://example.test"}, time.Second, 0)
			c.NoCache = true
			c.HTTPClient = &http.Client{Transport: boundsTransport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/html"}, "Content-Encoding": []string{wire}}, Body: io.NopCloser(bytes.NewReader(compressed.Bytes()))}, nil
			})}
			data, err := c.GetWithHeaders(context.Background(), "/park/search.php", nil, map[string]string{HTMLResponseHeader: "true"})
			if data != nil || !errors.Is(err, ErrDecodedBodyTooLarge) {
				t.Fatalf("data=%d err=%v", len(data), err)
			}
		})
	}
}
