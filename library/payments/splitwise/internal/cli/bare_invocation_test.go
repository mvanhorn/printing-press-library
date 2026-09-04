// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBareNoInputNovelCommandsReturnJSON(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Clear the XDG overrides too: DataDir/ConfigDir honour XDG_*_HOME before HOME.
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	commands := []string{
		"balances",
		"debts",
		"net",
		"audit",
		"recurring",
		"forecast",
		"activity",
		"spend",
		"fairness",
		"brief",
		"normalize",
		"report",
	}
	for _, name := range commands {
		t.Run(name, func(t *testing.T) {
			cmd := RootCmd()
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{name})

			if err := cmd.Execute(); err != nil {
				t.Fatalf("bare %s error = %v; stderr=%s", name, err, stderr.String())
			}
			got := stdout.String()
			if strings.Contains(got, "Usage:") {
				t.Fatalf("bare %s printed help instead of data:\n%s", name, got)
			}
			var decoded any
			if err := json.Unmarshal(stdout.Bytes(), &decoded); err != nil {
				t.Fatalf("bare %s stdout is not JSON: %v\nstdout=%s\nstderr=%s", name, err, got, stderr.String())
			}
			switch decoded.(type) {
			case map[string]any, []any:
			default:
				t.Fatalf("bare %s JSON must be an object or array, got %T", name, decoded)
			}
		})
	}
}

func TestBareBalancesHelpStillPrintsUsage(t *testing.T) {
	cmd := RootCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stdout)
	cmd.SetArgs([]string{"balances", "--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("balances --help error = %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("balances --help missing Usage:\n%s", stdout.String())
	}
}
