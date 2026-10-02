// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Library patch: persist request context alongside Sarvam chat responses.

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/sarvam/internal/store"
)

// chatConversationRecord is the durable local shape used by `chat resume`.
// Sarvam responses include the assistant message but not the request messages,
// so persisting the raw response alone cannot reconstruct a conversation.
type chatConversationRecord struct {
	ID       string          `json:"id"`
	Model    string          `json:"model,omitempty"`
	Messages []any           `json:"messages"`
	Response json.RawMessage `json:"response"`
}

func newChatConversationRecord(response json.RawMessage, messages []any, model string) (chatConversationRecord, error) {
	var err error
	response, err = chatResponseForHistory(response)
	if err != nil {
		return chatConversationRecord{}, err
	}
	var envelope struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response, &envelope); err != nil {
		return chatConversationRecord{}, fmt.Errorf("parsing chat response: %w", err)
	}
	if envelope.ID == "" {
		return chatConversationRecord{}, fmt.Errorf("chat response has no conversation id")
	}
	return chatConversationRecord{
		ID:       envelope.ID,
		Model:    model,
		Messages: append([]any(nil), messages...),
		Response: append(json.RawMessage(nil), response...),
	}, nil
}

// Sarvam streams chat completions as SSE chunks. Only a completed stream with
// a plain text first choice can be reconstructed into a resumable assistant
// message. Reasoning tokens are intentionally not added to the conversation.
func chatResponseForHistory(response json.RawMessage) (json.RawMessage, error) {
	trimmed := bytes.TrimSpace(response)
	if bytes.HasPrefix(trimmed, []byte("{")) {
		return response, nil
	}
	var id string
	var content strings.Builder
	completed := false
	for _, line := range bytes.Split(trimmed, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("data:")) {
			continue
		}
		payload := bytes.TrimSpace(line[len("data:"):])
		if bytes.Equal(payload, []byte("[DONE]")) {
			completed = true
			break
		}
		var chunk struct {
			ID      string `json:"id"`
			Choices []struct {
				Index int `json:"index"`
				Delta struct {
					Content   string          `json:"content"`
					ToolCalls json.RawMessage `json:"tool_calls"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(payload, &chunk); err != nil {
			return nil, fmt.Errorf("parsing streamed chat chunk: %w", err)
		}
		if chunk.ID != "" {
			if id != "" && id != chunk.ID {
				return nil, fmt.Errorf("streamed chat changed conversation id")
			}
			id = chunk.ID
		}
		for _, choice := range chunk.Choices {
			if choice.Index != 0 {
				continue
			}
			if len(choice.Delta.ToolCalls) > 0 && !bytes.Equal(choice.Delta.ToolCalls, []byte("null")) && !bytes.Equal(choice.Delta.ToolCalls, []byte("[]")) {
				return nil, fmt.Errorf("streamed chat with tool calls cannot be saved for resume")
			}
			content.WriteString(choice.Delta.Content)
		}
	}
	if !completed || id == "" || content.Len() == 0 {
		return nil, fmt.Errorf("streamed chat has no complete text reply to save")
	}
	return json.Marshal(map[string]any{
		"id":      id,
		"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": content.String()}}},
	})
}

func persistChatConversation(ctx context.Context, response json.RawMessage, messages []any, model string) error {
	record, err := newChatConversationRecord(response, messages, model)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encoding local chat history: %w", err)
	}
	db, err := store.OpenWithContext(ctx, defaultDBPath("sarvam-pp-cli"))
	if err != nil {
		return fmt.Errorf("opening local chat history: %w", err)
	}
	defer db.Close()
	if err := db.Upsert("chat", record.ID, raw); err != nil {
		return fmt.Errorf("saving local chat history: %w", err)
	}
	return nil
}

func decodeStoredChatConversation(raw json.RawMessage) (chatConversationRecord, error) {
	var record chatConversationRecord
	if err := json.Unmarshal(raw, &record); err != nil {
		return chatConversationRecord{}, fmt.Errorf("parsing stored conversation: %w", err)
	}
	if len(record.Response) > 0 || record.Messages != nil {
		if record.ID == "" {
			return chatConversationRecord{}, fmt.Errorf("stored conversation has no id")
		}
		return record, nil
	}

	// Compatibility with development databases that stored only the raw
	// Sarvam response before the request-context record was introduced.
	var legacy struct {
		ID    string `json:"id"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(raw, &legacy); err != nil || legacy.ID == "" {
		return chatConversationRecord{}, fmt.Errorf("stored conversation is not a recognized chat record")
	}
	return chatConversationRecord{
		ID:       legacy.ID,
		Model:    legacy.Model,
		Messages: []any{},
		Response: append(json.RawMessage(nil), raw...),
	}, nil
}

func buildChatResumeMessages(record chatConversationRecord, userMessage string) ([]any, error) {
	messages := append([]any(nil), record.Messages...)
	var response struct {
		Choices []struct {
			Message json.RawMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(record.Response, &response); err != nil {
		return nil, fmt.Errorf("parsing stored chat response: %w", err)
	}
	if len(response.Choices) > 0 && len(response.Choices[0].Message) > 0 {
		var assistantMessage any
		if err := json.Unmarshal(response.Choices[0].Message, &assistantMessage); err != nil {
			return nil, fmt.Errorf("parsing stored assistant message: %w", err)
		}
		messages = append(messages, assistantMessage)
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("stored conversation has no resumable messages")
	}
	messages = append(messages, map[string]any{"role": "user", "content": userMessage})
	return messages, nil
}
