// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"testing"
)

func TestShadowDiscoveredCommands_DryRunAndLocalGuard(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	t.Setenv("XDG_DATA_HOME", tmp)
	t.Setenv("XDG_CONFIG_HOME", tmp)

	dryRunCases := []struct {
		name string
		args []string
	}{
		{"account info", []string{"account", "info", "--dry-run"}},
		{"account subscriptions", []string{"account", "subscriptions", "--dry-run"}},
		{"account sessions", []string{"account", "sessions", "--dry-run"}},
		{"briefs list", []string{"briefs", "list", "--dry-run"}},
		{"briefs follow-ups", []string{"briefs", "follow-ups", "--dry-run"}},
		{"people list", []string{"people", "list", "--dry-run"}},
		{"workspaces members", []string{"workspaces", "members", "ws-123", "--dry-run"}},
		{"workspaces notes-count", []string{"workspaces", "notes-count", "--dry-run"}},
		{"workspaces integrations", []string{"workspaces", "integrations", "--workspace", "ws-123", "--dry-run"}},
		{"workspaces mcp-agents", []string{"workspaces", "mcp-agents", "--workspace", "ws-123", "--dry-run"}},
	}

	for _, tc := range dryRunCases {
		tc := tc
		t.Run("dry-run/"+tc.name, func(t *testing.T) {
			var out bytes.Buffer
			rc := RootCmd()
			rc.SetOut(&out)
			rc.SetErr(&out)
			rc.SetArgs(tc.args)
			if err := rc.Execute(); err != nil {
				t.Fatalf("%s --dry-run failed: %v", tc.name, err)
			}
		})
	}

	localCases := [][]string{
		{"account", "info", "--data-source", "local"},
		{"briefs", "list", "--data-source", "local"},
		{"people", "list", "--data-source", "local"},
		{"workspaces", "members", "ws-123", "--data-source", "local"},
		{"workspaces", "notes-count", "--data-source", "local"},
		{"workspaces", "integrations", "--data-source", "local"},
		{"workspaces", "mcp-agents", "--data-source", "local"},
	}

	for _, args := range localCases {
		var out bytes.Buffer
		rc := RootCmd()
		rc.SetOut(&out)
		rc.SetErr(&out)
		rc.SetArgs(args)
		if err := rc.Execute(); err == nil {
			t.Errorf("%v: expected error with --data-source local", args)
		}
	}
}
