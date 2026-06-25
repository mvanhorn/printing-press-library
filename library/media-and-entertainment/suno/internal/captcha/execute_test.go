package captcha

import (
	"strings"
	"testing"
)

func TestSolveJS_EmbedsSitekeyAndEndpoints(t *testing.T) {
	js := solveJS()
	for _, want := range []string{
		SunoHCaptchaSitekey,
		"size:'invisible'",
		hcaptchaEndpoint,
		"hcaptcha.execute(",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("solveJS missing %q", want)
		}
	}
}

func TestClassifyToken(t *testing.T) {
	cases := []struct {
		raw          string
		wantTok      string
		wantInteract bool
		wantErr      bool
	}{
		{"P1_eyJ...goodtoken", "P1_eyJ...goodtoken", false, false},
		{"", "", true, false},
		{"ERR:challenge-expired", "", true, false},
		{"ERR:hcaptcha is not defined", "", false, true},
	}
	for _, tc := range cases {
		tok, interactive, err := classifyToken(tc.raw)
		if tok != tc.wantTok {
			t.Errorf("%q: tok=%q want %q", tc.raw, tok, tc.wantTok)
		}
		if interactive != tc.wantInteract {
			t.Errorf("%q: interactive=%v want %v", tc.raw, interactive, tc.wantInteract)
		}
		if (err != nil) != tc.wantErr {
			t.Errorf("%q: err=%v wantErr=%v", tc.raw, err, tc.wantErr)
		}
	}
}
