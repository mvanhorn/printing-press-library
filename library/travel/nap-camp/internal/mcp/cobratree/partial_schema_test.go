// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
package cobratree

import (
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"testing"
)

func TestVariadicComparisonAdvertisesFirstPairAndTail(t *testing.T) {
	root := &cobra.Command{Use: "fixture"}
	planner := &cobra.Command{Use: "planner"}
	compare := &cobra.Command{Use: "compare <pair> [<pair>...]", Run: func(*cobra.Command, []string) {}}
	planner.AddCommand(compare)
	root.AddCommand(planner)
	s := server.NewMCPServer("fixture", "test")
	RegisterAll(s, root, func() (string, error) { return "fixture-binary", nil })
	registered := s.GetTool("planner_compare")
	if registered == nil {
		t.Fatal("comparison tool absent")
	}
	if registered.Tool.InputSchema.Properties["pair"] == nil || registered.Tool.InputSchema.Properties["args"] == nil {
		t.Fatalf("required firstpair or advertised tail missing: %#v", registered.Tool.InputSchema)
	}
	if len(registered.Tool.InputSchema.Required) != 1 || registered.Tool.InputSchema.Required[0] != "pair" {
		t.Fatalf("required first pair lost: %#v", registered.Tool.InputSchema.Required)
	}
	slots := positionalArgsForCommand(compare, map[string]bool{})
	if len(slots) != 1 || !slots[0].Required || !slots[0].Variadic {
		t.Fatalf("comparison is not required variadic: %#v", slots)
	}
}

func TestReadOnlyMirrorDoesNotAdvertiseLearningWriteToggle(t *testing.T) {
	root := &cobra.Command{Use: "fixture"}
	root.PersistentFlags().Bool("no-learn", false, "Disable learning")
	read := &cobra.Command{Use: "inspect", Annotations: map[string]string{ReadOnlyAnnotation: "true"}, Run: func(*cobra.Command, []string) {}}
	write := &cobra.Command{Use: "teach", Annotations: map[string]string{LocalWriteAnnotation: "true"}, Run: func(*cobra.Command, []string) {}}
	root.AddCommand(read, write)
	s := server.NewMCPServer("fixture", "test")
	RegisterAll(s, root, func() (string, error) { return "fixture", nil })
	r, w := s.GetTool("inspect"), s.GetTool("teach")
	if r == nil || w == nil {
		t.Fatal("fixture mirrors absent")
	}
	if r.Tool.InputSchema.Properties["no-learn"] != nil {
		t.Fatal("readonly mirror advertised switch to re-enable learning")
	}
	if w.Tool.InputSchema.Properties["no-learn"] == nil {
		t.Fatal("intentional local-write mirror lost learning option")
	}
}

func TestNamedVariadicPairSplitsAndProtectsEveryToken(t *testing.T) {
	slots := []positionalArg{{InputName: "pair", Required: true, Variadic: true}}
	got, err := positionalArgsFromMCP(map[string]any{"pair": "11007:20005062 11008:20005100"}, slots, true, nil)
	if err != nil || len(got) != 2 || got[0] != "11007:20005062" || got[1] != "11008:20005100" {
		t.Fatalf("named variadic %#v %v", got, err)
	}
	for _, value := range []any{"11007:20005062 --help", `11007:20005062 "--no-learn=false"`, []any{"11007:20005062", "11008:20005100"}} {
		if _, err := positionalArgsFromMCP(map[string]any{"pair": value}, slots, true, nil); err == nil {
			t.Fatalf("unsafe or unsupported named input accepted %#v", value)
		}
	}
	question := "pitch Japan with --help in its text"
	scalar, err := positionalArgsFromMCP(map[string]any{"query": question}, []positionalArg{{InputName: "query", Required: true}}, false, nil)
	if err != nil || len(scalar) != 1 || scalar[0] != question {
		t.Fatalf("scalar question was split %#v %v", scalar, err)
	}
}
