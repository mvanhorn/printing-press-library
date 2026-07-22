package cli

import (
	"strings"
	"testing"
)

func TestBuildLinqMessageBodyNestedShape(t *testing.T) {
	result := buildLinqMessageBody(linqMessageBuildOptions{
		Texts:               []string{"Congrats!"},
		Effect:              "screen:confetti",
		Decorations:         []string{"0:8:bold"},
		PreferredService:    "imessage",
		ReplyToMessageID:    "msg_123",
		ReplyToPartIndex:    0,
		HasReplyToPartIndex: true,
		IdempotencyKey:      "req_123",
	})
	if !result.Sendable {
		t.Fatalf("expected sendable body, got errors: %v", result.Errors)
	}
	message, ok := result.Body["message"].(map[string]any)
	if !ok {
		t.Fatalf("body did not contain nested message: %#v", result.Body)
	}
	if _, exists := result.Body["parts"]; exists {
		t.Fatalf("parts must not be top-level: %#v", result.Body)
	}
	if got := message["preferred_service"]; got != "iMessage" {
		t.Fatalf("preferred_service = %v, want iMessage", got)
	}
	if got := message["idempotency_key"]; got != "req_123" {
		t.Fatalf("idempotency_key = %v, want req_123", got)
	}
	effect := message["effect"].(map[string]any)
	if effect["type"] != "screen" || effect["name"] != "confetti" {
		t.Fatalf("unexpected effect: %#v", effect)
	}
	replyTo := message["reply_to"].(map[string]any)
	if replyTo["message_id"] != "msg_123" || replyTo["part_index"] != 0 {
		t.Fatalf("unexpected reply_to: %#v", replyTo)
	}
	parts := message["parts"].([]map[string]any)
	decorations := parts[0]["text_decorations"].([]map[string]any)
	if decorations[0]["style"] != "bold" {
		t.Fatalf("expected bold decoration, got %#v", decorations[0])
	}
}

func TestBuildLinqMessageBodyRejectsLinkMixedWithText(t *testing.T) {
	result := buildLinqMessageBody(linqMessageBuildOptions{
		Texts: []string{"Look"},
		Link:  "https://example.com",
	})
	if result.Sendable {
		t.Fatalf("expected mixed link/text to be rejected")
	}
	if !containsValidationError(result.Errors, "link parts must be the only part") {
		t.Fatalf("missing link-only error: %v", result.Errors)
	}
}

func TestBuildLinqMessageBodyRejectsConsecutiveText(t *testing.T) {
	result := buildLinqMessageBody(linqMessageBuildOptions{
		Texts: []string{"one", "two"},
	})
	if result.Sendable {
		t.Fatalf("expected consecutive text parts to be rejected")
	}
	if !containsValidationError(result.Errors, "consecutive text parts") {
		t.Fatalf("missing consecutive-text error: %v", result.Errors)
	}
}

func TestBuildLinqMessageBodyRejectsTooManyPublicURLMedia(t *testing.T) {
	urls := make([]string, 41)
	for i := range urls {
		urls[i] = "https://cdn.example/photo.jpg"
	}
	result := buildLinqMessageBody(linqMessageBuildOptions{MediaURLs: urls})
	if result.Sendable {
		t.Fatalf("expected too many public URL media parts to be rejected")
	}
	if !containsValidationError(result.Errors, "40 public-URL media") {
		t.Fatalf("missing public URL media error: %v", result.Errors)
	}
}

func TestBuildLinqMessageBodyRejectsBadPreferredService(t *testing.T) {
	result := buildLinqMessageBody(linqMessageBuildOptions{
		Texts:            []string{"hello"},
		PreferredService: "MMS",
	})
	if result.Sendable {
		t.Fatalf("expected bad preferred service to be rejected")
	}
	if !containsValidationError(result.Errors, "preferred-service") {
		t.Fatalf("missing preferred-service error: %v", result.Errors)
	}
}

func TestBuildLinqMessageBodyRejectsOverlappingAnimation(t *testing.T) {
	result := buildLinqMessageBody(linqMessageBuildOptions{
		Texts:       []string{"Hello world"},
		Decorations: []string{"0:5:shake", "0:5:bold"},
	})
	if result.Sendable {
		t.Fatalf("expected overlapping animation to be rejected")
	}
	if !containsValidationError(result.Errors, "animation decoration ranges") {
		t.Fatalf("missing overlap error: %v", result.Errors)
	}
}

func containsValidationError(errors []string, want string) bool {
	for _, err := range errors {
		if strings.Contains(err, want) {
			return true
		}
	}
	return false
}
