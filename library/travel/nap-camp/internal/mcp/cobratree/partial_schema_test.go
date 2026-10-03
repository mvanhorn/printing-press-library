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
