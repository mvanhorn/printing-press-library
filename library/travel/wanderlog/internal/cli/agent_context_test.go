// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildAgentContextOmitsDiscovery(t *testing.T) {
	t.Parallel()
	root := newRootCmd(&rootFlags{})
	raw, err := json.Marshal(buildAgentContextPayload(root, false))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["discovery"]; ok {
		t.Fatalf("default agent-context must omit discovery, got %v", m["discovery"])
	}
	if _, ok := m["commands"]; !ok {
		t.Fatal("default agent-context must include commands")
	}
}

func TestBuildAgentContextForEdit(t *testing.T) {
	t.Parallel()
	root := newRootCmd(&rootFlags{})
	raw, err := json.Marshal(buildAgentContextPayload(root, true))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	for _, drop := range []string{"discovery", "commands", "schema_version"} {
		if _, ok := m[drop]; ok {
			t.Errorf("--for-edit must omit %s", drop)
		}
	}
	for _, keep := range []string{"cli", "auth", "identifier_rules", "which_index", "hero_commands"} {
		if _, ok := m[keep]; !ok {
			t.Errorf("--for-edit missing %s", keep)
		}
	}
	heroes, _ := m["hero_commands"].([]any)
	joined := make([]string, 0, len(heroes))
	for _, h := range heroes {
		s, _ := h.(string)
		joined = append(joined, s)
	}
	blob := strings.Join(joined, "\n")
	for _, name := range []string{"plan", "trips", "which", "lodging", "auth"} {
		if !containsLine(joined, name) {
			t.Errorf("hero_commands missing %q in %s", name, blob)
		}
	}
	if containsLine(joined, "guides") || containsLine(joined, "places") {
		t.Errorf("hero_commands leaked non-hero parents: %s", blob)
	}
	rules, _ := m["identifier_rules"].([]any)
	if len(rules) == 0 {
		t.Fatal("identifier_rules empty")
	}
	idx, _ := m["which_index"].([]any)
	if len(idx) == 0 {
		t.Fatal("which_index empty")
	}
}

func TestAgentContextForEditSelect(t *testing.T) {
	t.Parallel()
	flags := &rootFlags{}
	root := newRootCmd(flags)
	flags.selectFields = "cli,auth"
	cmd := newAgentContextCmd(root, flags)
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"--for-edit"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("json: %v (%s)", err, buf.String())
	}
	if _, ok := m["cli"]; !ok {
		t.Fatal("expected cli after --select")
	}
	if _, ok := m["auth"]; !ok {
		t.Fatal("expected auth after --select")
	}
	if _, ok := m["which_index"]; ok {
		t.Fatal("--select cli,auth should drop which_index")
	}
}

func containsLine(lines []string, want string) bool {
	for _, l := range lines {
		if l == want {
			return true
		}
	}
	return false
}
