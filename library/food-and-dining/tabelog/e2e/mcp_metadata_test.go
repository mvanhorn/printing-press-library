package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"testing"
	"time"
)

func TestStdioMCPCatalogDistinguishesOfflineAndRemoteSavedWorkflows(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, recipeMCPBinary(t))
	cmd.Env = recipeEnv(t, t.TempDir())
	in, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close(); _ = cmd.Process.Kill(); _ = cmd.Wait() }()
	encoder := json.NewEncoder(in)
	scanner := bufio.NewScanner(out)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	request := func(id int, method string, params any) map[string]any {
		t.Helper()
		if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}); err != nil {
			t.Fatal(err)
		}
		for scanner.Scan() {
			var message map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
				t.Fatal(err)
			}
			if message["id"] == float64(id) {
				if message["error"] != nil {
					t.Fatalf("MCP catalog error: %v", message["error"])
				}
				return object(t, message["result"])
			}
		}
		t.Fatalf("missing MCP response: %v; %.400s", scanner.Err(), stderr.String())
		return nil
	}
	request(1, "initialize", map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "metadata-regression", "version": "1"}})
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	catalog := request(2, "tools/list", map[string]any{})
	tools := map[string]map[string]any{}
	for _, item := range catalog["tools"].([]any) {
		tool := object(t, item)
		tools[tool["name"].(string)] = tool
	}
	for _, tc := range []struct {
		name               string
		readOnly, external bool
	}{{"lists_refresh", false, true}, {"compare_saved_candidates_offline", true, false}, {"refresh_one_saved_candidate", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			tool := tools[tc.name]
			if tool == nil {
				t.Fatal("required saved workflow missing from stdio catalog")
			}
			hints := object(t, tool["annotations"])
			if v := hints["readOnlyHint"]; (v == true) != tc.readOnly {
				t.Fatalf("read-only hint misstates workflow: %v", hints)
			}
			equal(t, hints["destructiveHint"], false)
			equal(t, hints["openWorldHint"], tc.external)
			if tc.name == "lists_refresh" {
				equal(t, object(t, tool["_meta"])["pp:tenant-gate"], "child-cli")
				if len(object(t, object(t, tool["inputSchema"])["properties"])) == 0 {
					t.Fatal("refresh metadata override lost its argument schema")
				}
			}
		})
	}
}
