package cli

import (
	"context"
	"net/url"
	"strings"
	"testing"
)

func TestEtsySanitizeCallbackTextUnicode(t *testing.T) {
	tests := []struct {
		name, input, want string
	}{
		{"overrides", "access_denied\u202eevil\u202c", "access_deniedevil"},
		{"zero width and direction marks", "a\u200bb\u200fc", "abc"},
		{"isolates", "x\u2066y\u2069z", "xyz"},
		{"all requested controls", "a\u200b\u200c\u200d\u200e\u200f\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069b", "ab"},
		{"ANSI escapes", "\x1b[31mred\x1b[0m", "[31mred[0m"},
		{"whitespace", "tab\tnew\nline", "tab new line"},
		{"non ASCII", "café 日本", "café 日本"},
		{"other format characters retained", "a\u2065\u206ab", "a\u2065\u206ab"},
		{"invisible format characters", "a\u00ad\u061c\u2060\u2061\u2062\u2063\u2064\ufeffb", "ab"},
		{"invisible formats do not consume limit", strings.Repeat("\u00ad\u061c\u2060\u2061\u2062\u2063\u2064\ufeff", 250) + strings.Repeat("界", 201), strings.Repeat("界", 200)},
		{"rune limit", strings.Repeat("界", 300), strings.Repeat("界", 200)},
		{"controls do not consume limit", strings.Repeat("\u202e", 250) + "ok", "ok"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := etsySanitizeCallbackText(tc.input); got != tc.want {
				t.Fatalf("sanitize = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEtsyCallbackErrorDescriptionBidi(t *testing.T) {
	o := etsyTwoStepOptions(t)
	if _, err := etsyStartLogin(o); err != nil {
		t.Fatal(err)
	}
	query := url.Values{"error": {"access_denied"}, "error_description": {"declined\u202eevil"}}
	_, err := etsyCompleteLogin(context.Background(), o, o.RedirectURI+"?"+query.Encode())
	if err == nil {
		t.Fatal("accepted denied callback")
	}
	if got, want := err.Error(), "authorization error: access_denied: declinedevil"; got != want {
		t.Fatalf("callback error = %q, want %q", got, want)
	}
}
