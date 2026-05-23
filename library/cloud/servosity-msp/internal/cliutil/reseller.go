// Copyright 2026 servosity. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// Client is the subset of the HTTP client surface ResolveResellerID needs.
// Defined here (not imported) to keep cliutil free of the cli/client cycle.
// The Get signature matches client.Client.Get exactly.
type Client interface {
	Get(path string, params map[string]string) (json.RawMessage, error)
}

// ResolveResellerID returns the authenticated reseller's numeric ID. It
// resolves in this order:
//
//  1. SERVOSITY_MSP_RESELLER_ID env var (operator override; useful for CI)
//  2. The `reseller` URL field on the first company returned by /companies/
//     — every company in a partner-scoped tenant is keyed to the same
//     reseller, so the first hit is authoritative.
//
// Returns an error if neither source resolves.
func ResolveResellerID(ctx context.Context, c Client) (int64, error) {
	// Override: env var
	if s := os.Getenv("SERVOSITY_MSP_RESELLER_ID"); s != "" {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("SERVOSITY_MSP_RESELLER_ID is set but not a valid integer: %q", s)
		}
		return n, nil
	}

	// Probe /companies/ — partner tokens always have access; the first
	// record exposes the reseller URL.
	body, err := c.Get("/companies/", nil)
	if err != nil {
		return 0, fmt.Errorf("ResolveResellerID: GET /companies/ failed: %w", err)
	}
	// Response shape probed against the live API: paginated
	//   { "count": N, "next": ..., "previous": ..., "results": [ {"reseller": "https://.../resellers/{id}/", ...}, ... ] }
	// — modernized partner APIs may add count/next/previous wrappers.
	var env struct {
		Results []struct {
			Reseller string `json:"reseller"`
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return 0, fmt.Errorf("ResolveResellerID: companies response not parseable: %w", err)
	}
	if len(env.Results) == 0 {
		// Empty company list — tenant has no companies yet (new partner).
		// Caller must surface a clear error directing user to set the env var.
		return 0, fmt.Errorf("ResolveResellerID: no companies in account; set SERVOSITY_MSP_RESELLER_ID with your reseller ID from the partner portal")
	}
	return parseResellerURL(env.Results[0].Reseller)
}

var resellerURLRE = regexp.MustCompile(`/resellers/(\d+)/?$`)

// parseResellerURL extracts the numeric ID from a URL like
// "https://api.servosity.com/api/v1/resellers/2/". Exposed for tests.
func parseResellerURL(url string) (int64, error) {
	if url == "" {
		return 0, fmt.Errorf("parseResellerURL: empty URL")
	}
	m := resellerURLRE.FindStringSubmatch(url)
	if m == nil {
		return 0, fmt.Errorf("parseResellerURL: no reseller ID found in %q", url)
	}
	return strconv.ParseInt(m[1], 10, 64)
}
