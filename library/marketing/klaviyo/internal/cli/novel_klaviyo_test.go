package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestNovelCommandsRegistered(t *testing.T) {
	root := RootCmd()
	paths := [][]string{
		{"campaigns", "deploy"},
		{"campaigns", "image-swap"},
		{"flow-decay"},
		{"cohort"},
		{"attribution"},
		{"dedup"},
		{"reconcile"},
		{"plan", "brief-to-strategy"},
		{"plan", "qa-gate"},
	}
	for _, path := range paths {
		if findCommand(root, path) == nil {
			t.Fatalf("command %v not registered", path)
		}
	}
}

func TestNovelLocalAnalytics(t *testing.T) {
	rows := []resourceRow{
		{
			ID: "evt-1",
			Data: map[string]any{"data": map[string]any{"attributes": map[string]any{
				"datetime":    "2026-01-15T00:00:00Z",
				"metric_name": "Placed Order",
				"value":       125.0,
				"properties": map[string]any{
					"$attributed_flow":     "welcome",
					"$attributed_campaign": "spring",
					"utm_campaign":         "spring",
				},
			}, "relationships": map[string]any{"profile": map[string]any{"data": map[string]any{"id": "p1"}}}}},
		},
		{
			ID: "evt-2",
			Data: map[string]any{"data": map[string]any{"attributes": map[string]any{
				"datetime":    "2026-02-15T00:00:00Z",
				"metric_name": "Placed Order",
				"value":       75.0,
				"properties": map[string]any{
					"$attributed_flow":     "welcome",
					"$attributed_campaign": "spring",
					"utm_campaign":         "spring",
				},
			}, "relationships": map[string]any{"profile": map[string]any{"data": map[string]any{"id": "p1"}}}}},
		},
	}

	attr := attribution(rows, "Placed Order", "flow", "2026-01-01")
	if len(attr) != 1 || attr[0]["group"] != "welcome" || attr[0]["orders"] != 2 {
		t.Fatalf("attribution = %#v", attr)
	}
	cohorts := cohort(rows, "Placed Order", "month")
	if len(cohorts) != 1 || cohorts[0]["retained"] != 1 {
		t.Fatalf("cohort = %#v", cohorts)
	}
	rec := reconcile(rows, "spring", "2026-01-01")
	if rec["orders"] != 2 {
		t.Fatalf("reconcile = %#v", rec)
	}
}

func TestNovelDedupAndDecay(t *testing.T) {
	profiles := []resourceRow{
		{ID: "p1", Data: map[string]any{"data": map[string]any{"attributes": map[string]any{"email": "a@example.com", "phone": "+1555"}}}},
		{ID: "p2", Data: map[string]any{"data": map[string]any{"attributes": map[string]any{"email": "a@example.com", "phone": "+1666"}}}},
	}
	dupes := dedup(profiles, "email,phone")
	if len(dupes) != 1 || dupes[0]["field"] != "email" {
		t.Fatalf("dedup = %#v", dupes)
	}

	flows := []resourceRow{
		{ID: "f1", Data: map[string]any{"data": map[string]any{"attributes": map[string]any{"name": "Welcome", "open_rate": 0.20, "previous_open_rate": 0.40}}}},
	}
	decay := flowDecay(flows, 90, 0.15)
	if len(decay) != 1 || decay[0]["flagged"] != true {
		t.Fatalf("flowDecay = %#v", decay)
	}
}

func TestNovelPlanningHelpers(t *testing.T) {
	strategy := briefToStrategy("Launch a subscription winback offer for high intent customers before Mother's Day.")
	if strategy["summary"] == "" {
		t.Fatalf("strategy missing summary: %#v", strategy)
	}
	gate := qaGate(`<a href="https://example.com">Shop</a> SAVE20 {{ first_name|default:'there' }}`, "SAVE20", "America/Chicago")
	if gate["verdict"] != "warn" {
		t.Fatalf("qaGate verdict = %#v", gate)
	}
	if got := stripTags("<p>Hello&nbsp;there</p>"); got != "Hello there" {
		t.Fatalf("stripTags = %q", got)
	}
}

func findCommand(cmd *cobra.Command, path []string) *cobra.Command {
	for _, child := range cmd.Commands() {
		if child.Name() != path[0] {
			continue
		}
		if len(path) == 1 {
			return child
		}
		return findCommand(child, path[1:])
	}
	return nil
}
