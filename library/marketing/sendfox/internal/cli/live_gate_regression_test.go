package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/cliutil/testenv"
)

func TestLiveGateDryRunCommandsEmitJSONEnvelope(t *testing.T) {
	tests := [][]string{
		{"doctor", "--dry-run", "--json"},
		{"feedback", "list", "--dry-run", "--json"},
		{"profile", "list", "--dry-run", "--json"},
		{"sync", "--dry-run", "--json"},
		{"unsubscribe", "--email", "user@example.com", "--dry-run", "--json"},
		{"workflow", "archive", "--dry-run", "--json"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args[:len(args)-2], "_"), func(t *testing.T) {
			testenv.Isolate(t)
			root := newRootCmd(&rootFlags{})
			var out bytes.Buffer
			root.SetOut(&out)
			root.SetErr(&bytes.Buffer{})
			root.SetArgs(args)
			if err := root.Execute(); err != nil {
				t.Fatal(err)
			}
			var result map[string]any
			if err := json.Unmarshal(out.Bytes(), &result); err != nil {
				t.Fatalf("output is not JSON: %v\n%s", err, out.String())
			}
			if result["dry_run"] != true {
				t.Fatalf("dry_run = %#v, want true: %s", result["dry_run"], out.String())
			}
		})
	}
}

func TestLiveGateMetadataAvoidsSyntheticResourceIDs(t *testing.T) {
	root := newRootCmd(&rootFlags{})
	for _, path := range [][]string{{"contacts", "get-bulk-action"}, {"domains", "get"}} {
		cmd, _, err := root.Find(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := cmd.Annotations["pp:happy-args"]; got != "" {
			t.Fatalf("%s hard-codes a live resource fixture: %q", strings.Join(path, " "), got)
		}
	}
	bulkWait, _, err := root.Find([]string{"contacts", "bulk-wait"})
	if err != nil {
		t.Fatal(err)
	}
	if got := bulkWait.Annotations["pp:happy-args"]; !strings.Contains(got, "--dry-run") {
		t.Fatalf("bulk-wait dogfood fixture must not poll a synthetic live ID: %q", got)
	}
	webhooks, _, err := root.Find([]string{"webhooks"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(webhooks.Example, "sendfox-pp-cli webhooks") {
		t.Fatalf("webhooks example = %q", webhooks.Example)
	}
}
