// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/config"
)

// Canvas bodies come back as the HTML Slack serves from url_private_download.
// --format text only needs to be greppable, but it must not drop visible text or
// leak tag soup.
func TestCanvasHTMLToText(t *testing.T) {
	tests := []struct {
		name string
		html string
		want []string
		deny []string
	}{
		{
			name: "headings and paragraph survive",
			html: `<div class="quip-canvas-content"><h1 id="temp:C:aaa">Title</h1><p id="temp:C:bbb" class="line">Body text</p></div>`,
			want: []string{"Title", "Body text"},
			deny: []string{"<h1", "quip-canvas-content", "temp:C:aaa"},
		},
		{
			name: "list items each land on their own line",
			html: `<ul><li>one</li><li>two</li></ul>`,
			want: []string{"one\ntwo"},
		},
		{
			name: "attribute values are not emitted as text",
			html: `<p id="temp:C:ccc" class="line" data-x="should-not-appear">visible</p>`,
			want: []string{"visible"},
			deny: []string{"should-not-appear", "temp:C:ccc"},
		},
		{
			name: "blank runs collapse",
			html: `<div></div><div></div><p>only</p>`,
			want: []string{"only"},
			deny: []string{"\n\n"},
		},
		{
			name: "empty input stays empty",
			html: ``,
			want: []string{""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := canvasHTMLToText(tc.html)
			for _, w := range tc.want {
				if !strings.Contains(got, w) {
					t.Errorf("expected %q in output, got %q", w, got)
				}
			}
			for _, d := range tc.deny {
				if d != "" && strings.Contains(got, d) {
					t.Errorf("did not expect %q in output, got %q", d, got)
				}
			}
		})
	}
}

// FetchRaw sends the workspace credential, so it must refuse any host that is not
// Slack. files.info supplies the URL, but a compromised or spoofed response must
// not be able to redirect the token to an attacker.
func TestFetchRawRefusesNonSlackHosts(t *testing.T) {
	c := client.New(&config.Config{SlackUserToken: "xoxp-not-a-real-token"}, 0, 0)

	refuse := []string{
		"https://evil.example.com/files-pri/x/download/canvas",
		"https://slack.com.evil.example.com/canvas",
		"https://notslack.com/canvas",
		"http://files.slack.com/files-pri/x/download/canvas", // plaintext
		"https://files.slack.com.attacker.net/canvas",
	}
	for _, u := range refuse {
		t.Run(u, func(t *testing.T) {
			if _, _, err := c.FetchRaw(context.Background(), u); err == nil {
				t.Fatalf("expected refusal for %q, got nil error", u)
			}
		})
	}
}

// The legitimate hosts must not be refused by the guard. These do not reach the
// network: a bad token means the request fails later, so the assertion is only
// that the failure is not the host check.
func TestFetchRawAllowsSlackHosts(t *testing.T) {
	c := client.New(&config.Config{SlackUserToken: "xoxp-not-a-real-token"}, 0, 0)

	for _, u := range []string{
		"https://files.slack.com/files-pri/T000/download/canvas",
		"https://slack.com/api/files.info",
	} {
		t.Run(u, func(t *testing.T) {
			_, _, err := c.FetchRaw(context.Background(), u)
			if err != nil && strings.Contains(err.Error(), "refusing to send credentials") {
				t.Fatalf("host %q should be allowed, got %v", u, err)
			}
		})
	}
}
