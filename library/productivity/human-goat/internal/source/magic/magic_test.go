// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package magic

import "testing"

func TestIsInProgress(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{name: "pending", status: "PENDING", want: true},
		{name: "ongoing", status: "ONGOING", want: true},
		{name: "completed", status: "COMPLETED", want: false},
		{name: "cancelled", status: "CANCELLED", want: false},
		{name: "failed", status: "FAILED", want: false},
		{name: "unknown non-empty", status: "WEIRD", want: true},
		{name: "empty", status: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsInProgress(tt.status); got != tt.want {
				t.Fatalf("IsInProgress(%q) = %v, want %v", tt.status, got, tt.want)
			}
		})
	}
}

func TestAnswer(t *testing.T) {
	t.Run("last non-empty conversation content", func(t *testing.T) {
		request := Request{
			Result: "fallback result",
			Conversation: []ConversationMessage{
				{Content: "first"},
				{Content: "   "},
				{Content: "last"},
			},
		}
		if got := request.Answer(); got != "last" {
			t.Fatalf("Answer() = %q, want %q", got, "last")
		}
	})

	t.Run("result fallback", func(t *testing.T) {
		request := Request{Result: "from result", Conversation: []ConversationMessage{}}
		if got := request.Answer(); got != "from result" {
			t.Fatalf("Answer() = %q, want %q", got, "from result")
		}
	})

	t.Run("empty", func(t *testing.T) {
		request := Request{Conversation: []ConversationMessage{}}
		if got := request.Answer(); got != "" {
			t.Fatalf("Answer() = %q, want empty", got)
		}
	})
}
