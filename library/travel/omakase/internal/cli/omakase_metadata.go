// pp:data-source live
package cli

import (
	"bytes"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/omakase/internal/omakase"
	"io"
	"net/http"
	"strings"
)

// Source-metadata endpoints share the generated client with MCP. Keep raw HTML
// ephemeral and enforce the same first-party, GET-only, 2 MiB public boundary.
func init() {
	registerClientHook(func(c *client.Client) error {
		c.NoCache = true
		c.BaseURL = omakase.Origin
		base := c.HTTPClient.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		c.HTTPClient.Transport = &omakaseMetadataTransport{base: base}
		c.HTTPClient.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			if len(via) > 2 {
				return fmt.Errorf("OMAKASE metadata redirect limit")
			}
			return metadataRequestAllowed(r)
		}
		return nil
	})
}

type omakaseMetadataTransport struct{ base http.RoundTripper }

func metadataRequestAllowed(r *http.Request) error {
	if r.Method != http.MethodGet || r.URL.Scheme != "https" || r.URL.Host != "omakase.in" {
		return fmt.Errorf("OMAKASE metadata supports first-party public HTTPS GET only")
	}
	p := r.URL.Path
	if p == "/" || p == "/en" || p == "/en/r" {
		return nil
	}
	if strings.HasPrefix(p, "/en/r/") {
		return omakase.ValidateID(strings.TrimPrefix(p, "/en/r/"))
	}
	return fmt.Errorf("unsupported OMAKASE public metadata path")
}
func (t *omakaseMetadataTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if e := metadataRequestAllowed(r); e != nil {
		return nil, e
	}
	req := r.Clone(r.Context())
	req.Header = r.Header.Clone()
	req.Header.Set("Accept-Encoding", "identity")
	resp, e := t.base.RoundTrip(req)
	if e != nil {
		return nil, e
	}
	encoding := resp.Header.Get("Content-Encoding")
	if encoding != "" && !strings.EqualFold(encoding, "identity") {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("OMAKASE metadata received unsupported compressed response")
	}
	body, e := io.ReadAll(io.LimitReader(resp.Body, (2<<20)+1))
	_ = resp.Body.Close()
	if e != nil {
		return nil, e
	}
	if len(body) > 2<<20 {
		return nil, fmt.Errorf("OMAKASE metadata response exceeds 2 MiB limit")
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	return resp, nil
}
