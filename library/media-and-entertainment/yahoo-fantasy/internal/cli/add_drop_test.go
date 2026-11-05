package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestNovelAddDropCmd(t *testing.T) {
	t.Run("help on bare invocation", func(t *testing.T) {
		root := RootCmd()
		var out bytes.Buffer
		root.SetOut(&out)
		root.SetErr(&out)
		root.SetArgs([]string{"add-drop"})
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
		root.SetArgs([]string{"add-drop", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--add", "449.p.31862", "--drop", "449.p.30123", "--faab", "15", "--dry-run", "--json"})
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
		if !strings.Contains(stderr.String(), "<type>add/drop</type>") || !strings.Contains(stderr.String(), "<faab_bid>15</faab_bid>") {
			t.Fatalf("expected <fantasy_content> transaction XML on stderr, got: %s", stderr.String())
		}
	})

	t.Run("dry-run fails on missing --add/--drop or negative --faab", func(t *testing.T) {
		root := RootCmd()
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"add-drop", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--dry-run"})
		if err := root.Execute(); err == nil {
			t.Fatalf("expected error when neither --add nor --drop is provided under --dry-run")
		}
	})

	t.Run("builds add/drop transaction plan in local mode", func(t *testing.T) {
		root := RootCmd()
		var stdout, stderr bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(&stderr)
		root.SetArgs([]string{"add-drop", "--league", "449.l.365083", "--team", "449.l.365083.t.1", "--add", "449.p.31862", "--drop", "449.p.30123", "--faab", "15", "--data-source", "local", "--json"})
		if err := root.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var parsed map[string]any
		if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
			t.Fatalf("invalid JSON: %v (%s)", err, stdout.String())
		}
		xmlPayload, _ := parsed["xml_payload"].(string)
		if parsed["transaction_type"] != "add/drop" || !strings.Contains(xmlPayload, "<destination_team_key>449.l.365083.t.1</destination_team_key>") {
			t.Fatalf("expected transaction plan in JSON output, got: %+v", parsed)
		}
	})
}
