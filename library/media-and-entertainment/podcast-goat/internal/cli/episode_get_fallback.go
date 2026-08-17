// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Cookie-missing fallback hint: when a platform URL dead-ends on a cookie the
// user never captured, point at the free path for the same episode instead of
// stopping at "go log in". Cost depends on the URL form the user passes, not
// on the episode — the same content is frequently free via its YouTube upload.

package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// spotifyOEmbedTimeout bounds the error-path lookup. The hint is a courtesy;
// it must never make a failing command hang.
const spotifyOEmbedTimeout = 3 * time.Second

// Injection points for tests. The oEmbed endpoint is keyless and public, but
// its response is still network-controlled input — see shellSingleQuote.
var (
	spotifyOEmbedBase   = "https://open.spotify.com/oembed"
	spotifyOEmbedClient = &http.Client{Timeout: spotifyOEmbedTimeout}
)

// alternateSourceHint returns a ready-to-run suggestion for fetching the same
// episode through a free source, or "" when no suggestion applies. Currently
// implemented for open.spotify.com episode URLs: the keyless Spotify oEmbed
// endpoint yields the episode title, which the YouTube adapter can resolve
// via a ytsearch1: pseudo-URL.
func alternateSourceHint(ctx context.Context, episodeURL string) string {
	if !strings.Contains(episodeURL, "open.spotify.com/") {
		return ""
	}
	title := spotifyOEmbedTitle(ctx, episodeURL)
	if title == "" {
		return "hint: the same episode is often free via its YouTube upload — episode get accepts any URL form, and cost depends on the URL you pass, not the episode. Try: podcast-goat-pp-cli episode get 'ytsearch1:<episode title>'"
	}
	// The title came off the network. Never interpolate it into a copyable
	// command with double quotes: $(...) and backticks execute inside them.
	// POSIX single quotes disable all expansion.
	return "hint: the same episode is often free via its YouTube upload — try: podcast-goat-pp-cli episode get " + shellSingleQuote("ytsearch1:"+title)
}

// shellSingleQuote wraps s in POSIX single quotes, escaping embedded single
// quotes with the '\” idiom. Inside single quotes the shell performs no
// expansion at all, so network-sourced text cannot smuggle $(...), backticks,
// or variable references into a command the user copies.
func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// spotifyOEmbedTitle resolves an episode title via Spotify's public oEmbed
// endpoint (no auth, no cookie). Returns "" on any failure.
func spotifyOEmbedTitle(ctx context.Context, episodeURL string) string {
	ctx, cancel := context.WithTimeout(ctx, spotifyOEmbedTimeout)
	defer cancel()
	u := spotifyOEmbedBase + "?url=" + url.QueryEscape(episodeURL)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return ""
	}
	resp, err := spotifyOEmbedClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	var body struct {
		Title string `json:"title"`
	}
	// Cap the read: this is untrusted network input on an error path.
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&body); err != nil {
		return ""
	}
	return strings.TrimSpace(body.Title)
}
