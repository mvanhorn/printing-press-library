// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored shared helpers for MakerWorld novel commands (discover, movers,
// designers deltas). Reads the locally synced `designs` mirror and maintains a
// per-sync snapshot table so cross-sync deltas can be computed offline.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/makerworld/internal/store"
)

// designRow is the subset of a synced MakerWorld design used by the novel
// local-analytics commands. Synced list rows do NOT carry per-instance data
// (AMS/weight), so those filters enrich live in discover.
type designRow struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Slug        string  `json:"slug"`
	CreatorID   string  `json:"creator_id"`
	CreatorName string  `json:"creator_name"`
	Like        int     `json:"like_count"`
	Collection  int     `json:"collection_count"`
	Print       int     `json:"print_count"`
	Download    int     `json:"download_count"`
	Comment     int     `json:"comment_count"`
	DesignScore float64 `json:"design_score"`
	HotScore    float64 `json:"hot_score"`
	StaffPicked bool    `json:"staff_picked"`
	Printable   bool    `json:"printable"`
	NSFW        bool    `json:"nsfw"`
	CreateTime  string  `json:"create_time"`
}

// designEnvelope mirrors the raw MakerWorld JSON stored in the resources table.
type designEnvelope struct {
	ID              json.Number `json:"id"`
	Title           string      `json:"title"`
	Slug            string      `json:"slug"`
	LikeCount       int         `json:"likeCount"`
	CollectionCount int         `json:"collectionCount"`
	PrintCount      int         `json:"printCount"`
	DownloadCount   int         `json:"downloadCount"`
	CommentCount    int         `json:"commentCount"`
	DesignScore     float64     `json:"designScore"`
	HotScore        float64     `json:"hotScore"`
	IsStaffPicked   bool        `json:"isStaffPicked"`
	IsPrintable     bool        `json:"is_printable"`
	NSFW            bool        `json:"nsfw"`
	CreateTime      string      `json:"createTime"`
	DesignCreator   struct {
		UID  json.Number `json:"uid"`
		Name string      `json:"name"`
	} `json:"designCreator"`
}

// parseDesignRow decodes a stored design JSON blob into a designRow. Returns
// ok=false when the blob is unparseable or carries no usable id.
func parseDesignRow(data []byte) (designRow, bool) {
	var e designEnvelope
	if err := json.Unmarshal(data, &e); err != nil {
		return designRow{}, false
	}
	id := e.ID.String()
	if id == "" || id == "0" {
		return designRow{}, false
	}
	return designRow{
		ID:          id,
		Title:       e.Title,
		Slug:        e.Slug,
		CreatorID:   e.DesignCreator.UID.String(),
		CreatorName: e.DesignCreator.Name,
		Like:        e.LikeCount,
		Collection:  e.CollectionCount,
		Print:       e.PrintCount,
		Download:    e.DownloadCount,
		Comment:     e.CommentCount,
		DesignScore: e.DesignScore,
		HotScore:    e.HotScore,
		StaffPicked: e.IsStaffPicked,
		Printable:   e.IsPrintable,
		NSFW:        e.NSFW,
		CreateTime:  e.CreateTime,
	}, true
}

// loadDesignRows reads every synced design from the resources mirror.
func loadDesignRows(ctx context.Context, sqlDB *sql.DB) ([]designRow, error) {
	rows, err := sqlDB.QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'designs'`)
	if err != nil {
		return nil, fmt.Errorf("reading designs mirror: %w", err)
	}
	defer rows.Close()
	var out []designRow
	for rows.Next() {
		var raw sql.NullString
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scanning design row: %w", err)
		}
		if !raw.Valid {
			continue // NULL payload — skip, not an error
		}
		if dr, ok := parseDesignRow([]byte(raw.String)); ok {
			out = append(out, dr)
		}
	}
	return out, rows.Err()
}

// toSnapshotRows converts mirror rows into store snapshot rows. The
// design_snapshots table and its serialized writer (Store.RecordDesignSnapshots)
// live in the store package, created under the migration lock; novel commands no
// longer create the table or write to the raw handle themselves.
func toSnapshotRows(rows []designRow) []store.SnapshotRow {
	out := make([]store.SnapshotRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, store.SnapshotRow{
			DesignID:    r.ID,
			Title:       r.Title,
			CreatorID:   r.CreatorID,
			CreatorName: r.CreatorName,
			Like:        r.Like,
			Download:    r.Download,
			Print:       r.Print,
			Collection:  r.Collection,
			Comment:     r.Comment,
		})
	}
	return out
}

// latestTwoSnapshots returns the two most recent distinct snapshot sync
// timestamps (newest first). Fewer than two means deltas cannot be computed yet.
func latestTwoSnapshots(ctx context.Context, sqlDB *sql.DB) (current, previous string, err error) {
	rows, err := sqlDB.QueryContext(ctx, `SELECT DISTINCT sync_at FROM design_snapshots ORDER BY sync_at DESC LIMIT 2`)
	if err != nil {
		return "", "", err
	}
	defer rows.Close()
	var stamps []string
	for rows.Next() {
		var s sql.NullString
		if err := rows.Scan(&s); err != nil {
			return "", "", fmt.Errorf("scanning snapshot timestamp: %w", err)
		}
		if s.Valid {
			stamps = append(stamps, s.String)
		}
	}
	if err := rows.Err(); err != nil {
		return "", "", err
	}
	switch len(stamps) {
	case 0:
		return "", "", nil
	case 1:
		return stamps[0], "", nil
	default:
		return stamps[0], stamps[1], nil
	}
}

// snapshotMetrics holds the counts captured for one design at one sync.
type snapshotMetrics struct {
	Title       string
	CreatorID   string
	CreatorName string
	Like        int
	Download    int
	Print       int
	Collection  int
}

// loadSnapshot reads one snapshot batch keyed by design_id.
func loadSnapshot(ctx context.Context, sqlDB *sql.DB, syncAt string) (map[string]snapshotMetrics, error) {
	rows, err := sqlDB.QueryContext(ctx, `SELECT design_id, title, creator_id, creator_name,
		like_count, download_count, print_count, collection_count
		FROM design_snapshots WHERE sync_at = ?`, syncAt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]snapshotMetrics)
	for rows.Next() {
		var id sql.NullString
		var m snapshotMetrics
		var title, cid, cname sql.NullString
		if err := rows.Scan(&id, &title, &cid, &cname, &m.Like, &m.Download, &m.Print, &m.Collection); err != nil {
			return nil, fmt.Errorf("scanning snapshot row: %w", err)
		}
		if !id.Valid {
			continue
		}
		m.Title = title.String
		m.CreatorID = cid.String
		m.CreatorName = cname.String
		out[id.String] = m
	}
	return out, rows.Err()
}

// metricValue selects a snapshot count by metric name.
func metricValue(m snapshotMetrics, metric string) int {
	switch metric {
	case "likes", "like":
		return m.Like
	case "prints", "print":
		return m.Print
	case "collections", "collection", "saves":
		return m.Collection
	default: // downloads
		return m.Download
	}
}

// computeMovers ranks designs present in both snapshots by the largest positive
// delta in the chosen metric. Pure function over snapshot maps (no IO).
func computeMovers(cur, prev map[string]snapshotMetrics, metric string, limit int) []moverItem {
	movers := make([]moverItem, 0, len(cur))
	for id, c := range cur {
		p, ok := prev[id]
		if !ok {
			continue // new since last snapshot — surfaced by designers deltas
		}
		delta := metricValue(c, metric) - metricValue(p, metric)
		if delta <= 0 {
			continue
		}
		movers = append(movers, moverItem{
			ID:       id,
			Title:    c.Title,
			Creator:  c.CreatorName,
			Metric:   metric,
			Current:  metricValue(c, metric),
			Previous: metricValue(p, metric),
			Delta:    delta,
			URL:      "https://makerworld.com/en/models/" + id,
		})
	}
	sort.SliceStable(movers, func(i, j int) bool {
		return movers[i].Delta > movers[j].Delta
	})
	if limit > 0 && len(movers) > limit {
		movers = movers[:limit]
	}
	return movers
}

// aggregateDesignerDeltas rolls up per-designer new uploads and engagement
// deltas between two snapshots. Pure function over snapshot maps (no IO).
func aggregateDesignerDeltas(cur, prev map[string]snapshotMetrics, limit int) []designerDelta {
	agg := make(map[string]*designerDelta)
	get := func(id, name string) *designerDelta {
		d, ok := agg[id]
		if !ok {
			d = &designerDelta{CreatorID: id, Creator: name}
			agg[id] = d
		}
		if d.Creator == "" {
			d.Creator = name
		}
		return d
	}
	for id, c := range cur {
		if c.CreatorID == "" {
			continue
		}
		d := get(c.CreatorID, c.CreatorName)
		p, ok := prev[id]
		if !ok {
			d.NewDesigns++
			d.NewDesignIDs = append(d.NewDesignIDs, id)
			continue
		}
		d.LikeDelta += c.Like - p.Like
		d.DownloadDelta += c.Download - p.Download
	}
	deltas := make([]designerDelta, 0, len(agg))
	for _, d := range agg {
		if d.NewDesigns == 0 && d.LikeDelta == 0 && d.DownloadDelta == 0 {
			continue
		}
		sort.Strings(d.NewDesignIDs)
		deltas = append(deltas, *d)
	}
	sort.SliceStable(deltas, func(i, j int) bool {
		if deltas[i].NewDesigns != deltas[j].NewDesigns {
			return deltas[i].NewDesigns > deltas[j].NewDesigns
		}
		return deltas[i].DownloadDelta > deltas[j].DownloadDelta
	})
	if limit > 0 && len(deltas) > limit {
		deltas = deltas[:limit]
	}
	return deltas
}
