// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

func fixtureCommandTree() *cobra.Command {
	root := &cobra.Command{Use: "wanderlog-pp-cli"}
	root.AddCommand(newNovelPlanCmd(&rootFlags{}), newLodgingCmd(&rootFlags{}))
	return root
}
func fixtureFind(t *testing.T, root *cobra.Command, path string) *cobra.Command {
	t.Helper()
	cmd, args, err := root.Find(strings.Fields(path))
	if err != nil || len(args) != 0 {
		t.Fatalf("find %s: %v %v", path, args, err)
	}
	return cmd
}

func TestDogfoodFixtureAnnotationsAreOptInAndPreserveRuntimeFlags(t *testing.T) {
	root := fixtureCommandTree()
	now := time.Date(2030, 1, 20, 0, 0, 0, 0, time.UTC)
	configureWanderlogDogfoodFixtures(root, func(string) string { return "" }, now)
	for _, path := range []string{"plan edit", "plan block get", "plan block rename", "plan section delete"} {
		if fixtureFind(t, root, path).Annotations["pp:happy-args"] != "" {
			t.Fatalf("opt-out command changed: %s", path)
		}
	}
	lodging := fixtureFind(t, root, "lodging search")
	if got := lodging.Annotations["pp:happy-args"]; got != "--start-date=2030-02-19;--end-date=2030-02-26" {
		t.Fatal(got)
	}
	if lodging.Flags().Lookup("start-date").Value.String() != "" || lodging.Flags().Lookup("end-date").Value.String() != "" {
		t.Fatal("fixture annotations changed actual search dates")
	}
}

func TestDogfoodFixtureTargetsAndMutatorDryRuns(t *testing.T) {
	root := fixtureCommandTree()
	env := map[string]string{"WANDERLOG_DOGFOOD_PLAN_KEY": "abcdefghijklmnop", "WANDERLOG_DOGFOOD_NOTE_BLOCK_ID": "101", "WANDERLOG_DOGFOOD_CHANGES_FILE": "/tmp/fixture folder/changes;reviewed.json"}
	configureWanderlogDogfoodFixtures(root, func(key string) string { return env[key] }, time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC))
	paths := []string{"plan block attachment remove", "plan block rename", "plan budget expense edit", "plan budget expense remove", "plan budget payment remove", "plan checklist item add", "plan checklist item check", "plan checklist item remove", "plan place replace", "plan reservation edit", "plan reservation remove", "plan section delete", "plan edit"}
	for _, path := range paths {
		cmd := fixtureFind(t, root, path)
		got := cmd.Annotations["pp:happy-args"]
		if !strings.HasPrefix(got, "--target-key=abcdefghijklmnop;") {
			t.Fatalf("unsafe fixture %s: %s", path, got)
		}
		for _, name := range []string{"--dry-run", "--markdown", "--checked", "--apply"} {
			if strings.Contains(got, name+"=") {
				t.Fatalf("boolean annotation unsupported by runner: %s", got)
			}
		}
		if !strings.Contains(cmd.Example, "--dry-run") {
			t.Fatalf("mutator example must retain real dry-run flag: %s", path)
		}
		if flag := cmd.Flags().Lookup("apply"); flag != nil && flag.Value.String() != "false" {
			t.Fatal("fixture changed apply flag")
		}
	}
	if got := fixtureFind(t, root, "plan edit").Annotations["pp:happy-args"]; !strings.Contains(got, `--changes-file=/tmp/fixture folder/changes\;reviewed.json`) {
		t.Fatal("runner semicolon escaping lost", got)
	}
	if got := fixtureFind(t, root, "plan block get").Annotations["pp:happy-args"]; !strings.Contains(got, "--block-id=101") {
		t.Fatal(got)
	}
	if got := fixtureFind(t, root, "plan section delete").Annotations["pp:happy-args"]; !strings.Contains(got, "--day=8") {
		t.Fatal(got)
	}
	if got := fixtureFind(t, root, "plan reservation edit").Annotations["pp:happy-args"]; !strings.Contains(got, "--day=2;--block-index=0") {
		t.Fatal(got)
	}
	if got := fixtureFind(t, root, "plan checklist item remove").Annotations["pp:happy-args"]; !strings.Contains(got, "--day=1;--block-index=2;--item-index=0") {
		t.Fatal(got)
	}
}

func TestDogfoodMissingOptionalFixturesDoNotInventInputs(t *testing.T) {
	root := fixtureCommandTree()
	configureWanderlogDogfoodFixtures(root, func(key string) string {
		if key == "WANDERLOG_DOGFOOD_PLAN_KEY" {
			return "abcdefghijklmnop"
		}
		return ""
	}, time.Now())
	for _, path := range []string{"plan block get", "plan edit"} {
		if fixtureFind(t, root, path).Annotations["pp:happy-args"] != "" {
			t.Fatalf("invented %s fixture", path)
		}
	}
}

func TestDogfoodMutatorShortHelpExposesRunnerDryRun(t *testing.T) {
	for _, key := range []string{"WANDERLOG_DOGFOOD_PLAN_KEY", "WANDERLOG_DOGFOOD_NOTE_BLOCK_ID", "WANDERLOG_DOGFOOD_CHANGES_FILE"} {
		t.Setenv(key, "")
	}
	for _, path := range []string{"plan edit", "plan block rename", "plan checklist item check", "plan reservation remove", "plan section delete"} {
		root := newRootCmd(&rootFlags{})
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs(append(strings.Fields(path), "--help"))
		if err := root.Execute(); err != nil {
			t.Fatalf("%s help: %v", path, err)
		}
		if !strings.Contains(out.String(), "--dry-run") || !strings.Contains(out.String(), "Common flags:") {
			t.Fatalf("runner cannot discover dry-run for %s: %s", path, out.String())
		}
		if path == "plan checklist item check" && !strings.Contains(out.String(), "--checked --dry-run") {
			t.Fatal("check example lost its boolean flag")
		}
	}
}
