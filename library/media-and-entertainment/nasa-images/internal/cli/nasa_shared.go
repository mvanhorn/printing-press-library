// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.
// Hand-written by the Printing Press operator on top of generated scaffolding.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/nasa-images/internal/store"
)

// nasaItem mirrors the Collection+JSON item shape returned by /search and /album.
type nasaItem struct {
	Href  string          `json:"href"`
	Data  []nasaAssetData `json:"data,omitempty"`
	Links []nasaLink      `json:"links,omitempty"`
}

type nasaAssetData struct {
	NasaID           string   `json:"nasa_id,omitempty"`
	Title            string   `json:"title,omitempty"`
	Description      string   `json:"description,omitempty"`
	Description508   string   `json:"description_508,omitempty"`
	MediaType        string   `json:"media_type,omitempty"`
	DateCreated      string   `json:"date_created,omitempty"`
	Center           string   `json:"center,omitempty"`
	Photographer     string   `json:"photographer,omitempty"`
	SecondaryCreator string   `json:"secondary_creator,omitempty"`
	Location         string   `json:"location,omitempty"`
	Keywords         []string `json:"keywords,omitempty"`
	Album            []string `json:"album,omitempty"`
}

type nasaLink struct {
	Href   string `json:"href"`
	Rel    string `json:"rel,omitempty"`
	Render string `json:"render,omitempty"`
	Prompt string `json:"prompt,omitempty"`
}

type nasaCollectionResponse struct {
	Collection struct {
		Items    []nasaItem `json:"items"`
		Links    []nasaLink `json:"links"`
		Metadata struct {
			TotalHits int `json:"total_hits"`
		} `json:"metadata"`
	} `json:"collection"`
}

// parseNasaCollection unmarshals raw API JSON into the typed Collection+JSON shape.
func parseNasaCollection(raw json.RawMessage) (*nasaCollectionResponse, error) {
	var c nasaCollectionResponse
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("parsing collection: %w", err)
	}
	return &c, nil
}

// ensureNasaSchema creates the auxiliary tables nasa-images novel commands depend on.
// Each command that touches downloads or album_members calls this first; CREATE IF
// NOT EXISTS makes repeated calls a no-op.
func ensureNasaSchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS downloads (
			nasa_id TEXT NOT NULL,
			variant TEXT NOT NULL,
			url TEXT NOT NULL,
			local_path TEXT NOT NULL,
			bytes_downloaded INTEGER NOT NULL DEFAULT 0,
			bytes_total INTEGER NOT NULL DEFAULT 0,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at TEXT NOT NULL,
			completed_at TEXT,
			PRIMARY KEY (nasa_id, variant)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status)`,
		`CREATE TABLE IF NOT EXISTS album_members (
			album_name TEXT NOT NULL,
			nasa_id TEXT NOT NULL,
			position INTEGER NOT NULL DEFAULT 0,
			mirrored_at TEXT NOT NULL,
			PRIMARY KEY (album_name, nasa_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_album_members_nasa_id ON album_members(nasa_id)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("ensuring nasa schema: %w", err)
		}
	}
	return nil
}

// openNasaStore opens the local SQLite store and ensures the auxiliary schema.
func openNasaStore(ctx context.Context, dbPath string) (*store.Store, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("nasa-images-pp-cli")
	}
	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	if err := ensureNasaSchema(ctx, s.DB()); err != nil {
		_ = s.Close()
		return nil, err
	}
	return s, nil
}

// upsertAsset stores one nasaAssetData row in the generic resources table under
// resource_type='asset', keyed by nasa_id. The data column carries the full
// JSON-encoded asset for use by every downstream local-query command.
func upsertAsset(s *store.Store, item nasaItem) (string, error) {
	if len(item.Data) == 0 || item.Data[0].NasaID == "" {
		return "", nil
	}
	nasaID := item.Data[0].NasaID
	raw, err := json.Marshal(item.Data[0])
	if err != nil {
		return "", fmt.Errorf("marshaling asset %q: %w", nasaID, err)
	}
	if err := s.Upsert("asset", nasaID, raw); err != nil {
		return "", fmt.Errorf("upserting asset %q: %w", nasaID, err)
	}
	return nasaID, nil
}

// recordAlbumMember inserts or updates the (album_name, nasa_id) link.
func recordAlbumMember(ctx context.Context, db *sql.DB, album, nasaID string, position int) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO album_members (album_name, nasa_id, position, mirrored_at)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(album_name, nasa_id) DO UPDATE SET position=excluded.position, mirrored_at=excluded.mirrored_at`,
		album, nasaID, position, time.Now().UTC().Format(time.RFC3339))
	return err
}

// nextPageFromLinks scans Collection+JSON links for the rel="next" entry and
// returns its href (upgraded to https). Returns "" when no next page exists.
func nextPageFromLinks(links []nasaLink) string {
	for _, l := range links {
		if strings.EqualFold(l.Rel, "next") && l.Href != "" {
			return upgradeToHTTPS(l.Href)
		}
	}
	return ""
}

// upgradeToHTTPS rewrites http:// URLs from NASA's response envelopes to https://.
// NASA echoes http:// in collection.href and link hrefs even when called over
// https; both hosts (images-api.nasa.gov, images-assets.nasa.gov) serve TLS.
func upgradeToHTTPS(raw string) string {
	if strings.HasPrefix(raw, "http://") {
		return "https://" + strings.TrimPrefix(raw, "http://")
	}
	return raw
}

// hrefPathAndParams parses a full URL and returns the path plus query map.
// Used to convert NASA's "next" link URL into a (path, params) pair the
// generated client.Get can call.
func hrefPathAndParams(rawURL string) (string, map[string]string, error) {
	u, err := url.Parse(upgradeToHTTPS(rawURL))
	if err != nil {
		return "", nil, err
	}
	params := make(map[string]string, len(u.Query()))
	for k, v := range u.Query() {
		if len(v) > 0 {
			params[k] = v[0]
		}
	}
	return u.Path, params, nil
}

// httpResponseBodyCap bounds responses read into memory by httpGetBody.
// Caption files and metadata sidecars are kilobytes; 16 MiB is a generous
// safety margin against misconfigured CDNs or HTML error pages.
const httpResponseBodyCap = 16 << 20

// httpClientForFlags builds an *http.Client that honors the CLI's --timeout
// flag for direct fetches against images-assets.nasa.gov (the asset CDN).
// The generated client.Client is API-host-only and applies the same timeout
// to /search, /asset, /metadata, /captions, /album — this helper extends the
// guarantee to the indirection-followed and bulk-download paths so a stalled
// CDN connection respects --timeout instead of hanging indefinitely.
func httpClientForFlags(flags *rootFlags) *http.Client {
	if flags == nil || flags.timeout <= 0 {
		return http.DefaultClient
	}
	return &http.Client{Timeout: flags.timeout}
}

// httpGetBody fetches an arbitrary URL and returns the response body bytes,
// bounded by httpResponseBodyCap to defend against misconfigured CDNs or HTML
// error pages. Used by the indirection-following commands (captions fetch,
// metadata fetch). Bypasses the generated client because these requests hit
// images-assets.nasa.gov, not the API host; the per-call client honors the
// CLI's --timeout flag.
func httpGetBody(ctx context.Context, flags *rootFlags, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", upgradeToHTTPS(rawURL), nil)
	if err != nil {
		return nil, err
	}
	resp, err := httpClientForFlags(flags).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, httpResponseBodyCap))
}

// quoteFTS wraps a user-supplied FTS5 query in double quotes when it
// contains characters that FTS5 treats as syntax (hyphen, colon, etc.).
// FTS5 parses "apollo-11" as "apollo NOT 11" because the bare hyphen is the
// negation operator; quoting forces a phrase match. Inner double quotes are
// doubled per FTS5's escape rules.
func quoteFTS(q string) string {
	q = strings.TrimSpace(q)
	if q == "" {
		return q
	}
	// Already quoted? Return as-is so power users can hand-author FTS syntax.
	if strings.HasPrefix(q, "\"") && strings.HasSuffix(q, "\"") {
		return q
	}
	// Quote when any non-word character is present.
	needsQuote := false
	for _, r := range q {
		switch r {
		case '-', ':', '*', '"', '(', ')', '\'':
			needsQuote = true
		}
		if needsQuote {
			break
		}
	}
	if !needsQuote {
		return q
	}
	return "\"" + strings.ReplaceAll(q, "\"", "\"\"") + "\""
}

// classifyVariant tags a NASA asset URL by rendition kind based on filename
// suffix. Returns one of "orig", "large", "medium", "small", "thumb", "mobile",
// "preview", "metadata", "captions", "audio_orig", "audio_128k", or "other".
func classifyVariant(href string) string {
	lower := strings.ToLower(href)
	switch {
	case strings.Contains(lower, "metadata.json"):
		return "metadata"
	case strings.HasSuffix(lower, ".srt"), strings.HasSuffix(lower, ".vtt"):
		return "captions"
	case strings.Contains(lower, "~orig."):
		if strings.HasSuffix(lower, ".mp3") || strings.HasSuffix(lower, ".m4a") {
			return "audio_orig"
		}
		return "orig"
	case strings.Contains(lower, "~large."):
		return "large"
	case strings.Contains(lower, "~medium."):
		return "medium"
	case strings.Contains(lower, "~small."):
		return "small"
	case strings.Contains(lower, "~thumb."):
		return "thumb"
	case strings.Contains(lower, "~mobile."):
		return "mobile"
	case strings.Contains(lower, "~preview."):
		return "preview"
	case strings.HasSuffix(lower, "~128k.mp3"), strings.HasSuffix(lower, "~128k.m4a"):
		return "audio_128k"
	default:
		return "other"
	}
}
