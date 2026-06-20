package athlinks

import (
	"encoding/base64"
	"testing"
)

func TestRacerIDFromToken(t *testing.T) {
	payload := base64.RawURLEncoding.EncodeToString([]byte(`{"athlinks-racer-id":534464435,"sub":"x"}`))
	tok := "Bearer header." + payload + ".sig"
	got, err := RacerIDFromToken(tok)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "534464435" {
		t.Fatalf("got %q want %q", got, "534464435")
	}
}

func TestRacerIDFromTokenBad(t *testing.T) {
	if _, err := RacerIDFromToken("not-a-jwt"); err == nil {
		t.Fatal("expected error for malformed token")
	}
}
