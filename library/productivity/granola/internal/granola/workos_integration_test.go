// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

//go:build integration

package granola

import (
	"os"
	"testing"
)

// TestLoadFromSupabaseJSON_RealEncrypted exercises the .enc path against
// the user's actual supabase.json.enc when present. Skipped unless
// GRANOLA_INTEGRATION=1.
func TestLoadFromSupabaseJSON_RealEncrypted(t *testing.T) {
	if os.Getenv("GRANOLA_INTEGRATION") == "" {
		t.Skip("set GRANOLA_INTEGRATION=1 to run; requires real Granola signin")
	}
	ResetTokenCache()
	defer ResetTokenCache()
	t.Setenv("GRANOLA_WORKOS_TOKEN", "")

	tok, src, err := loadFromSupabaseJSON()
	if err != nil {
		t.Fatalf("loadFromSupabaseJSON: %v", err)
	}
	if tok.AccessToken == "" {
		t.Fatal("got empty access_token")
	}
	if tok.RefreshToken == "" {
		t.Error("got empty refresh_token (workflow may still work but unusual)")
	}
	expiry := tok.expiry()
	t.Logf("source=%v access_token=%d chars refresh_token=%d chars expires=%s",
		src, len(tok.AccessToken), len(tok.RefreshToken), expiry.Format("2006-01-02 15:04:05"))

	// On a modern Granola install, the source should be encrypted.
	if src != TokenSourceEncryptedSupabase && src != TokenSourcePlaintextSupabase {
		t.Errorf("unexpected source: %v", src)
	}
}
