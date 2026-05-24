// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Helpers used by the 7 deprecated-endpoint stubs (audio-features, audio-
// analysis, recommendations, artists/related-artists, browse/featured-playlists,
// browse/categories/{id}/playlists). Spotify removed access to these endpoints
// for apps created after 2024-11-27; new-app credentials receive 403/404 on
// every request. The stub prints a structured deprecation payload by default
// and only attempts the real call when --legacy-app is set.
//
// Reference: https://developer.spotify.com/blog/2024-11-27-changes-to-the-web-api

package cli

const (
	deprecationBlogURL   = "https://developer.spotify.com/blog/2024-11-27-changes-to-the-web-api"
	stubStatusDeprecated = "stub_deprecated"
)

// deprecatedStubPayload returns the JSON body emitted by every deprecated-
// endpoint command when --legacy-app is NOT set. endpoint is the human label
// (e.g. "GET /audio-features/{id}") and nextAction is per-command guidance.
func deprecatedStubPayload(endpoint, nextAction string) map[string]any {
	return map[string]any{
		"status":           stubStatusDeprecated,
		"reason":           endpoint + " — Spotify removed access to this endpoint for apps created after 2024-11-27. New-app credentials receive 403/404.",
		"next_action":      nextAction,
		"spotify_blog_url": deprecationBlogURL,
	}
}
