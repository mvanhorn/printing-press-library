// Copyright 2026 OrcaRouter contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/orcarouter/internal/client"
)

// TestChatResponseJSON verifies the OpenAI-compatible chat completion response
// unmarshals from a realistic OrcaRouter payload.
func TestChatResponseJSON(t *testing.T) {
	raw := `{
		"id": "gen-1788067319-FThBBLUc29M4EsYhP4cM",
		"object": "chat.completion",
		"created": 1788067319,
		"model": "openai/gpt-oss-120b",
		"provider": "AkashML",
		"choices": [{
			"index": 0,
			"finish_reason": "stop",
			"message": {"role": "assistant", "content": "OK"}
		}],
		"usage": {"prompt_tokens": 72, "completion_tokens": 10, "total_tokens": 82, "cost": 0.000023}
	}`
	var resp client.ChatResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "OK" {
		t.Fatalf("expected content OK, got %q", resp.Choices[0].Message.Content)
	}
	if resp.Usage.TotalTokens != 82 {
		t.Fatalf("expected 82 total tokens, got %d", resp.Usage.TotalTokens)
	}
}

// TestModelsJSON verifies the model catalog response unmarshals.
func TestModelsJSON(t *testing.T) {
	raw := `{
		"data": [
			{"id": "orcarouter/free", "object": "model", "created": 0, "owned_by": "orcarouter", "supported_endpoint_types": ["openai", "anthropic"]},
			{"id": "orcarouter/fusion", "object": "model", "created": 0, "owned_by": "orcarouter", "context_length": 1000000, "supported_endpoint_types": ["openai"]}
		]
	}`
	var list client.ModelList
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(list.Data) != 2 {
		t.Fatalf("expected 2 models, got %d", len(list.Data))
	}
	if list.Data[1].ContextLength != 1000000 {
		t.Fatalf("expected context_length 1000000, got %d", list.Data[1].ContextLength)
	}
}

// TestExitCode verifies the error-to-exit-code mapping.
func TestExitCode(t *testing.T) {
	if code := ExitCode(nil); code != 0 {
		t.Fatalf("expected 0 for nil error, got %d", code)
	}
	if code := ExitCode(exitCodeError(2)); code != 2 {
		t.Fatalf("expected 2 for cliError, got %d", code)
	}
	if code := ExitCode(errors.New("boom")); code != 1 {
		t.Fatalf("expected 1 for generic error, got %d", code)
	}
}
