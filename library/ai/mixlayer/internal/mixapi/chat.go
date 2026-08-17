// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package mixapi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/client"
)

type Message struct {
	Role             string          `json:"role"`
	Content          string          `json:"content,omitempty"`
	ReasoningContent string          `json:"reasoning_content,omitempty"`
	ToolCallID       string          `json:"tool_call_id,omitempty"`
	ToolCalls        json.RawMessage `json:"tool_calls,omitempty"`
}

type ChatRequest struct {
	Model               string          `json:"model"`
	Messages            []Message       `json:"messages"`
	Temperature         *float64        `json:"temperature,omitempty"`
	TopP                *float64        `json:"top_p,omitempty"`
	TopK                *int            `json:"top_k,omitempty"`
	FrequencyPenalty    *float64        `json:"frequency_penalty,omitempty"`
	PresencePenalty     *float64        `json:"presence_penalty,omitempty"`
	RepetitionPenalty   *float64        `json:"repetition_penalty,omitempty"`
	MaxCompletionTokens *int            `json:"max_completion_tokens,omitempty"`
	MaxTokens           *int            `json:"max_tokens,omitempty"`
	Stop                []string        `json:"stop,omitempty"`
	Seed                *int64          `json:"seed,omitempty"`
	Stream              bool            `json:"stream,omitempty"`
	ResponseFormat      json.RawMessage `json:"response_format,omitempty"`
	Thinking            *bool           `json:"thinking,omitempty"`
	ReasoningEffort     string          `json:"reasoning_effort,omitempty"`
	Tools               json.RawMessage `json:"tools,omitempty"`
}

type ChatResult struct {
	ID               string          `json:"id,omitempty"`
	Model            string          `json:"model,omitempty"`
	Answer           string          `json:"answer,omitempty"`
	Reasoning        string          `json:"reasoning,omitempty"`
	FinishReason     string          `json:"finish_reason,omitempty"`
	PromptTokens     int             `json:"prompt_tokens,omitempty"`
	CompletionTokens int             `json:"completion_tokens,omitempty"`
	TotalTokens      int             `json:"total_tokens,omitempty"`
	Raw              json.RawMessage `json:"raw,omitempty"`
	LatencyMS        int64           `json:"latency_ms,omitempty"`
}

func Chat(ctx context.Context, c *client.Client, req ChatRequest) (ChatResult, error) {
	start := time.Now()
	raw, _, err := c.PostWithParams(ctx, "/chat/completions", nil, req)
	if err != nil {
		return ChatResult{}, err
	}
	result, err := ParseChat(raw)
	if err != nil {
		return ChatResult{}, err
	}
	result.LatencyMS = time.Since(start).Milliseconds()
	return result, nil
}

func ParseChat(raw json.RawMessage) (ChatResult, error) {
	var envelope struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content          string `json:"content"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return ChatResult{}, fmt.Errorf("parse chat response: %w", err)
	}
	out := ChatResult{
		ID:               envelope.ID,
		Model:            envelope.Model,
		PromptTokens:     envelope.Usage.PromptTokens,
		CompletionTokens: envelope.Usage.CompletionTokens,
		TotalTokens:      envelope.Usage.TotalTokens,
		Raw:              raw,
	}
	if len(envelope.Choices) > 0 {
		out.Answer = envelope.Choices[0].Message.Content
		out.Reasoning = envelope.Choices[0].Message.ReasoningContent
		out.FinishReason = envelope.Choices[0].FinishReason
	}
	return out, nil
}
