package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestLLMSPlainTextCommandsPreserveTextAndJSONModes(t *testing.T) {
	responses := map[string]string{
		"/api/llms/agent-configuration.txt": "agent configuration",
		"/api/llms/agent-icons.txt":         "circle\nsquare\n",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := responses[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	t.Setenv("PAPERCLIP_BASE_URL", server.URL)

	commands := []struct {
		name string
		path string
		new  func(*rootFlags) *cobra.Command
	}{
		{name: "agent configuration", path: "/api/llms/agent-configuration.txt", new: newLlmsListCmd},
		{name: "agent icons", path: "/api/llms/agent-icons.txt", new: newLlmsListAgenticonstxtCmd},
	}
	for _, tc := range commands {
		t.Run(tc.name+" plain", func(t *testing.T) {
			flags := &rootFlags{plain: true, configPath: filepath.Join(t.TempDir(), "config.toml")}
			cmd := tc.new(flags)
			cmd.SetContext(context.Background())
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatalf("RunE returned error: %v", err)
			}
			want := responses[tc.path]
			if want[len(want)-1] != '\n' {
				want += "\n"
			}
			if got := out.String(); got != want {
				t.Fatalf("plain output = %q, want %q", got, want)
			}
		})
		t.Run(tc.name+" json", func(t *testing.T) {
			flags := &rootFlags{asJSON: true, configPath: filepath.Join(t.TempDir(), "config.toml")}
			cmd := tc.new(flags)
			cmd.SetContext(context.Background())
			var out bytes.Buffer
			cmd.SetOut(&out)
			if err := cmd.RunE(cmd, nil); err != nil {
				t.Fatalf("RunE returned error: %v", err)
			}
			var payload map[string]string
			if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
				t.Fatalf("JSON output invalid: %v: %q", err, out.String())
			}
			if got := payload["content"]; got != responses[tc.path] {
				t.Fatalf("content = %q, want %q", got, responses[tc.path])
			}
		})
	}
}
