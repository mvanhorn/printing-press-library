package e2e

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var recipeMCPBuild struct {
	sync.Once
	path string
	err  error
}

func recipeMCPBinary(t *testing.T) string {
	t.Helper()
	recipeMCPBuild.Do(func() {
		recipeMCPBuild.path = filepath.Join(filepath.Dir(binaryPath), "tabelog-pp-mcp")
		cmd := exec.Command("go", "build", "-o", recipeMCPBuild.path, "./cmd/tabelog-pp-mcp")
		cmd.Dir = ".."
		out, err := cmd.CombinedOutput()
		if err != nil {
			recipeMCPBuild.err = fmt.Errorf("build MCP: %w\n%s", err, out)
		}
	})
	if recipeMCPBuild.err != nil {
		t.Fatal(recipeMCPBuild.err)
	}
	return recipeMCPBuild.path
}

func callStdioRecipe(t *testing.T, env []string, name string, arguments map[string]any) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, recipeMCPBinary(t))
	cmd.Env = env
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
	read := func(id float64) map[string]any {
		for scanner.Scan() {
			var response map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response["id"] == id {
				return response
			}
		}
		t.Fatalf("MCP response missing: %v; stderr=%s", scanner.Err(), stderr.String())
		return nil
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "recipe-regression", "version": "1"}}}); err != nil {
		t.Fatal(err)
	}
	if response := read(1); response["error"] != nil {
		t.Fatalf("MCP initialization failed: %v", response)
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "method": "notifications/initialized"}); err != nil {
		t.Fatal(err)
	}
	if err := encoder.Encode(map[string]any{"jsonrpc": "2.0", "id": 2, "method": "tools/call", "params": map[string]any{"name": name, "arguments": arguments}}); err != nil {
		t.Fatal(err)
	}
	response := read(2)
	if response["error"] != nil {
		t.Fatalf("MCP transport failure: %v", response)
	}
	return object(t, response["result"])
}

func recipeEnv(t *testing.T, home string) []string {
	t.Helper()
	env := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "TABELOG_") {
			env = append(env, value)
		}
	}
	return append(env, "TABELOG_HOME="+home)
}

func TestRecipeRefreshRequiredIDCannotBecomeFlags(t *testing.T) {
	for _, id := range []string{"--dry-run", "--json", "", "   "} {
		t.Run(id, func(t *testing.T) {
			home := t.TempDir()
			result := callStdioRecipe(t, recipeEnv(t, home), "refresh_one_saved_candidate", map[string]any{"slug": "empty-recipe-list", "id": id})
			equal(t, result["isError"], true)
			files := 0
			err := filepath.WalkDir(home, func(_ string, entry os.DirEntry, err error) error {
				if err == nil && !entry.IsDir() {
					files++
				}
				return err
			})
			if err != nil {
				t.Fatal(err)
			}
			equal(t, files, 0)
		})
	}
}

func TestRecipeCompareAcceptsLiteralFlagShapedListName(t *testing.T) {
	r := tripReplay(t)
	home := t.TempDir()
	env := append(recipeEnv(t, home), "TABELOG_TEST_MODE=1", "TABELOG_TEST_BASE_URL="+r.server.URL, "HTTP_PROXY=", "HTTPS_PROXY=", "ALL_PROXY=", "NO_PROXY=127.0.0.1,localhost")
	w := &workspace{env: env}
	mustSucceed(t, w.run(t, "show", sushiURL, "--agent"))
	mustSucceed(t, w.run(t, "lists", "add", "--agent", "--", "--dry-run", "13294162"))
	before := r.count()
	result := callStdioRecipe(t, env, "compare_saved_candidates_offline", map[string]any{"slug": "--dry-run"})
	if result["isError"] == true {
		t.Fatalf("valid literal list failed: %v", result)
	}
	content := result["content"].([]any)
	text := object(t, content[0])["text"].(string)
	var payload map[string]any
	if err := json.Unmarshal([]byte(text), &payload); err != nil {
		t.Fatal(err)
	}
	equal(t, items(t, payload)[0]["id"], "13294162")
	equal(t, r.count(), before)
}
