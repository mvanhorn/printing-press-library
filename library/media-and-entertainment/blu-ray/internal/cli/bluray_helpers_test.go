package cli

// PATCH: Regression coverage for Blu-ray.com helper edge cases from final review.

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/blu-ray/internal/config"
)

type testRoundTripFunc func(*http.Request) (*http.Response, error)

func (f testRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestDecodeLatin1(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{name: "latin1", raw: []byte{0xC9, 0x6D, 0x69, 0x6C, 0x69, 0x65}, want: "Émilie"},
		{name: "ascii", raw: []byte{0x68, 0x69}, want: "hi"},
		{name: "empty", raw: []byte{}, want: ""},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := decodeLatin1(tt.raw); got != tt.want {
				t.Fatalf("decodeLatin1 = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBluRayGetRejectsUnexpectedHost(t *testing.T) {
	t.Parallel()

	called := false
	c := client.New(&config.Config{BaseURL: "https://www.blu-ray.com"}, 0, 0)
	c.NoCache = true
	c.HTTPClient = &http.Client{Transport: testRoundTripFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("unexpected")),
		}, nil
	})}
	_, err := bluRayGet(context.Background(), c, "https://evil.example.com/x", false)
	if err == nil {
		t.Fatal("bluRayGet succeeded, want host mismatch error")
	}
	if msg := err.Error(); !strings.Contains(msg, "host") || !strings.Contains(msg, "www.blu-ray.com") {
		t.Fatalf("error = %q, want host mismatch with expected hostname", msg)
	}
	if called {
		t.Fatal("bluRayGet made a network call before rejecting unexpected host")
	}
}

func TestParseSitemapLocsAndIndex(t *testing.T) {
	t.Parallel()

	urlset := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url><loc>https://www.blu-ray.com/movies/Example-Blu-ray/123/</loc></url>
	<url><loc>https://www.blu-ray.com/dvd/Other-DVD/456/</loc></url>
</urlset>`)
	locs, err := parseSitemapLocs(urlset)
	if err != nil {
		t.Fatalf("parseSitemapLocs: %v", err)
	}
	if got, want := strings.Join(locs, "\n"), "https://www.blu-ray.com/movies/Example-Blu-ray/123/\nhttps://www.blu-ray.com/dvd/Other-DVD/456/"; got != want {
		t.Fatalf("locs = %q, want %q", got, want)
	}

	index := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<sitemap><loc>https://www.blu-ray.com/sitemap_1.xml.gz</loc></sitemap>
	<sitemap><loc>https://www.blu-ray.com/sitemap_2.xml.gz</loc></sitemap>
</sitemapindex>`)
	shards, err := parseSitemapIndex(index)
	if err != nil {
		t.Fatalf("parseSitemapIndex: %v", err)
	}
	if got, want := strings.Join(shards, "\n"), "https://www.blu-ray.com/sitemap_1.xml.gz\nhttps://www.blu-ray.com/sitemap_2.xml.gz"; got != want {
		t.Fatalf("shards = %q, want %q", got, want)
	}
}

func TestDecodeMaybeBinaryEnvelope(t *testing.T) {
	t.Parallel()

	payload := []byte("<urlset><url><loc>https://www.blu-ray.com/movies/Example-Blu-ray/123/</loc></url></urlset>")
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(payload); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	gzipBytes := gz.Bytes()
	envelope, err := json.Marshal(map[string]any{
		"_pp_binary":   true,
		"content_type": "application/gzip",
		"encoding":     "base64",
		"bytes":        len(gzipBytes),
		"data":         base64.StdEncoding.EncodeToString(gzipBytes),
	})
	if err != nil {
		t.Fatalf("marshal envelope: %v", err)
	}

	tests := []struct {
		name    string
		raw     []byte
		want    []byte
		wantErr bool
	}{
		{name: "base64 gzip envelope", raw: envelope, want: gzipBytes},
		{name: "plain body", raw: []byte("<html><body>ok</body></html>"), want: []byte("<html><body>ok</body></html>")},
		{name: "invalid base64 envelope", raw: []byte(`{"_pp_binary":true,"content_type":"application/gzip","encoding":"base64","bytes":3,"data":"!!!notbase64!!!"}`), wantErr: true},
		{name: "unknown binary envelope encoding", raw: []byte(`{"_pp_binary":true,"content_type":"application/gzip","encoding":"gzip","bytes":3,"data":"abc"}`), wantErr: true},
		{name: "missing binary discriminator", raw: []byte(`{"content_type":"application/gzip","encoding":"base64","bytes":3,"data":"abc"}`), want: []byte(`{"content_type":"application/gzip","encoding":"base64","bytes":3,"data":"abc"}`)},
		{name: "false binary discriminator", raw: []byte(`{"_pp_binary":false,"content_type":"application/gzip","encoding":"base64","bytes":3,"data":"abc"}`), want: []byte(`{"_pp_binary":false,"content_type":"application/gzip","encoding":"base64","bytes":3,"data":"abc"}`)},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := decodeMaybeBinaryEnvelope(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("decodeMaybeBinaryEnvelope error = nil, want error")
				}
				if got != nil {
					t.Fatalf("decodeMaybeBinaryEnvelope decoded = %q, want nil on error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("decodeMaybeBinaryEnvelope error = %v, want nil", err)
			}
			if !bytes.Equal(got, tt.want) {
				t.Fatalf("decodeMaybeBinaryEnvelope = %q, want %q", got, tt.want)
			}
			if tt.name == "base64 gzip envelope" {
				plain, err := gunzipBytes(got)
				if err != nil {
					t.Fatalf("gunzipBytes: %v", err)
				}
				if !bytes.Equal(plain, payload) {
					t.Fatalf("gunzipBytes(decodeMaybeBinaryEnvelope) = %q, want %q", plain, payload)
				}
			}
		})
	}
}
