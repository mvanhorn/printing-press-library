package dd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/bandsintown/internal/client"
)

// EscapeArtistName double-escapes the four spec-listed special characters that
// Bandsintown's REST routing requires (per the spec note: "/" → %252F, "?" →
// %253F, "*" → %252A, '"' → %27C). Other characters pass through the standard
// path encoder by the HTTP client itself.
func EscapeArtistName(name string) string {
	r := strings.NewReplacer(
		"/", "%252F",
		"?", "%253F",
		"*", "%252A",
		"\"", "%27C",
	)
	return r.Replace(name)
}

// FetchArtist calls GET /artists/{name}?app_id=<id> and unmarshals into Artist.
func FetchArtist(c *client.Client, name, appID string) (*Artist, error) {
	if appID == "" {
		return nil, fmt.Errorf("BANDSINTOWN_APP_ID not set; partner credentials required since 2025")
	}
	path := "/artists/" + EscapeArtistName(name)
	data, err := c.Get(path, map[string]string{"app_id": appID})
	if err != nil {
		return nil, err
	}
	// API may use string or integer id; tolerate both.
	var raw struct {
		ID                 json.RawMessage `json:"id"`
		Name               string          `json:"name"`
		MBID               string          `json:"mbid"`
		URL                string          `json:"url"`
		ImageURL           string          `json:"image_url"`
		ThumbURL           string          `json:"thumb_url"`
		FacebookPageURL    string          `json:"facebook_page_url"`
		TrackerCount       int             `json:"tracker_count"`
		UpcomingEventCount int             `json:"upcoming_event_count"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode artist: %w", err)
	}
	idStr := strings.Trim(strings.TrimSpace(string(raw.ID)), `"`)
	if raw.Name == "" {
		raw.Name = name
	}
	return &Artist{
		Name:               raw.Name,
		ID:                 idStr,
		MBID:               raw.MBID,
		URL:                raw.URL,
		ImageURL:           raw.ImageURL,
		ThumbURL:           raw.ThumbURL,
		FacebookPageURL:    raw.FacebookPageURL,
		TrackerCount:       raw.TrackerCount,
		UpcomingEventCount: raw.UpcomingEventCount,
	}, nil
}

// FetchEvents calls GET /artists/{name}/events with the given date filter
// (which may be "upcoming", "past", "all", or a YYYY-MM-DD,YYYY-MM-DD range).
func FetchEvents(c *client.Client, name, appID, date string) ([]Event, error) {
	if appID == "" {
		return nil, fmt.Errorf("BANDSINTOWN_APP_ID not set; partner credentials required since 2025")
	}
	path := "/artists/" + EscapeArtistName(name) + "/events"
	params := map[string]string{"app_id": appID}
	if date != "" {
		params["date"] = date
	}
	data, err := c.Get(path, params)
	if err != nil {
		return nil, err
	}
	type rawVenue struct {
		Name      string `json:"name"`
		City      string `json:"city"`
		Region    string `json:"region"`
		Country   string `json:"country"`
		Latitude  string `json:"latitude"`
		Longitude string `json:"longitude"`
	}
	type rawOffer struct {
		Type   string `json:"type"`
		Status string `json:"status"`
		URL    string `json:"url"`
	}
	type rawEvent struct {
		ID          json.RawMessage `json:"id"`
		ArtistID    json.RawMessage `json:"artist_id"`
		Datetime    string          `json:"datetime"`
		Description string          `json:"description"`
		URL         string          `json:"url"`
		OnSaleAt    string          `json:"on_sale_datetime"`
		Venue       rawVenue        `json:"venue"`
		Lineup      []string        `json:"lineup"`
		Offers      []rawOffer      `json:"offers"`
	}
	var raws []rawEvent
	if err := json.Unmarshal(data, &raws); err != nil {
		return nil, fmt.Errorf("decode events: %w", err)
	}
	out := make([]Event, 0, len(raws))
	for _, r := range raws {
		idStr := strings.Trim(strings.TrimSpace(string(r.ID)), `"`)
		lat, _ := parseFloat(r.Venue.Latitude)
		lng, _ := parseFloat(r.Venue.Longitude)
		ev := Event{
			ID:           idStr,
			ArtistName:   name,
			Datetime:     r.Datetime,
			Description:  r.Description,
			URL:          r.URL,
			OnSaleAt:     r.OnSaleAt,
			VenueName:    r.Venue.Name,
			VenueCity:    r.Venue.City,
			VenueRegion:  r.Venue.Region,
			VenueCountry: r.Venue.Country,
			VenueLat:     lat,
			VenueLng:     lng,
			Lineup:       r.Lineup,
		}
		for _, o := range r.Offers {
			ev.Offers = append(ev.Offers, Offer{Type: o.Type, Status: o.Status, URL: o.URL})
		}
		out = append(out, ev)
	}
	return out, nil
}

func parseFloat(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// UpsertArtist writes the artist row, returning whether the row was newly
// inserted vs updated.
func UpsertArtist(ctx context.Context, db *sql.DB, a *Artist) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO dd_artists (name, bit_id, mbid, url, image_url, thumb_url, facebook_page_url, tracker_count, upcoming_event_count, fetched_at)
		VALUES (?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(name) DO UPDATE SET
			bit_id=excluded.bit_id,
			mbid=excluded.mbid,
			url=excluded.url,
			image_url=excluded.image_url,
			thumb_url=excluded.thumb_url,
			facebook_page_url=excluded.facebook_page_url,
			tracker_count=excluded.tracker_count,
			upcoming_event_count=excluded.upcoming_event_count,
			fetched_at=CURRENT_TIMESTAMP
	`, a.Name, a.ID, a.MBID, a.URL, a.ImageURL, a.ThumbURL, a.FacebookPageURL, a.TrackerCount, a.UpcomingEventCount)
	return err
}

// ReplaceEvents replaces all events for a given artist atomically.
// Bandsintown returns a complete event list per call; this matches that
// semantics (a removed-from-API event disappears from the local store on the
// next sync, which is exactly the "removed" diff signal sync's caller expects).
func ReplaceEvents(ctx context.Context, db *sql.DB, artistName string, events []Event) (added, removed int, err error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = tx.Rollback() }()

	// Capture the current event-ID set for this artist so we can compute a true
	// set-difference diff (added = in new, not in old; removed = in old, not in
	// new). Net count alone misses the rescheduled-tour case: 5 events replaced
	// by 5 different events reports 0/0 even though the entire lineup changed.
	oldIDs := map[string]struct{}{}
	rows, err := tx.QueryContext(ctx, `SELECT id FROM dd_events WHERE artist_name=?`, artistName)
	if err != nil {
		return 0, 0, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, 0, err
		}
		oldIDs[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, err
	}
	rows.Close()
	// Delete child rows (lineup members, offers) BEFORE deleting their parent
	// events — the subqueries need the parent table populated to resolve the
	// IN-clause. Swapping the order means a lineup change between pulls (e.g.,
	// Artist B dropped from Event X) actually removes the stale B→X row instead
	// of letting INSERT OR IGNORE silently preserve it, which would inflate
	// LineupCoBill counts forever. Same correctness issue applied to dd_offers:
	// without the explicit delete here, cancelled offers stayed in the table.
	if _, err := tx.ExecContext(ctx, `DELETE FROM dd_lineup_members WHERE event_id IN (SELECT id FROM dd_events WHERE artist_name=?)`, artistName); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dd_offers WHERE event_id IN (SELECT id FROM dd_events WHERE artist_name=?)`, artistName); err != nil {
		return 0, 0, err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM dd_events WHERE artist_name=?`, artistName); err != nil {
		return 0, 0, err
	}

	for _, e := range events {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO dd_events (id, artist_name, datetime, description, url, on_sale_at,
				venue_name, venue_city, venue_region, venue_country, venue_lat, venue_lng, fetched_at)
			VALUES (?,?,?,?,?,?,?,?,?,?,?,?,CURRENT_TIMESTAMP)
			ON CONFLICT(id) DO UPDATE SET
				artist_name=excluded.artist_name,
				datetime=excluded.datetime,
				description=excluded.description,
				url=excluded.url,
				on_sale_at=excluded.on_sale_at,
				venue_name=excluded.venue_name,
				venue_city=excluded.venue_city,
				venue_region=excluded.venue_region,
				venue_country=excluded.venue_country,
				venue_lat=excluded.venue_lat,
				venue_lng=excluded.venue_lng,
				fetched_at=CURRENT_TIMESTAMP
		`, e.ID, e.ArtistName, e.Datetime, e.Description, e.URL, e.OnSaleAt,
			e.VenueName, e.VenueCity, e.VenueRegion, e.VenueCountry, e.VenueLat, e.VenueLng); err != nil {
			return 0, 0, fmt.Errorf("insert event %s: %w", e.ID, err)
		}
		for _, m := range e.Lineup {
			if m == "" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO dd_lineup_members (event_id, artist_name) VALUES (?,?)`, e.ID, m); err != nil {
				return 0, 0, err
			}
		}
		for _, o := range e.Offers {
			if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO dd_offers (event_id, type, status, url) VALUES (?,?,?,?)`, e.ID, o.Type, o.Status, o.URL); err != nil {
				return 0, 0, err
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	// True set-difference diff (not net count delta). New event IDs that didn't
	// exist before are "added"; old IDs that didn't appear in the new list are
	// "removed". A 5-for-5 reschedule now reports 5/5, not 0/0.
	newIDs := make(map[string]struct{}, len(events))
	for _, e := range events {
		newIDs[e.ID] = struct{}{}
	}
	for id := range newIDs {
		if _, ok := oldIDs[id]; !ok {
			added++
		}
	}
	for id := range oldIDs {
		if _, ok := newIDs[id]; !ok {
			removed++
		}
	}
	return added, removed, nil
}

// SnapshotArtist records a tracker_count / upcoming_event_count point in time.
// Returns the snapshot timestamp written.
func SnapshotArtist(ctx context.Context, db *sql.DB, a *Artist) (string, error) {
	ts := time.Now().UTC().Format(time.RFC3339)
	_, err := db.ExecContext(ctx, `
		INSERT OR REPLACE INTO dd_artist_snapshots (artist_name, snapped_at, tracker_count, upcoming_event_count)
		VALUES (?,?,?,?)
	`, a.Name, ts, a.TrackerCount, a.UpcomingEventCount)
	return ts, err
}
