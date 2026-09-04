package cobratree

import (
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestToolOptionsForFlagsOmitsBlockedRootFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	for name := range blockedRootFlags {
		root.PersistentFlags().String(name, "", "blocked")
	}
	root.PersistentFlags().Bool("json", false, "json output")
	child := &cobra.Command{Use: "find"}
	child.Flags().String("format", "", "file format")
	root.AddCommand(child)

	tool := mcplib.NewTool("find", toolOptionsForFlags(child)...)
	props := tool.InputSchema.Properties
	for name := range blockedRootFlags {
		if _, ok := props[name]; ok {
			t.Errorf("blocked flag %q advertised in MCP schema: %#v", name, props)
		}
	}
	if _, ok := props["format"]; !ok {
		t.Fatalf("command-local --format missing from schema: %#v", props)
	}
	if _, ok := props["json"]; !ok {
		t.Fatalf("inherited --json missing from schema: %#v", props)
	}
}

func TestVisitToolFlagsSkipsBlockedRootFlags(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("config", "", "config file")
	root.PersistentFlags().String("deliver", "", "deliver")
	root.PersistentFlags().String("profile", "", "profile")
	root.PersistentFlags().String("base-url", "", "base url")
	root.PersistentFlags().String("timeout", "", "timeout")
	cmd := &cobra.Command{Use: "find"}
	cmd.Flags().String("format", "", "format")
	root.AddCommand(cmd)

	names := map[string]bool{}
	visitToolFlags(cmd, func(flag *pflag.Flag) {
		names[flag.Name] = true
	})
	for _, blocked := range []string{"config", "deliver", "profile", "base-url"} {
		if names[blocked] {
			t.Errorf("blocked flag %q exposed in tool schema", blocked)
		}
	}
	if !names["format"] {
		t.Error("per-command flag format missing from tool schema")
	}
	if !names["timeout"] {
		t.Error("unblocked inherited flag timeout missing from tool schema")
	}
}

func TestVisitToolFlagsKeepsLocalFlagEvenIfBlockedName(t *testing.T) {
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().String("profile", "", "root profile")
	cmd := &cobra.Command{Use: "find"}
	cmd.Flags().String("profile", "", "local profile override")
	root.AddCommand(cmd)

	names := map[string]bool{}
	visitToolFlags(cmd, func(flag *pflag.Flag) {
		names[flag.Name] = true
	})
	if !names["profile"] {
		t.Fatal("local flag named profile should still appear in the tool schema")
	}
}
