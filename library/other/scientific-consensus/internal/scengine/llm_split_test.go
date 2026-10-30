package scengine

import (
	"context"
	"errors"
	"testing"
)

// stubProvider swaps the provider list for one stub with a set key and counts
// its calls.
func stubProvider(t *testing.T, reply string, err error) *int {
	t.Helper()
	calls := 0
	prev := providers
	providers = []provider{{
		name:   "stub",
		envVar: "SC_TEST_LLM_KEY",
		call: func(ctx context.Context, apiKey, prompt string) (string, error) {
			calls++
			return reply, err
		},
	}}
	t.Cleanup(func() { providers = prev })
	t.Setenv("SC_TEST_LLM_KEY", "test-key")
	return &calls
}

func TestClaimSplitAutoUsesLLMOnlyWhenPICOCannotSplit(t *testing.T) {
	calls := stubProvider(t, `{"intervention":"chewing gum","outcome":"postoperative ileus"}`, nil)

	iv, out, gate := ClaimSplitAuto(context.Background(), "coffee improves alertness")
	if gate != GatePICO || *calls != 0 {
		t.Fatalf("splittable claim: gate=%q calls=%d, want pico with no LLM call", gate, *calls)
	}
	if len(iv) == 0 || len(out) == 0 {
		t.Fatalf("splittable claim: iv=%v out=%v", iv, out)
	}

	iv, out, gate = ClaimSplitAuto(context.Background(), "chewing gum after bowel surgery")
	if gate != "llm_split:stub" || *calls != 1 {
		t.Fatalf("unsplittable claim: gate=%q calls=%d, want llm_split:stub with one call", gate, *calls)
	}
	if len(iv) == 0 {
		t.Fatalf("iv = %v", iv)
	}
	if len(out) == 0 {
		t.Fatalf("out = %v", out)
	}
}

func TestClaimSplitAutoFallsBack(t *testing.T) {
	tests := []struct {
		name  string
		reply string
		err   error
	}{
		{name: "provider error", err: errors.New("boom")},
		{name: "not json", reply: "sorry"},
		{name: "empty side", reply: `{"intervention":"chewing gum","outcome":""}`},
		{name: "stopword side", reply: `{"intervention":"chewing gum","outcome":"the"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := stubProvider(t, tt.reply, tt.err)
			iv, out, gate := ClaimSplitAuto(context.Background(), "chewing gum after bowel surgery")
			if gate != GateTokenOverlap || iv != nil || out != nil {
				t.Fatalf("gate=%q iv=%v out=%v, want token_overlap and no tokens", gate, iv, out)
			}
			if *calls != 1 {
				t.Fatalf("calls = %d, want 1", *calls)
			}
		})
	}
}

func TestClaimSplitAutoNoKeyMakesNoCall(t *testing.T) {
	calls := stubProvider(t, `{"intervention":"chewing gum","outcome":"ileus"}`, nil)
	t.Setenv("SC_TEST_LLM_KEY", "")
	if _, _, gate := ClaimSplitAuto(context.Background(), "chewing gum after bowel surgery"); gate != GateTokenOverlap {
		t.Fatalf("gate = %q, want token_overlap", gate)
	}
	if *calls != 0 {
		t.Fatalf("calls = %d, want 0", *calls)
	}
}
