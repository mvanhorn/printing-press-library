// Copyright 2026 Matt and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-written regression tests for the sitemaps feedpath URL-encode fix across
// the submit/get/delete sitemap commands. A feedpath like
// "https://example.com/sitemap.xml" must be percent-encoded as a single path
// segment before it reaches the wire, or Google returns 404.

package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// sitemapsRawURI runs a webmasters sitemaps subcommand against an httptest
// server and returns the raw request URI the server saw.
func sitemapsRawURI(t *testing.T, subcmd, siteUrl, feedpath string) string {
	t.Helper()

	var gotRequestURI string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRequestURI = r.RequestURI
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer srv.Close()

	t.Setenv("GSC_ACCESS_TOKEN", "test-token")
	t.Setenv("GOOGLE_SEARCH_CONSOLE_BASE_URL", srv.URL)

	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{
		"--config", filepath.Join(t.TempDir(), "missing-config.toml"),
		"--json",
		"webmasters",
		subcmd,
		siteUrl,
		feedpath,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("%s returned error: %v\noutput:\n%s", subcmd, err, out.String())
	}
	return gotRequestURI
}

// TestWebmastersSitemapsEncodeFeedpath covers the submit/get/delete sitemap
// commands: an absolute-URL feedpath must travel as one encoded segment. The
// routing defect is the scheme's slashes: "https://" inserted a raw "/" that
// split the feedpath into extra path segments, 404ing the request. Go keeps
// ":" raw (a legal path sub-delimiter) but percent-encodes the slashes. Assert
// the slashes are encoded, no raw scheme reaches the wire, and siteUrl is left
// unchanged.
func TestWebmastersSitemapsEncodeFeedpath(t *testing.T) {
	for _, sub := range []string{"submit-sitemap", "get-sitemap", "delete-sitemap"} {
		t.Run(sub, func(t *testing.T) {
			gotRequestURI := sitemapsRawURI(t, sub, "sc-domain:usenoreply.com", "https://usenoreply.com/sitemap.xml")

			if !strings.Contains(gotRequestURI, "%2F%2Fusenoreply.com%2Fsitemap.xml") {
				t.Fatalf("feedpath slashes were not URL-encoded on the wire: got request URI %q, want it to contain %q", gotRequestURI, "%2F%2Fusenoreply.com%2Fsitemap.xml")
			}
			if strings.Contains(gotRequestURI, "sitemaps/https://") {
				t.Fatalf("feedpath scheme reached the wire raw: %q", gotRequestURI)
			}
			if !strings.Contains(gotRequestURI, "sc-domain:usenoreply.com") {
				t.Fatalf("siteUrl should be left unencoded but was altered: got request URI %q", gotRequestURI)
			}
		})
	}
}

// TestWebmastersSitemapsLeavePlainFeedpathUnchanged ensures a feedpath without
// an absolute scheme still routes correctly across all three commands.
func TestWebmastersSitemapsLeavePlainFeedpathUnchanged(t *testing.T) {
	for _, sub := range []string{"submit-sitemap", "get-sitemap", "delete-sitemap"} {
		t.Run(sub, func(t *testing.T) {
			gotRequestURI := sitemapsRawURI(t, sub, "sc-domain:usenoreply.com", "sitemap.xml")

			if !strings.Contains(gotRequestURI, "/sitemaps/sitemap.xml") {
				t.Fatalf("plain feedpath was unexpectedly altered: got request URI %q", gotRequestURI)
			}
		})
	}
}
