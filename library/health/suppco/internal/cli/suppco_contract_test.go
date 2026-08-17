package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/client"
	"github.com/mvanhorn/printing-press-library/library/health/suppco/internal/config"
)

func TestConfigureSuppCoClientIsStatelessAndPinsOrigin(t *testing.T) {
	c := &client.Client{
		BaseURL:    "https://api.supp.co",
		Config:     &config.Config{AccessToken: "synthetic-token"},
		HTTPClient: &http.Client{},
	}
	if err := configureSuppCoClient(c); err != nil {
		t.Fatalf("configure canonical client: %v", err)
	}
	if !c.NoCache {
		t.Fatal("SuppCo client must disable the response cache")
	}

	first, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.supp.co/api/users/me_compact/", nil)
	cross, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://example.test/redirect", nil)
	if err := c.HTTPClient.CheckRedirect(cross, []*http.Request{first}); err == nil || !strings.Contains(err.Error(), "cross-origin") {
		t.Fatalf("cross-origin redirect error = %v", err)
	}

	same, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://api.supp.co/next", nil)
	if err := c.HTTPClient.CheckRedirect(same, []*http.Request{first}); err != nil {
		t.Fatalf("same-origin redirect rejected: %v", err)
	}
}

func TestDoctorDoesNotProbeOrAdvertiseRemovedSync(t *testing.T) {
	t.Setenv("SUPPCO_ACCESS_TOKEN", "fixture-token-value")
	root := RootCmd()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--home", t.TempDir(), "--json", "doctor"})
	if err := root.Execute(); err != nil {
		t.Fatalf("doctor error = %v", err)
	}
	text := out.String()
	if !strings.Contains(text, "configured (not probed; private read required") {
		t.Fatalf("doctor did not report local-only provider validation: %s", text)
	}
	if strings.Contains(text, "sync") || strings.Contains(text, "cache") || strings.Contains(text, "unreachable") {
		t.Fatalf("doctor retained inapplicable provider guidance: %s", text)
	}
}

func TestConfigureSuppCoClientRejectsNonCanonicalBaseURL(t *testing.T) {
	c := &client.Client{
		BaseURL:    (&url.URL{Scheme: "https", Host: "example.test"}).String(),
		HTTPClient: &http.Client{},
	}
	if err := configureSuppCoClient(c); err == nil || !strings.Contains(err.Error(), "canonical SuppCo API origin") {
		t.Fatalf("noncanonical base URL error = %v", err)
	}
}

func TestConfigureSuppCoClientRejectsDisabledTLSVerification(t *testing.T) {
	c := &client.Client{
		BaseURL:    "https://api.supp.co",
		Config:     &config.Config{SkipTLSVerify: true},
		HTTPClient: &http.Client{},
	}
	if err := configureSuppCoClient(c); err == nil || !strings.Contains(err.Error(), "TLS verification disabled") {
		t.Fatalf("insecure client error = %v", err)
	}
}

func TestVerifyEnvironmentCannotRedirectARealBearerToken(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
	c := &client.Client{
		BaseURL:    "http://127.0.0.1:43210",
		Config:     &config.Config{AccessToken: "synthetic-real-token"},
		HTTPClient: &http.Client{},
	}
	if err := configureSuppCoClient(c); err == nil || !strings.Contains(err.Error(), "canonical SuppCo API origin") {
		t.Fatalf("loopback with non-verifier credential error = %v", err)
	}
}

func TestPrintingPressMockCredentialMayUseLoopback(t *testing.T) {
	t.Setenv("PRINTING_PRESS_VERIFY", "1")
	t.Setenv("PRINTING_PRESS_VERIFY_LIVE_HTTP", "1")
	c := &client.Client{
		BaseURL:    "http://127.0.0.1:43210",
		Config:     &config.Config{AccessToken: "mock-token-for-testing"},
		HTTPClient: &http.Client{},
	}
	if err := configureSuppCoClient(c); err != nil {
		t.Fatalf("PrintingPress mock client rejected: %v", err)
	}
}
