// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/deepline"
)

func TestRequireDeeplineKeyMissing(t *testing.T) {
	// Sandbox $HOME so the resolver's file-discovery step can't find the
	// real ~/.local/deepline/<host>/.env on the dev machine. Without this,
	// "no env/flag" doesn't actually mean "no key" — auto-discovery would
	// pick up the user's real key and the assertion would flip.
	withFakeHome(t, t.TempDir())
	err := requireDeeplineKey(&deeplineFlags{})
	if err == nil {
		t.Fatal("requireDeeplineKey with no env/flag should error")
	}
	if !errors.Is(err, deepline.ErrMissingKey) {
		t.Errorf("err = %v, want ErrMissingKey wrapped inside", err)
	}
}

func TestRequireDeeplineKeyMalformed(t *testing.T) {
	withFakeHome(t, t.TempDir())
	t.Setenv("DEEPLINE_API_KEY", "foo_not_dlp")
	err := requireDeeplineKey(&deeplineFlags{})
	if err == nil {
		t.Fatal("requireDeeplineKey with malformed key should error")
	}
	if !errors.Is(err, deepline.ErrInvalidKeyPrefix) {
		t.Errorf("err = %v, want ErrInvalidKeyPrefix wrapped inside", err)
	}
}

func TestRequireDeeplineKeyFromFlag(t *testing.T) {
	withFakeHome(t, t.TempDir())
	if err := requireDeeplineKey(&deeplineFlags{apiKey: "dlp_abc"}); err != nil {
		t.Errorf("flag key should satisfy preflight: %v", err)
	}
}

func TestRequireDeeplineKeyFromEnv(t *testing.T) {
	t.Setenv("DEEPLINE_API_KEY", "dlp_env_value")
	if err := requireDeeplineKey(&deeplineFlags{}); err != nil {
		t.Errorf("env key should satisfy preflight: %v", err)
	}
}

func TestPreflightWaterfallDeeplineRequireBYOKSatisfied(t *testing.T) {
	err := preflightWaterfallDeepline("", true, map[string]string{"hunter": "HUNTER_API_KEY"})
	if err != nil {
		t.Errorf("--byok + BYOK configured should skip Deepline preflight: %v", err)
	}
}

func TestPreflightWaterfallDeeplineRequiresKeyWithoutBYOK(t *testing.T) {
	err := preflightWaterfallDeepline("", false, nil)
	if err == nil {
		t.Fatal("no key + no BYOK should fail")
	}
	if !errors.Is(err, deepline.ErrMissingKey) {
		t.Errorf("err = %v, want ErrMissingKey wrapped inside", err)
	}
}

func TestPreflightWaterfallDeeplineValidKey(t *testing.T) {
	if err := preflightWaterfallDeepline("dlp_abc", false, nil); err != nil {
		t.Errorf("valid key should pass: %v", err)
	}
}

func TestShouldPreflightDossier(t *testing.T) {
	cases := []struct {
		name         string
		sections     []string
		enrichEmail  bool
		wantPreflight bool
	}{
		{"no-email-no-enrich", []string{"profile", "research"}, false, false},
		{"email-section-no-enrich", []string{"profile", "research", "email"}, false, true},
		{"enrich-email-only", []string{"profile", "research"}, true, true},
		{"both", []string{"profile", "research", "email"}, true, true},
	}
	for _, c := range cases {
		got := shouldPreflightDossier(c.sections, c.enrichEmail)
		if got != c.wantPreflight {
			t.Errorf("shouldPreflightDossier(%v, %v) = %v, want %v",
				c.sections, c.enrichEmail, got, c.wantPreflight)
		}
	}
}

func TestNormalizePersonInputHandlesURLVariants(t *testing.T) {
	// This helper is the one Unit 3 relies on for the linkedin_url ->
	// linkedin_username fix. Lock the behavior in a test.
	cases := map[string]string{
		"williamhgates":                               "williamhgates",
		"https://www.linkedin.com/in/williamhgates":   "williamhgates",
		"https://www.linkedin.com/in/williamhgates/":  "williamhgates",
		"http://linkedin.com/in/alonsovelasco":        "alonsovelasco",
		"/in/alonsovelasco/":                          "alonsovelasco",
		"https://www.linkedin.com/in/mkscrg":          "mkscrg",
	}
	for in, want := range cases {
		if got := normalizePersonInput(in); got != want {
			t.Errorf("normalizePersonInput(%q) = %q, want %q", in, got, want)
		}
	}
}
