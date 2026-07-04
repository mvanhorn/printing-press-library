package pricing

import "testing"

func TestEstimateKnownModel(t *testing.T) {
	got := Estimate("qwen/qwen3.5-9b", 1_000_000, 500_000)
	want := 0.30
	if got < want-0.000001 || got > want+0.000001 {
		t.Fatalf("Estimate = %v, want %v", got, want)
	}
}

func TestEstimateUnknownModelIsZero(t *testing.T) {
	if got := Estimate("unknown/model", 1_000_000, 1_000_000); got != 0 {
		t.Fatalf("Estimate unknown = %v, want 0", got)
	}
}

func TestKnownModelsIncludeConsoleVisibleIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, model := range KnownModels {
		seen[model.ID] = true
	}
	for _, id := range []string{"qwen/qwen3.6-27b", "qwen/qwen3.6-35b-a3b", "moonshotai/kimi-k2.7-code"} {
		if !seen[id] {
			t.Fatalf("KnownModels missing %s", id)
		}
	}
}
