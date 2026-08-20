// Copyright 2026 Amit and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

func newTrackCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track <share-link-or-id>",
		Short: "Track a Wolt order by its share link (best-effort: see Known Gaps)",
		Long: "Parses a Wolt order share URL like https://wolt.com/en/track/<id> and\n" +
			"prints the extracted order id plus the public tracking URL. The JSON\n" +
			"endpoint behind the page is not yet discovered — see README's Known\n" +
			"Gaps section. Help us by capturing the tracking-page XHR in DevTools\n" +
			"and filing it as an issue.",
		Example: "  wolt-pp-cli track https://wolt.com/en/track/5f9132c7b4d5bd0196951924",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			raw := args[0]
			id := extractTrackID(raw)
			if id == "" {
				return fmt.Errorf("could not extract order id from %q (expected wolt.com/<lang>/track/<id> or just <id>)", raw)
			}
			out := struct {
				Status       string `json:"status"`
				OrderID      string `json:"order_id"`
				TrackingURL  string `json:"tracking_url"`
				EndpointNote string `json:"endpoint_note"`
			}{
				Status:       "stub",
				OrderID:      id,
				TrackingURL:  "https://wolt.com/en/track/" + id,
				EndpointNote: "Live JSON tracking endpoint is undocumented. Open the share link in a browser to see status. Help: capture the network call wolt.com makes when loading the tracking page and file an issue with the request shape.",
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func extractTrackID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "/") && !strings.Contains(raw, " ") {
		return raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	// PATCH(track-extract-strict): only accept URLs whose path contains a
	// "track" or "tracking" segment. The previous fallback returned the
	// last path segment for ANY URL with at least one segment, which made
	// the command emit plausible-looking but entirely wrong order IDs for
	// non-tracking URLs (e.g. wolt.com/en/foo/menu → "menu").
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i, p := range parts {
		if (p == "track" || p == "tracking") && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}
