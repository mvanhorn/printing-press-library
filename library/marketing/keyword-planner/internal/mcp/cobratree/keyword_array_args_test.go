// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package cobratree

import (
	"reflect"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

func TestStringCollectionFlagsUseMCPStringArraySchemas(t *testing.T) {
	cmd := &cobra.Command{Use: "keywords"}
	cmd.Flags().StringArray("keyword", nil, "submitted keyword")
	cmd.Flags().StringSlice("geo", nil, "target geographies")
	cmd.Flags().String("language", "", "language resource")

	tool := mcplib.NewTool("keywords",
		toolOptionForFlag(cmd.Flag("keyword")),
		toolOptionForFlag(cmd.Flag("geo")),
		toolOptionForFlag(cmd.Flag("language")),
	)
	for _, name := range []string{"keyword", "geo"} {
		property, ok := tool.InputSchema.Properties[name].(map[string]any)
		if !ok {
			t.Fatalf("schema property %q has type %T, want map[string]any", name, tool.InputSchema.Properties[name])
		}
		if got := property["type"]; got != "array" {
			t.Fatalf("schema property %q type = %v, want array", name, got)
		}
		items, ok := property["items"].(map[string]any)
		if !ok {
			t.Fatalf("schema property %q items has type %T, want map[string]any", name, property["items"])
		}
		if got := items["type"]; got != "string" {
			t.Fatalf("schema property %q item type = %v, want string", name, got)
		}
	}
	plain, ok := tool.InputSchema.Properties["language"].(map[string]any)
	if !ok || plain["type"] != "string" {
		t.Fatalf("scalar string schema = %#v, want string property", tool.InputSchema.Properties["language"])
	}
}

func TestMCPStringArrayArgsPreserveItemsThroughCobra(t *testing.T) {
	cmd := &cobra.Command{Use: "keywords"}
	var got []string
	cmd.Flags().StringArrayVar(&got, "keyword", nil, "submitted keyword")

	argv := cliArgsFromMCP(map[string]any{
		"keyword": []any{"salt, pepper", "beef brisket"},
	}, nil)
	wantArgv := []string{"--keyword", "salt, pepper", "--keyword", "beef brisket"}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("MCP StringArray argv = %#v, want %#v", argv, wantArgv)
	}
	if err := cmd.ParseFlags(argv); err != nil {
		t.Fatalf("Cobra parsed StringArray argv with error: %v", err)
	}
	wantValues := []string{"salt, pepper", "beef brisket"}
	if !reflect.DeepEqual(got, wantValues) {
		t.Fatalf("Cobra StringArray values = %#v, want %#v", got, wantValues)
	}
}

func TestMCPStringSliceArgsRetainPflagCommaSemantics(t *testing.T) {
	cmd := &cobra.Command{Use: "keywords"}
	var got []string
	cmd.Flags().StringSliceVar(&got, "keyword", nil, "keyword list")

	argv := cliArgsFromMCP(map[string]any{
		"keyword": []any{"salt,pepper", "beef brisket"},
	}, nil)
	wantArgv := []string{"--keyword", "salt,pepper", "--keyword", "beef brisket"}
	if !reflect.DeepEqual(argv, wantArgv) {
		t.Fatalf("MCP StringSlice argv = %#v, want %#v", argv, wantArgv)
	}
	if err := cmd.ParseFlags(argv); err != nil {
		t.Fatalf("Cobra parsed StringSlice argv with error: %v", err)
	}
	wantValues := []string{"salt", "pepper", "beef brisket"}
	if !reflect.DeepEqual(got, wantValues) {
		t.Fatalf("Cobra StringSlice values = %#v, want %#v", got, wantValues)
	}
}
