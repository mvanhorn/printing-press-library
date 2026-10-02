package client

import (
	"bufio"
	"compress/flate"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"net/http"
)

const drivePlazaResponseLimit = 2 << 20

// ApplyDrivePlazaLimits decorates the generated transport. Its existing
// AdaptiveLimiter and HTTP 429 handling still own outbound request pacing.
// This is a preserved extension rather than a patch to generated ReadAll.
func ApplyDrivePlazaLimits(c *Client) {
	if c == nil || c.HTTPClient == nil {
		return
	}
	base := c.HTTPClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	if _, ok := base.(*drivePlazaLimitedTransport); !ok {
		c.HTTPClient.Transport = &drivePlazaLimitedTransport{base: base}
	}
	c.NoCache = true
}

type drivePlazaLimitedTransport struct{ base http.RoundTripper }

func (t *drivePlazaLimitedTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(r)
	if err != nil {
		return nil, err
	}
	if resp.ContentLength > drivePlazaResponseLimit {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("Drive Plaza source response exceeds 2 MiB cap")
	}
	resp.Body = &drivePlazaLimitedBody{ReadCloser: resp.Body, left: drivePlazaResponseLimit}
	encodings := contentEncodingTokens(resp.Header.Get("Content-Encoding"))
	if len(encodings) > 0 {
		var reader io.Reader = resp.Body
		closers := []io.Closer{resp.Body}
		for i := len(encodings) - 1; i >= 0; i-- {
			var decoder io.ReadCloser
			switch encodings[i] {
			case "gzip", "x-gzip":
				decoder, err = gzip.NewReader(reader)
			case "deflate":
				buffer := bufio.NewReader(reader)
				header, _ := buffer.Peek(2)
				if len(header) == 2 && header[0]&15 == 8 && (int(header[0])*256+int(header[1]))%31 == 0 {
					decoder, err = zlib.NewReader(buffer)
				} else {
					decoder = flate.NewReader(buffer)
				}
			default:
				err = fmt.Errorf("unsupported Content-Encoding %q", encodings[i])
			}
			if err != nil {
				for _, c := range closers {
					_ = c.Close()
				}
				return nil, err
			}
			closers = append(closers, decoder)
			reader = &drivePlazaLimitedBody{ReadCloser: decoder, left: drivePlazaResponseLimit}
		}
		resp.Body = &drivePlazaDecodedBody{Reader: reader, closers: closers}
		resp.Header.Del("Content-Encoding")
		resp.ContentLength = -1
		resp.Uncompressed = true
	}
	return resp, nil
}

type drivePlazaDecodedBody struct {
	io.Reader
	closers []io.Closer
}

func (r *drivePlazaDecodedBody) Close() error {
	var err error
	for i := len(r.closers) - 1; i >= 0; i-- {
		if e := r.closers[i].Close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

type drivePlazaLimitedBody struct {
	io.ReadCloser
	left int
}

func (r *drivePlazaLimitedBody) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if r.left == 0 {
		var b [1]byte
		n, e := r.ReadCloser.Read(b[:])
		if n > 0 {
			return 0, fmt.Errorf("Drive Plaza source response exceeds 2 MiB cap")
		}
		return 0, e
	}
	if len(p) > r.left {
		p = p[:r.left]
	}
	n, e := r.ReadCloser.Read(p)
	r.left -= n
	return n, e
}
