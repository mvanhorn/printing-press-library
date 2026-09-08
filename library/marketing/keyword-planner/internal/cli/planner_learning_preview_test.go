// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlannerLearningDryRunJSONEmitsPreviewWithoutPersistence(t *testing.T) {
	for _, test := range []struct {
		name   string
		args   func(dbPath string) []string
		action string
	}{
		{
			name: "teach",
			args: func(dbPath string) []string {
				return []string{
					"teach",
					"--query", "find all widgets in inventory",
					"--resource", "widget-42",
					"--resource-type", "items",
					"--db", dbPath,
					"--dry-run", "--json",
				}
			},
			action: "teach",
		},
		{
			name: "playbook amend",
			args: func(dbPath string) []string {
				return []string{
					"playbook", "amend",
					"--query", "record all widgets in inventory",
					"--add-note", "use the exact resource ID",
					"--db", dbPath,
					"--dry-run", "--json",
				}
			},
			action: "playbook amend",
		},
		{
			name: "teach-playbook",
			args: func(dbPath string) []string {
				return []string{
					"teach-playbook",
					"--query", "record all widgets in inventory",
					"--notes", "use the exact resource ID",
					"--db", dbPath,
					"--dry-run", "--json",
				}
			},
			action: "teach-playbook",
		},
		{
			name: "teach-pattern",
			args: func(dbPath string) []string {
				return []string{
					"teach-pattern",
					"--query-template", "items in {entity}",
					"--resource-template", "GROUP-{entity:category}",
					"--resource-type", "items",
					"--entity-kind", "category",
					"--db", dbPath,
					"--dry-run", "--json",
				}
			},
			action: "teach-pattern",
		},
		{
			name: "teach-lookup",
			args: func(dbPath string) []string {
				return []string{
					"teach-lookup",
					"--kind", "country",
					"--canonical", "United States",
					"--value", "USA",
					"--db", dbPath,
					"--dry-run", "--json",
				}
			},
			action: "teach-lookup",
		},
		{
			name: "learnings forget",
			args: func(dbPath string) []string {
				return []string{
					"learnings", "forget", "find all widgets in inventory",
					"--all",
					"--db", dbPath,
					"--dry-run", "--json",
				}
			},
			action: "learnings forget",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			dbPath := filepath.Join(home, "learning.db")
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			for _, name := range []string{
				"KEYWORD_PLANNER_CONFIG", "KEYWORD_PLANNER_CONFIG_DIR",
				"KEYWORD_PLANNER_DATA_DIR", "KEYWORD_PLANNER_STATE_DIR",
				"KEYWORD_PLANNER_CACHE_DIR", "KEYWORD_PLANNER_HOME",
				"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME",
			} {
				t.Setenv(name, "")
			}
			t.Setenv(noLearnEnvVar, "")

			root := RootCmd()
			var stdout, stderr strings.Builder
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(test.args(dbPath))
			if err := root.Execute(); err != nil {
				t.Fatalf("dry-run %s: %v (stderr=%q)", test.name, err, stderr.String())
			}

			var result dryRunResult
			if err := json.Unmarshal([]byte(stdout.String()), &result); err != nil {
				t.Fatalf("dry-run %s JSON: %v (stdout=%q)", test.name, err, stdout.String())
			}
			if !result.DryRun || result.Action != test.action || !strings.Contains(result.Would, "no changes made") {
				t.Fatalf("dry-run %s result = %#v, want action %q and no-change preview", test.name, result, test.action)
			}
			assertPlannerLearningHomeEmpty(t, home)
		})
	}
}

func assertPlannerLearningHomeEmpty(t *testing.T, home string) {
	t.Helper()
	err := filepath.WalkDir(home, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path != home {
			t.Errorf("dry-run persisted learning artifact %q", path)
			if entry.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("inspect temp learning home: %v", err)
	}
	if _, err := os.Stat(filepath.Join(home, "learning.db")); !os.IsNotExist(err) {
		t.Fatalf("dry-run learning DB stat error = %v, want absent", err)
	}
}
