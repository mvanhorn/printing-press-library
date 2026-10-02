// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// cli-printing-press: novel-scaffold-test
// Novel command scaffold tests. Keep the wiring smoke test and add behavior cases as needed.

package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

// TestNovelChatResumeHelpWires smoke-tests that the chat resume command
// resolves at runtime and renders useful --help output. Catches wiring
// regressions (missing AddCommand, panicking RunE on --help, etc.) before
// review. Keep this smoke test when adding behavior-specific cases.
func TestNovelChatResumeHelpWires(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"chat", "resume", "--help"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("chat resume --help error = %v (novel command not wired correctly?)", err)
	}
	help := out.String()
	for _, want := range []string{"Usage:", "resume"} {
		if !strings.Contains(help, want) {
			t.Fatalf("chat resume --help missing %q in output:\n%s", want, help)
		}
	}
}

func TestChatConversationRecordPreservesRequestContext(t *testing.T) {
	response := json.RawMessage(`{"id":"chat-next","model":"sarvam-105b","choices":[{"message":{"role":"assistant","content":"The conclusion was 42."}}]}`)
	messages := []any{
		map[string]any{"role": "system", "content": "Be concise."},
		map[string]any{"role": "user", "content": "What is the answer?"},
	}
	record, err := newChatConversationRecord(response, messages, "sarvam-105b")
	if err != nil {
		t.Fatalf("newChatConversationRecord() error = %v", err)
	}
	if record.ID != "chat-next" || record.Model != "sarvam-105b" || len(record.Messages) != 2 {
		t.Fatalf("record = %#v", record)
	}

	resumed, err := buildChatResumeMessages(record, "Why?")
	if err != nil {
		t.Fatalf("buildChatResumeMessages() error = %v", err)
	}
	if len(resumed) != 4 {
		t.Fatalf("resumed messages = %#v, want four full-context messages", resumed)
	}
	for i, wantRole := range []string{"system", "user", "assistant", "user"} {
		message, ok := resumed[i].(map[string]any)
		if !ok || message["role"] != wantRole {
			t.Fatalf("resumed message %d = %#v, want role %q", i, resumed[i], wantRole)
		}
	}
}

func TestDecodeStoredChatConversationSupportsLegacyResponse(t *testing.T) {
	raw := json.RawMessage(`{"id":"legacy-chat","model":"sarvam-105b","choices":[{"message":{"role":"assistant","content":"Earlier reply"}}]}`)
	record, err := decodeStoredChatConversation(raw)
	if err != nil {
		t.Fatalf("decodeStoredChatConversation() error = %v", err)
	}
	resumed, err := buildChatResumeMessages(record, "Continue")
	if err != nil {
		t.Fatalf("buildChatResumeMessages() error = %v", err)
	}
	if len(resumed) != 2 {
		t.Fatalf("legacy resumed messages = %#v, want assistant plus user", resumed)
	}
}

func TestNewChatConversationRecordRequiresResponseID(t *testing.T) {
	if _, err := newChatConversationRecord(json.RawMessage(`{"choices":[]}`), []any{}, "sarvam-105b"); err == nil {
		t.Fatal("newChatConversationRecord() unexpectedly accepted a response without id")
	}
}

func TestStreamingChatHistoryReconstructsCompletedText(t *testing.T) {
	response := json.RawMessage(": stream opened\n\nevent: message\n" +
		"data: {\"id\":\"stream-1\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":\"The\"}}]}\n\n" +
		"data: {\"id\":\"stream-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\" answer\"}}]}\n\n" +
		"data: {\"id\":\"stream-1\",\"choices\":[]}\n\n" +
		"data: [DONE]\n\n")
	record, err := newChatConversationRecord(response, []any{map[string]any{"role": "user", "content": "Question"}}, "sarvam-105b")
	if err != nil {
		t.Fatal(err)
	}
	resumed, err := buildChatResumeMessages(record, "Why?")
	if err != nil {
		t.Fatal(err)
	}
	if len(resumed) != 3 || resumed[1].(map[string]any)["content"] != "The answer" {
		t.Fatalf("streamed reply was not reconstructed: %#v", resumed)
	}
}

func TestStreamingChatHistoryRejectsIncompleteOrToolCallStreams(t *testing.T) {
	for _, response := range []string{
		"data: {\"id\":\"stream-1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"}}]}\n\n",
		"data: {\"id\":\"stream-1\",\"choices\":[{\"index\":0,\"delta\":{\"tool_calls\":[{\"index\":0}]}}]}\n\ndata: [DONE]\n\n",
	} {
		if _, err := newChatConversationRecord(json.RawMessage(response), []any{}, "sarvam-105b"); err == nil {
			t.Fatal("incomplete streamed context was accepted for resume")
		}
	}
}
