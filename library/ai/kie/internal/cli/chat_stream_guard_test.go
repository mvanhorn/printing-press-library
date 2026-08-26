package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

func TestGuardNonSSEChatCompletionsStream(t *testing.T) {
	tests := []struct {
		name        string
		body        map[string]any
		dryRun      bool
		wantStream  any
		wantErrPart string
	}{
		{name: "absent defaults false", body: map[string]any{}, wantStream: false},
		{name: "null defaults false", body: map[string]any{"stream": nil}, wantStream: false},
		{name: "false live remains false", body: map[string]any{"stream": false}, wantStream: false},
		{name: "true dry run remains true", body: map[string]any{"stream": true}, dryRun: true, wantStream: true},
		{name: "true live rejects", body: map[string]any{"stream": true}, wantStream: true, wantErrPart: "stream:true is not supported"},
		{name: "non boolean rejects", body: map[string]any{"stream": "true"}, wantStream: "true", wantErrPart: "stream must be a JSON boolean or null"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := guardNonSSEChatCompletionsStream(tt.body, tt.dryRun)
			if tt.wantErrPart != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrPart) {
					t.Fatalf("guard error = %v, want %q", err, tt.wantErrPart)
				}
			} else if err != nil {
				t.Fatalf("guard returned unexpected error: %v", err)
			}
			if got := tt.body["stream"]; got != tt.wantStream {
				t.Fatalf("stream = %#v, want %#v", got, tt.wantStream)
			}
		})
	}
}

func TestNonSSEChatCompletionsStreamGuardCommandPreflight(t *testing.T) {
	stderr := runKieRootCaptureProcessStderr(t, []string{"--dry-run", "gemini-2-5-flash", "chat-completions"}, "")
	if !strings.Contains(stderr, `"stream": false`) {
		t.Fatalf("default dry-run body did not force stream false:\n%s", stderr)
	}

	err := runKieRootErr(t, []string{"gemini-2-5-flash", "chat-completions", "--messages", "[]", "--stream"}, "")
	if err == nil || !strings.Contains(err.Error(), "stream:true is not supported") {
		t.Fatalf("live --stream error = %v, want stream guard", err)
	}

	err = runKieRootErr(t, []string{"gemini-2-5-flash", "chat-completions", "--stdin"}, `{"messages":[],"stream":true}`)
	if err == nil || !strings.Contains(err.Error(), "stream:true is not supported") {
		t.Fatalf("live stdin stream:true error = %v, want stream guard", err)
	}

	stderr = runKieRootCaptureProcessStderr(t, []string{"--dry-run", "gemini-2-5-flash", "chat-completions", "--stream"}, "")
	if !strings.Contains(stderr, `"stream": true`) {
		t.Fatalf("dry-run --stream did not preserve true preview:\n%s", stderr)
	}
}

func TestNonSSEChatCompletionsStreamGuardWiring(t *testing.T) {
	files := []string{
		"gemini-2-5-flash_chat-completions.go",
		"gemini-2-5-pro_chat-completions.go",
		"gemini_3-6-flash.go",
		"gemini_3-flash-v1betamodels.go",
		"gemini-3-pro_chat-completions.go",
		"gemini_3-5-flash.go",
		"promoted_claude.go",
		"promoted_gemini-3-5-flash-openai.go",
		"promoted_gemini-3-1-pro.go",
		"promoted_gemini-3-6-flash-openai.go",
		"promoted_gemini-3-flash.go",
	}

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.Count(string(data), "guardNonSSEChatCompletionsStream(body, flags.dryRun)")
			if got != 1 {
				t.Fatalf("guard call count = %d, want 1", got)
			}
		})
	}
}

func runKieRootErr(t *testing.T, args []string, stdin string) error {
	t.Helper()
	withTestConfig(t)
	restoreStdin := setProcessStdin(t, stdin)
	defer restoreStdin()

	root := RootCmd()
	root.SetArgs(args)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	return root.Execute()
}

func runKieRootCaptureProcessStderr(t *testing.T, args []string, stdin string) string {
	t.Helper()
	withTestConfig(t)
	restoreStdin := setProcessStdin(t, stdin)
	defer restoreStdin()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	originalStderr := os.Stderr
	os.Stderr = write
	defer func() { os.Stderr = originalStderr }()

	root := RootCmd()
	root.SetArgs(args)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	execErr := root.Execute()
	if closeErr := write.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	output, readErr := io.ReadAll(read)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr := read.Close(); closeErr != nil {
		t.Fatal(closeErr)
	}
	if execErr != nil {
		t.Fatalf("root.Execute() = %v; stderr:\n%s", execErr, output)
	}
	return string(output)
}

func withTestConfig(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("KIE_BEARER_AUTH", "test-token")
}

func setProcessStdin(t *testing.T, input string) func() {
	t.Helper()
	originalStdin := os.Stdin
	if input == "" {
		return func() { os.Stdin = originalStdin }
	}
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(write, input); err != nil {
		t.Fatal(err)
	}
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stdin = read
	return func() {
		os.Stdin = originalStdin
		_ = read.Close()
	}
}
