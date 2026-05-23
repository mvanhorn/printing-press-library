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
