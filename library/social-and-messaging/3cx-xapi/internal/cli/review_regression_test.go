package cli

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHazardousEndpointClassification(t *testing.T) {
	for _, endpoint := range []string{
		"purge-all-logs.purge_all_logs",
		"services.restart-operating-system",
		"users.regenerate-passwords",
		"voicemail-settings.delete-all-user-voicemails",
	} {
		if !hazardousEndpoint(endpoint) {
			t.Fatalf("%s must require --yes", endpoint)
		}
	}
	if hazardousEndpoint("users.list") {
		t.Fatal("read endpoint was classified as hazardous")
	}
}

func TestAgentModeDoesNotImplicitlyConfirmHazardousEndpoint(t *testing.T) {
	cmd := RootCmd()
	cmd.SetArgs([]string{"services", "restart-operating-system", "--agent"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "explicit --yes") {
		t.Fatalf("agent mode must require explicit confirmation, got %v", err)
	}
}

func TestSearchRejectsLiveAndInvalidLimit(t *testing.T) {
	for _, tc := range []struct {
		flags rootFlags
		args  []string
		want  string
	}{
		{flags: rootFlags{dataSource: "live"}, args: []string{"query"}, want: "no read-only live endpoint"},
		{flags: rootFlags{dataSource: "auto"}, args: []string{"query", "--limit=-1"}, want: "greater than zero"},
	} {
		cmd := newSearchCmd(&tc.flags)
		cmd.SetArgs(tc.args)
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("args %v error = %v, want %q", tc.args, err, tc.want)
		}
	}
}

func TestImportFailureResult(t *testing.T) {
	if err := importFailureResult(2, false); err == nil || ExitCode(err) == 0 {
		t.Fatalf("failed imports must return non-zero: %v", err)
	}
	if err := importFailureResult(2, true); err != nil {
		t.Fatalf("allow-partial-failure returned %v", err)
	}
}

func TestInitialTailResult(t *testing.T) {
	fetchErr := errors.New("fetch failed")
	if !errors.Is(initialTailResult(false, fetchErr), fetchErr) {
		t.Fatal("single-poll mode swallowed the fetch error")
	}
	if err := initialTailResult(true, fetchErr); err != nil {
		t.Fatalf("follow mode should continue after initial failure: %v", err)
	}
}

func TestMintClientCredentialsTokenHonorsHTTPTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	started := time.Now()
	_, err := mintClientCredentialsToken(&http.Client{Timeout: 25 * time.Millisecond}, server.URL, "client", "secret")
	if err == nil {
		t.Fatal("token request unexpectedly succeeded")
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("token request ignored configured timeout: %s", elapsed)
	}
}
