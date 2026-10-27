// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package magic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/human-goat/internal/cliutil"
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

	t.Run("skips client messages", func(t *testing.T) {
		request := Request{
			Conversation: []ConversationMessage{
				{Content: "Look up the hours", SourceType: "CLIENT"},
			},
		}
		if got := request.Answer(); got != "" {
			t.Fatalf("Answer() = %q, want empty while only our instructions exist", got)
		}
		request.Conversation = append(request.Conversation,
			ConversationMessage{Content: "10am to 5pm", SourceType: "OPERATOR"},
			ConversationMessage{Content: "thanks", SourceType: "CLIENT"})
		if got := request.Answer(); got != "10am to 5pm" {
			t.Fatalf("Answer() = %q, want the operator reply", got)
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

func TestSendAppendsAPIRequestNotice(t *testing.T) {
	var sent map[string]any
	c := &Client{
		baseURL: "https://example.invalid",
		httpClient: &http.Client{Transport: magicRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			if err := json.NewDecoder(r.Body).Decode(&sent); err != nil {
				return nil, err
			}
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"req_1","status":"PENDING"}`))}, nil
		})},
		limiter:     cliutil.NewAdaptiveLimiter(defaultRateLimit),
		keyProvider: func() (string, error) { return "k", nil },
	}
	if _, err := c.Send(context.Background(), SendParams{Title: "t", Instructions: "Look up the hours.", Objective: "o"}); err != nil {
		t.Fatal(err)
	}
	got, _ := sent["instructions"].(string)
	if !strings.HasPrefix(got, "Look up the hours.") || !strings.HasSuffix(got, apiRequestNotice) {
		t.Fatalf("instructions = %q, want caller text followed by the API notice", got)
	}
}

func TestAsksForSubscription(t *testing.T) {
	pitch := "You don't have an active Magic 24/7 plan. Would you like to proceed with the subscription?"
	cases := []struct {
		name string
		conv []ConversationMessage
		want bool
	}{
		{"operator pitch", []ConversationMessage{{Content: "hours?", SourceType: "CLIENT"}, {Content: pitch, SourceType: "ASSISTANT"}}, true},
		{"our own decline after pitch", []ConversationMessage{{Content: pitch, SourceType: "ASSISTANT"}, {Content: "No subscription", SourceType: "CLIENT"}}, true},
		{"answered after pitch", []ConversationMessage{{Content: pitch, SourceType: "ASSISTANT"}, {Content: "Open 10am to 5pm", SourceType: "ASSISTANT"}}, false},
		{"no operator reply", []ConversationMessage{{Content: "subscription question", SourceType: "CLIENT"}}, false},
		{"says no subscription needed", []ConversationMessage{{Content: "No subscription is needed; I'm working on it now.", SourceType: "ASSISTANT"}}, false},
		{"confirms a cancellation", []ConversationMessage{{Content: "I've cancelled your Magic 24/7 plan subscription as requested.", SourceType: "ASSISTANT"}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := &Request{Conversation: tc.conv}
			if got := r.AsksForSubscription(); got != tc.want {
				t.Fatalf("AsksForSubscription() = %v, want %v", got, tc.want)
			}
		})
	}
}
