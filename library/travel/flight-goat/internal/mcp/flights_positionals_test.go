// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// PATCH(amend-2026-09-29: one bracket per positional) — regression guard for
// the flights MCP tool. A single "[origin destination date]" bracket group in
// the command's Use string collapsed into one schema property whose value was
// passed to the CLI as ONE argv element, failing with "accepts 3 arg(s),
// received 1". The tool must expose origin, destination and date separately.
func TestMCPFlightsToolExposesSeparatePositionals(t *testing.T) {
	s := server.NewMCPServer("flight-goat", "test")
	RegisterTools(s)

	flights, ok := s.ListTools()["flights"]
	if !ok {
		t.Fatal("flights tool missing from registered tools")
	}
	props := flights.Tool.InputSchema.Properties
	for _, name := range []string{"origin", "destination", "date"} {
		if _, ok := props[name]; !ok {
			t.Errorf("flights tool schema missing positional property %q; got %v", name, keys(props))
		}
	}
	if _, ok := props["origin destination date"]; ok {
		t.Error(`flights tool schema still exposes the collapsed "origin destination date" property`)
	}
	for _, req := range flights.Tool.InputSchema.Required {
		switch req {
		case "origin", "destination", "date":
			t.Errorf("positional %q must stay optional (--trip and --segment replace the positional form)", req)
		}
	}
}

func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// PATCH(amend-2026-09-29: one bracket per positional) — call-level guard. The
// schema test above cannot catch a handler that still packs the positionals
// into one argv element or drops the round-trip flags, so invoke the real
// flights tool against an argv-echoing fake CLI and assert the exact argv.
func TestMCPFlightsToolPassesPositionalsAsSeparateArgv(t *testing.T) {
	t.Setenv("FLIGHT_GOAT_CLI_PATH", writeFlightsArgvHelper(t))
	s := server.NewMCPServer("flight-goat", "test")
	RegisterTools(s)
	flights, ok := s.ListTools()["flights"]
	if !ok {
		t.Fatal("flights tool missing from registered tools")
	}

	result, err := flights.Handler(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{
		Name: "flights",
		Arguments: map[string]any{
			"origin":          "AMS",
			"destination":     "BCN",
			"date":            "2026-10-16",
			"return":          "2026-10-20",
			"select-outbound": 2,
		},
	}})
	if err != nil {
		t.Fatalf("flights handler returned transport error: %v", err)
	}
	text := ""
	if len(result.Content) > 0 {
		if tc, ok := result.Content[0].(mcplib.TextContent); ok {
			text = tc.Text
		}
	}
	if result.IsError {
		t.Fatalf("flights handler returned tool error: %s", text)
	}
	var argv []string
	if err := json.Unmarshal([]byte(text), &argv); err != nil {
		t.Fatalf("decode argv: %v; text=%q", err, text)
	}

	// Cobra accepts flags before or after positionals, so pin the command
	// name and the three positionals as separate, ordered trailing elements.
	if len(argv) < 4 || argv[0] != "flights" || !reflect.DeepEqual(argv[len(argv)-3:], []string{"AMS", "BCN", "2026-10-16"}) {
		t.Fatalf("argv = %#v, want flights ... AMS BCN 2026-10-16 as separate elements", argv)
	}
	joined := " " + strings.Join(argv, " ") + " "
	for _, want := range []string{"--return 2026-10-20", "--select-outbound 2"} {
		if !strings.Contains(joined, " "+want+" ") && !strings.Contains(joined, " "+strings.Replace(want, " ", "=", 1)+" ") {
			t.Errorf("argv %#v missing %q", argv, want)
		}
	}
}

// writeFlightsArgvHelper builds a stand-in companion CLI that prints its argv
// as a JSON array, so the test sees exactly what the MCP handler exec'd.
func writeFlightsArgvHelper(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "argvhelper.go")
	body := "package main\n\nimport (\n\t\"encoding/json\"\n\t\"os\"\n)\n\nfunc main() {\n\t_ = json.NewEncoder(os.Stdout).Encode(os.Args[1:])\n}\n"
	if err := os.WriteFile(src, []byte(body), 0o644); err != nil {
		t.Fatalf("write argv helper source: %v", err)
	}
	bin := filepath.Join(dir, "argvhelper")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	if out, err := exec.Command("go", "build", "-o", bin, src).CombinedOutput(); err != nil {
		t.Fatalf("build argv helper: %v\n%s", err, out)
	}
	return bin
}
