// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// UpsertCampaign inserts or replaces a campaign row by campaign_id.
func (s *Store) UpsertCampaign(ctx context.Context, c CampaignLocal) error {
	if c.CampaignID == "" {
		return errors.New("store: campaign id required")
	}
	if c.LastSyncedAt == 0 {
		c.LastSyncedAt = time.Now().UnixMilli()
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO campaigns_local
		  (campaign_id, service, category, status, balance_usdc, balance_spent_usdc,
		   trust_score, x402_payer_wallet, bearer_token, created_at, updated_at, last_synced_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(campaign_id) DO UPDATE SET
		  service             = excluded.service,
		  category            = excluded.category,
		  status              = excluded.status,
		  balance_usdc        = excluded.balance_usdc,
		  balance_spent_usdc  = excluded.balance_spent_usdc,
		  trust_score         = excluded.trust_score,
		  x402_payer_wallet   = excluded.x402_payer_wallet,
		  bearer_token        = COALESCE(excluded.bearer_token, campaigns_local.bearer_token),
		  created_at          = COALESCE(campaigns_local.created_at, excluded.created_at),
		  updated_at          = excluded.updated_at,
		  last_synced_at      = excluded.last_synced_at`,
		c.CampaignID, c.Service, c.Category, c.Status, c.BalanceUSDC, c.BalanceSpentUSDC,
		c.TrustScore, c.X402PayerWallet, c.BearerToken, c.CreatedAt, c.UpdatedAt, c.LastSyncedAt)
	if err != nil {
		return fmt.Errorf("store: upsert campaign: %w", err)
	}
	return nil
}

// ListCampaigns returns all locally mirrored campaigns ordered by
// last_synced_at desc.
func (s *Store) ListCampaigns(ctx context.Context) ([]CampaignLocal, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT campaign_id, service, category, status, balance_usdc, balance_spent_usdc,
		       trust_score, x402_payer_wallet, bearer_token, created_at, updated_at, last_synced_at
		  FROM campaigns_local ORDER BY last_synced_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("store: list campaigns: %w", err)
	}
	defer rows.Close()
	return scanCampaignRows(rows)
}

// GetCampaign returns a campaign by id or sql.ErrNoRows.
func (s *Store) GetCampaign(ctx context.Context, id string) (*CampaignLocal, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT campaign_id, service, category, status, balance_usdc, balance_spent_usdc,
		       trust_score, x402_payer_wallet, bearer_token, created_at, updated_at, last_synced_at
		  FROM campaigns_local WHERE campaign_id=?`, id)
	c, err := scanOneCampaign(row)
	return c, err
}

// GroupByWallet buckets campaigns by their x402 payer wallet so a publisher
// can see which advertisers share funding. Campaigns with an empty wallet
// are grouped under "" so the caller can surface them as "unknown wallet".
func (s *Store) GroupByWallet(ctx context.Context) (map[string][]CampaignLocal, error) {
	all, err := s.ListCampaigns(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]CampaignLocal)
	for _, c := range all {
		out[c.X402PayerWallet] = append(out[c.X402PayerWallet], c)
	}
	return out, nil
}

type campaignScanner interface {
	Scan(dest ...any) error
}

func scanOneCampaign(s campaignScanner) (*CampaignLocal, error) {
	var (
		c        CampaignLocal
		service  sql.NullString
		category sql.NullString
		status   sql.NullString
		bal      sql.NullFloat64
		spent    sql.NullFloat64
		trust    sql.NullFloat64
		wallet   sql.NullString
		bearer   sql.NullString
		created  sql.NullInt64
		updated  sql.NullInt64
	)
	if err := s.Scan(&c.CampaignID, &service, &category, &status, &bal, &spent,
		&trust, &wallet, &bearer, &created, &updated, &c.LastSyncedAt); err != nil {
		return nil, err
	}
	if service.Valid {
		c.Service = service.String
	}
	if category.Valid {
		c.Category = category.String
	}
	if status.Valid {
		c.Status = status.String
	}
	if bal.Valid {
		v := bal.Float64
		c.BalanceUSDC = &v
	}
	if spent.Valid {
		v := spent.Float64
		c.BalanceSpentUSDC = &v
	}
	if trust.Valid {
		v := trust.Float64
		c.TrustScore = &v
	}
	if wallet.Valid {
		c.X402PayerWallet = wallet.String
	}
	if bearer.Valid {
		c.BearerToken = bearer.String
	}
	if created.Valid {
		v := created.Int64
		c.CreatedAt = &v
	}
	if updated.Valid {
		v := updated.Int64
		c.UpdatedAt = &v
	}
	return &c, nil
}

func scanCampaignRows(rows *sql.Rows) ([]CampaignLocal, error) {
	var out []CampaignLocal
	for rows.Next() {
		c, err := scanOneCampaign(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}
