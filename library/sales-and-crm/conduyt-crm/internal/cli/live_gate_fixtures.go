// Copyright 2026 Paul Taramona and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

// PATCH(gocli1219-live-gate-fixtures): command metadata the live dogfood matrix reads, kept in one table so the
// generated command files stay as printed (a reprint keeps this file; TestLiveGateFixturesTargetExistingCommands
// fails when a path is renamed or dropped).
//
//   - pp:typed-exit-codes: the documented non-zero outcome of a command whose feature is off for a workspace or that
//     answers an error by design. The matrix records such a run as a skip, never a pass.
//   - pp:happy-args: real inputs for commands whose spec has no derivable example (the API validates the value).
//   - pp:preview-happy-path: the command's run with these args spends and writes nothing (contacts verify-line-type
//     --estimate counts the local store); the paid loop needs --smart-list or --contact-ids.
//   - Example: help examples for groups and commands that printed none, or printed one that does not parse.
type liveGateFixture struct {
	path        string
	annotations map[string]string
	example     string
}

var liveGateFixtures = []liveGateFixture{
	// Documented errors of features that are off for the workspace (AI teammates, SSO / SCIM, Project Blue) or of a
	// browser-only flow: the command exits typed (3 not found, 5 API error) with the API's message.
	{path: "chat get-agents", annotations: map[string]string{"pp:typed-exit-codes": "0,3"}},                      // 404 ai_teammates_off
	{path: "scim get-v2-users", annotations: map[string]string{"pp:typed-exit-codes": "0,3"}},                    // 404 when SCIM is not provisioned
	{path: "sso get-connection", annotations: map[string]string{"pp:typed-exit-codes": "0,3"}},                   // 404 when SSO is off for the key
	{path: "sso get-scim-tokens", annotations: map[string]string{"pp:typed-exit-codes": "0,3"}},                  // 404 when SSO is off for the key
	{path: "settings get-project-blue-lines", annotations: map[string]string{"pp:typed-exit-codes": "0,5"}},      // 409 not_configured
	{path: "conduyt-auth read-sso-signup-consent", annotations: map[string]string{"pp:typed-exit-codes": "0,5"}}, // 410 without a pending browser sign-in
	{path: "chat get-agents-me-mcp", annotations: map[string]string{"pp:typed-exit-codes": "0,5"}},               // 405 by design: the MCP endpoint answers POST only
	// The spec types these CSV bodies as binary: --json is refused with a usage error (2); stdout or --deliver file:.
	{path: "companies get-export", annotations: map[string]string{"pp:typed-exit-codes": "0,2"}},
	{path: "deals get-export", annotations: map[string]string{"pp:typed-exit-codes": "0,2"}},
	// Real inputs the API accepts.
	{path: "dialer get-inbound-lookup", annotations: map[string]string{"pp:happy-args": "--phone=+15555550100"}},
	{path: "emails get-sequences-senders", annotations: map[string]string{"pp:happy-args": "--channel=email"}},
	{path: "imports blame", annotations: map[string]string{"pp:happy-args": "--latest"}},
	{path: "imports watch", annotations: map[string]string{"pp:happy-args": "--latest"}},
	{path: "tail", annotations: map[string]string{"pp:happy-args": "resource=messages;--follow=false"}},
	{path: "contacts verify-line-type", annotations: map[string]string{"pp:happy-args": "--estimate", "pp:preview-happy-path": "true"}},
	// Help examples.
	{path: "drips", example: "  conduyt-crm-pp-cli drips audit --since 2026-07-01 --json"},
	{path: "users", example: "  conduyt-crm-pp-cli users list --json\n  conduyt-crm-pp-cli users invite --email new.rep@example.com --role member"},
	{path: "push unsubscribe", example: "  conduyt-crm-pp-cli push unsubscribe --endpoint https://push.example.com/subscriptions/abc123"},
	// The printed example passed a bool flag as `--use-in-compose true`, which cobra reads as a stray positional.
	{path: "me put-my-email-signature", example: "  conduyt-crm-pp-cli me put-my-email-signature --design-id x02 --options '{\"accent\":\"#0E1F3A\",\"addons\":{\"disclaimer\":{\"on\":true}}}' --overrides '{\"dept\":\"Support\",\"title\":\"Head of Customer Support\"}' --use-in-compose=true"},
}

// findCommandPath resolves a space-separated command path exactly (no alias or prefix matching).
func findCommandPath(root *cobra.Command, path string) *cobra.Command {
	cur := root
	for _, name := range strings.Fields(path) {
		var next *cobra.Command
		for _, child := range cur.Commands() {
			if child.Name() == name {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

func applyLiveGateFixtures(root *cobra.Command) {
	for _, f := range liveGateFixtures {
		cmd := findCommandPath(root, f.path)
		if cmd == nil {
			continue
		}
		if len(f.annotations) > 0 && cmd.Annotations == nil {
			cmd.Annotations = map[string]string{}
		}
		for k, v := range f.annotations {
			cmd.Annotations[k] = v
		}
		if f.example != "" {
			cmd.Example = f.example
		}
	}
}
