// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Shared helpers for the 12 transcendence commands. Splits out the
// "fetch from API or local store + cache + iterate" boilerplate so each
// feature file stays small and obvious.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/spotify/internal/store"
)

// storeQueryer is the read-side subset of database/sql APIs that the
// transcendence query helpers need. *sql.DB satisfies it, as does any
// fake or in-memory implementation a unit test brings in.
type storeQueryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// storeExecQueryer adds Exec for write-side tests.
type storeExecQueryer interface {
	storeQueryer
	Exec(query string, args ...any) (sql.Result, error)
}

// openTranscendenceStore opens the local SQLite database and ensures the
// hand-written transcendence schema exists. Caller must Close().
func openTranscendenceStore(ctx context.Context) (*store.Store, error) {
	dbPath := defaultDBPath("spotify-pp-cli")
	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening local database: %w", err)
	}
	if err := db.EnsureTranscendenceSchema(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ensuring transcendence schema: %w", err)
	}
	return db, nil
}

// playlistTrackItem mirrors a row from /playlists/{id}/tracks. Defined once
// so the three commands that consume full playlist contents (T1 diff,
// T2 dedupe, T3 merge) share a single track shape.
type playlistTrackItem struct {
	AddedAt string `json:"added_at"`
	AddedBy struct {
		ID string `json:"id"`
	} `json:"added_by"`
	Track struct {
		ID      string `json:"id"`
		URI     string `json:"uri"`
		Name    string `json:"name"`
		Artists []struct {
			Name string `json:"name"`
		} `json:"artists"`
		ExternalIDs struct {
			ISRC string `json:"isrc"`
		} `json:"external_ids"`
	} `json:"track"`
}

// PATCH (fix-playlist-track-pagination):
// fetchFullPlaylist returns a playlist's metadata + every track row,
// paginating /playlists/{id}/tracks to bypass the 100-item embed cap on
// GET /playlists/{id}. Calls one metadata fetch (with ?fields= to keep
// the payload small) and one paginated /tracks fetch. Both commands
// that snapshot a playlist (T1, T2) and the source-walking pass in T3
// route through here so the truncation cannot recur per call-site.
func fetchFullPlaylist(c *client.Client, playlistID string) (id, name, snapshotID string, items []playlistTrackItem, err error) {
	metaData, err := c.Get("/playlists/"+playlistID+"?fields=id,name,snapshot_id", nil)
	if err != nil {
		return "", "", "", nil, err
	}
	var meta struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		SnapshotID string `json:"snapshot_id"`
	}
	if err := json.Unmarshal(metaData, &meta); err != nil {
		return "", "", "", nil, fmt.Errorf("decoding playlist metadata: %w", err)
	}

	raw, err := fetchAllPaged(c, "/playlists/"+playlistID+"/tracks", map[string]string{"limit": "50"}, 0)
	if err != nil {
		return meta.ID, meta.Name, meta.SnapshotID, nil, fmt.Errorf("paginating /playlists/%s/tracks: %w", playlistID, err)
	}
	items = make([]playlistTrackItem, 0, len(raw))
	for _, r := range raw {
		var item playlistTrackItem
		if json.Unmarshal(r, &item) == nil {
			items = append(items, item)
		}
	}
	return meta.ID, meta.Name, meta.SnapshotID, items, nil
}

// fetchAllPaged repeatedly hits a Spotify list endpoint following `next`
// cursors until exhausted or `limit` items collected. Returns the merged
// `items` array as raw JSON.
func fetchAllPaged(c *client.Client, path string, params map[string]string, limit int) ([]json.RawMessage, error) {
	if params == nil {
		params = map[string]string{}
	}
	if _, ok := params["limit"]; !ok {
		params["limit"] = "50"
	}
	collected := []json.RawMessage{}
	cursor := path
	cursorParams := params
	for {
		data, err := c.Get(cursor, cursorParams)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Items []json.RawMessage `json:"items"`
			Next  string            `json:"next"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			return nil, fmt.Errorf("decoding paged response: %w", err)
		}
		collected = append(collected, resp.Items...)
		if limit > 0 && len(collected) >= limit {
			collected = collected[:limit]
			break
		}
		if resp.Next == "" {
			break
		}
		nextPath, nextParams, err := splitURL(resp.Next)
		if err != nil {
			return nil, err
		}
		cursor = nextPath
		cursorParams = nextParams
	}
	return collected, nil
}

// splitURL converts a full Spotify next-URL (e.g.
// "https://api.spotify.com/v1/me/tracks?limit=50&offset=50") back into
// (relative path, params) so the client.Get path-builder can be reused.
func splitURL(full string) (string, map[string]string, error) {
	u, err := url.Parse(full)
	if err != nil {
		return "", nil, fmt.Errorf("parsing next URL: %w", err)
	}
	p := strings.TrimPrefix(u.Path, "/v1")
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	params := map[string]string{}
	for k, v := range u.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	return p, params, nil
}

// fetchCurrentUserID returns the authenticated user's Spotify ID. Used to
// scope per-user tables (saved_tracks, followed_artists, etc.).
func fetchCurrentUserID(c *client.Client) (string, error) {
	data, err := c.Get("/me", nil)
	if err != nil {
		return "", err
	}
	var me struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &me); err != nil {
		return "", fmt.Errorf("decoding /me: %w", err)
	}
	if me.ID == "" {
		return "", fmt.Errorf("/me returned empty id")
	}
	return me.ID, nil
}

// extractURI strips the "spotify:track:" prefix if present and returns just
// the bare ID. Accepts either bare IDs or full URIs.
func bareID(s string) string {
	if i := strings.LastIndex(s, ":"); i >= 0 {
		return s[i+1:]
	}
	return s
}
