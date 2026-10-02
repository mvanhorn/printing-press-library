// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"testing"
)

func TestMCPMarketResourcePathKeepsIDsInOneSegment(t *testing.T) {
	for _, tc := range []struct {
		id, wantURI string
	}{
		{id: "firecrawl", wantURI: "/server/firecrawl"},
		{id: "acme/private", wantURI: "/server/acme%2Fprivate"},
		{id: "..", wantURI: "/server/%2E%2E"},
		{id: "agent tool", wantURI: "/server/agent%20tool"},
	} {
		req, err := http.NewRequest(http.MethodGet, "https://mcpmarket.com"+mcpMarketResourcePath("server", tc.id), nil)
		if err != nil {
			t.Fatalf("request for %q: %v", tc.id, err)
		}
		if got := req.URL.RequestURI(); got != tc.wantURI {
			t.Errorf("request URI for %q = %q, want %q", tc.id, got, tc.wantURI)
		}
	}
}
