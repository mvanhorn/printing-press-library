// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build kooky

package auth

import (
	"context"
	"fmt"

	"github.com/browserutils/kooky"
	_ "github.com/browserutils/kooky/browser/chrome" // register the Chrome cookie-store finder

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
)

// ExtractFromChrome (kooky build) reads the Medium session cookie directly from
// the local Chrome profile via kooky, then selects the medium.com
// sid/uid/cf_clearance through the shared, hermetically tested
// selectMediumCookies seam.
//
// This path is deliberately behind the `kooky` build tag and is NOT in the
// shipped binary: on macOS, Chrome encrypts its cookie store with a key held in
// the login Keychain, so the first read triggers a Keychain authorization prompt
// (and grants a per-binary ACL). That prompt cannot be satisfied unattended, and
// kooky pulls CGO/keychain dependencies that would break the default binary's
// pure-Go, cross-compilable posture. Building with `-tags kooky` is the explicit,
// power-user opt-in.
func ExtractFromChrome() (source.Cookies, error) {
	ctx := context.Background()
	// Only the Chrome finder is registered (blank import above), so this reads
	// Chrome stores exclusively. Valid drops expired cookies; the domain filter
	// limits the read (and the Keychain decryption) to medium.com.
	kc, err := kooky.ReadCookies(ctx, kooky.Valid, kooky.DomainHasSuffix("medium.com"))
	if err != nil {
		return source.Cookies{}, fmt.Errorf("reading Chrome cookies: %w", err)
	}
	browserCookies := make([]BrowserCookie, 0, len(kc))
	for _, c := range kc {
		if c == nil {
			continue
		}
		browserCookies = append(browserCookies, BrowserCookie{
			Name:   c.Name,
			Value:  c.Value,
			Domain: c.Domain,
		})
	}
	return selectMediumCookies(browserCookies)
}
