package cli

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/config"
)

type hcRoundTripFunc func(*http.Request) (*http.Response, error)

func (f hcRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestHCMetadataBodyBounds(t *testing.T) {
	for _, tc := range []struct {
		name     string
		size     int
		encoding string
		ok       bool
	}{{"small", 100, "", true}, {"maximum", hcMetadataResponseLimit, "", true}, {"oversized", hcMetadataResponseLimit + 1, "", false}, {"unexpected encoding", 100, "gzip", false}} {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", "https://api-public.odpt.org/api/v4/gbfs/hellocycling/gbfs.json", nil)
			transport := hcMetadataTransport{base: hcRoundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.Header.Get("Accept-Encoding") != "identity" {
					t.Fatal("not decoded-bound")
				}
				header := http.Header{}
				if tc.encoding != "" {
					header.Set("Content-Encoding", tc.encoding)
				}
				return &http.Response{StatusCode: 200, Header: header, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", tc.size)))}, nil
			})}
			resp, e := transport.RoundTrip(req)
			if (e == nil) != tc.ok {
				t.Fatalf("err=%v", e)
			}
			if e == nil {
				defer resp.Body.Close()
				b, e := io.ReadAll(resp.Body)
				if e != nil || len(b) != tc.size {
					t.Fatal("bounded body changed")
				}
			}
		})
	}
}
func TestHCGeneratedClientHook(t *testing.T) {
	c := client.New(&config.Config{}, time.Second, 0)
	if e := ApplyClientHooks(c); e != nil {
		t.Fatal(e)
	}
	if _, ok := c.HTTPClient.Transport.(hcMetadataTransport); !ok {
		t.Fatal("CLI/MCP shared hook absent")
	}
}
