// Copyright 2026 laci141 and contributors. Licensed under Apache-2.0. See LICENSE.

// sources.go wires the hand-authored secondary-source clients (internal/source)
// into the CLI: one shared rate-limited transport plus the optional-key/env
// lookups. Keeping this here means the novel commands stay focused on logic.
package cli

import (
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/health/clinical-trials/internal/source"
)

// sourceClient builds the shared secondary-source transport, inheriting the
// root --timeout and --rate-limit so `boundCtx` and global pacing apply.
func (f *rootFlags) sourceClient() *source.Client {
	to := time.Duration(0)
	if f != nil {
		to = f.timeout
	}
	if to <= 0 {
		to = 30 * time.Second
	}
	rate := 0.0
	if f != nil {
		rate = f.rateLimit
	}
	return source.New(to, rate)
}

func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

// openFDAKey returns an optional openFDA key (higher rate limits only).
func openFDAKey() string { return firstEnv("OPENFDA_API_KEY", "CLINICAL_TRIALS_OPENFDA_API_KEY") }

// ncbiKey returns an optional NCBI E-utilities key (higher rate limits only).
func ncbiKey() string {
	return firstEnv("NCBI_API_KEY", "PUBMED_API_KEY", "CLINICAL_TRIALS_NCBI_API_KEY")
}

// openAlexMailto returns an optional contact email for OpenAlex's polite pool.
func openAlexMailto() string { return firstEnv("OPENALEX_MAILTO", "CLINICAL_TRIALS_CONTACT_EMAIL") }
