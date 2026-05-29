package advisor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func sampleTags() json.RawMessage {
	return json.RawMessage(`{"models":[
		{"name":"qwen3-coder:480b","model":"qwen3-coder:480b","details":{"family":"qwen"}},
		{"name":"gpt-oss:120b","model":"gpt-oss:120b","details":{"family":"gpt-oss"}},
		{"name":"gpt-oss:20b","model":"gpt-oss:20b","details":{"family":"gpt-oss"}},
		{"name":"qwen3-vl:235b","model":"qwen3-vl:235b","details":{"family":"qwen"}},
		{"name":"gemma3:4b","model":"gemma3:4b","details":{"family":"gemma"}}
	]}`)
}

func sampleOverlay() []byte {
	return []byte(`{
		"schema_version": 1,
		"models": [
			{"id_patterns":["qwen3-coder*"],"ctx_window":262144,"latency_p50_ms":4200,"supports_tools":true,"strengths":["coding","long-context","agentic"]},
			{"id_patterns":["gpt-oss:120b*"],"ctx_window":131072,"latency_p50_ms":2800,"supports_tools":true,"strengths":["reasoning","tools","general"]},
			{"id_patterns":["gpt-oss:20b*"],"ctx_window":131072,"latency_p50_ms":1200,"supports_tools":true,"strengths":["cheap","fast","general"]},
			{"id_patterns":["qwen3-vl*"],"ctx_window":131072,"latency_p50_ms":3800,"supports_vision":true,"strengths":["vision","multimodal"]},
			{"id_patterns":["gemma*"],"ctx_window":8192,"latency_p50_ms":900,"strengths":["cheap","fast"]}
		],
		"default":{"ctx_window":32768,"latency_p50_ms":4000}
	}`)
}

func TestLoadCatalogMergesOverlay(t *testing.T) {
	cat, err := LoadCatalog(sampleTags(), sampleOverlay())
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(cat) != 5 {
		t.Fatalf("want 5 models, got %d", len(cat))
	}
	for _, m := range cat {
		if m.ID == "qwen3-coder:480b" && m.CtxWindow != 262144 {
			t.Errorf("qwen3-coder ctx_window=%d want 262144", m.CtxWindow)
		}
		if m.ID == "qwen3-vl:235b" && !m.SupportsVision {
			t.Error("qwen3-vl should support vision")
		}
	}
}

func TestAdviseCodingPromptPicksCoder(t *testing.T) {
	cat, _ := LoadCatalog(sampleTags(), sampleOverlay())
	prompt := "Write a Go function that parses JSON.\n```go\nfunc parse(data []byte) error {\n```\nstep by step please"
	rec, err := Advise(context.Background(), Request{Prompt: prompt, TaskHint: "coding"}, cat, true)
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if rec.Recommended != "qwen3-coder:480b" {
		t.Errorf("coding prompt should pick qwen3-coder; got %s", rec.Recommended)
	}
}

func TestAdviseLatencyConstraintFiltersSlow(t *testing.T) {
	cat, _ := LoadCatalog(sampleTags(), sampleOverlay())
	rec, err := Advise(context.Background(), Request{Prompt: "hello", TaskHint: "cheap", MaxLatencyMs: 1500}, cat, true)
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if rec.Recommended != "gemma3:4b" && rec.Recommended != "gpt-oss:20b" {
		t.Errorf("with max-latency=1500, should pick fast model; got %s", rec.Recommended)
	}
}

func TestAdviseRequireToolsFiltersVision(t *testing.T) {
	cat, _ := LoadCatalog(sampleTags(), sampleOverlay())
	rec, err := Advise(context.Background(), Request{Prompt: "do stuff", RequireTools: true}, cat, true)
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	for _, c := range rec.Alternatives {
		if !c.Model.SupportsTools && !c.Filtered {
			t.Errorf("non-tool model %s in unfiltered alternatives", c.ModelID)
		}
	}
}

func TestAdviseEmptyCatalogErrors(t *testing.T) {
	_, err := Advise(context.Background(), Request{Prompt: "x"}, nil, false)
	if err == nil {
		t.Error("empty catalog should error")
	}
}

func TestAdviseAllExcludedErrors(t *testing.T) {
	cat, _ := LoadCatalog(sampleTags(), sampleOverlay())
	excl := make([]string, 0, len(cat))
	for _, m := range cat {
		excl = append(excl, m.ID)
	}
	_, err := Advise(context.Background(), Request{Prompt: "x", Exclude: excl}, cat, false)
	if err == nil || !strings.Contains(err.Error(), "no candidates") {
		t.Errorf("all-excluded should error with 'no candidates'; got %v", err)
	}
}

func TestValidateCatalogReportsDrift(t *testing.T) {
	tags := json.RawMessage(`{"models":[{"name":"qwen3-coder:480b"},{"name":"some-random-model:1b"}]}`)
	overlay := []byte(`{"schema_version":1,"models":[
		{"id_patterns":["qwen3-coder*"]},
		{"id_patterns":["future-model-*"]}
	],"default":{}}`)
	drift, err := ValidateCatalog(tags, overlay)
	if err != nil {
		t.Fatalf("ValidateCatalog: %v", err)
	}
	if len(drift.UncuratedLive) != 1 || drift.UncuratedLive[0] != "some-random-model:1b" {
		t.Errorf("expected uncurated_live=[some-random-model:1b]; got %v", drift.UncuratedLive)
	}
	if len(drift.CuratedNotInLive) != 1 || drift.CuratedNotInLive[0] != "future-model-*" {
		t.Errorf("expected curated_not_in_live=[future-model-*]; got %v", drift.CuratedNotInLive)
	}
}

func TestExtractFeaturesReasoningAndTools(t *testing.T) {
	f := ExtractFeatures("Please step by step compute this. Use the python tool.", nil)
	if f.ReasoningDepthHints == 0 {
		t.Error("should detect reasoning hint")
	}
	if f.ToolUseMentions == 0 {
		t.Error("should detect tool-use mention")
	}
	if f.InputTokens == 0 {
		t.Error("token count should be non-zero")
	}
}

func TestDeterministicScoring(t *testing.T) {
	cat, _ := LoadCatalog(sampleTags(), sampleOverlay())
	prompt := "Write a Go function that parses JSON.\n```go\nfunc parse(data []byte) error {\n```\nstep by step please"
	var prev string
	for i := 0; i < 5; i++ {
		rec, err := Advise(context.Background(), Request{Prompt: prompt, TaskHint: "coding"}, cat, false)
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
		if i > 0 && rec.Recommended != prev {
			t.Errorf("non-deterministic across runs: %s vs %s", prev, rec.Recommended)
		}
		prev = rec.Recommended
	}
}

func TestLoadProviderOverlayStampsProvider(t *testing.T) {
	overlay := []byte(`{
		"schema_version": 1,
		"provider": "local-llama",
		"models": [
			{"id_patterns":["qwen3.6-35b*"],"family":"qwen","ctx_window":65536,"latency_p50_ms":80,"supports_tools":true,"strengths":["coding","fast"],"measured":true},
			{"id_patterns":["gemma-4-e4b*"],"family":"gemma","ctx_window":32768,"latency_p50_ms":25,"strengths":["cheap","fast"]}
		],
		"default":{}
	}`)
	models, err := LoadProviderOverlay(overlay)
	if err != nil {
		t.Fatalf("LoadProviderOverlay: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("want 2 models, got %d", len(models))
	}
	for _, m := range models {
		if m.Provider != "local-llama" {
			t.Errorf("model %q missing provider stamp: %+v", m.ID, m)
		}
		if m.Source != "overlay:local-llama" {
			t.Errorf("model %q wrong source: %q", m.ID, m.Source)
		}
	}
	if models[0].ID != "qwen3.6-35b" {
		t.Errorf("first concrete id should be qwen3.6-35b (stripped wildcard), got %q", models[0].ID)
	}
	if !models[0].SupportsTools {
		t.Error("qwen3.6-35b should support tools")
	}
}

func TestQualifiedIDOnlyStampsNonCloud(t *testing.T) {
	cases := []struct {
		m    Model
		want string
	}{
		{Model{ID: "qwen3-coder:480b"}, "qwen3-coder:480b"},
		{Model{ID: "qwen3-coder:480b", Provider: "ollama-cloud"}, "qwen3-coder:480b"},
		{Model{ID: "qwen3.6-35b", Provider: "local-llama"}, "qwen3.6-35b@local-llama"},
		{Model{ID: "gpt-4", Provider: "openrouter"}, "gpt-4@openrouter"},
	}
	for _, c := range cases {
		got := c.m.QualifiedID()
		if got != c.want {
			t.Errorf("QualifiedID(%+v) = %q, want %q", c.m, got, c.want)
		}
	}
}

func TestCrossProviderAdviseRoutesToLocalForCheap(t *testing.T) {
	// Cloud catalog includes a slow + a fast cloud model
	cloudTags := json.RawMessage(`{"models":[
		{"name":"qwen3-coder:480b","model":"qwen3-coder:480b","details":{"family":"qwen"}},
		{"name":"gpt-oss:20b","model":"gpt-oss:20b","details":{"family":"gpt-oss"}}
	]}`)
	cloudOverlay := []byte(`{
		"schema_version":1,
		"models":[
			{"id_patterns":["qwen3-coder*"],"ctx_window":262144,"latency_p50_ms":4200,"supports_tools":true,"strengths":["coding","long-context"]},
			{"id_patterns":["gpt-oss:20b*"],"ctx_window":131072,"latency_p50_ms":1200,"supports_tools":true,"strengths":["cheap","fast"]}
		],
		"default":{}
	}`)
	cat, err := LoadCatalog(cloudTags, cloudOverlay)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	// Local sibling overlay with a much faster model
	localOverlay := []byte(`{
		"schema_version":1,
		"provider":"local-llama",
		"models":[
			{"id_patterns":["gemma-4-e4b*"],"ctx_window":32768,"latency_p50_ms":25,"strengths":["cheap","fast"]}
		],
		"default":{}
	}`)
	siblings, err := LoadProviderOverlay(localOverlay)
	if err != nil {
		t.Fatalf("LoadProviderOverlay: %v", err)
	}
	cat = append(cat, siblings...)

	rec, err := Advise(context.Background(), Request{Prompt: "ping", TaskHint: "cheap", MaxLatencyMs: 1500}, cat, false)
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if rec.Recommended != "gemma-4-e4b@local-llama" {
		t.Errorf("cheap+low-latency prompt should pick local-llama gemma-4-e4b (25ms p50, provider-qualified); got %q", rec.Recommended)
	}
}

func TestAdviseFallbackDiffersFromRecommendedAfterTiebreak(t *testing.T) {
	// Two cloud models with identical metadata tie on score, so the tiebreaker
	// fires. The tiebreaker promotes the second-ranked candidate to winner; the
	// fallback must still resolve to a *different* model so a routing layer that
	// retries against it has an escape path. Regression: fallback was hardcoded
	// to live[1], which equals the recommendation once the tiebreaker promotes
	// it (and the len>=3 guard does not fire with exactly two models).
	tags := json.RawMessage(`{"models":[
		{"name":"aa-model:1b","model":"aa-model:1b","details":{"family":"x"}},
		{"name":"bb-model:1b","model":"bb-model:1b","details":{"family":"x"}}
	]}`)
	overlay := []byte(`{
		"schema_version":1,
		"models":[
			{"id_patterns":["aa-model*"],"ctx_window":131072,"latency_p50_ms":1000,"strengths":["general"]},
			{"id_patterns":["bb-model*"],"ctx_window":131072,"latency_p50_ms":1000,"strengths":["general"]}
		],
		"default":{}
	}`)
	cat, err := LoadCatalog(tags, overlay)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	tb := func(_ context.Context, _ string, top []Candidate) (string, error) {
		return top[1].ModelID, nil // promote the second-ranked candidate
	}
	rec, err := Advise(context.Background(), Request{
		Prompt:         "hello there",
		EnableTiebreak: true,
		Tiebreaker:     tb,
	}, cat, false)
	if err != nil {
		t.Fatalf("Advise: %v", err)
	}
	if !rec.TiebreakUsed {
		t.Fatalf("expected tiebreak to fire and be used; rec=%+v", rec)
	}
	if rec.Recommended != "bb-model:1b" {
		t.Errorf("recommended should be promoted bb-model:1b; got %q", rec.Recommended)
	}
	if rec.Fallback == rec.Recommended {
		t.Errorf("fallback must differ from recommended; both = %q", rec.Fallback)
	}
	if rec.Fallback != "aa-model:1b" {
		t.Errorf("fallback should be aa-model:1b; got %q", rec.Fallback)
	}
}

func TestDetectLanguagesDeterministicOrder(t *testing.T) {
	// Multi-language prompt; detection must return a fixed order across runs
	// (regression: map iteration randomised the Languages list in --explain).
	prompt := "package main\nimport \"x\"\ndef f():\nconst y\nselect 1"
	first := detectLanguages(prompt)
	for i := 0; i < 20; i++ {
		got := detectLanguages(prompt)
		if len(got) != len(first) {
			t.Fatalf("length varied: %v vs %v", first, got)
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("order varied across runs: %v vs %v", first, got)
			}
		}
	}
	want := []string{"go", "python", "typescript", "sql"}
	if len(first) != len(want) {
		t.Fatalf("got %v want %v", first, want)
	}
	for i := range want {
		if first[i] != want[i] {
			t.Fatalf("got %v want %v", first, want)
		}
	}
}

func TestApplyOverlayDefaultCarriesPricing(t *testing.T) {
	// A live model not matched by any overlay pattern must inherit the default
	// pricing, not be treated as free (regression: default branch dropped
	// PriceInPer1M/PriceOutPer1M, biasing scoreCost toward unrecognised models).
	tags := json.RawMessage(`{"models":[{"name":"unknown-model:1b","model":"unknown-model:1b","details":{"family":"x"}}]}`)
	overlay := []byte(`{
		"schema_version":1,
		"models":[{"id_patterns":["qwen3-coder*"],"ctx_window":262144,"price_in_per_1m":1.0,"price_out_per_1m":2.0}],
		"default":{"ctx_window":32768,"latency_p50_ms":4000,"price_in_per_1m":5.0,"price_out_per_1m":9.0}
	}`)
	cat, err := LoadCatalog(tags, overlay)
	if err != nil {
		t.Fatalf("LoadCatalog: %v", err)
	}
	if len(cat) != 1 {
		t.Fatalf("want 1 model, got %d", len(cat))
	}
	if cat[0].PriceInPer1M != 5.0 || cat[0].PriceOutPer1M != 9.0 {
		t.Errorf("default pricing not applied: in=%v out=%v want 5/9", cat[0].PriceInPer1M, cat[0].PriceOutPer1M)
	}
}

func TestGlobMatchHandlesColon(t *testing.T) {
	if !globMatch("qwen3-coder*", "qwen3-coder:480b") {
		t.Error("qwen3-coder* should match qwen3-coder:480b")
	}
	if !globMatch("gpt-oss:120b*", "gpt-oss:120b") {
		t.Error("gpt-oss:120b* should match gpt-oss:120b")
	}
	if globMatch("qwen3-vl*", "qwen3-coder:480b") {
		t.Error("qwen3-vl* should NOT match qwen3-coder")
	}
}
