package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cli"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/cliutil/testenv"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/mcp/cobratree"
	"github.com/mvanhorn/printing-press-library/library/travel/kurumatabi/internal/parks"
)

func TestActualComparisonPartialThroughMirrorAndRecipe(t *testing.T) {
	// Build before isolating HOME so existing module/toolchain caches can be read.
	bin := filepath.Join(t.TempDir(), "companion")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	build := exec.Command("go", "build", "-o", bin, "../../cmd/kurumatabi-pp-cli")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("companion build: %v: %s", err, output)
	}
	testenv.Isolate(t, cliutil.DataDir)
	t.Setenv("KURUMATABI_NO_LEARN", "")
	dir, err := cliutil.DataDir()
	if err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile("../parks/testdata/rvpark-1086.html")
	if err != nil {
		t.Fatal(err)
	}
	p, err := parks.ParseDetail("rvpark/1086", body, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = parks.Save(context.Background(), filepath.Join(dir, "data.db"), []parks.Park{p}); err != nil {
		t.Fatal(err)
	}
	root := cli.RootCmd()
	s := server.NewMCPServer("test", "0")
	cobratree.RegisterAll(s, root, func() (string, error) { return bin, nil })
	tool, ok := s.ListTools()["parks_compare"]
	if !ok {
		t.Fatal("native compare was not mirrored")
	}
	req := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"args": "rvpark/1086 yypark/213", "agent": true, "data-source": "local", "no-learn": false}}}
	mirrored, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	assertPartialParkMCP(t, mirrored)
	named := mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"park-id": "rvpark/1086 yypark/213", "agent": true, "data-source": "local", "no-learn": false}}}
	namedPartial, err := tool.Handler(context.Background(), named)
	if err != nil {
		t.Fatal(err)
	}
	assertPartialParkMCP(t, namedPartial)
	// The recipe has no source-mode argument; trusted companion selection is
	// simulated by a wrapper that forces the same isolated local scenario.
	wrapper := filepath.Join(t.TempDir(), "local-companion.sh")
	script := "#!/bin/sh\nexec '" + strings.ReplaceAll(bin, "'", "'\\''") + "' \"$@\" --data-source local\n"
	if runtime.GOOS == "windows" {
		wrapper = filepath.Join(filepath.Dir(wrapper), "local-companion.bat")
		script = "@echo off\r\n\"" + bin + "\" %* --data-source local\r\nexit /b %errorlevel%\r\n"
	}
	if err = os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old, oldErr := recipeCLIPath, recipeCLIPathErr
	t.Cleanup(func() { recipeCLIPath, recipeCLIPathErr = old, oldErr })
	recipeCLIPath, recipeCLIPathErr = wrapper, nil
	recipe, err := handleParkComparison(context.Background(), mcplib.CallToolRequest{Params: mcplib.CallToolParams{Arguments: map[string]any{"path": "rvpark/1086", "path2": "yypark/213"}}})
	if err != nil {
		t.Fatal(err)
	}
	assertPartialParkMCP(t, recipe)
	yyBody, err := os.ReadFile("../parks/testdata/yypark-213.html")
	if err != nil {
		t.Fatal(err)
	}
	yy, err := parks.ParseDetail("yypark/213", yyBody, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err = parks.Save(context.Background(), filepath.Join(dir, "data.db"), []parks.Park{yy}); err != nil {
		t.Fatal(err)
	}
	complete, err := tool.Handler(context.Background(), named)
	if err != nil || complete.IsError {
		t.Fatalf("named comparison failed: %v %+v", err, complete)
	}
	var completeEnv struct {
		Meta struct {
			Requested int `json:"requested_records"`
			Compared  int `json:"compared_records"`
		} `json:"meta"`
		Results []parks.CompareField `json:"results"`
	}
	if err = json.Unmarshal([]byte(complete.Content[0].(mcplib.TextContent).Text), &completeEnv); err != nil || completeEnv.Meta.Requested != 2 || completeEnv.Meta.Compared != 2 || len(completeEnv.Results) != 22 {
		t.Fatalf("named variadic envelope lost: %+v err=%v", completeEnv, err)
	}
	named.Params.Arguments = map[string]any{"park-id": "rvpark/1086 --no-learn=false --data-source=live", "agent": true, "data-source": "local"}
	injected, err := tool.Handler(context.Background(), named)
	if err != nil || !injected.IsError || !strings.Contains(injected.Content[0].(mcplib.TextContent).Text, "flag-like") {
		t.Fatalf("named variadic flag accepted: %+v %v", injected, err)
	}
	// The caller did not disable learning; the trusted native adapters did.
	stateDir, err := cliutil.StateDir()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(filepath.Join(stateDir, "learn")); !os.IsNotExist(err) {
		t.Fatalf("read-only planning created learning journal/offset state: %v", err)
	}
}

func assertPartialParkMCP(t *testing.T, result *mcplib.CallToolResult) {
	t.Helper()
	if result == nil || !result.IsError || len(result.Content) != 2 {
		t.Fatalf("partial command must stay failed: %+v", result)
	}
	first, ok := result.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatal("missing JSON block")
	}
	var env struct {
		Meta struct {
			Source    string              `json:"source"`
			Requested int                 `json:"requested_records"`
			Compared  int                 `json:"compared_records"`
			Failures  []map[string]string `json:"fetch_failures"`
		} `json:"meta"`
		Results []parks.CompareField `json:"results"`
	}
	if err := json.Unmarshal([]byte(first.Text), &env); err != nil {
		t.Fatalf("stdout not separate JSON: %v", err)
	}
	if env.Meta.Source != "local" || env.Meta.Requested != 2 || env.Meta.Compared != 1 || len(env.Meta.Failures) != 1 || env.Meta.Failures[0]["id"] != "yypark/213" || len(env.Results) != 22 {
		t.Fatalf("partial evidence lost: %s", first.Text)
	}
	diagnostic, ok := result.Content[1].(mcplib.TextContent)
	if !ok || !strings.Contains(diagnostic.Text, "exit status 5") {
		t.Fatal("exit failure diagnostic lost")
	}
}
