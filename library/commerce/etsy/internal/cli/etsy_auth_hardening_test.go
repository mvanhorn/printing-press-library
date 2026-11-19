package cli

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/commerce/etsy/internal/cliutil/testenv"
)

func TestEtsyCallbackErrorControls(t *testing.T) {
	o := etsyTwoStepOptions(t)
	if _, err := etsyStartLogin(o); err != nil {
		t.Fatal(err)
	}
	_, err := etsyCompleteLogin(context.Background(), o, o.RedirectURI+"?error=access_denied%1B%5B31m&error_description=bad%0D%0Aline%C2%85x%00%7F%C2%9F%09")
	if err == nil {
		t.Fatal("accepted denied callback")
	}
	for _, r := range err.Error() {
		if r < 0x20 || (r >= 0x7f && r <= 0x9f) {
			t.Fatalf("control rune %U", r)
		}
	}
	for _, part := range []string{"access_denied", "bad", "line"} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("missing readable part %q", part)
		}
	}
	if got := etsySanitizeCallbackText("a\tb\nc\rd\x1b\x00\u0085\x7f"); got != "a b c d" {
		t.Fatalf("sanitize = %q", got)
	}
	if got := etsySanitizeCallbackText(strings.Repeat("界", 201)); utf8.RuneCountInString(got) != 200 {
		t.Fatal("missing rune limit")
	}
}

func TestEtsyLoginHelpFlows(t *testing.T) {
	testenv.Isolate(t)
	out, _, err := s2Run(t, "auth", "login", "--help")
	s2Must(t, err)
	for _, flag := range []string{"--all-scopes", "--redirect-uri", "--callback-url"} {
		if !strings.Contains(out, flag) {
			t.Errorf("help missing %s", flag)
		}
	}
}
