package mixapi

import (
	"encoding/json"
	"testing"
)

func TestParseChatExtractsAnswerReasoningAndUsage(t *testing.T) {
	raw := json.RawMessage(`{
		"id": "chat_1",
		"model": "qwen/qwen3.5-9b",
		"choices": [{"finish_reason": "stop", "message": {"content": "answer", "reasoning_content": "thoughts"}}],
		"usage": {"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18}
	}`)
	got, err := ParseChat(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "chat_1" || got.Model != "qwen/qwen3.5-9b" || got.Answer != "answer" || got.Reasoning != "thoughts" {
		t.Fatalf("unexpected parse: %#v", got)
	}
	if got.PromptTokens != 11 || got.CompletionTokens != 7 || got.TotalTokens != 18 {
		t.Fatalf("unexpected usage: %#v", got)
	}
}

func TestParseChatHandlesNoChoices(t *testing.T) {
	got, err := ParseChat(json.RawMessage(`{"id":"chat_2","choices":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got.Answer != "" || got.Reasoning != "" || got.ID != "chat_2" {
		t.Fatalf("unexpected parse without choices: %#v", got)
	}
}

func TestParseChatKeepsRawResponse(t *testing.T) {
	raw := json.RawMessage(`{"id":"chat_3"}`)
	got, err := ParseChat(raw)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Raw) != string(raw) {
		t.Fatalf("raw response not preserved: %s", got.Raw)
	}
}

func TestParseChatReturnsDecodeError(t *testing.T) {
	if _, err := ParseChat(json.RawMessage(`{"id":`)); err == nil {
		t.Fatal("ParseChat invalid JSON error = nil, want error")
	}
}
