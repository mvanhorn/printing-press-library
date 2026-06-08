package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestParseContinuenteHARCookieSets(t *testing.T) {
	t.Parallel()

	raw := []byte(`{
  "log": {
    "entries": [
      {
        "request": {
          "url": "https://www.continente.pt/login/",
          "cookies": [
            {
              "name": "sid",
              "value": "abc",
              "domain": ".continente.pt",
              "path": "/",
              "secure": true,
              "httpOnly": true
            }
          ]
        },
        "response": {
          "cookies": [
            {
              "name": "idm.session",
              "value": "def",
              "domain": "login.continente.pt",
              "path": "/",
              "secure": true
            }
          ]
        }
      },
      {
        "request": {
          "url": "https://example.com/",
          "cookies": [
            {
              "name": "ignore",
              "value": "x"
            }
          ]
        },
        "response": {
          "cookies": []
        }
      }
    ]
  }
}`)

	got, err := parseContinuenteHARCookieSets(raw)
	if err != nil {
		t.Fatalf("parseContinuenteHARCookieSets: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	cookies := got["https://www.continente.pt/"]
	if len(cookies) != 2 {
		t.Fatalf("len(cookies) = %d, want 2", len(cookies))
	}
}

func TestContinenteHost(t *testing.T) {
	t.Parallel()

	if !continenteHost("www.continente.pt") {
		t.Fatal("expected continente host to match")
	}
	if !continenteHost(".login.continente.pt") {
		t.Fatal("expected dotted continente subdomain to match")
	}
	if continenteHost("example.com") {
		t.Fatal("unexpected match for foreign domain")
	}
}

func TestParseContinuenteCookieExportSets(t *testing.T) {
	t.Parallel()

	raw := []byte(`[
  {
    "domain": ".continente.pt",
    "name": "sid",
    "value": "abc",
    "path": "/",
    "secure": true,
    "httpOnly": true
  },
  {
    "domain": "login.continente.pt",
    "name": "idm.session",
    "value": "def",
    "path": "/",
    "secure": true
  },
  {
    "domain": ".example.com",
    "name": "ignore",
    "value": "x"
  }
]`)

	got, err := parseContinuenteCookieExportSets(raw)
	if err != nil {
		t.Fatalf("parseContinuenteCookieExportSets: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if len(got["https://continente.pt/"]) != 1 {
		t.Fatalf("expected continente.pt cookie bucket, got %#v", got)
	}
	if len(got["https://login.continente.pt/"]) != 1 {
		t.Fatalf("expected login.continente.pt cookie bucket, got %#v", got)
	}
}

func TestGeneratePKCEPair(t *testing.T) {
	t.Parallel()

	verifier, challenge, err := generatePKCEPair()
	if err != nil {
		t.Fatalf("generatePKCEPair: %v", err)
	}
	if verifier == "" || challenge == "" {
		t.Fatal("expected non-empty PKCE values")
	}
	if strings.Contains(verifier, "=") || strings.Contains(challenge, "=") {
		t.Fatalf("expected raw-url-safe PKCE values without padding: verifier=%q challenge=%q", verifier, challenge)
	}
}

func TestParseAuthorizationCode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "object authorizationCode", raw: `{"authorizationCode":"abc123"}`, want: "abc123"},
		{name: "object code", raw: `{"code":"xyz"}`, want: "xyz"},
		{name: "json string", raw: `"plain-code"`, want: "plain-code"},
		{name: "plain text", raw: `raw-code`, want: "raw-code"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseAuthorizationCode([]byte(tt.raw))
			if err != nil {
				t.Fatalf("parseAuthorizationCode: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q want %q", got, tt.want)
			}
		})
	}
}

func TestParseAuthorizationCodeRejectsUnsupportedPayload(t *testing.T) {
	t.Parallel()

	_, err := parseAuthorizationCode([]byte(`{"unexpected":"value"}`))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReadSecretFromStdin(t *testing.T) {
	t.Parallel()

	original := stdinReader
	t.Cleanup(func() { stdinReader = original })
	stdinReader = strings.NewReader("secret-value\n")

	got, err := readSecretFromStdin()
	if err != nil {
		t.Fatalf("readSecretFromStdin: %v", err)
	}
	if got != "secret-value" {
		t.Fatalf("got %q", got)
	}
}

func TestReadSecretFromStdinPropagatesReadErrors(t *testing.T) {
	t.Parallel()

	original := stdinReader
	t.Cleanup(func() { stdinReader = original })
	stdinReader = errReader{err: errors.New("boom")}

	_, err := readSecretFromStdin()
	if err == nil {
		t.Fatal("expected error")
	}
}

type errReader struct {
	err error
}

func (r errReader) Read(_ []byte) (int, error) {
	return 0, r.err
}
