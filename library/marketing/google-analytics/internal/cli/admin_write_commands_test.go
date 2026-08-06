package cli

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// requireExplicitYes is the security-critical gate on destructive writes. It must
// inspect the flag's Changed state, not rootFlags.yes, so that --agent (which
// implies --yes) can never authorize a mutation on its own.
func TestRequireExplicitYes(t *testing.T) {
	cases := []struct {
		name        string
		extra       []string
		wantDestrue bool // expect the destructive-and-irreversible error
	}{
		{"explicit --yes authorizes", []string{"--yes"}, false},
		{"--agent alone is not enough", []string{"--agent"}, true},
		{"neither flag", []string{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear every credential source so the --yes case can never mint a
			// token and issue a live DELETE from `go test`. It must fail at
			// credential resolution, well before any network call.
			t.Setenv("GOOGLE_ANALYTICS_ADC", "")
			t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
			var f rootFlags
			root := newRootCmd(&f)
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			args := append([]string{"data-streams", "delete", "--name", "properties/1/dataStreams/2"}, tc.extra...)
			root.SetArgs(args)
			err := root.Execute()
			if tc.wantDestrue {
				if err == nil || !strings.Contains(err.Error(), "destructive") {
					t.Fatalf("expected destructive gate error, got %v", err)
				}
				return
			}
			// Explicit --yes must sail past requireExplicitYes; any later failure is
			// downstream (e.g. missing credentials), never the destructive gate.
			if err != nil && strings.Contains(err.Error(), "destructive") {
				t.Fatalf("unexpected destructive gate error despite --yes: %v", err)
			}
		})
	}
}

func TestParseBody(t *testing.T) {
	// Inline JSON parses.
	v, err := parseBody(`{"a":1,"b":"two"}`)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := v.(map[string]any)
	if !ok || m["a"] != float64(1) || m["b"] != "two" {
		t.Fatalf("inline parse produced %#v", v)
	}

	// @file reads from disk.
	dir := t.TempDir()
	path := filepath.Join(dir, "body.json")
	if err := os.WriteFile(path, []byte(`{"x":[1,2]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	v, err = parseBody("@" + path)
	if err != nil {
		t.Fatal(err)
	}
	m, ok = v.(map[string]any)
	if !ok || m["x"] == nil {
		t.Fatalf("@file parse produced %#v", v)
	}

	// Invalid JSON mentions --body.
	if _, err := parseBody(`{not json`); err == nil || !strings.Contains(err.Error(), "--body") {
		t.Fatalf("expected invalid-json error mentioning --body, got %v", err)
	}
}

func TestCredentialPathResolutionOrder(t *testing.T) {
	t.Setenv("GOOGLE_ANALYTICS_ADC", "/adc.json")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "/app.json")

	// --credentials beats GOOGLE_ANALYTICS_ADC beats GOOGLE_APPLICATION_CREDENTIALS.
	if got := credentialPath(&rootFlags{credentials: "/flag.json"}); got != "/flag.json" {
		t.Fatalf("--credentials should win, got %q", got)
	}
	if got := credentialPath(&rootFlags{}); got != "/adc.json" {
		t.Fatalf("GOOGLE_ANALYTICS_ADC should beat GOOGLE_APPLICATION_CREDENTIALS, got %q", got)
	}

	// GAC-only fallback.
	t.Setenv("GOOGLE_ANALYTICS_ADC", "")
	if got := credentialPath(&rootFlags{}); got != "/app.json" {
		t.Fatalf("GOOGLE_APPLICATION_CREDENTIALS fallback failed, got %q", got)
	}

	// ~/ is expanded against the home dir.
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "~/keys/ga.json")
	got := credentialPath(&rootFlags{})
	if !strings.HasPrefix(got, string(os.PathSeparator)) || !strings.HasSuffix(got, "keys/ga.json") || strings.HasPrefix(got, "~/") {
		t.Fatalf("~ expansion produced %q", got)
	}

	// Nothing set -> empty.
	t.Setenv("GOOGLE_ANALYTICS_ADC", "")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	if got := credentialPath(&rootFlags{}); got != "" {
		t.Fatalf("expected empty credential path, got %q", got)
	}
}
