// Copyright 2026 zjsng. Licensed under Apache-2.0.
package client

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"context"
	"encoding/json"
	"errors"
	"github.com/mvanhorn/printing-press-library/library/travel/michi-no-eki/internal/config"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type boundedTestTransport func(*http.Request) (*http.Response, error)

func (f boundedTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type countedTestBody struct {
	reader io.Reader
	read   int
	closes int
}

func (b *countedTestBody) Read(p []byte) (int, error) {
	n, e := b.reader.Read(p)
	b.read += n
	return n, e
}
func (b *countedTestBody) Close() error { b.closes++; return nil }

type repeatedTestReader struct{}

func (repeatedTestReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'a'
	}
	return len(p), nil
}
func boundedTestClient(body *countedTestBody, status int, length int64, contentType, encoding string) (*Client, *int) {
	calls := new(int)
	c := New(&config.Config{BaseURL: "https://fixture.invalid"}, time.Second, 0)
	c.HTTPClient = &http.Client{Transport: boundedTestTransport(func(r *http.Request) (*http.Response, error) {
		*calls++
		return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{contentType}, "Content-Encoding": []string{encoding}}, Body: body, ContentLength: length, Request: r}, nil
	})}
	return c, calls
}

func TestDeclaredHTMLReadBudgetStopsWireAndCloses(t *testing.T) {
	for _, status := range []int{200, 429, 503} {
		for _, length := range []int64{-1, 1, 1 << 40} {
			body := &countedTestBody{reader: repeatedTestReader{}}
			// A misleading binary MIME label must not bypass a declared HTML budget.
			c, calls := boundedTestClient(body, status, length, "application/octet-stream", "")
			out, err := c.GetWithHeadersNoCache(context.Background(), "/stations/views/10001", nil, map[string]string{HTMLResponseHeader: "true"})
			if !errors.Is(err, ErrResponseBodyTooLarge) || out != nil {
				t.Fatalf("status%d length%d: output=%d err=%v", status, length, len(out), err)
			}
			if body.read != maxHTMLResponseBytes+1 || body.closes != 1 || *calls != 1 {
				t.Fatalf("read=%d closes=%d calls=%d", body.read, body.closes, *calls)
			}
		}
	}
}

func TestDeclaredHTMLExactBoundaryIsUsable(t *testing.T) {
	data := []byte(`"` + strings.Repeat("a", maxHTMLResponseBytes-2) + `"`)
	body := &countedTestBody{reader: bytes.NewReader(data)}
	c, _ := boundedTestClient(body, 200, -1, "application/json", "")
	out, err := c.GetWithHeadersNoCache(context.Background(), "/stations/views/10001", nil, map[string]string{HTMLResponseHeader: "true"})
	if err != nil || !json.Valid(out) || !bytes.Equal(out, data) || body.closes != 1 {
		t.Fatalf("output=%d err=%v closes=%d", len(out), err, body.closes)
	}
}

func encodeBoundedTest(t *testing.T, kind string, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	var w io.WriteCloser
	switch kind {
	case "gzip":
		w = gzip.NewWriter(&b)
	case "zlib":
		w = zlib.NewWriter(&b)
	case "raw":
		var e error
		w, e = flate.NewWriter(&b, flate.DefaultCompression)
		if e != nil {
			t.Fatal(e)
		}
	}
	if _, e := w.Write(data); e != nil {
		t.Fatal(e)
	}
	if e := w.Close(); e != nil {
		t.Fatal(e)
	}
	return b.Bytes()
}
func TestDeclaredHTMLInflationBudgetAcrossEncodings(t *testing.T) {
	expanded := bytes.Repeat([]byte("a"), maxHTMLResponseBytes+1)
	cases := []struct {
		name, header string
		data         []byte
	}{
		{"gzip", "gzip", encodeBoundedTest(t, "gzip", expanded)},
		{"xgzip", "x-gzip", encodeBoundedTest(t, "gzip", expanded)},
		{"zlib", "deflate", encodeBoundedTest(t, "zlib", expanded)},
		{"rawdeflate", "deflate", encodeBoundedTest(t, "raw", expanded)},
		{"stacked", "gzip, deflate", encodeBoundedTest(t, "zlib", encodeBoundedTest(t, "gzip", expanded))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := &countedTestBody{reader: bytes.NewReader(tc.data)}
			c, _ := boundedTestClient(body, 200, -1, "text/html", tc.header)
			out, err := c.GetWithHeadersNoCache(context.Background(), "/stations/views/10001", nil, map[string]string{HTMLResponseHeader: "true"})
			if !errors.Is(err, ErrDecodedBodyTooLarge) || out != nil || body.closes != 1 {
				t.Fatalf("output=%d err=%v closes=%d", len(out), err, body.closes)
			}
		})
	}
}

func TestOrdinaryResponseBudgetAndSuccessfulBinarySemantics(t *testing.T) {
	body := &countedTestBody{reader: repeatedTestReader{}}
	c, _ := boundedTestClient(body, 200, 1, "application/json", "")
	out, err := c.GetWithHeadersNoCache(context.Background(), "/ordinary", nil, nil)
	if !errors.Is(err, ErrResponseBodyTooLarge) || out != nil || body.read != maxDecodedBodyBytes+1 || body.closes != 1 {
		t.Fatalf("read=%d output=%d err=%v", body.read, len(out), err)
	}
	data := bytes.Repeat([]byte("b"), maxHTMLResponseBytes+1)
	for _, marked := range []bool{false, true} {
		body = &countedTestBody{reader: bytes.NewReader(data)}
		c, _ = boundedTestClient(body, 200, -1, "application/octet-stream", "")
		headers := map[string]string{}
		if marked {
			headers[BinaryResponseHeader] = "true"
		}
		out, err = c.GetWithHeadersNoCache(context.Background(), "/binary", nil, headers)
		decoded, _, ok := UnwrapBinaryResponse(out)
		if err != nil || !ok || !bytes.Equal(decoded, data) || body.closes != 1 {
			t.Fatalf("marked=%v binary output=%d err=%v", marked, len(out), err)
		}
	}
}
