// Copyright 2026 Greg Ceccarelli and contributors. Licensed under Apache-2.0. See LICENSE.
package cobratree

import (
	"context"
	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"strings"
	"testing"
)

func TestMirroredAgentConsentOnlyExplicitBoolean(t *testing.T) {
	pinMCPReadOnlyOff(t)
	root := &cobra.Command{Use: "tella"}
	root.PersistentFlags().Bool("yes", false, "consent")
	parent := &cobra.Command{Use: "agent-chats"}
	parent.AddCommand(&cobra.Command{Use: "start", Short: "Start paid chat", RunE: func(*cobra.Command, []string) error { return nil }})
	root.AddCommand(parent)
	bin := writeArgvHelper(t)
	s := server.NewMCPServer("test", "test")
	RegisterAll(s, root, func() (string, error) { return bin, nil })
	entry := s.ListTools()["agent_chats_start"]
	if entry == nil {
		t.Fatal("missing mirror")
	}
	for _, value := range []any{nil, false, "true", true} {
		args := map[string]any{}
		if value != nil {
			args["yes"] = value
		}
		result, err := entry.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: args}})
		if err != nil || result.IsError {
			t.Fatal(result, err)
		}
		argv := decodeArgvResult(t, result)
		hasYes := strings.Contains(strings.Join(argv, " "), "--yes=true")
		if hasYes != (value == true) {
			t.Fatal(value, argv)
		}
	}
}
func TestLessonMCPArtifactPathsCannotEscape(t *testing.T) {
	for _, name := range []string{"../secret.webp", "/tmp/secret.webp", ".env", "subdir/sheet.webp"} {
		if _, err := lessonMCPArguments("videos storyboard-image", map[string]any{"output": name}); err == nil {
			t.Fatal(name)
		}
	}
	args, err := lessonMCPArguments("videos storyboard-image", map[string]any{"output": "sheet.webp"})
	if err != nil || !strings.HasSuffix(args["output"].(string), "/tella-artifacts/sheet.webp") {
		t.Fatal(args, err)
	}
	if _, err := lessonMCPArguments("lessons pass", map[string]any{"recipe": "credentials.json"}); err == nil {
		t.Fatal("non-recipe allowed")
	}
}
