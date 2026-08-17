package config

import (
	"path/filepath"
	"testing"
)

func TestResolvedAuthModeAutoDetectsCredentials(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want string
	}{
		{name: "none", cfg: Config{}, want: "none"},
		{name: "session", cfg: Config{SessionCookie: "session"}, want: "board-session"},
		{name: "board API key", cfg: Config{BoardAPIKey: "key"}, want: "board-api-key"},
		{name: "agent bearer", cfg: Config{AgentToken: "token"}, want: "agent-bearer"},
		{name: "explicit mode wins", cfg: Config{AuthMode: "agent-bearer", SessionCookie: "session", AgentToken: "token"}, want: "agent-bearer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.ResolvedAuthMode(); got != tc.want {
				t.Fatalf("ResolvedAuthMode() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestValidateAuthRequiresCredentialForExplicitMode(t *testing.T) {
	for _, mode := range []string{"board-session", "board-api-key", "agent-bearer"} {
		t.Run(mode, func(t *testing.T) {
			if err := (&Config{AuthMode: mode}).ValidateAuth(); err == nil {
				t.Fatalf("ValidateAuth() returned nil for %s without credentials", mode)
			}
		})
	}
	if err := (&Config{AuthMode: "unknown"}).ValidateAuth(); err == nil {
		t.Fatal("ValidateAuth() returned nil for an unknown mode")
	}
}

func TestLoadDefersAuthValidationUntilCallerOverridesAreApplied(t *testing.T) {
	t.Setenv("PAPERCLIP_AUTH_MODE", "board-api-key")
	cfg, err := Load(filepath.Join(t.TempDir(), "config.toml"))
	if err != nil {
		t.Fatalf("Load returned error before caller could apply a credential override: %v", err)
	}
	if err := cfg.ValidateAuth(); err == nil {
		t.Fatal("ValidateAuth returned nil before the credential override")
	}
	cfg.BoardAPIKey = "from-flag"
	if err := cfg.ValidateAuth(); err != nil {
		t.Fatalf("ValidateAuth returned error after credential override: %v", err)
	}
}
