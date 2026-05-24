// Package advisor is the heart of ollama-cloud-pp-cli. Given a prompt + session +
// constraints, it picks the hosted model with the best fit and emits a stable
// JSON envelope. Heuristic-first; an optional cheap meta-LLM tiebreak fires
// only when the top-2 scores are within 5% — and falls back deterministically.
package advisor

import (
	"context"
	"time"
)

const SchemaVersion = 1

type Model struct {
	ID             string   `json:"id"`
	Provider       string   `json:"provider,omitempty"`
	Family         string   `json:"family,omitempty"`
	CtxWindow      int      `json:"ctx_window,omitempty"`
	PriceInPer1M   float64  `json:"price_in_per_1m,omitempty"`
	PriceOutPer1M  float64  `json:"price_out_per_1m,omitempty"`
	LatencyP50Ms   int      `json:"latency_p50_ms,omitempty"`
	SupportsTools  bool     `json:"supports_tools,omitempty"`
	SupportsVision bool     `json:"supports_vision,omitempty"`
	Strengths      []string `json:"strengths,omitempty"`
	Source         string   `json:"source,omitempty"`
}

// QualifiedID returns the provider-qualified model id (e.g. "qwen3.6-35b@local-llama")
// when Provider is set, or just the bare ID for backward compatibility with
// schema_version=1 single-provider envelopes.
func (m Model) QualifiedID() string {
	if m.Provider == "" || m.Provider == "ollama-cloud" {
		return m.ID
	}
	return m.ID + "@" + m.Provider
}

type Features struct {
	InputTokens          int      `json:"input_tokens"`
	InputTokensMethod    string   `json:"input_tokens_method"`
	InputTokensMarginPct float64  `json:"input_tokens_margin_pct"`
	CodeFenceDensity     float64  `json:"code_fence_density"`
	Languages            []string `json:"languages,omitempty"`
	ReasoningDepthHints  int      `json:"reasoning_depth_hints"`
	ToolUseMentions      int      `json:"tool_use_mentions"`
	AttachmentCount      int      `json:"attachment_count"`
	HasVisionInput       bool     `json:"has_vision_input"`
	SessionTurnCount     int      `json:"session_turn_count,omitempty"`
}

type Request struct {
	Prompt               string
	Session              *Session
	TaskHint             string
	BudgetRemainingUSD   float64
	MaxLatencyMs         int
	RequireTools         bool
	Exclude              []string
	ExpectedOutputTokens int
	EnableTiebreak       bool
	Tiebreaker           TiebreakFunc
}

type TiebreakFunc func(ctx context.Context, prompt string, top []Candidate) (string, error)

type Session struct {
	Messages           []SessionMessage `json:"messages,omitempty"`
	BudgetRemainingUSD float64          `json:"budget_remaining_usd,omitempty"`
	PriorModelChoices  []string         `json:"prior_model_choices,omitempty"`
}

type SessionMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Candidate struct {
	Model        Model              `json:"-"`
	ModelID      string             `json:"model"`
	Score        float64            `json:"score"`
	Why          string             `json:"why"`
	Components   map[string]float64 `json:"components,omitempty"`
	Filtered     bool               `json:"filtered,omitempty"`
	FilterReason string             `json:"filter_reason,omitempty"`
}

type Recommendation struct {
	SchemaVersion     int         `json:"schema_version"`
	Recommended       string      `json:"recommended"`
	Why               string      `json:"why"`
	Alternatives      []Candidate `json:"alternatives"`
	Filtered          []Candidate `json:"filtered,omitempty"`
	EstInputTokens    int         `json:"est_input_tokens"`
	EstOutputTokens   int         `json:"est_output_tokens"`
	EstCostUSD        float64     `json:"est_cost_usd"`
	EstLatencyMs      int         `json:"est_latency_ms"`
	Fallback          string      `json:"fallback,omitempty"`
	Features          *Features   `json:"features,omitempty"`
	TiebreakAttempted bool        `json:"tiebreak_attempted,omitempty"`
	TiebreakUsed      bool        `json:"tiebreak_used,omitempty"`
	TiebreakError     string      `json:"tiebreak_error,omitempty"`
	AdvisedAt         time.Time   `json:"advised_at"`
}

type CatalogDrift struct {
	UncuratedLive    []string  `json:"uncurated_live"`
	CuratedNotInLive []string  `json:"curated_not_in_live"`
	GeneratedAt      time.Time `json:"generated_at"`
}
