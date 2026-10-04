// Copyright 2026 Jet Sng and contributors. Licensed under Apache-2.0.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/wheelog/internal/wheelog"
)

type WheelogObservation struct {
	Latest   wheelog.Spot  `json:"latest"`
	Previous *wheelog.Spot `json:"previous"`
}

func (s *Store) InitWheelog(ctx context.Context) error {
	if err := s.checkWheelogWritePath(); err != nil {
		return err
	}
	_, err := s.wheelogExec(ctx, `CREATE TABLE IF NOT EXISTS wheelog_shortlist (id INTEGER PRIMARY KEY, previous_json TEXT, latest_json TEXT NOT NULL)`)
	if err == nil {
		err = s.checkWheelogWritePath()
	}
	return err
}

func (s *Store) WheelogList(ctx context.Context) ([]WheelogObservation, error) {
	rows, err := s.wheelogQuery(ctx, `SELECT previous_json,latest_json FROM wheelog_shortlist ORDER BY id LIMIT 51`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]WheelogObservation, 0)
	for rows.Next() {
		var previous sql.NullString
		var latest string
		if err := rows.Scan(&previous, &latest); err != nil {
			return nil, err
		}
		var item WheelogObservation
		if err := json.Unmarshal([]byte(latest), &item.Latest); err != nil {
			return nil, fmt.Errorf("invalid saved WheeLog observation")
		}
		if previous.Valid {
			var old wheelog.Spot
			if err := json.Unmarshal([]byte(previous.String), &old); err != nil {
				return nil, fmt.Errorf("invalid saved WheeLog baseline")
			}
			item.Previous = &old
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(items) > wheelog.MaxSpots {
		return nil, fmt.Errorf("saved shortlist exceeds 50 spots")
	}
	return items, nil
}

// ObserveWheelog rotates normalized observations and replaces their search/index
// projections in one transaction. Missing or zero reports replace old evidence.
func (s *Store) ObserveWheelog(ctx context.Context, spot wheelog.Spot) error {
	if err := s.checkWheelogWritePath(); err != nil {
		return err
	}
	data, err := json.Marshal(spot)
	if err != nil {
		return err
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.wheelogBegin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var previous string
	err = tx.QueryRowContext(ctx, `SELECT latest_json FROM wheelog_shortlist WHERE id=?`, spot.ID).Scan(&previous)
	if err == sql.ErrNoRows {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM wheelog_shortlist`).Scan(&count); err != nil {
			return err
		}
		if count >= wheelog.MaxSpots {
			return fmt.Errorf("shortlist is limited to 50 selected public spots")
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO wheelog_shortlist(id,latest_json) VALUES (?,?)`, spot.ID, string(data))
	} else if err == nil {
		_, err = tx.ExecContext(ctx, `UPDATE wheelog_shortlist SET previous_json=?,latest_json=? WHERE id=?`, previous, string(data), spot.ID)
	}
	if err != nil {
		return err
	}
	id := strconv.FormatInt(spot.ID, 10)
	if err := s.upsertGenericResourceTx(tx, "spots", id, data); err != nil {
		return err
	}
	obj, err := DecodeJSONObject(data)
	if err != nil {
		return err
	}
	obj["created"] = nil
	obj["updated"] = nil
	obj["type"] = "spot"
	if spot.Created != nil {
		obj["created"] = *spot.Created
	}
	if spot.Updated != nil {
		obj["updated"] = *spot.Updated
	}
	if err := s.upsertSpotsTx(tx, id, obj, data); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM wheelog_shortlist`).Scan(&count); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO sync_state(resource_type,last_cursor,last_synced_at,total_count,last_attempt_complete) VALUES('spots','selected-shortlist',?,?,1) ON CONFLICT(resource_type) DO UPDATE SET last_cursor=excluded.last_cursor,last_synced_at=excluded.last_synced_at,total_count=excluded.total_count,last_attempt_complete=1`, time.Now().UTC().Format(time.RFC3339), count)
	if err != nil {
		return err
	}
	if err := s.checkWheelogWritePath(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.checkWheelogWritePath()
}

func (s *Store) RemoveWheelog(ctx context.Context, id int64) error {
	if err := s.checkWheelogWritePath(); err != nil {
		return err
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.wheelogBegin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM wheelog_shortlist WHERE id=?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM resources WHERE resource_type='spots' AND id=?`, strconv.FormatInt(id, 10)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid=?`, ftsRowID("spots", strconv.FormatInt(id, 10))); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM spots WHERE id=?`, strconv.FormatInt(id, 10)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE sync_state SET total_count=(SELECT count(*) FROM wheelog_shortlist) WHERE resource_type='spots'`); err != nil {
		return err
	}
	if err := s.checkWheelogWritePath(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.checkWheelogWritePath()
}
