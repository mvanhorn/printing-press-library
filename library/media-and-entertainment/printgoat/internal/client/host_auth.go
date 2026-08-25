// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"context"
	"encoding/base64"
	"net/url"
	"os"
	"strings"
)

// authHeaderForURL selects per-source credentials for printgoat's 3-site
// combo. cli-printing-press's multi-spec merge (mergeSpecsWithOptions in
// internal/cli/root.go) does not carry the primary spec's tier_routing
// block into the merged APISpec, so the generated Config only ever reflects
// Printables' auth (none). Thingiverse and Cults3D credentials are resolved
// here by target host instead, independent of the generated Config.
// AuthHeaderForURL exports authHeaderForURL for hand-written novel commands
// (internal/cli/download.go, job_download.go) that stream raw file bytes via
// their own *http.Client rather than routing the request through Client's
// Get/Post. Those downloads need the same per-host credential dispatch as
// ordinary API calls whenever the file URL is proxied through a
// credentialed host (e.g. Thingiverse's api.thingiverse.com download_url,
// as opposed to an unauthenticated CDN direct_url).
func (c *Client) AuthHeaderForURL(ctx context.Context, targetURL string) (string, error) {
	return c.authHeaderForURL(ctx, targetURL)
}

func (c *Client) authHeaderForURL(ctx context.Context, targetURL string) (string, error) {
	host := ""
	if u, err := url.Parse(targetURL); err == nil {
		host = u.Hostname()
	}
	switch {
	case strings.HasSuffix(host, "thingiverse.com"):
		token := strings.TrimSpace(os.Getenv("THINGIVERSE_TOKEN"))
		if token == "" {
			return "", nil
		}
		return "Bearer " + token, nil
	case strings.HasSuffix(host, "cults3d.com"):
		username := strings.TrimSpace(os.Getenv("CULTS3D_USERNAME"))
		apiKey := strings.TrimSpace(os.Getenv("CULTS3D_API_KEY"))
		if username == "" || apiKey == "" {
			return "", nil
		}
		return "Basic " + base64.StdEncoding.EncodeToString([]byte(username+":"+apiKey)), nil
	default:
		return c.authHeader(ctx)
	}
}
