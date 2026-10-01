// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package config

import (
	"os"
	"strings"
)

// Native sync-server credentials. These are separate from the generated
// Config (which describes the actual-http-api sidecar) because the native
// read path talks to the Actual server itself. Each value may be given
// directly or as a path in the matching _FILE variable (Docker secrets
// convention, same as @actual-app/cli).

const (
	EnvServerURL          = "ACTUAL_SERVER_URL"
	EnvPassword           = "ACTUAL_PASSWORD"
	EnvSessionToken       = "ACTUAL_SESSION_TOKEN"
	EnvSyncID             = "ACTUAL_SYNC_ID"
	EnvEncryptionPassword = "ACTUAL_ENCRYPTION_PASSWORD" // #nosec G101 -- env var name, not a credential
)

func envOrFile(name string) string {
	if v := strings.TrimSpace(os.Getenv(name)); v != "" {
		return v
	}
	if p := strings.TrimSpace(os.Getenv(name + "_FILE")); p != "" {
		// The operator names this secret file (Docker secrets convention).
		if data, err := os.ReadFile(p); err == nil { // #nosec G304 G703 -- operator-chosen secret file path, by design
			return strings.TrimRight(string(data), "\r\n")
		}
	}
	return ""
}

// ActualServerURL is the Actual sync server base URL (e.g. http://localhost:5006).
func ActualServerURL() string { return strings.TrimRight(envOrFile(EnvServerURL), "/") }

// ActualPassword is the Actual server password.
func ActualPassword() string { return envOrFile(EnvPassword) }

// ActualSessionToken is an optional pre-issued session token (skips login).
func ActualSessionToken() string { return envOrFile(EnvSessionToken) }

// ActualSyncID is the budget's Sync ID (Settings → Advanced → Sync ID).
func ActualSyncID() string { return envOrFile(EnvSyncID) }

// ActualEncryptionPassword is the end-to-end encryption password, if any.
func ActualEncryptionPassword() string { return envOrFile(EnvEncryptionPassword) }
