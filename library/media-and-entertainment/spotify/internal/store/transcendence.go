// Copyright 2026 rob-coco. Licensed under Apache-2.0. See LICENSE.

// Transcendence-feature schema. These tables back the 12 novel features that
// distinguish spotify-pp-cli from the field (snapshot diff, top-tracks drift,
// release radar replacement, etc.). The generator-emitted schema (store.go)
// covers raw API resource caching; this file adds the per-user denormalized
// state that those features query: saved_tracks, top_*_snapshot, play_history,
// followed_artists, etc.
//
// Tables are created on demand via EnsureTranscendenceSchema(); this side-
// steps the generator's migration framework (store.go is overwritten by
// regen) and keeps the additions in a single hand-written file.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// EnsureTranscendenceSchema creates the hand-written transcendence tables if
// they don't already exist. Safe to call repeatedly. Run before any
// transcendence-feature query.
func (s *Store) EnsureTranscendenceSchema() error {
	stmts := []string{
		// --- Saved library tables -----------------------------------------
		`CREATE TABLE IF NOT EXISTS saved_tracks (
			user_id TEXT NOT NULL,
			track_id TEXT NOT NULL,
			saved_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, track_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_saved_tracks_user ON saved_tracks(user_id)`,
		`CREATE TABLE IF NOT EXISTS saved_albums (
			user_id TEXT NOT NULL,
			album_id TEXT NOT NULL,
			saved_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, album_id)
		)`,
		`CREATE TABLE IF NOT EXISTS saved_shows (
			user_id TEXT NOT NULL,
			show_id TEXT NOT NULL,
			saved_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, show_id)
		)`,
		`CREATE TABLE IF NOT EXISTS saved_episodes (
			user_id TEXT NOT NULL,
			episode_id TEXT NOT NULL,
			saved_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, episode_id)
		)`,

		// --- Followed artists --------------------------------------------
		// Spotify's API doesn't expose a followed_at timestamp; first_seen_at
		// records when the local store first observed the artist as followed.
		`CREATE TABLE IF NOT EXISTS followed_artists (
			user_id TEXT NOT NULL,
			artist_id TEXT NOT NULL,
			artist_name TEXT,
			first_seen_at DATETIME NOT NULL,
			PRIMARY KEY (user_id, artist_id)
		)`,

		// --- Top snapshots (drift queries depend on captured_at) ----------
		`CREATE TABLE IF NOT EXISTS top_tracks_snapshot (
			captured_at DATETIME NOT NULL,
			time_range TEXT NOT NULL,
			position INTEGER NOT NULL,
			track_id TEXT NOT NULL,
			track_name TEXT,
			PRIMARY KEY (captured_at, time_range, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_top_tracks_range ON top_tracks_snapshot(time_range, captured_at)`,
		`CREATE TABLE IF NOT EXISTS top_artists_snapshot (
			captured_at DATETIME NOT NULL,
			time_range TEXT NOT NULL,
			position INTEGER NOT NULL,
			artist_id TEXT NOT NULL,
			artist_name TEXT,
			artist_genres TEXT,
			PRIMARY KEY (captured_at, time_range, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_top_artists_range ON top_artists_snapshot(time_range, captured_at)`,

		// --- Play history (extended beyond API's 50-cap via repeated sync)
		`CREATE TABLE IF NOT EXISTS play_history (
			played_at DATETIME NOT NULL,
			track_id TEXT NOT NULL,
			track_name TEXT,
			context_uri TEXT,
			context_type TEXT,
			PRIMARY KEY (played_at, track_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_play_history_context ON play_history(context_uri)`,
		`CREATE INDEX IF NOT EXISTS idx_play_history_track ON play_history(track_id)`,

		// --- Snapshot-aware playlist tracks (T1 diff) ---------------------
		// Distinct from the generator's playlists_tracks (which is keyed by
		// the track ID alone, no snapshot history). This one is snapshot-
		// versioned so two snapshots of the same playlist can coexist for
		// the diff query.
		`CREATE TABLE IF NOT EXISTS playlist_snapshot_tracks (
			playlist_id TEXT NOT NULL,
			snapshot_id TEXT NOT NULL,
			captured_at DATETIME NOT NULL,
			position INTEGER NOT NULL,
			track_id TEXT NOT NULL,
			track_uri TEXT,
			track_name TEXT,
			isrc TEXT,
			added_at DATETIME,
			added_by TEXT,
			PRIMARY KEY (playlist_id, snapshot_id, position)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_playlist_snap_pl ON playlist_snapshot_tracks(playlist_id, captured_at)`,
		`CREATE INDEX IF NOT EXISTS idx_playlist_snap_isrc ON playlist_snapshot_tracks(isrc)`,

		// --- Devices observed --------------------------------------------
		`CREATE TABLE IF NOT EXISTS devices_seen (
			id TEXT PRIMARY KEY,
			name TEXT,
			type TEXT,
			is_active INTEGER,
			volume_percent INTEGER,
			last_seen_at DATETIME NOT NULL
		)`,
	}
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transcendence schema tx: %w", err)
	}
	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("creating transcendence schema: %w", err)
		}
	}
	return tx.Commit()
}

// ---------- Insert helpers ----------------------------------------------------

// InsertSavedTrack records a saved-tracks row. Idempotent on (user_id, track_id).
func (s *Store) InsertSavedTrack(userID, trackID string, savedAt time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO saved_tracks (user_id, track_id, saved_at) VALUES (?, ?, ?)`,
		userID, trackID, savedAt.UTC().Format(time.RFC3339))
	return err
}

// InsertSavedAlbum records a saved-albums row.
func (s *Store) InsertSavedAlbum(userID, albumID string, savedAt time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO saved_albums (user_id, album_id, saved_at) VALUES (?, ?, ?)`,
		userID, albumID, savedAt.UTC().Format(time.RFC3339))
	return err
}

// InsertFollowedArtist records a follow. first_seen_at is preserved on conflict
// — the second observation does not move the timestamp back.
func (s *Store) InsertFollowedArtist(userID, artistID, name string, firstSeenAt time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO followed_artists (user_id, artist_id, artist_name, first_seen_at) VALUES (?, ?, ?, ?)`,
		userID, artistID, name, firstSeenAt.UTC().Format(time.RFC3339))
	return err
}

// InsertTopTrack records one slot in a top-tracks snapshot.
func (s *Store) InsertTopTrack(capturedAt time.Time, timeRange string, position int, trackID, trackName string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR REPLACE INTO top_tracks_snapshot (captured_at, time_range, position, track_id, track_name) VALUES (?, ?, ?, ?, ?)`,
		capturedAt.UTC().Format(time.RFC3339), timeRange, position, trackID, trackName)
	return err
}

// InsertTopArtist records one slot in a top-artists snapshot.
func (s *Store) InsertTopArtist(capturedAt time.Time, timeRange string, position int, artistID, artistName string, genres []string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	genresJSON, _ := json.Marshal(genres)
	_, err := s.db.Exec(`INSERT OR REPLACE INTO top_artists_snapshot (captured_at, time_range, position, artist_id, artist_name, artist_genres) VALUES (?, ?, ?, ?, ?, ?)`,
		capturedAt.UTC().Format(time.RFC3339), timeRange, position, artistID, artistName, string(genresJSON))
	return err
}

// InsertPlayHistory records a play event. Primary key dedupes within a
// single played_at + track_id pairing.
func (s *Store) InsertPlayHistory(playedAt time.Time, trackID, trackName, contextURI, contextType string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(`INSERT OR IGNORE INTO play_history (played_at, track_id, track_name, context_uri, context_type) VALUES (?, ?, ?, ?, ?)`,
		playedAt.UTC().Format(time.RFC3339), trackID, trackName, contextURI, contextType)
	return err
}

// InsertPlaylistSnapshotTrack records one (playlist, snapshot, position) row.
func (s *Store) InsertPlaylistSnapshotTrack(playlistID, snapshotID string, capturedAt time.Time, position int, trackID, trackURI, trackName, isrc string, addedAt time.Time, addedBy string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var addedAtStr sql.NullString
	if !addedAt.IsZero() {
		addedAtStr = sql.NullString{String: addedAt.UTC().Format(time.RFC3339), Valid: true}
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO playlist_snapshot_tracks
		(playlist_id, snapshot_id, captured_at, position, track_id, track_uri, track_name, isrc, added_at, added_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		playlistID, snapshotID, capturedAt.UTC().Format(time.RFC3339), position, trackID, trackURI, trackName, isrc, addedAtStr, addedBy)
	return err
}

// InsertDeviceSeen records a device observation.
func (s *Store) InsertDeviceSeen(id, name, devType string, isActive bool, volumePercent int, lastSeenAt time.Time) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	active := 0
	if isActive {
		active = 1
	}
	_, err := s.db.Exec(`INSERT OR REPLACE INTO devices_seen (id, name, type, is_active, volume_percent, last_seen_at) VALUES (?, ?, ?, ?, ?, ?)`,
		id, name, devType, active, volumePercent, lastSeenAt.UTC().Format(time.RFC3339))
	return err
}
