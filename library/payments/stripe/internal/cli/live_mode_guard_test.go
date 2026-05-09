// Copyright 2026 chris-rodriguez. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCheckLiveModeGuard(t *testing.T) {
	cases := []struct {
		name        string
		annotations map[string]string
		envKey      string
		envConfirm  string
		flagConfirm bool
		wantBlock   bool
	}{
		{
			name:        "framework command no annotation passes",
			annotations: nil,
			envKey:      "sk_live_REAL",
			wantBlock:   false,
		},
		{
			name:        "GET with live key passes",
			annotations: map[string]string{"pp:method": "GET"},
			envKey:      "sk_live_REAL",
			wantBlock:   false,
		},
		{
			name:        "POST with test key passes",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_test_FINE",
			wantBlock:   false,
		},
		{
			name:        "POST with live key and no confirmation blocks",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "DELETE with live key and no confirmation blocks",
			annotations: map[string]string{"pp:method": "DELETE"},
			envKey:      "sk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "PATCH with live key and no confirmation blocks",
			annotations: map[string]string{"pp:method": "PATCH"},
			envKey:      "sk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "POST with live key but --confirm-live passes",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_live_REAL",
			flagConfirm: true,
			wantBlock:   false,
		},
		{
			name:        "POST with live key but STRIPE_CONFIRM_LIVE=1 passes",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "sk_live_REAL",
			envConfirm:  "1",
			wantBlock:   false,
		},
		{
			name:        "rk_live_ restricted-key prefix also triggers guard",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "rk_live_REAL",
			wantBlock:   true,
		},
		{
			name:        "no key set passes (no live-mode signal)",
			annotations: map[string]string{"pp:method": "POST"},
			envKey:      "",
			wantBlock:   false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("STRIPE_SECRET_KEY", tc.envKey)
			t.Setenv("STRIPE_CONFIRM_LIVE", tc.envConfirm)
			// Force the config probe to a non-existent file so config-stored
			// keys don't bleed in from the host's real ~/.config tree.
			t.Setenv("STRIPE_CONFIG", filepath.Join(t.TempDir(), "missing.toml"))

			cmd := &cobra.Command{Use: "test"}
			if tc.annotations != nil {
				cmd.Annotations = tc.annotations
			}
			flags := &rootFlags{confirmLive: tc.flagConfirm}

			err := checkLiveModeGuard(cmd, flags)
			gotBlock := err != nil
			if gotBlock != tc.wantBlock {
				t.Errorf("checkLiveModeGuard: wantBlock=%v gotBlock=%v err=%v",
					tc.wantBlock, gotBlock, err)
			}
		})
	}
}

func TestCheckLiveModeGuard_PersistedConfig(t *testing.T) {
	// Build a TOML config file with a live-mode access_token (the field that
	// `auth set-token` writes to). Guard MUST detect the live key from the
	// persisted config even though no env var is set.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := `base_url = "https://api.stripe.com"` + "\n" +
		`access_token = "sk_live_PERSISTED_KEY"` + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_BASIC_AUTH", "")
	t.Setenv("STRIPE_CONFIRM_LIVE", "")
	t.Setenv("STRIPE_CONFIG", cfgPath)

	cmd := &cobra.Command{
		Use:         "test",
		Annotations: map[string]string{"pp:method": "POST"},
	}
	flags := &rootFlags{}

	err := checkLiveModeGuard(cmd, flags)
	if err == nil {
		t.Fatalf("expected guard to block POST against config-stored live key, got nil")
	}
}

func TestCheckLiveModeGuard_PersistedConfig_TestKeyPasses(t *testing.T) {
	// A config-stored test-mode key must NOT trip the guard.
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.toml")
	cfg := `access_token = "sk_test_TESTONLY"` + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("STRIPE_SECRET_KEY", "")
	t.Setenv("STRIPE_BASIC_AUTH", "")
	t.Setenv("STRIPE_CONFIRM_LIVE", "")
	t.Setenv("STRIPE_CONFIG", cfgPath)

	cmd := &cobra.Command{
		Use:         "test",
		Annotations: map[string]string{"pp:method": "POST"},
	}
	flags := &rootFlags{}

	if err := checkLiveModeGuard(cmd, flags); err != nil {
		t.Fatalf("expected guard to pass for config-stored test key, got: %v", err)
	}
}

func TestHasLivePrefix(t *testing.T) {
	cases := map[string]bool{
		"":                      false,
		"sk_test_xyz":           false,
		"sk_live_xyz":           true,
		"rk_live_xyz":           true,
		"pk_live_xyz":           false, // publishable, not a write credential
		"Bearer sk_live_xyz":    true,
		"Bearer sk_test_xyz":    false,
		"Basic sk_live_xyz":     true,
		"  sk_live_xyz  ":       true, // whitespace tolerance
		"sk_live":               false, // no underscore-suffix
	}
	for input, want := range cases {
		got := hasLivePrefix(input)
		if got != want {
			t.Errorf("hasLivePrefix(%q) = %v, want %v", input, got, want)
		}
	}
}
