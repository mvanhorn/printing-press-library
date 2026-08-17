// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"github.com/mvanhorn/printing-press-library/library/ai/context-dev/internal/config"
	"strings"
	"testing"
)

func TestFirstArrayRequiresKnownEnvelopeKey(t *testing.T) {
	got := firstArray(map[string]any{
		"suggestions": []any{"try another query"},
	}, "results", "data", "items")
	if got != nil {
		t.Fatalf("firstArray returned fallback array %#v, want nil", got)
	}

	want := []any{"hit"}
	got = firstArray(map[string]any{
		"suggestions": []any{"try another query"},
		"results":     want,
	}, "results", "data", "items")
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("firstArray = %#v, want %#v", got, want)
	}
}

func TestHealthcareSpecificCommandsAreNotPublicSurface(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"doctor-discover", "clinic-enrich"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := runContextDevCommand(name, "--help")
			if err == nil {
				t.Fatalf("%s should not be registered in the public CLI", name)
			}
			if !strings.Contains(err.Error(), "unknown command") {
				t.Fatalf("error = %q, want unknown command", err.Error())
			}
		})
	}
}

func TestNormalizeDomainArgPreservesWWWDomain(t *testing.T) {
	t.Parallel()
	got, err := normalizeDomainArg("www.example.com")
	if err != nil {
		t.Fatal(err)
	}
	if got != "www.example.com" {
		t.Fatalf("normalizeDomainArg stripped www prefix: got %q", got)
	}
}

func TestCrawlEstimateAndConfirmGate(t *testing.T) {
	t.Setenv("CONTEXT_DEV_API_KEY", "test-key")
	out, err := runContextDevCommand("crawl", "https://example.com", "--max-pages", "30", "--estimate")
	if err != nil {
		t.Fatalf("estimate returned error: %v", err)
	}
	if !strings.Contains(out.String(), "estimated credits") {
		t.Fatalf("missing estimate output: %s", out.String())
	}
	_, err = runContextDevCommand("crawl", "https://example.com", "--max-pages", "30")
	if err == nil {
		t.Fatal("expected confirm gate error")
	}
	if ExitCode(err) != 2 {
		t.Fatalf("ExitCode = %d, want 2", ExitCode(err))
	}
}

func TestAuthEnvPrecedence(t *testing.T) {
	t.Setenv("CONTEXT_DEV_API_KEY", "primary")
	t.Setenv("CONTEXT_API_KEY", "fallback")
	cfg, err := config.Load("")
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.AuthHeader(); got != "Bearer primary" {
		t.Fatalf("AuthHeader = %q, want primary", got)
	}
}

func runContextDevCommand(args ...string) (*bytes.Buffer, error) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	out := &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out, err
}
