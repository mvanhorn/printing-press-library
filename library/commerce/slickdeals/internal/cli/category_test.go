// Copyright 2026 david. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// fixtureRSSCategory is a minimal RSS fixture for category command tests.
const fixtureRSSCategory = `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0"
  xmlns:dc="http://purl.org/dc/elements/1.1/"
  xmlns:content="http://purl.org/rss/1.0/modules/content/">
  <channel>
    <title>Slickdeals Category RSS</title>
    <link>https://slickdeals.net/</link>
    <item>
      <title><![CDATA[Tech Deal $49.99]]></title>
      <link>https://slickdeals.net/f/11111111-tech-deal</link>
      <description>A great tech deal.</description>
      <content:encoded><![CDATA[<div>Thumb Score: +15 </div><a data-product-exitWebsite="amazon.com">link</a>]]></content:encoded>
      <pubDate>Mon, 11 May 26 10:00:00 +0000</pubDate>
      <category domain="https://slickdeals.net/">Computer Deals</category>
      <dc:creator>dealmaster</dc:creator>
      <guid>thread-11111111</guid>
    </item>
    <item>
      <title>Another Tech Deal $29.99</title>
      <link>https://slickdeals.net/f/22222222-another</link>
      <description>Another deal.</description>
      <content:encoded><![CDATA[<div>Thumb Score: +8 </div>]]></content:encoded>
      <pubDate>Mon, 11 May 26 09:00:00 +0000</pubDate>
      <category domain="https://slickdeals.net/">Computer Deals</category>
      <dc:creator>techposter</dc:creator>
      <guid>thread-22222222</guid>
    </item>
  </channel>
</rss>`

// runCategoryCmd executes the category command with a mock HTTP server serving fixture XML.
// It returns the combined stdout output as a string.
func runCategoryCmd(t *testing.T, srv *httptest.Server, args []string, asJSON bool) string {
	t.Helper()
	var flags rootFlags
	if asJSON {
		flags.asJSON = true
	}

	root := &cobra.Command{Use: "root"}
	cmd := newCategoryCmd(&flags)
	root.AddCommand(cmd)

	// We need to intercept the HTTP calls to the category command.
	// The category command calls rss.LiveCategory which uses http.DefaultClient.
	// Swap DefaultClient transport for the test.
	orig := http.DefaultClient
	http.DefaultClient = srv.Client()
	defer func() { http.DefaultClient = orig }()

	var buf bytes.Buffer
	root.SetOut(&buf)
	cmd.SetOut(&buf)

	root.SetArgs(args)
	_ = root.Execute()
	return buf.String()
}

// TestCategoryCmd_List exercises the JSON path because the human-readable
// table is gated behind isTerminal(stdout), and a bytes.Buffer is never a
// terminal — so the printer always falls through to JSON when run from a
// test. TestCategoryCmd_ListJSON below covers the same code path
// explicitly.
func TestCategoryCmd_List(t *testing.T) {
	var flags rootFlags
	var buf bytes.Buffer
	cmd := newCategoryCmd(&flags)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("category --list error: %v", err)
	}
	out := buf.String()
	// v0.3 uses the five Slickdeals-advertised forum IDs only. Check for at
	// least one verified alias rather than the dropped v0.2 "tech" alias.
	if !strings.Contains(out, "hot") || !strings.Contains(out, "freebies") {
		t.Errorf("--list output missing verified category aliases (hot/freebies), got: %s", out)
	}
}

func TestCategoryCmd_ListJSON(t *testing.T) {
	var flags rootFlags
	flags.asJSON = true

	var buf bytes.Buffer
	cmd := newCategoryCmd(&flags)
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{"--list"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("category --list --json error: %v", err)
	}

	var entries []struct {
		ForumID int      `json:"forum_id"`
		Names   []string `json:"names"`
	}
	if err := json.Unmarshal(buf.Bytes(), &entries); err != nil {
		t.Fatalf("--list --json produced invalid JSON: %v\nOutput: %s", err, buf.String())
	}
	if len(entries) == 0 {
		t.Error("--list --json returned empty array")
	}
}

func TestCategoryCmd_NoArgs_ReturnsHelp(t *testing.T) {
	var flags rootFlags
	var buf bytes.Buffer
	cmd := newCategoryCmd(&flags)
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{})
	// Help returns nil, not an error.
	if err := cmd.Execute(); err != nil {
		t.Fatalf("expected nil for no-args, got: %v", err)
	}
}

func TestCategoryCmd_DryRun(t *testing.T) {
	var flags rootFlags
	flags.dryRun = true

	var buf bytes.Buffer
	cmd := newCategoryCmd(&flags)
	cmd.SetOut(&buf)

	cmd.SetArgs([]string{"tech"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("--dry-run should not error: %v", err)
	}
	// No output expected in dry-run mode.
}

func TestCategoryCmd_UnknownName(t *testing.T) {
	var flags rootFlags

	cmd := newCategoryCmd(&flags)
	cmd.SetArgs([]string{"not-a-real-category-xyz"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error for unknown category name, got nil")
	}
}

func TestCategoryCmd_WithMockServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = w.Write([]byte(fixtureRSSCategory))
	}))
	defer srv.Close()

	// Point category command at mock server by overriding http.DefaultTransport.
	origTransport := http.DefaultTransport
	http.DefaultTransport = &hostRewriteTransport{
		base:    srv.Client().Transport,
		replace: srv.URL,
	}
	defer func() { http.DefaultTransport = origTransport }()

	var flags rootFlags
	flags.asJSON = true

	var buf bytes.Buffer
	cmd := newCategoryCmd(&flags)
	cmd.SetOut(&buf)
	// v0.3: "hot" is the canonical alias for forum 9 (Slickdeals Hot Deals
	// Forum). v0.2's "tech" alias was dropped because it pointed at the wrong
	// forum ID (25 = Contests, not Computers).
	cmd.SetArgs([]string{"hot", "--limit", "2"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("category hot error: %v", err)
	}

	// Output should be valid JSON with a "results" key.
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(buf.Bytes(), &envelope); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, buf.String())
	}
	if _, ok := envelope["results"]; !ok {
		t.Errorf("output missing 'results' key; got keys: %v", mapKeys(envelope))
	}
	if _, ok := envelope["meta"]; !ok {
		t.Errorf("output missing 'meta' key; got keys: %v", mapKeys(envelope))
	}
}

// hostRewriteTransport rewrites all requests to a fixed base URL (for test isolation).
type hostRewriteTransport struct {
	base    http.RoundTripper
	replace string // e.g. "http://127.0.0.1:PORT"
}

func (t *hostRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL.Scheme = "http"
	clone.URL.Host = strings.TrimPrefix(t.replace, "http://")
	if t.base != nil {
		return t.base.RoundTrip(clone)
	}
	return http.DefaultTransport.RoundTrip(clone)
}

func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
