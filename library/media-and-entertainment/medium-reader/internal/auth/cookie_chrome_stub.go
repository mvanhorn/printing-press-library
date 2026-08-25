// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

//go:build !kooky

package auth

import "github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"

// ExtractFromChrome (default build) is a clearly-messaged stub. Auto-extraction
// from Chrome requires the kooky dependency (and, on macOS, a Keychain
// authorization prompt + CGO), which would break the binary's pure-Go,
// cross-compilable, agent-safe posture. The shipped binary therefore points the
// user at the env / --cookie-file paths, which cover every real use case. A
// `-tags kooky` build replaces this with cookie_chrome_kooky.go's real reader.
func ExtractFromChrome() (source.Cookies, error) {
	return source.Cookies{}, ErrChromeExtractUnavailable
}
