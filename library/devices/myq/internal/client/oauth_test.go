package client

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestPKCEChallengeShapes(t *testing.T) {
	challenge, verifier := pkceChallenge()
	if challenge == "" || verifier == "" {
		t.Fatalf("expected challenge and verifier, got %q / %q", challenge, verifier)
	}
	if strings.ContainsAny(challenge, "+/=") {
		t.Fatalf("challenge should be URL-safe, got %q", challenge)
	}
	if strings.ContainsAny(verifier, "+/=") {
		t.Fatalf("verifier should be URL-safe, got %q", verifier)
	}
}

func TestVerificationToken(t *testing.T) {
	doc, err := html.Parse(strings.NewReader(`<html><body><form><input name="__RequestVerificationToken" value="token-123"></form></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	if got := verificationToken(doc); got != "token-123" {
		t.Fatalf("verificationToken() = %q, want %q", got, "token-123")
	}
}
