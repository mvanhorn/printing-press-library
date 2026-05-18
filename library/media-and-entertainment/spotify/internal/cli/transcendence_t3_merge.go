// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// T3 — Cross-playlist merge with dedupe.
//
//	playlists merge <src-1> <src-2> [...] --into <dest> [--dedupe-by isrc|track-id] [--order keep|shuffle|by-date]
//
// Pulls source playlists' tracks (cache-friendly via local snapshot table
// when present), dedupes, and POSTs to the destination playlist in
// 100-track chunks. Spotify ships no merge primitive — users currently
// script this against spotipy.

package cli

import (
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/cliutil"
)

func newPlaylistsMergeCmd(flags *rootFlags) *cobra.Command {
	var destPlaylist string
	var dedupeBy string
	var order string
	cmd := &cobra.Command{
		Use:   "merge <src-1> <src-2> [...] --into <dest>",
		Short: "Merge multiple playlists into one with dedupe",
		Long: `Reads source playlists' tracks (paginated), dedupes across sources via
--dedupe-by, orders them per --order, and writes the result to --into.
The first chunk uses PUT /playlists/{id}/tracks which REPLACES the
destination's current contents; subsequent chunks (for merges > 100
tracks) append. Re-running with the same sources produces the same
destination state (idempotent); switching sources cleanly replaces.
Pass the global --dry-run flag to preview without writing.`,
		Example: "  spotify-pp-cli playlists merge 37i9dQZF1DXcBWIGoYBM5M 37i9dQZF1DX0XUsuxWHRQd --into 1xL2KvJlHWXKxLrYrmEZ7K",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return usageErr(fmt.Errorf("merge requires at least 2 source playlist IDs"))
			}
			if destPlaylist == "" {
				return usageErr(fmt.Errorf("--into <dest-playlist-id> is required"))
			}
			switch dedupeBy {
			case "isrc", "track-id":
			default:
				return usageErr(fmt.Errorf("--dedupe-by must be 'isrc' or 'track-id'"))
			}
			switch order {
			case "keep", "shuffle", "by-date":
			default:
				return usageErr(fmt.Errorf("--order must be 'keep', 'shuffle', or 'by-date'"))
			}

			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dry_run":     true,
					"sources":     args,
					"destination": destPlaylist,
					"dedupe_by":   dedupeBy,
					"order":       order,
				}, flags)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type trackEntry struct {
				URI     string
				ID      string
				ISRC    string
				AddedAt string
				Name    string
			}
			// PATCH (fix-playlist-track-pagination, fix-merge-replace-and-paginate):
			// Paginate each source via /playlists/{id}/tracks so playlists
			// with more than 100 tracks contribute their full contents to
			// the merge (the embedded tracks field on GET /playlists/{id}
			// caps at 100 and would silently drop the tail).
			var plan []trackEntry
			for _, src := range args {
				srcID := bareID(src)
				_, _, _, items, err := fetchFullPlaylist(c, srcID)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, item := range items {
					plan = append(plan, trackEntry{
						URI:     item.Track.URI,
						ID:      item.Track.ID,
						ISRC:    strings.ToUpper(item.Track.ExternalIDs.ISRC),
						AddedAt: item.AddedAt,
						Name:    item.Track.Name,
					})
				}
			}

			// Dedupe.
			seen := map[string]bool{}
			var deduped []trackEntry
			for _, t := range plan {
				var key string
				if dedupeBy == "isrc" {
					key = t.ISRC
					if key == "" {
						key = "id:" + t.ID
					}
				} else {
					key = t.ID
				}
				if seen[key] {
					continue
				}
				seen[key] = true
				deduped = append(deduped, t)
			}

			// Order.
			switch order {
			case "shuffle":
				rand.Shuffle(len(deduped), func(i, j int) { deduped[i], deduped[j] = deduped[j], deduped[i] })
			case "by-date":
				sort.SliceStable(deduped, func(i, j int) bool { return deduped[i].AddedAt < deduped[j].AddedAt })
			}

			uris := make([]string, 0, len(deduped))
			for _, t := range deduped {
				if t.URI != "" {
					uris = append(uris, t.URI)
				}
			}

			// PATCH (fix-merge-replace-and-paginate):
			// Write to destination in 100-track chunks. The first chunk uses
			// PUT /playlists/{id}/tracks which REPLACES the destination's
			// existing contents — otherwise re-running merge silently doubles
			// the destination because POST only appends. Subsequent chunks
			// use POST to append. Net effect: re-running with the same sources
			// produces the same destination state (idempotent), and switching
			// sources cleanly replaces.
			const chunkSize = 100
			added := 0
			if len(uris) == 0 {
				// Nothing to write; explicit empty PUT clears the destination
				// so re-running with empty sources also stays idempotent.
				_, _, err := c.Put("/playlists/"+destPlaylist+"/tracks", map[string]any{"uris": []string{}})
				if err != nil {
					return classifyAPIError(err, flags)
				}
			}
			for i := 0; i < len(uris); i += chunkSize {
				end := i + chunkSize
				if end > len(uris) {
					end = len(uris)
				}
				body := map[string]any{"uris": uris[i:end]}
				var err error
				if i == 0 {
					_, _, err = c.Put("/playlists/"+destPlaylist+"/tracks", body)
				} else {
					_, _, err = c.Post("/playlists/"+destPlaylist+"/tracks", body)
				}
				if err != nil {
					return classifyAPIError(err, flags)
				}
				added += end - i
			}

			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"destination": destPlaylist,
				"sources":     args,
				"dedupe_by":   dedupeBy,
				"order":       order,
				"added":       added,
				"deduped":     len(plan) - len(deduped),
			}, flags)
		},
	}
	cmd.Flags().StringVar(&destPlaylist, "into", "", "Destination playlist ID")
	cmd.Flags().StringVar(&dedupeBy, "dedupe-by", "isrc", "Dedupe key: isrc or track-id")
	cmd.Flags().StringVar(&order, "order", "keep", "Track order: keep, shuffle, or by-date")
	return cmd
}
