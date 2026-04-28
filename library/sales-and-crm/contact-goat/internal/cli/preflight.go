// Copyright 2026 matt-van-horn. Licensed under Apache-2.0. See LICENSE.

// preflight.go: shared preflight helpers used by commands to short-circuit
// before doing expensive work when a required credential is missing.

package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/contact-goat/internal/deepline"
)

// requireDeeplineKey validates that a Deepline API key is available in the
// environment or on the deeplineFlags, without contacting the upstream API.
// It is intended to be called from PreRunE on commands that will definitely
// spend Deepline credits, so the user learns the key is missing before the
// command burns time on free-chain steps (waterfall) or user confirmation
// prompts.
//
// The check is cheap: it reads env + flag + validates the shape. It does
// NOT ping code.deepline.com (a ping itself would cost nothing but adds
// latency and a network dependency to the preflight).
//
// dlFlags may be nil; when nil, only DEEPLINE_API_KEY env is checked.
func requireDeeplineKey(dlFlags *deeplineFlags) error {
	flagValue := ""
	if dlFlags != nil {
		flagValue = dlFlags.apiKey
	}
	key, _ := resolveDeeplineKey(flagValue)
	if strings.TrimSpace(key) == "" {
		return authErr(fmt.Errorf("%w\nhint: export DEEPLINE_API_KEY=dlp_... (keys at https://code.deepline.com/dashboard/api-keys)\n      or pass --deepline-key dlp_... where the flag is available\n      or run 'deepline auth status --reveal' if you've already authenticated with the Deepline CLI",
			deepline.ErrMissingKey))
	}
	if !strings.HasPrefix(key, deepline.KeyPrefix) {
		return authErr(deepline.ErrInvalidKeyPrefix)
	}
	return nil
}

// hasAnyEnv returns true when any of the named env vars is set and non-empty.
// Used by waterfall's preflight bypass: if the user has BYOK keys configured
// for Hunter or Apollo, requiring a Deepline key up front is wrong since the
// --byok path may never call Deepline-managed endpoints.
func hasAnyEnv(names ...string) bool {
	for _, n := range names {
		if strings.TrimSpace(os.Getenv(n)) != "" {
			return true
		}
	}
	return false
}

// preflightWaterfallDeepline returns an error if the waterfall is about to
// run without any way to reach Deepline. The caller passes the BYOK config
// and the --byok/--require-byok flag so we can decide whether a Deepline key
// is strictly required.
//
// Rule:
//   - requireBYOK=true with BYOK keys configured -> ok (waterfall will error
//     separately if the BYOK providers can't serve the requested fields)
//   - otherwise, a valid DEEPLINE_API_KEY is required for any Deepline step
func preflightWaterfallDeepline(dlKey string, requireBYOK bool, byok map[string]string) error {
	if requireBYOK && len(byok) > 0 {
		return nil
	}
	if strings.TrimSpace(dlKey) == "" {
		return authErr(fmt.Errorf("%w\nhint: waterfall needs either DEEPLINE_API_KEY (see https://code.deepline.com/dashboard/api-keys)\n      or --byok with configured BYOK providers (`config byok set hunter HUNTER_API_KEY`)\n      or run 'deepline auth status --reveal' if you've already authenticated with the Deepline CLI",
			deepline.ErrMissingKey))
	}
	if !strings.HasPrefix(dlKey, deepline.KeyPrefix) {
		return authErr(deepline.ErrInvalidKeyPrefix)
	}
	return nil
}

// shouldPreflightDossier returns whether the dossier command's current flags
// will reach Deepline. Dossier only needs a Deepline key when email/phone
// enrichment is explicitly requested.
func shouldPreflightDossier(sections []string, enrichEmail bool) bool {
	if enrichEmail {
		return true
	}
	for _, s := range sections {
		if strings.EqualFold(s, "email") {
			return true
		}
	}
	return false
}

// silence the unused-import warning if preflight helpers are trimmed later.
var _ = errors.New
