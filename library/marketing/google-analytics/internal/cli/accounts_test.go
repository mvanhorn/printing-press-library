package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type accountTransport func(*http.Request) (*http.Response, error)

func (f accountTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSavedAccountCommands(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GOOGLE_ANALYTICS_ACCOUNT", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	t.Setenv("GA4_PROPERTY_ID", "")
	t.Setenv("GA4_PROPERTY_IDS", "")
	private, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
	minted := []string{}
	authorized := []string{}
	old := http.DefaultTransport
	http.DefaultTransport = accountTransport(func(r *http.Request) (*http.Response, error) {
		body := ""
		if r.URL.Host == "oauth2.googleapis.com" {
			if err := r.ParseForm(); err != nil {
				t.Fatal(err)
			}
			parts := strings.Split(r.Form.Get("assertion"), ".")
			if len(parts) != 3 {
				t.Fatal("invalid JWT")
			}
			b, err := base64.RawURLEncoding.DecodeString(parts[1])
			if err != nil {
				t.Fatal(err)
			}
			var claims map[string]any
			if err := json.Unmarshal(b, &claims); err != nil {
				t.Fatal(err)
			}
			identity := claims["iss"].(string)
			minted = append(minted, identity)
			body = `{"access_token":"synthetic-token-` + identity + `"}`
		} else {
			authorized = append(authorized, r.Header.Get("Authorization"))
			body = `{"accountSummaries":[]}`
		}
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	run := func(args ...string) (string, error) {
		var out bytes.Buffer
		root := RootCmd()
		root.SetArgs(args)
		root.SetOut(&out)
		root.SetErr(&out)
		err := root.Execute()
		if strings.Contains(out.String(), "PRIVATE KEY") || strings.Contains(out.String(), "synthetic-token-") {
			t.Fatal("secret leaked")
		}
		return out.String(), err
	}
	var original string
	for _, name := range []string{"bestself", "littlemight"} {
		b, _ := json.Marshal(map[string]string{"client_email": name + "@example.invalid", "private_key": keyPEM})
		source := filepath.Join(t.TempDir(), "credentials.json")
		if err := os.WriteFile(source, b, 0o600); err != nil {
			t.Fatal(err)
		}
		original = source
		if _, err := run("auth", "add", name, "--from-credentials", source); err != nil {
			t.Fatal(err)
		}
		p, _ := accountPath(name)
		info, err := os.Stat(p)
		if err != nil || info.Mode().Perm() != 0o600 {
			t.Fatal("permissions", err)
		}
		if _, err := run("properties", "--account", name, "--json"); err != nil {
			t.Fatal(err)
		}
		ctx, err := run("agent-context", "--account", name)
		if err != nil || !strings.Contains(ctx, `"active_account": "`+name+`"`) {
			t.Fatal("context account", err)
		}
		out, err := run("doctor", "--account", name, "--agent")
		if err != nil || !strings.Contains(out, `"active_account": "`+name+`"`) {
			t.Fatal("doctor account", err)
		}
	}
	if len(minted) != 4 || minted[0] != "bestself@example.invalid" || minted[2] != "littlemight@example.invalid" {
		t.Fatal("wrong minted identities")
	}
	if len(authorized) != 4 || authorized[0] != "Bearer synthetic-token-bestself@example.invalid" || authorized[2] != "Bearer synthetic-token-littlemight@example.invalid" {
		t.Fatal("wrong request authorization")
	}
	if _, err := run("auth", "set-default", "bestself"); err != nil {
		t.Fatal(err)
	}
	if got := credentialPath(&rootFlags{}); !strings.Contains(got, "bestself") {
		t.Fatal("saved default")
	}
	t.Setenv("GOOGLE_ANALYTICS_ACCOUNT", "littlemight")
	if got := credentialPath(&rootFlags{}); !strings.Contains(got, "littlemight") {
		t.Fatal("env selection")
	}
	if got := credentialPath(&rootFlags{accountName: "bestself"}); !strings.Contains(got, "bestself") {
		t.Fatal("flag selection")
	}
	if _, _, err := resolveAccount(&rootFlags{credentials: original}); err == nil {
		t.Fatal("conflicting selectors")
	}
	t.Setenv("GOOGLE_ANALYTICS_ACCOUNT", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", original)
	if got := credentialPath(&rootFlags{}); !strings.Contains(got, "bestself") {
		t.Fatal("ambient credentials changed saved account")
	}
	if _, err := run("auth", "remove", "bestself", "--yes"); err == nil {
		t.Fatal("removed current default")
	}
	if _, err := run("auth", "set-default", "default"); err != nil {
		t.Fatal(err)
	}
	if credentialPath(&rootFlags{}) != original {
		t.Fatal("legacy env path")
	}
	if credentialPath(&rootFlags{credentials: "explicit.json"}) != "explicit.json" {
		t.Fatal("legacy explicit path")
	}
	if _, err := run("auth", "remove", "littlemight"); err == nil {
		t.Fatal("missing confirmation")
	}
	if _, err := run("auth", "remove", "littlemight", "--yes"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(original); err != nil {
		t.Fatal("deleted original")
	}
	if _, err := run("auth", "add", "bestself", "--from-credentials", original); err == nil {
		t.Fatal("overwrote saved account")
	}
	if _, err := run("auth", "accounts"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"../bad", "a/b", "", "A", "a..b", "default-account", "config.toml"} {
		if _, err := accountPath(name); err == nil {
			t.Fatal("invalid name")
		}
	}
	malformed := filepath.Join(t.TempDir(), "malformed.json")
	if err := os.WriteFile(malformed, []byte(`{"private_key":secret-never-print}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := run("doctor", "--credentials", malformed, "--agent")
	if err != nil || strings.Contains(out, "secret-never-print") {
		t.Fatal("redaction", err)
	}
}

func TestExclusiveCredentialPublication(t *testing.T) {
	p := filepath.Join(t.TempDir(), "credentials.json")
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- writePrivate(p, []byte("synthetic-key"), true) }()
	}
	successes := 0
	for i := 0; i < 2; i++ {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatal("concurrent creation overwrote a saved key")
	}
	info, err := os.Stat(p)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("credential permissions", err)
	}
}
