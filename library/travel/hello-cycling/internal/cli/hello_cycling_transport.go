// Copyright 2026 zjsng. Licensed under Apache-2.0.
// The provider-published GBFS endpoints support standard HTTP. This preserved
// post-construction hook applies to both the generated CLI and MCP clients.
package cli

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/travel/hello-cycling/internal/client"
)

const hcMetadataResponseLimit = 256 << 10

type hcMetadataTransport struct{ base http.RoundTripper }

func (t hcMetadataTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	request := r.Clone(r.Context())
	request.Header = r.Header.Clone()
	// Identity encoding makes this bound apply to decoded JSON too. These small
	// metadata feeds have no need for a compressed transport profile.
	request.Header.Set("Accept-Encoding", "identity")
	response, e := t.base.RoundTrip(request)
	if e != nil {
		return nil, e
	}
	if strings.TrimSpace(response.Header.Get("Content-Encoding")) != "" {
		if closeErr := response.Body.Close(); closeErr != nil {
			return nil, fmt.Errorf("close unsupported encoded metadata response: %w", closeErr)
		}
		return nil, fmt.Errorf("GBFS metadata source ignored identity encoding; refusing an unbounded decoded body")
	}
	defer response.Body.Close()
	body, e := io.ReadAll(io.LimitReader(response.Body, hcMetadataResponseLimit+1))
	if e != nil {
		return nil, e
	}
	if len(body) > hcMetadataResponseLimit {
		return nil, fmt.Errorf("GBFS metadata response exceeds %d bytes; no partial response used", hcMetadataResponseLimit)
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	return response, nil
}
func init() {
	registerClientHook(func(c *client.Client) error {
		// Override the sniffed generator's h2-only fingerprint for these verified
		// anonymous JSON endpoints; retain client deadlines, pacing and error policy.
		c.HTTPClient.Transport = hcMetadataTransport{base: http.DefaultTransport.(*http.Transport).Clone()}
		c.HTTPClient.CheckRedirect = func(r *http.Request, via []*http.Request) error {
			return fmt.Errorf("unexpected GBFS metadata redirect to %s", r.URL.Host)
		}
		return nil
	})
}
