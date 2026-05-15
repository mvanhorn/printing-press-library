// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// UpsertPlacement inserts or replaces a placement row by id.
func (s *Store) UpsertPlacement(ctx context.Context, p Placement) error {
	if p.ID == "" {
		return errors.New("store: placement id required")
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO placements
		  (id, request_context_json, response_decision, response_reason,
		   winner_advertiser_id, winner_service, scout_score, bid_price,
		   placement_type, auction_json, meta_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
		  request_context_json = excluded.request_context_json,
		  response_decision    = excluded.response_decision,
		  response_reason      = excluded.response_reason,
		  winner_advertiser_id = excluded.winner_advertiser_id,
		  winner_service       = excluded.winner_service,
		  scout_score          = excluded.scout_score,
		  bid_price            = excluded.bid_price,
		  placement_type       = excluded.placement_type,
		  auction_json         = excluded.auction_json,
		  meta_json            = excluded.meta_json`,
		p.ID, p.RequestContextJSON, p.ResponseDecision, p.ResponseReason,
		p.WinnerAdvertiserID, p.WinnerService, p.ScoutScore, p.BidPrice,
		p.PlacementType, p.AuctionJSON, p.MetaJSON, p.CreatedAt)
	if err != nil {
		return fmt.Errorf("store: upsert placement: %w", err)
	}
	return nil
}

// GetPlacement returns a single placement or sql.ErrNoRows.
func (s *Store) GetPlacement(ctx context.Context, id string) (*Placement, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, request_context_json, response_decision, response_reason,
		       winner_advertiser_id, winner_service, scout_score, bid_price,
		       placement_type, auction_json, meta_json, created_at
		  FROM placements WHERE id=?`, id)
	p := Placement{}
	var (
		reason  sql.NullString
		winAdv  sql.NullString
		winSvc  sql.NullString
		score   sql.NullFloat64
		bid     sql.NullInt64
		ptype   sql.NullString
		auction sql.NullString
		meta    sql.NullString
	)
	if err := row.Scan(&p.ID, &p.RequestContextJSON, &p.ResponseDecision, &reason,
		&winAdv, &winSvc, &score, &bid, &ptype, &auction, &meta, &p.CreatedAt); err != nil {
		return nil, err
	}
	if reason.Valid {
		p.ResponseReason = reason.String
	}
	if winAdv.Valid {
		p.WinnerAdvertiserID = winAdv.String
	}
	if winSvc.Valid {
		p.WinnerService = winSvc.String
	}
	if score.Valid {
		v := score.Float64
		p.ScoutScore = &v
	}
	if bid.Valid {
		v := bid.Int64
		p.BidPrice = &v
	}
	if ptype.Valid {
		p.PlacementType = ptype.String
	}
	if auction.Valid {
		p.AuctionJSON = auction.String
	}
	if meta.Valid {
		p.MetaJSON = meta.String
	}
	return &p, nil
}

// ListRecentPlacements returns the N most recent placements, newest first.
func (s *Store) ListRecentPlacements(ctx context.Context, limit int) ([]Placement, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, request_context_json, response_decision, response_reason,
		       winner_advertiser_id, winner_service, scout_score, bid_price,
		       placement_type, auction_json, meta_json, created_at
		  FROM placements
		 ORDER BY created_at DESC
		 LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store: list placements: %w", err)
	}
	defer rows.Close()
	return scanPlacementRows(rows)
}

// WatchPlacements returns placements created after sinceMs, oldest first so
// the caller can stream them in chronological order.
func (s *Store) WatchPlacements(ctx context.Context, sinceMs int64) ([]Placement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, request_context_json, response_decision, response_reason,
		       winner_advertiser_id, winner_service, scout_score, bid_price,
		       placement_type, auction_json, meta_json, created_at
		  FROM placements
		 WHERE created_at >= ?
		 ORDER BY created_at ASC`, sinceMs)
	if err != nil {
		return nil, fmt.Errorf("store: watch placements: %w", err)
	}
	defer rows.Close()
	return scanPlacementRows(rows)
}

func scanPlacementRows(rows *sql.Rows) ([]Placement, error) {
	var out []Placement
	for rows.Next() {
		p := Placement{}
		var (
			reason  sql.NullString
			winAdv  sql.NullString
			winSvc  sql.NullString
			score   sql.NullFloat64
			bid     sql.NullInt64
			ptype   sql.NullString
			auction sql.NullString
			meta    sql.NullString
		)
		if err := rows.Scan(&p.ID, &p.RequestContextJSON, &p.ResponseDecision, &reason,
			&winAdv, &winSvc, &score, &bid, &ptype, &auction, &meta, &p.CreatedAt); err != nil {
			return nil, err
		}
		if reason.Valid {
			p.ResponseReason = reason.String
		}
		if winAdv.Valid {
			p.WinnerAdvertiserID = winAdv.String
		}
		if winSvc.Valid {
			p.WinnerService = winSvc.String
		}
		if score.Valid {
			v := score.Float64
			p.ScoutScore = &v
		}
		if bid.Valid {
			v := bid.Int64
			p.BidPrice = &v
		}
		if ptype.Valid {
			p.PlacementType = ptype.String
		}
		if auction.Valid {
			p.AuctionJSON = auction.String
		}
		if meta.Valid {
			p.MetaJSON = meta.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
