// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
//
// HAND-AUTHORED. Not generated, and must survive `generate --force`.

package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/other/nepra/internal/cliutil"
)

func gzipBytes(t *testing.T, s string) []byte {
	t.Helper()
	var b bytes.Buffer
	zw := gzip.NewWriter(&b)
	if _, err := zw.Write([]byte(s)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// TestDecodeContentEncoding is the regression guard for the silent-empty bug.
//
// The client sets Accept-Encoding explicitly, which turns OFF net/http's
// transparent decompression and leaves Content-Encoding on the response.
// Nothing decompressed it, so every gzipped body reached the HTML parser as
// binary and the command printed an empty result under an HTTP 200 with
// provenance "source: live" — the most misleading shape a failure can take.
func TestDecodeContentEncoding(t *testing.T) {
	const html = `<html><head><title>FCA</title></head><body><table><tr><td>9.9095</td></tr></table></body></html>`

	t.Run("gzip is decompressed", func(t *testing.T) {
		in := gzipBytes(t, html)
		if bytes.HasPrefix(in, []byte(html[:8])) {
			t.Fatal("fixture is not actually compressed")
		}
		// The magic number is what reached the parser before the fix.
		if !bytes.HasPrefix(in, []byte{0x1f, 0x8b}) {
			t.Fatalf("fixture lacks gzip magic: % x", in[:2])
		}
		out, err := decodeContentEncoding("gzip", in)
		if err != nil {
			t.Fatalf("gzip: %v", err)
		}
		if string(out) != html {
			t.Errorf("got %q, want the original document", string(out))
		}
		if len(in) >= len(out) {
			t.Errorf("compressed %d B did not expand (%d B out)", len(in), len(out))
		}
	})

	t.Run("x-gzip is decompressed", func(t *testing.T) {
		out, err := decodeContentEncoding("x-gzip", gzipBytes(t, html))
		if err != nil || string(out) != html {
			t.Errorf("x-gzip: (%q, %v)", string(out), err)
		}
	})

	t.Run("case and whitespace are tolerated", func(t *testing.T) {
		out, err := decodeContentEncoding("  GZIP  ", gzipBytes(t, html))
		if err != nil || string(out) != html {
			t.Errorf("GZIP: (%q, %v)", string(out), err)
		}
	})

	t.Run("deflate as zlib", func(t *testing.T) {
		var b bytes.Buffer
		zw := zlib.NewWriter(&b)
		_, _ = zw.Write([]byte(html))
		_ = zw.Close()
		out, err := decodeContentEncoding("deflate", b.Bytes())
		if err != nil {
			t.Fatalf("zlib deflate: %v", err)
		}
		if string(out) != html {
			t.Errorf("got %q", string(out))
		}
	})

	t.Run("deflate as raw flate", func(t *testing.T) {
		// Servers disagree about what "deflate" means; raw RFC 1951 is common
		// in the wild and must not be rejected.
		var b bytes.Buffer
		fw, _ := flate.NewWriter(&b, flate.DefaultCompression)
		_, _ = fw.Write([]byte(html))
		_ = fw.Close()
		out, err := decodeContentEncoding("deflate", b.Bytes())
		if err != nil {
			t.Fatalf("raw deflate: %v", err)
		}
		if string(out) != html {
			t.Errorf("got %q", string(out))
		}
	})

	t.Run("identity and empty pass through untouched", func(t *testing.T) {
		for _, enc := range []string{"", "identity", "  "} {
			out, err := decodeContentEncoding(enc, []byte(html))
			if err != nil || string(out) != html {
				t.Errorf("%q: (%q, %v)", enc, string(out), err)
			}
		}
	})

	t.Run("a body net/http already decompressed is untouched", func(t *testing.T) {
		// net/http strips Content-Encoding when it decodes transparently, so
		// the patch must be a no-op on that path.
		out, err := decodeContentEncoding("", []byte(html))
		if err != nil || string(out) != html {
			t.Errorf("(%q, %v)", string(out), err)
		}
	})

	t.Run("an empty body is never an error", func(t *testing.T) {
		out, err := decodeContentEncoding("gzip", nil)
		if err != nil || len(out) != 0 {
			t.Errorf("(%v, %v)", out, err)
		}
	})
}

// TestDecodeContentEncodingRefusals checks the failure modes are LOUD. Passing
// an unreadable body through as if it were plaintext is what produced an empty
// result under a 200 in the first place.
func TestDecodeContentEncodingRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, enc, wantIn string
		body              []byte
	}{
		{"brotli is refused, not passed through", "br", "unsupported Content-Encoding", []byte("\x1b\x0e\x00")},
		{"zstd is refused", "zstd", "unsupported Content-Encoding", []byte("\x28\xb5\x2f\xfd")},
		{"stacked encodings are refused", "gzip, br", "stacked", []byte("whatever")},
		{"a body that lies about being gzip is refused", "gzip", "not valid gzip", []byte("<html>plain</html>")},
		{"a body that lies about being deflate is refused", "deflate", "neither zlib nor raw deflate", []byte("\xff\xfe\xfd\xfc bad")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := decodeContentEncoding(tc.enc, tc.body)
			if err == nil {
				t.Fatalf("%q was accepted, returning %d bytes; want an error", tc.enc, len(out))
			}
			if !strings.Contains(err.Error(), tc.wantIn) {
				t.Errorf("error %q does not mention %q", err, tc.wantIn)
			}
			if out != nil {
				t.Errorf("a refused body still returned %d bytes", len(out))
			}
		})
	}
}

// TestDecodeContentEncodingBombIsCapped keeps a compression bomb from
// exhausting memory.
func TestDecodeContentEncodingBombIsCapped(t *testing.T) {
	// Highly compressible payload; the cap is what matters, not the ratio.
	big := strings.Repeat("A", 4<<20)
	out, err := decodeContentEncoding("gzip", gzipBytes(t, big))
	if err != nil {
		t.Fatalf("a 4 MB body is well under the cap but failed: %v", err)
	}
	if len(out) != len(big) {
		t.Errorf("got %d bytes, want %d", len(out), len(big))
	}
	if maxDecompressedBytes < 316<<20 {
		t.Errorf("the cap %d is below the largest real artifact (a ~316 MB SIR PDF)", maxDecompressedBytes)
	}
}

// TestClientDecompressesGzipResponse guards the CALL SITE, not just the
// function. The unit tests above still pass with the one-line call in
// client.go removed — which is exactly the line a `generate --force` will
// drop, because client.go is generated. This test drives a real gzipped
// response through the client and fails if the body arrives compressed.
func TestClientDecompressesGzipResponse(t *testing.T) {
	const html = `<html><head><title>FCA</title></head><body><table><tr><td>9.9095</td></tr></table></body></html>`

	var sawAcceptEncoding string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Record what the client asked for: an EXPLICIT Accept-Encoding is
		// what disables net/http's transparent decompression, and the spec
		// declares that header as required, so it must stay explicit.
		sawAcceptEncoding = r.Header.Get("Accept-Encoding")
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Content-Encoding", "gzip")
		zw := gzip.NewWriter(w)
		_, _ = zw.Write([]byte(html))
		_ = zw.Close()
	}))
	defer srv.Close()

	c := &Client{
		BaseURL:    srv.URL,
		HTTPClient: srv.Client(),
		NoCache:    true,
		limiter:    cliutil.NewAdaptiveLimiter(0),
	}
	body, err := c.GetWithHeaders(context.Background(), "/sheet001.htm", map[string]string{},
		map[string]string{HTMLResponseHeader: "true"})
	if err != nil {
		t.Fatalf("GetWithHeaders: %v", err)
	}
	if bytes.HasPrefix(body, []byte{0x1f, 0x8b}) {
		t.Fatalf("the body arrived still gzip-compressed (% x...); the decompression call site is missing", body[:4])
	}
	if string(body) != html {
		t.Errorf("body = %q, want the decompressed document", string(body))
	}
	if sawAcceptEncoding == "" {
		t.Error("the client sent no Accept-Encoding; NEPRA silently truncates some pages without it")
	}
	if !strings.Contains(sawAcceptEncoding, "gzip") {
		t.Errorf("Accept-Encoding = %q, want it to request gzip", sawAcceptEncoding)
	}
}
