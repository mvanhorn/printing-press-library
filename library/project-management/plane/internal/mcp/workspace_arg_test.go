// Copyright 2026 Anton Sidorov aka anticodeguy and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/client"
	"github.com/mvanhorn/printing-press-library/library/project-management/plane/internal/config"
)

func newTestClient() *client.Client {
	cfg := &config.Config{
		BaseURL:      "https://api.plane.so/api/v1/workspaces/{slug}",
		TemplateVars: map[string]string{"slug": "default-ws"},
	}
	return client.New(cfg, 0, 0)
}

func TestApplyWorkspaceArg_OverridesSlug(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"bare slug", "bbm", "bbm"},
		{"trailing slash", "bbm/", "bbm"},
		{"browser url", "https://app.plane.so/bbm/projects/x", "bbm"},
		{"api base prefix", "https://plane.bbm.academy/api/v1/workspaces/bbm", "bbm"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newTestClient()
			args := map[string]any{"workspace": tc.in, "endpoint_id": "x"}
			applyWorkspaceArg(c, args)
			if got := c.Config.TemplateVars["slug"]; got != tc.want {
				t.Fatalf("slug = %q, want %q", got, tc.want)
			}
			if _, leaked := args["workspace"]; leaked {
				t.Fatalf("workspace arg must be consumed, not leaked into args")
			}
			if args["endpoint_id"] != "x" {
				t.Fatalf("unrelated args must be preserved")
			}
		})
	}
}

func TestApplyWorkspaceArg_NoopCases(t *testing.T) {
	// Absent arg leaves the config-resolved slug untouched.
	c := newTestClient()
	applyWorkspaceArg(c, map[string]any{"endpoint_id": "x"})
	if got := c.Config.TemplateVars["slug"]; got != "default-ws" {
		t.Fatalf("absent workspace: slug = %q, want default-ws", got)
	}

	// Blank/whitespace arg is a no-op but is still consumed.
	c = newTestClient()
	args := map[string]any{"workspace": "   "}
	applyWorkspaceArg(c, args)
	if got := c.Config.TemplateVars["slug"]; got != "default-ws" {
		t.Fatalf("blank workspace: slug = %q, want default-ws (unchanged)", got)
	}
	if _, leaked := args["workspace"]; leaked {
		t.Fatalf("blank workspace arg must still be consumed")
	}

	// Non-string arg is consumed without mutating the slug.
	c = newTestClient()
	args = map[string]any{"workspace": 42}
	applyWorkspaceArg(c, args)
	if got := c.Config.TemplateVars["slug"]; got != "default-ws" {
		t.Fatalf("non-string workspace: slug = %q, want default-ws", got)
	}
	if _, leaked := args["workspace"]; leaked {
		t.Fatalf("non-string workspace arg must be consumed")
	}

	// Nil args / nil client must not panic.
	applyWorkspaceArg(nil, nil)
	applyWorkspaceArg(newTestClient(), nil)
}
