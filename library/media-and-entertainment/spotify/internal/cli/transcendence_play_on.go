// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// PATCH (add-play-on-device-by-name):
// play-on <device-name> — start playback on a device referenced by friendly
// name instead of opaque Spotify device ID. Resolves the name against the
// live /me/player/devices list first, then falls back to the cached
// devices_seen table (populated by sync-extras). Surfaces a typed hint when
// a device is known to the cache but not currently online with Spotify
// Connect, since that's the most common Echo/Sonos failure mode.

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

type deviceRow struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	IsActive bool   `json:"is_active"`
	Source   string `json:"source"` // "live" or "cache"
}

func newPlayOnCmd(flags *rootFlags) *cobra.Command {
	var urisFlag string
	var contextURI string
	var noRefresh bool
	cmd := &cobra.Command{
		Use:   "play-on <device-name>",
		Short: "Play on a device by friendly name (looks up live + cached devices_seen)",
		Long: `Resolves a device by friendly name (case-insensitive, exact > prefix >
substring) against both the live /me/player/devices list and the locally
cached devices_seen table, then starts playback on it.

Without --uris or --context-uri, resumes whatever was playing.

Spotify Connect requires the target device to be online to accept playback.
play-on can't bypass that — but it can tell you whether the device is in
your cache vs visible right now, which is the half of the answer that needs
local state.`,
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2"},
		Example: `  spotify-pp-cli play-on "family room"
  spotify-pp-cli play-on iphone --uris '["spotify:track:0nys6GusuHnjSYLW0PYYb7"]'
  spotify-pp-cli play-on "mad's room" --context-uri spotify:album:1ER3B6zev5JEAaqhnyyfbf`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			query := strings.TrimSpace(args[0])

			db, err := openTranscendenceStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run": true,
					"query":   query,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Step 1: refresh live device list (unless caller opted out)
			// and write any new sightings back to devices_seen so the cache
			// stays current as a side effect of every play-on call.
			liveDevices := []deviceRow{}
			if !noRefresh {
				data, err := c.Get("/me/player/devices", nil)
				if err == nil {
					var resp struct {
						Devices []struct {
							ID       string `json:"id"`
							Name     string `json:"name"`
							Type     string `json:"type"`
							IsActive bool   `json:"is_active"`
							Volume   int    `json:"volume_percent"`
						} `json:"devices"`
					}
					if json.Unmarshal(data, &resp) == nil {
						now := time.Now().UTC()
						for _, d := range resp.Devices {
							if d.ID == "" {
								continue
							}
							liveDevices = append(liveDevices, deviceRow{
								ID:       d.ID,
								Name:     d.Name,
								Type:     d.Type,
								IsActive: d.IsActive,
								Source:   "live",
							})
							_ = db.InsertDeviceSeen(d.ID, d.Name, d.Type, d.IsActive, d.Volume, now)
						}
					}
				}
			}

			// Step 2: load cache and merge — live entries shadow cache.
			cached, err := readCachedDevices(db.DB())
			if err != nil {
				return err
			}
			liveByID := map[string]bool{}
			for _, d := range liveDevices {
				liveByID[d.ID] = true
			}
			merged := append([]deviceRow{}, liveDevices...)
			for _, d := range cached {
				if !liveByID[d.ID] {
					merged = append(merged, d)
				}
			}

			// Step 3: name resolution.
			match, matchErr := resolveDeviceByName(query, merged)
			if matchErr != nil {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"error":          matchErr.Error(),
					"query":          query,
					"available_now":  liveDevices,
					"cached_offline": onlyCached(cached, liveByID),
				}, flags)
			}

			// Step 4: build body + URL and send.
			body := map[string]any{}
			if urisFlag != "" {
				var parsed []string
				if err := json.Unmarshal([]byte(urisFlag), &parsed); err != nil {
					return fmt.Errorf("--uris must be a JSON array of Spotify track URIs: %w", err)
				}
				body["uris"] = parsed
			}
			if contextURI != "" {
				body["context_uri"] = contextURI
			}
			path := "/me/player/play?device_id=" + url.QueryEscape(match.ID)

			var sendBody any
			if len(body) > 0 {
				sendBody = body
			}
			_, statusCode, err := c.Put(path, sendBody)
			if err != nil {
				// 404 from /me/player/play almost always means "device
				// went offline between list and play". Status code via
				// typed APIError, not string-match. Fires for live OR
				// cached sources, since a device can drop offline in the
				// hundreds of milliseconds between list and play.
				var apiErr *client.APIError
				if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error":         "device not currently online with Spotify Connect",
						"device":        match,
						"hint":          wakeHintFor(match.Type),
						"available_now": liveDevices,
					}, flags)
				}
				return classifyAPIError(err, flags)
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"played":  true,
				"device":  match,
				"status":  statusCode,
				"context": contextURI,
				"uris":    body["uris"],
			}, flags)
		},
	}
	cmd.Flags().StringVar(&urisFlag, "uris", "", "JSON array of Spotify track URIs to play, e.g. '[\"spotify:track:...\"]'")
	cmd.Flags().StringVar(&contextURI, "context-uri", "", "Spotify URI of an album, artist, or playlist to play")
	cmd.Flags().BoolVar(&noRefresh, "no-refresh", false, "Skip the live /me/player/devices refresh; resolve from cache only")
	cmd.MarkFlagsMutuallyExclusive("uris", "context-uri")
	return cmd
}

// wakeHintFor returns the right "how do I bring this device online" message
// based on Spotify's device type. Echo/Sonos speakers wake via voice or the
// Spotify-Connect picker in the Alexa/Sonos app; phones and tablets wake by
// opening the Spotify app; computers wake by opening Spotify desktop. The
// hint matters because the offline-resurrection path differs per platform.
func wakeHintFor(devType string) string {
	switch strings.ToLower(devType) {
	case "smartphone", "tablet":
		return "open the Spotify app on this device to bring it online with Spotify Connect, then retry play-on"
	case "computer":
		return "open Spotify on this computer to bring it online with Spotify Connect, then retry play-on"
	case "speaker", "avr", "stb", "tv", "audiodongle", "gameconsole", "castvideo", "castaudio":
		return "ask the device (Alexa/Sonos/etc.) to play something briefly, or pick it from the Spotify Connect menu in the controlling app, then retry play-on"
	default:
		return "bring the device online with Spotify Connect (open its Spotify app, or play something briefly) then retry play-on"
	}
}

func readCachedDevices(db storeQueryer) ([]deviceRow, error) {
	rows, err := db.Query(`SELECT id, name, type, is_active FROM devices_seen ORDER BY last_seen_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("reading devices_seen: %w", err)
	}
	defer rows.Close()
	var out []deviceRow
	for rows.Next() {
		var d deviceRow
		var active int
		if err := rows.Scan(&d.ID, &d.Name, &d.Type, &active); err != nil {
			return nil, err
		}
		d.IsActive = active != 0
		d.Source = "cache"
		out = append(out, d)
	}
	return out, rows.Err()
}

func onlyCached(cached []deviceRow, liveIDs map[string]bool) []deviceRow {
	out := make([]deviceRow, 0, len(cached))
	for _, d := range cached {
		if !liveIDs[d.ID] {
			out = append(out, d)
		}
	}
	return out
}

// resolveDeviceByName finds the single best match for query against the
// device list. Precedence: exact (case-insensitive) > prefix > substring.
// Returns (nil, error) for zero matches or ambiguous matches at the top
// precedence level.
func resolveDeviceByName(query string, devices []deviceRow) (*deviceRow, error) {
	if len(devices) == 0 {
		return nil, fmt.Errorf("no devices known — run 'spotify-pp-cli sync-extras' or play something on the target device first")
	}
	q := strings.ToLower(query)
	var exact, prefix, substr []deviceRow
	for _, d := range devices {
		n := strings.ToLower(d.Name)
		switch {
		case n == q:
			exact = append(exact, d)
		case strings.HasPrefix(n, q):
			prefix = append(prefix, d)
		case strings.Contains(n, q):
			substr = append(substr, d)
		}
	}
	for _, bucket := range [][]deviceRow{exact, prefix, substr} {
		if len(bucket) == 1 {
			return &bucket[0], nil
		}
		if len(bucket) > 1 {
			names := make([]string, len(bucket))
			for i, d := range bucket {
				names[i] = fmt.Sprintf("%q (%s)", d.Name, d.Source)
			}
			return nil, fmt.Errorf("ambiguous device name %q matches: %s", query, strings.Join(names, ", "))
		}
	}
	return nil, fmt.Errorf("no device matches %q", query)
}
