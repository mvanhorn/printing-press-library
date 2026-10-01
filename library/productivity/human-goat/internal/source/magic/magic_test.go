// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package magic

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"
	"testing"
)

type magicRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn magicRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestVerifyModeShortCircuitsMutatingRequests(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "")
	var transportCalls atomic.Int32
	var keyCalls atomic.Int32
	c := &Client{
		baseURL: "https://example.invalid",
		httpClient: &http.Client{Transport: magicRoundTripFunc(func(*http.Request) (*http.Response, error) {
			transportCalls.Add(1)
			return nil, errors.New("transport must not run")
		})},
		keyProvider: func() (string, error) {
			keyCalls.Add(1)
			return "", errors.New("key provider must not run")
		},
	}

	request, err := c.Send(context.Background(), SendParams{
		Title:        "verify",
		Instructions: "do not send",
		Objective:    "prove the transport is gated",
	})
	if err != nil {
		t.Fatalf("Send() in verify mode: %v", err)
	}
	if request == nil {
		t.Fatal("Send() returned a nil synthetic request")
	}
	if got := transportCalls.Load(); got != 0 {
		t.Fatalf("transport calls = %d, want 0", got)
	}
	if got := keyCalls.Load(); got != 0 {
		t.Fatalf("key-provider calls = %d, want 0", got)
	}
}

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
