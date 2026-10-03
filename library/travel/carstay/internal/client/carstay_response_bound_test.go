// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package client

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/carstay/internal/config"
)

type carstayBoundTransport func(*http.Request) (*http.Response, error)

func (f carstayBoundTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type carstayCountBody struct {
	remaining int64
	read      int64
	closed    bool
}

func (b *carstayCountBody) Read(p []byte) (int, error) {
	if b.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > b.remaining {
		n = int(b.remaining)
	}
	for i := 0; i < n; i++ {
		p[i] = ' '
	}
	b.remaining -= int64(n)
	b.read += int64(n)
	return n, nil
}
func (b *carstayCountBody) Close() error { b.closed = true; return nil }

func carstayResponseClient(t *testing.T, response *http.Response) *Client {
	t.Helper()
	c := New(&config.Config{BaseURL: "https://carstay.example.test"}, time.Second, 0)
	c.NoCache = true
	c.HTTPClient = &http.Client{Transport: carstayBoundTransport(func(*http.Request) (*http.Response, error) { return response, nil })}
	return c
}
func TestCarstayPublicResponsesStopAtReadCap(t *testing.T) {
	for _, status := range []int{200, 429, 503} {
		for _, claimed := range []int64{-1, 1, maxPublicResponseBodyBytes * 2} {
			t.Run(strconv.Itoa(status)+"-length-"+strconv.FormatInt(claimed, 10), func(t *testing.T) {
				body := &carstayCountBody{remaining: maxPublicResponseBodyBytes * 2}
				response := &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: body, ContentLength: claimed}
				c := carstayResponseClient(t, response)
				raw, err := c.Get(context.Background(), "/ja/api/data/no-route/stations", nil)
				if !errors.Is(err, ErrPublicResponseTooLarge) || raw != nil || body.read != maxPublicResponseBodyBytes+1 || !body.closed {
					t.Fatalf("unbounded/unclosed response: bytes=%d closed=%v raw=%d err=%v", body.read, body.closed, len(raw), err)
				}
			})
		}
	}
}
func TestCarstayPublicExactCapAndInflation(t *testing.T) {
	exact := []byte(`{"fixture":"` + strings.Repeat("x", maxPublicResponseBodyBytes-len(`{"fixture":""}`)) + `"}`)
	c := carstayResponseClient(t, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(exact))})
	if got, err := c.Get(context.Background(), "/ja/api/data/no-route/stations", nil); err != nil || len(got) != len(exact) {
		t.Fatalf("exact-cap JSON rejected: %d %v", len(got), err)
	}
	var zipped bytes.Buffer
	gz := gzip.NewWriter(&zipped)
	_, _ = gz.Write(append(exact, ' '))
	_ = gz.Close()
	c = carstayResponseClient(t, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}, "Content-Encoding": []string{"gzip"}}, Body: io.NopCloser(bytes.NewReader(zipped.Bytes()))})
	if got, err := c.Get(context.Background(), "/ja/api/data/no-route/stations", nil); !errors.Is(err, ErrPublicResponseTooLarge) || got != nil {
		t.Fatalf("inflated overflow accepted: %d %v", len(got), err)
	}
}
func TestCarstayPublicBoundPreservesBinaryResponse(t *testing.T) {
	// The verified Carstay JSON path is bounded; the generated optional binary
	// envelope/timeout behavior is deliberately preserved.
	payload := bytes.Repeat([]byte{0x23}, maxPublicResponseBodyBytes+1)
	c := carstayResponseClient(t, &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/octet-stream"}}, Body: io.NopCloser(bytes.NewReader(payload))})
	got, err := c.Get(context.Background(), "/fixture-binary", nil)
	if err != nil || !bytes.Contains(got, []byte(`"_pp_binary":true`)) {
		t.Fatalf("binary envelope changed: %v", err)
	}
}
