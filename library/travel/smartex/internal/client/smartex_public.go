package client

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/smartex/internal/cliutil"
)

// The generated reference and MCP clients share the public planning budget.
// Keep the API-specific limit outside generator-reserved limiter code.
const SmartEXPublicBodyLimit = 512 << 10

var ErrSmartEXPublicBodyTooLarge = errors.New("public response exceeds 512KiB limit")

func applySmartEXPublicLimits(c *Client, rate float64) {
	if !(rate > 0 && rate <= 2) {
		rate = 2
	}
	c.limiter = cliutil.NewAdaptiveLimiter(rate)
	next := c.HTTPClient.Transport
	if next == nil {
		next = http.DefaultTransport
	}
	c.HTTPClient.Transport = smartEXPublicTransport{next: next}
}

type smartEXPublicTransport struct{ next http.RoundTripper }

func (t smartEXPublicTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// An explicit negotiation prevents the inner Go transport from gunzipping
	// first; both wire and decoded bytes must pass through our caps.
	req = req.Clone(req.Context())
	if req.Header == nil {
		req.Header = http.Header{}
	}
	if req.Header.Get("Accept-Encoding") == "" {
		req.Header.Set("Accept-Encoding", "gzip, deflate")
	}
	resp, err := t.next.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	raw := &smartEXPublicBody{reader: resp.Body, original: resp.Body, remaining: SmartEXPublicBodyLimit}
	bounded := &smartEXPublicBody{reader: raw, original: resp.Body, remaining: SmartEXPublicBodyLimit}
	encodings := strings.Split(resp.Header.Get("Content-Encoding"), ",")
	for i := len(encodings) - 1; i >= 0; i-- {
		var decoded io.ReadCloser
		switch strings.ToLower(strings.TrimSpace(encodings[i])) {
		case "", "identity":
			continue
		case "gzip", "x-gzip":
			decoded, err = gzip.NewReader(bounded.reader)
		case "deflate":
			buffer := bufio.NewReader(bounded.reader)
			header, _ := buffer.Peek(2)
			if len(header) == 2 && header[0]&15 == 8 && (int(header[0])*256+int(header[1]))%31 == 0 {
				decoded, err = zlib.NewReader(buffer)
			} else {
				decoded = flate.NewReader(buffer)
			}
		default:
			err = errors.New("unsupported public response content encoding")
		}
		if err != nil {
			_ = bounded.Close()
			return nil, err
		}
		bounded.reader = &smartEXPublicBody{reader: decoded, original: decoded, remaining: SmartEXPublicBodyLimit}
		bounded.decoders = append(bounded.decoders, decoded)
	}
	resp.Body = bounded
	resp.Header.Del("Content-Encoding")
	resp.ContentLength = -1
	resp.Uncompressed = true
	return resp, nil
}

type smartEXPublicBody struct {
	reader    io.Reader
	original  io.ReadCloser
	decoders  []io.Closer
	remaining int
}

func (b *smartEXPublicBody) Read(p []byte) (int, error) {
	if len(p) > b.remaining+1 {
		p = p[:b.remaining+1]
	}
	n, err := b.reader.Read(p)
	if n > b.remaining {
		return 0, ErrSmartEXPublicBodyTooLarge
	}
	b.remaining -= n
	return n, err
}

func (b *smartEXPublicBody) Close() error {
	for i := len(b.decoders) - 1; i >= 0; i-- {
		_ = b.decoders[i].Close()
	}
	return b.original.Close()
}
