// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestCompactDiscoveryPreservesEveryCommandAndSafetyAnnotation(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	full := buildAgentContext(root)
	compact := compactAgentContext(root)
	var compare func([]agentContextCommand, []agentContextCommand)
	compare = func(a, b []agentContextCommand) {
		if len(a) != len(b) {
			t.Fatal("commands lost")
		}
		for i := range a {
			if a[i].Name != b[i].Name || a[i].Use != b[i].Use || a[i].Short != b[i].Short || !reflect.DeepEqual(a[i].Annotations, b[i].Annotations) {
				t.Fatal("command or safety annotation lost", a[i].Name)
			}
			if len(b[i].Flags) > 0 {
				t.Fatal("eager flags")
			}
			compare(a[i].Subcommands, b[i].Subcommands)
		}
	}
	compare(full.Commands, compact.Commands)
	f, _ := json.Marshal(full)
	c, _ := json.Marshal(compact)
	if len(c)*100 > len(f)*65 {
		t.Fatalf("insufficient discovery reduction: %d vs %d", len(c), len(f))
	}
	if compact.SchemaVersion != "4" || full.SchemaVersion != "3" {
		t.Fatal("schema contract")
	}
}
func TestDiscoveryScopedFullAndInvalidModes(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
		bad  bool
	}{
		{nil, `"view":"summary"`, false}, {[]string{"--full"}, `"schema_version":"3"`, false}, {[]string{"--command", "plan edit"}, `"changes-file"`, false}, {[]string{"--command", "plan invented"}, "", true}, {[]string{"--full", "--for-edit"}, "", true}, {[]string{"unexpected"}, "", true},
	} {
		root := newRootCmd(&rootFlags{})
		cmd := newAgentContextCmd(root, &rootFlags{})
		var b bytes.Buffer
		cmd.SetOut(&b)
		cmd.SetErr(&b)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		if (err != nil) != tc.bad {
			t.Fatalf("%v %v", tc.args, err)
		}
		if !tc.bad && !strings.Contains(b.String(), tc.want) {
			t.Fatalf("%v missing %s", tc.args, tc.want)
		}
	}
}

func TestTaskDiscoveryUsesLiveSchemasAndExcludesUnrelatedInventory(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, task := range []string{"review", "create", "edit"} {
		payload, err := taskAgentContext(root, task)
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(payload)
		var view struct {
			Steps []struct {
				Path    string
				Command agentContextCommand
			}
		}
		if err := json.Unmarshal(data, &view); err != nil {
			t.Fatal(err)
		}
		if len(view.Steps) < 3 || len(view.Steps) > 5 {
			t.Fatal("unbounded workflow", task)
		}
		for _, step := range view.Steps {
			scoped, err := scopedAgentContext(root, step.Path)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := json.Marshal(scoped.(map[string]any)["command"])
			got, _ := json.Marshal(step.Command)
			if !bytes.Equal(want, got) {
				t.Fatalf("%s loses schema or safety", step.Path)
			}
		}
		if bytes.Contains(data, []byte("feedback_endpoint_configured")) || bytes.Contains(data, []byte("collaborators")) {
			t.Fatal("unrelated inventory leaked")
		}
	}
	for _, args := range [][]string{{"--task", "unknown"}, {"--task", "review", "--full"}, {"--task", "edit", "--command", "plan day"}, {"--task", "create", "--for-edit"}} {
		cmd := newAgentContextCmd(root, &rootFlags{})
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil {
			t.Fatal("invalid combination accepted", args)
		}
	}
	cmd := newAgentContextCmd(root, &rootFlags{})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--task", "edit"})
	if err := cmd.Execute(); err != nil || !strings.Contains(out.String(), `"view":"task"`) {
		t.Fatal(err, out.String())
	}
}
