// Hand-authored helpers for the snapshot-backed novel commands (momentum,
// narrative). Each run captures the current heatmap into topic_snapshots so the
// time dimension the live API cannot serve accumulates locally.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/techtwitter/internal/store"
)

// ttSnapshotMinInterval throttles snapshot writes so rapid re-runs (and the
// momentum+narrative pair) share one baseline instead of flooding the table.
const ttSnapshotMinInterval = time.Hour

// ttCurrentTopics returns the current heatmap topics, preferring a live fetch
// and falling back to the synced `command` table.
func ttCurrentTopics(ctx context.Context, flags *rootFlags, db *store.Store) ([]ttTopic, string) {
	if flags.dataSource != "local" {
		if c, err := flags.newClient(); err == nil {
			if data, err := c.Get(ctx, "/api/command/heatmap", nil); err == nil {
				var env struct {
					Topics []ttTopic `json:"topics"`
				}
				if json.Unmarshal(data, &env) == nil && len(env.Topics) > 0 {
					return env.Topics, "live"
				}
			}
		}
	}
	rows, err := db.DB().QueryContext(ctx,
		`SELECT keyword, COALESCE(slug,''), COALESCE(count,0), COALESCE(engagement,0)
		 FROM command WHERE keyword IS NOT NULL ORDER BY engagement DESC`)
	if err != nil {
		return nil, "local"
	}
	defer rows.Close()
	out := make([]ttTopic, 0)
	for rows.Next() {
		var t ttTopic
		if err := rows.Scan(&t.Keyword, &t.Slug, &t.Count, &t.Engagement); err != nil {
			return out, "local"
		}
		out = append(out, t)
	}
	if err := rows.Err(); err != nil {
		// Discard a partial result so a mid-iteration cursor error degrades to
		// "no topics available" rather than a misleading partial snapshot.
		return nil, "local"
	}
	return out, "local"
}

// ttLatestSnapshotTime returns the most recent captured_at, or "" if none.
func ttLatestSnapshotTime(db *sql.DB) string {
	var s sql.NullString
	_ = db.QueryRow(`SELECT MAX(captured_at) FROM topic_snapshots`).Scan(&s)
	return s.String
}

// ttCaptureSnapshot records a new snapshot unless one was taken within
// ttSnapshotMinInterval. Returns the captured_at to treat as "current".
func ttCaptureSnapshot(db *sql.DB, topics []ttTopic) (string, error) {
	latest := ttLatestSnapshotTime(db)
	if latest != "" {
		if ts, err := time.Parse(time.RFC3339, latest); err == nil && time.Since(ts) < ttSnapshotMinInterval {
			return latest, nil
		}
	}
	if len(topics) == 0 {
		if latest != "" {
			return latest, nil
		}
		return "", nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := db.Begin()
	if err != nil {
		return "", err
	}
	stmt, err := tx.Prepare(`INSERT INTO topic_snapshots (captured_at, keyword, slug, count, engagement) VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}
	for _, t := range topics {
		if _, err := stmt.Exec(now, t.Keyword, t.Slug, t.Count, t.Engagement); err != nil {
			_ = stmt.Close()
			_ = tx.Rollback()
			return "", err
		}
	}
	_ = stmt.Close()
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return now, nil
}

// ttDistinctSnapshotTimes returns distinct captured_at values, newest first.
func ttDistinctSnapshotTimes(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT captured_at FROM topic_snapshots ORDER BY captured_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]string, 0)
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ttSnapshotTopics loads a snapshot keyed by keyword.
func ttSnapshotTopics(db *sql.DB, capturedAt string) (map[string]ttTopic, error) {
	rows, err := db.Query(`SELECT keyword, COALESCE(slug,''), COALESCE(count,0), COALESCE(engagement,0)
		FROM topic_snapshots WHERE captured_at = ?`, capturedAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]ttTopic{}
	for rows.Next() {
		var t ttTopic
		if err := rows.Scan(&t.Keyword, &t.Slug, &t.Count, &t.Engagement); err != nil {
			return nil, err
		}
		out[t.Keyword] = t
	}
	return out, rows.Err()
}

// ttPriorSnapshotWithin returns the oldest snapshot time within the window that
// precedes `latest`, falling back to the immediately-previous snapshot. Returns
// "" when there is no prior snapshot to compare against.
func ttPriorSnapshotWithin(times []string, latest, cutoff string) string {
	prior := ""
	for _, t := range times {
		if t == latest {
			continue
		}
		// times is DESC; the first entry within the window is the most recent
		// prior, but we want the oldest within-window point for window momentum.
		if t >= cutoff {
			prior = t // keep walking; later (older) within-window points overwrite
		} else if prior == "" {
			prior = t // nothing within window; use the most recent older one
			break
		}
	}
	return prior
}
