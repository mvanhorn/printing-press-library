package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestAgentModeSetsGlobalFlags(t *testing.T) {
	var f rootFlags
	cmd := newRootCmd(&f)
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetArgs([]string{"agent-context", "--agent"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !f.asJSON || !f.compact || !f.noInput || !f.yes {
		t.Fatalf("agent flags not applied: %#v", f)
	}
}
func TestEveryCommandHasAgentSafetyGlobals(t *testing.T) {
	root := RootCmd()
	for _, name := range []string{"report", "pivot", "batch", "realtime", "metadata", "compatibility", "properties", "property", "streams", "channels", "sources", "top-pages", "events", "conversions", "funnel", "compare", "whats-changed", "revenue", "audience", "cohort", "health", "doctor"} {
		cmd, _, err := root.Find([]string{name})
		if err != nil || cmd == nil || cmd.Name() != name {
			t.Fatalf("missing command %s", name)
		}
		for _, flag := range []string{"agent", "json", "compact", "no-input", "yes", "property"} {
			if cmd.InheritedFlags().Lookup(flag) == nil && cmd.Flags().Lookup(flag) == nil {
				t.Fatalf("%s missing --%s", name, flag)
			}
		}
	}
}
func TestRequirePropertyError(t *testing.T) {
	t.Setenv("GA4_PROPERTY_ID", "")
	_, err := requireProperty(&rootFlags{})
	if err == nil || !strings.Contains(err.Error(), "--property") {
		t.Fatalf("expected missing property error, got %v", err)
	}
}
