// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0.

package granola

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// isolateTokenEnv points every token probe at an empty temp directory so a real
// Granola install on the developer's machine cannot leak into these tests.
func isolateTokenEnv(t *testing.T) string {
	t.Helper()
	support := t.TempDir()
	t.Setenv("GRANOLA_SUPPORT_DIR", support)
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")
	t.Setenv("GRANOLA_WORKOS_REFRESH", "")
	dir := filepath.Join(t.TempDir(), "granola-pp-cli")
	t.Setenv("GRANOLA_CLI_SESSION_PATH", filepath.Join(dir, "session.json"))
	ResetTokenCache()
	t.Cleanup(ResetTokenCache)
	return support
}

func storeCLISession(t *testing.T) CLISession {
	t.Helper()
	s := CLISession{
		AccessToken:  "cli-access",
		RefreshToken: "cli-refresh",
		ExpiresAt:    time.Now().Add(time.Hour).UTC(),
		ObtainedAt:   time.Now().UTC(),
		AccountEmail: "cli@example.com",
	}
	if err := SaveCLISession(s); err != nil {
		t.Fatalf("SaveCLISession: %v", err)
	}
	return s
}

func TestCLISessionOutranksDesktopProbes(t *testing.T) {
	support := isolateTokenEnv(t)
	// A desktop-owned plaintext token that would otherwise be selected.
	writeFixtureSupabase(t, support)
	storeCLISession(t)

	tok, src, err := loadTokensRaw()
	if err != nil {
		t.Fatalf("loadTokensRaw: %v", err)
	}
	if src != TokenSourceCLISession {
		t.Fatalf("want TokenSourceCLISession, got %v", src)
	}
	if tok.AccessToken != "cli-access" {
		t.Errorf("wrong token selected: %q", tok.AccessToken)
	}
}

func TestEnvOverrideStillOutranksCLISession(t *testing.T) {
	isolateTokenEnv(t)
	storeCLISession(t)
	t.Setenv("GRANOLA_WORKOS_TOKEN", "env-token")

	tok, src, err := loadTokensRaw()
	if err != nil {
		t.Fatalf("loadTokensRaw: %v", err)
	}
	if src != TokenSourceEnvOverride {
		t.Fatalf("a deliberate env override must still win, got %v", src)
	}
	if tok.AccessToken != "env-token" {
		t.Errorf("wrong token: %q", tok.AccessToken)
	}
}

func TestNoCLISessionFallsThroughToDesktopProbes(t *testing.T) {
	support := isolateTokenEnv(t)
	writeFixtureSupabase(t, support)

	_, src, err := loadTokensRaw()
	if err != nil {
		t.Fatalf("loadTokensRaw: %v", err)
	}
	if src == TokenSourceCLISession {
		t.Fatal("selected a CLI session that does not exist")
	}
	if src == TokenSourceUnknown {
		t.Fatal("fell through past the desktop probes entirely")
	}
}

// An unusable session must surface, not silently degrade into "no token found"
// three probes later.
func TestUnusableCLISessionSurfaces(t *testing.T) {
	support := isolateTokenEnv(t)
	writeFixtureSupabase(t, support)
	storeCLISession(t)
	if err := os.Chmod(CLISessionPath(), 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	_, _, err := loadTokensRaw()
	if !errors.Is(err, ErrSessionPermissions) {
		t.Fatalf("want ErrSessionPermissions to surface, got %v", err)
	}
}

// D6 is the reason the desktop arms exist: rotating a chain another client
// holds signs that client out. The CLI-owned chain is the one exception.
func TestRefreshRefusalMatrix(t *testing.T) {
	support := isolateTokenEnv(t)
	// Presence of supabase.json.enc is what makes stored-accounts desktop-owned.
	if err := os.WriteFile(filepath.Join(support, "supabase.json.enc"), []byte("x"), 0o600); err != nil {
		t.Fatalf("write enc: %v", err)
	}

	for _, tc := range []struct {
		src      TokenSource
		refused  bool
		whyItsSo string
	}{
		{TokenSourceCLISession, false, "the CLI owns this chain outright"},
		{TokenSourceEncryptedSupabase, true, "desktop owns supabase.json.enc"},
		{TokenSourcePlaintextSupabaseDesktopFallback, true, "may share the desktop's single-use token"},
		{TokenSourceStoredAccounts, true, "desktop-owned while supabase.json.enc is present"},
		{TokenSourceEnvOverride, false, "user opted in to managing refresh"},
	} {
		err := refreshRefusalFor(tc.src)
		if tc.refused && err == nil {
			t.Errorf("source %v: expected refusal (%s)", tc.src, tc.whyItsSo)
		}
		if !tc.refused && err != nil {
			t.Errorf("source %v: unexpected refusal %v (%s)", tc.src, err, tc.whyItsSo)
		}
		if tc.refused && !errors.Is(err, ErrRefreshRefused) {
			t.Errorf("source %v: refusal must match ErrRefreshRefused, got %v", tc.src, err)
		}
	}
}

func TestPersistRotatedCLISessionKeepsIdentity(t *testing.T) {
	isolateTokenEnv(t)
	orig := storeCLISession(t)

	err := persistRotatedCLISession(RefreshAccessTokenResponse{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresIn:    3600,
	}, RotationHandle{epoch: RevocationEpoch()})
	if err != nil {
		t.Fatalf("persistRotatedCLISession: %v", err)
	}
	got, err := LoadCLISession()
	if err != nil {
		t.Fatalf("LoadCLISession: %v", err)
	}
	if got.AccessToken != "new-access" || got.RefreshToken != "new-refresh" {
		t.Error("rotated pair did not reach disk")
	}
	if got.AccountEmail != orig.AccountEmail {
		t.Errorf("account identity lost on rotation: %q", got.AccountEmail)
	}
	if !got.ExpiresAt.After(time.Now()) {
		t.Error("expiry not advanced")
	}
}

// A refresh response that omits the refresh token must not blank the stored
// one. Granola's proxy returns the same token rather than a rotated one, so
// treating "absent" as "clear it" would destroy a working session.
func TestPersistRotatedKeepsExistingRefreshWhenOmitted(t *testing.T) {
	isolateTokenEnv(t)
	storeCLISession(t)

	if err := persistRotatedCLISession(RefreshAccessTokenResponse{
		AccessToken: "new-access-only",
		ExpiresIn:   3600,
	}, RotationHandle{epoch: RevocationEpoch()}); err != nil {
		t.Fatalf("persistRotatedCLISession: %v", err)
	}
	got, _ := LoadCLISession()
	if got.RefreshToken != "cli-refresh" {
		t.Errorf("refresh token was clobbered by a response that omitted it: %q", got.RefreshToken)
	}
}

// writeFixtureSupabase drops a plaintext supabase.json that the desktop probes
// will accept, so fall-through order can be observed.
func writeFixtureSupabase(t *testing.T, support string) {
	t.Helper()
	blob := `{"workos_tokens":"{\"access_token\":\"desktop-access\",\"refresh_token\":\"desktop-refresh\",\"expires_in\":3600,\"obtained_at\":1785000000000,\"token_type\":\"Bearer\"}"}`
	if err := os.WriteFile(filepath.Join(support, "supabase.json"), []byte(blob), 0o600); err != nil {
		t.Fatalf("write supabase.json: %v", err)
	}
}
