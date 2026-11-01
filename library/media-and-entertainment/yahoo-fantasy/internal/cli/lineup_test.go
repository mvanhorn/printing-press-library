package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNovelLineupCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"lineup"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.String(), "Usage:") {
			t.Fatalf("expected Usage in help output, got: %s", out.String())
		}
	})

	t.Run("dry-run produces JSON envelope and XML preview on stderr", func(t *testing.T) {
		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"lineup", "--team", "449.l.365083.t.1", "--week", "9", "--set", "QB=449.p.31862,BN=449.p.30123", "--dry-run", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var env map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
			t.Fatalf("invalid JSON stdout: %v (%s)", err, stdout.String())
		}
		if env["dry_run"] != true {
			t.Fatalf("expected dry_run=true, got %+v", env)
		}
		if !strings.Contains(stderr.String(), "<fantasy_content>") {
			t.Fatalf("expected <fantasy_content> XML preview on stderr, got: %s", stderr.String())
		}
	})

	t.Run("builds XML payload when given --team and --set", func(t *testing.T) {
		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"lineup", "--team", "449.l.365083.t.1", "--week", "9", "--set", "QB=449.p.31862", "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v (%s)", err, stdout.String())
		}
		xmlPayload, _ := parsed["xml_payload"].(string)
		if !strings.Contains(xmlPayload, "449.p.31862") || !strings.Contains(xmlPayload, "<roster>") {
			t.Fatalf("expected lineup plan with XML payload in JSON output, got: %s", xmlPayload)
		}
	})
}
