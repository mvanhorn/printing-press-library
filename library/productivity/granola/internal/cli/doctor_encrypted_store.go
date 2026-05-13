// Copyright 2026 Granola Printing Press contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/granola/internal/granola"
)

// PATCH(encrypted-cache): doctor section that observes the encrypted-store
// state without itself invoking safestorage.Decrypt. The four states it
// distinguishes mirror plan U5:
//
//   - INFO  no Granola install detected            (support dir missing)
//   - INFO  not in use (Granola pre-encryption)    (support dir but no .enc)
//   - INFO  present; run `sync` to authorize ...   (.enc present, no sync run yet)
//   - OK    ok (last decrypted: <relative>)         (sync state recorded ok)
//   - ERROR last sync failed to decrypt (<class>)  (sync state recorded failure)
//
// Pure observation - reads files in the support dir and the sync state
// file, never decrypts. The user runs `doctor` to diagnose, not to
// authenticate.

func collectEncryptedStoreReport(report map[string]any) {
	supportDir := granolaSupportDirFromEnv()
	if _, err := os.Stat(supportDir); os.IsNotExist(err) {
		report["encrypted_store"] = "INFO no Granola install detected"
		report["encrypted_store_hint"] = "Sign in to Granola desktop to enable cache access."
		return
	}

	encPath := filepath.Join(supportDir, "cache-v6.json.enc")
	supabaseEncPath := filepath.Join(supportDir, "supabase.json.enc")
	cacheEncPresent := fileExists(encPath)
	supabaseEncPresent := fileExists(supabaseEncPath)

	if !cacheEncPresent && !supabaseEncPresent {
		report["encrypted_store"] = "INFO not in use (Granola pre-encryption)"
		return
	}

	// Both .enc paths exist (or at least one). Consult sync state.
	state, err := granola.ReadSyncState()
	if granola.IsSyncStateMissing(err) {
		report["encrypted_store"] = "INFO present; run `granola-pp-cli sync` to authorize Keychain access"
		report["encrypted_store_hint"] = "First sync triggers the macOS Keychain prompt. Click Always Allow."
		return
	}
	if err != nil {
		report["encrypted_store"] = fmt.Sprintf("INFO sync state read error: %v", err)
		return
	}

	switch state.LastDecryptStatus {
	case granola.DecryptStatusOK:
		report["encrypted_store"] = "OK ok"
		if !state.LastSyncAt.IsZero() {
			report["encrypted_store_last_sync"] = state.LastSyncAt.Format(time.RFC3339)
			report["encrypted_store_last_sync_relative"] = relativeTime(state.LastSyncAt)
		}
		if state.LastTokenSource != "" {
			report["encrypted_store_token_source"] = state.LastTokenSource
		}
		if state.LastDocumentsFetched > 0 {
			report["encrypted_store_documents_fetched"] = state.LastDocumentsFetched
		}
		if state.LastHydrateErrorMsg != "" {
			report["encrypted_store_hydrate_error"] = state.LastHydrateErrorMsg
			report["encrypted_store_hint"] = "Decrypt succeeded; document hydration from /v2/get-documents failed (auth or network). Cached transcripts/folders/recipes are still usable; meetings list may be stale."
		}
	case granola.DecryptStatusFailed:
		msg := "ERROR last sync failed to decrypt"
		if state.LastDecryptErrorClass != "" {
			msg = fmt.Sprintf("ERROR last sync failed to decrypt (%s)", state.LastDecryptErrorClass)
		}
		report["encrypted_store"] = msg
		if state.LastDecryptErrorMsg != "" {
			report["encrypted_store_error"] = state.LastDecryptErrorMsg
		}
		switch state.LastDecryptErrorClass {
		case "key_unavailable":
			report["encrypted_store_hint"] = "Sign into Granola desktop and re-run sync. Click Always Allow on the Keychain prompt."
		case "decrypt_failed":
			report["encrypted_store_hint"] = "Encryption scheme may have drifted. File an issue at the Printing Press repo with this doctor output."
		case "unsupported_platform":
			report["encrypted_store_hint"] = "Linux and Windows decryption are deferred to follow-up work. macOS only this round."
		}
	default:
		report["encrypted_store"] = fmt.Sprintf("INFO sync state status: %q", state.LastDecryptStatus)
	}
}

// granolaSupportDirFromEnv mirrors the resolver in internal/granola/
// without an import cycle. Keeps GRANOLA_SUPPORT_DIR honored.
func granolaSupportDirFromEnv() string {
	if v := os.Getenv("GRANOLA_SUPPORT_DIR"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "Granola")
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func relativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d/time.Minute))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d hr ago", int(d/time.Hour))
	default:
		return fmt.Sprintf("%d days ago", int(d/(24*time.Hour)))
	}
}
