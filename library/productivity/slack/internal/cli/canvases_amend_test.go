// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/slack/internal/config"
)

func TestPostCanvasReportsVerifyMutationAsNoop(t *testing.T) {
	profileDir := t.TempDir()
	t.Setenv("HOME", profileDir)
	t.Setenv("USERPROFILE", profileDir)
	t.Setenv("XDG_CONFIG_HOME", profileDir)
	t.Setenv("SLACK_BASE_URL", "http://127.0.0.1:1")
	t.Setenv("SLACK_BOT_TOKEN", "xoxb-test-placeholder")
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")

	flags := &rootFlags{asJSON: true, noCache: true}
	cmd := newCanvasesCreateCmd(flags)
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--title", "Verify-only canvas"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute canvases create in verify mode: %v", err)
	}
	var output struct {
		Success    bool `json:"success"`
		VerifyNoop bool `json:"verify_noop"`
		Data       struct {
			Synthetic bool   `json:"__pp_verify_synthetic__"`
			Status    string `json:"status"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("decode canvas output: %v\n%s", err, stdout.String())
	}
	if output.Success {
		t.Fatal("verify-mode canvas mutation reported success")
	}
	if !output.VerifyNoop || !output.Data.Synthetic || output.Data.Status != "noop" {
		t.Fatalf("verify-mode output = %#v, want an explicit synthetic no-op", output)
	}
}

func TestPostCanvasReportsVerifyNoopAfterOutputFilters(t *testing.T) {
	for _, tc := range []struct {
		name  string
		flags rootFlags
	}{
		{"select", rootFlags{asJSON: true, noCache: true, selectFields: "status"}},
		{"compact", rootFlags{asJSON: true, noCache: true, compact: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profileDir := t.TempDir()
			t.Setenv("HOME", profileDir)
			t.Setenv("USERPROFILE", profileDir)
			t.Setenv("XDG_CONFIG_HOME", profileDir)
			t.Setenv("SLACK_BASE_URL", "http://127.0.0.1:1")
			t.Setenv("SLACK_BOT_TOKEN", "xoxb-test-placeholder")
			t.Setenv("PRINTING_PRESS_VERIFY", "1")
			t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")

			flags := &tc.flags
			cmd := newCanvasesCreateCmd(flags)
			var stdout bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"--title", "Verify-only canvas"})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("execute canvases create in verify mode: %v", err)
			}
			var output struct {
				Success    bool           `json:"success"`
				VerifyNoop bool           `json:"verify_noop"`
				Data       map[string]any `json:"data"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
				t.Fatalf("decode canvas output: %v\n%s", err, stdout.String())
			}
			if output.Success || !output.VerifyNoop {
				t.Fatalf("filtered verify-mode output = %#v, want success=false and verify_noop=true", output)
			}
			if tc.name == "select" {
				if _, ok := output.Data["__pp_verify_synthetic__"]; ok {
					t.Fatalf("select output retained synthetic marker: %#v", output.Data)
				}
			}
		})
	}
}

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
