package cli

import (
	"crypto/sha256"
	"encoding/base64"
	"strings"
	"testing"
)

func TestNewPKCEState_Shape(t *testing.T) {
	p, err := newPKCEState()
	if err != nil {
		t.Fatalf("newPKCEState: %v", err)
	}
	// RFC 7636 says verifier MUST be 43-128 chars in the unreserved-character set.
	if len(p.Verifier) < 43 {
		t.Errorf("verifier too short: %d chars (RFC 7636 min 43)", len(p.Verifier))
	}
	if strings.ContainsAny(p.Verifier, "+/=") {
		t.Errorf("verifier must be base64url (no +/=); got %q", p.Verifier)
	}
	if len(p.State) < 16 {
		t.Errorf("state too short: %d chars", len(p.State))
	}
	if p.Challenge == "" {
		t.Error("challenge empty")
	}
}

func TestNewPKCEState_ChallengeIsS256OfVerifier(t *testing.T) {
	p, err := newPKCEState()
	if err != nil {
		t.Fatalf("newPKCEState: %v", err)
	}
	sum := sha256.Sum256([]byte(p.Verifier))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	if p.Challenge != want {
		t.Errorf("challenge != S256(verifier)\n got %q\nwant %q", p.Challenge, want)
	}
}

func TestNewPKCEState_DistinctValues(t *testing.T) {
	a, _ := newPKCEState()
	b, _ := newPKCEState()
	if a.Verifier == b.Verifier {
		t.Error("two PKCE states share a verifier (rand.Read failed?)")
	}
	if a.State == b.State {
		t.Error("two PKCE states share a state (rand.Read failed?)")
	}
}

func TestBuildTeslaAuthURL_Shape(t *testing.T) {
	p := &pkceState{Verifier: "verif", Challenge: "chall", State: "stateA"}
	got := buildTeslaAuthURL(p)
	if !strings.HasPrefix(got, teslaAuthURL+"?") {
		t.Errorf("auth URL prefix: %q", got)
	}
	for _, want := range []string{
		"client_id=ownerapi",
		"redirect_uri=https%3A%2F%2Fauth.tesla.com%2Fvoid%2Fcallback",
		"response_type=code",
		"scope=openid+email+offline_access",
		"code_challenge=chall",
		"code_challenge_method=S256",
		"state=stateA",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("auth URL missing %q\nfull: %s", want, got)
		}
	}
}

func TestParseCallbackURL(t *testing.T) {
	cases := []struct {
		name      string
		in        string
		wantCode  string
		wantState string
		wantErr   string
	}{
		{
			name:      "happy_path",
			in:        "https://auth.tesla.com/void/callback?code=abc123&state=s1&issuer=https%3A%2F%2Fauth.tesla.com%2Foauth2%2Fv3",
			wantCode:  "abc123",
			wantState: "s1",
		},
		{
			name:      "tolerates_trailing_whitespace",
			in:        "  https://auth.tesla.com/void/callback?code=xyz&state=s2  \n",
			wantCode:  "xyz",
			wantState: "s2",
		},
		{
			name:      "tolerates_surrounding_quotes",
			in:        `"https://auth.tesla.com/void/callback?code=q1&state=q2"`,
			wantCode:  "q1",
			wantState: "q2",
		},
		{
			name:    "empty_url",
			in:      "",
			wantErr: "empty URL",
		},
		{
			name:    "non_url",
			in:      "this is not a url",
			wantErr: "missing scheme",
		},
		{
			name:    "url_with_no_code",
			in:      "https://auth.tesla.com/oauth2/v3/authorize?client_id=ownerapi",
			wantErr: "missing ?code=",
		},
		{
			name:    "login_cancelled",
			in:      "https://auth.tesla.com/void/callback?error=login_cancelled",
			wantErr: "cancelled",
		},
		{
			name:    "error_with_description",
			in:      "https://auth.tesla.com/void/callback?error=invalid_request&error_description=missing+state",
			wantErr: "invalid_request",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, state, err := parseCallbackURL(c.in)
			if c.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q; got code=%q state=%q", c.wantErr, code, state)
				}
				if !strings.Contains(err.Error(), c.wantErr) {
					t.Errorf("error: got %q want substring %q", err.Error(), c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if code != c.wantCode {
				t.Errorf("code: got %q want %q", code, c.wantCode)
			}
			if state != c.wantState {
				t.Errorf("state: got %q want %q", state, c.wantState)
			}
		})
	}
}

func TestReadSingleLine(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"hello\n", "hello"},
		{"  spaced  \n", "spaced"},
		{"no-newline-eof", "no-newline-eof"},
		{"", ""},
		{"line1\nline2\n", "line1"}, // single-line: stops at first newline
	}
	for _, c := range cases {
		got, err := readSingleLine(strings.NewReader(c.in))
		if err != nil {
			t.Fatalf("readSingleLine(%q): %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("readSingleLine(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestReadSingleLine_OversizeRejected(t *testing.T) {
	// > 8KB without a newline
	huge := strings.Repeat("x", 9000)
	_, err := readSingleLine(strings.NewReader(huge))
	if err == nil {
		t.Error("expected oversize error")
	}
}
