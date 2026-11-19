package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/cliutil/testenv"
	"github.com/spf13/cobra"
)

func TestReadOnlyGroupFirstExample(t *testing.T) {
	testenv.Isolate(t)
	t.Setenv("READWISE_API_TOKEN", "")
	if err := os.Unsetenv("READWISE_API_TOKEN"); err != nil {
		t.Fatal(err)
	}
	root := RootCmd()
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd.HasSubCommands() && cmd.Annotations["mcp:read-only"] == "true" && strings.TrimSpace(cmd.Example) != "" {
			t.Run(cmd.CommandPath(), func(t *testing.T) {
				var first string
				for _, line := range strings.Split(cmd.Example, "\n") {
					if first = strings.TrimSpace(line); first != "" {
						break
					}
				}
				args := strings.Fields(first)
				if len(args) < 2 || args[0] != "readwise-pp-cli" {
					t.Fatalf("invalid first Example: %q", first)
				}
				resolved, _, err := root.Find(args[1:])
				if err != nil {
					t.Fatalf("resolve %q: %v", first, err)
				}
				if resolved.HasSubCommands() || resolved.Annotations["mcp:read-only"] != "true" {
					t.Fatalf("read-only group first Example %q resolves to %s, want read-only leaf", first, resolved.CommandPath())
				}
			})
		}
		for _, child := range cmd.Commands() {
			walk(child)
		}
	}
	walk(root)
}

func TestWebhookListEmptyJSON(t *testing.T) {
	home := testenv.Isolate(t)
	t.Setenv("READWISE_API_TOKEN", "")
	if err := os.Unsetenv("READWISE_API_TOKEN"); err != nil {
		t.Fatal(err)
	}
	root := RootCmd()
	root.SetArgs([]string{"webhook", "list", "--json", "--db", filepath.Join(home, "empty.db")})
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Fatalf("empty webhook list = %q, want []", got)
	}
}
