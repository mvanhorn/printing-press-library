package scengine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Relevance gate labels reported next to stance_method, so the recall cost of
// the LLM split is visible when comparing runs.
const (
	// GatePICO means PICOTokens split the claim itself.
	GatePICO = "pico"
	// GateTokenOverlap means the title/topic overlap gate ran.
	GateTokenOverlap = "token_overlap"
	// gateLLMSplitPrefix is followed by the provider name, e.g.
	// "llm_split:anthropic": the PICO gate ran on an LLM split.
	gateLLMSplitPrefix = "llm_split:"
)

// ClaimSplitAuto returns the intervention and outcome tokens for the PICO gate
// plus a label for the gate they select. PICOTokens always goes first. An LLM
// split is tried only when it returns an empty side and a provider key is
// already configured. Any LLM error or empty split returns no tokens and
// GateTokenOverlap, which tells the caller to use the overlap gate.
func ClaimSplitAuto(ctx context.Context, claim string) (ivTokens, outTokens []string, gate string) {
	if iv, out := PICOTokens(claim); len(iv) > 0 && len(out) > 0 {
		return iv, out, GatePICO
	}
	if LLMConfigured() {
		if iv, out, err := SplitClaimWithLLM(ctx, claim); err == nil {
			return iv, out, gateLLMSplitPrefix + LLMProviderName()
		}
	}
	return nil, nil, GateTokenOverlap
}

// SplitClaimWithLLM asks the first configured provider for the claim's
// intervention and outcome. Replies go through picoSideTokens, so they match
// abstracts the same way PICOTokens output does. It returns an error when no
// key is set, the call fails, the reply cannot be parsed, or a side is empty.
func SplitClaimWithLLM(ctx context.Context, claim string) (ivTokens, outTokens []string, err error) {
	p, key := activeProvider()
	if p == nil {
		return nil, nil, errors.New("no LLM provider configured")
	}

	ctx, cancel := context.WithTimeout(ctx, llmTimeout)
	defer cancel()

	raw, err := p.call(ctx, key, splitPrompt(claim))
	if err != nil {
		return nil, nil, fmt.Errorf("%s claim split failed: %w", p.name, err)
	}
	return parseSplitJSON(raw)
}

// splitPrompt pins the model to STRICT JSON with the two sides we parse.
func splitPrompt(claim string) string {
	var b strings.Builder
	b.WriteString("Split a scientific claim into the two things it relates.\n\n")
	b.WriteString("Claim: ")
	b.WriteString(claim)
	b.WriteString("\n\nThe intervention is the exposure, treatment or cause. ")
	b.WriteString("The outcome is what it is claimed to affect. ")
	b.WriteString("Use short noun phrases taken from the claim, and leave a side empty if the claim does not name it.\n\n")
	b.WriteString("Respond with STRICT JSON ONLY, no prose, no markdown fences, ")
	b.WriteString(`exactly this shape: {"intervention": "...", "outcome": "..."}.`)
	return b.String()
}

type splitReply struct {
	Intervention string `json:"intervention"`
	Outcome      string `json:"outcome"`
}

// parseSplitJSON turns a model reply into gate tokens. An empty side is an
// error: the gate needs both sides, and the caller falls back on any error.
func parseSplitJSON(raw string) ([]string, []string, error) {
	match := jsonObjectRe.FindString(raw)
	if match == "" {
		return nil, nil, fmt.Errorf("no JSON object in model reply: %q", truncateForErr(raw))
	}

	var reply splitReply
	if err := json.Unmarshal([]byte(match), &reply); err != nil {
		return nil, nil, fmt.Errorf("invalid claim split JSON: %w", err)
	}

	iv := picoSideTokens(reply.Intervention)
	out := picoSideTokens(reply.Outcome)
	if len(iv) == 0 || len(out) == 0 {
		return nil, nil, errors.New("claim split has an empty side")
	}
	return iv, out, nil
}
