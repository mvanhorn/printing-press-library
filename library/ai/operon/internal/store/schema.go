// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

// Schema is applied verbatim on Open() via a single Exec call. All statements
// are idempotent (IF NOT EXISTS) so re-running over an existing DB is a no-op.
const Schema = `
CREATE TABLE IF NOT EXISTS demand_entries (
  id TEXT PRIMARY KEY,
  service TEXT NOT NULL,
  service_type TEXT NOT NULL,
  category TEXT NOT NULL,
  description TEXT,
  domain TEXT,
  assets_json TEXT,
  type TEXT NOT NULL,
  first_seen_at INTEGER NOT NULL,
  last_seen_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_demand_category ON demand_entries(category);
CREATE INDEX IF NOT EXISTS idx_demand_last_seen ON demand_entries(last_seen_at);
CREATE VIRTUAL TABLE IF NOT EXISTS demand_entries_fts USING fts5(service, description, domain, content='demand_entries', content_rowid='rowid');

CREATE TABLE IF NOT EXISTS placements (
  id TEXT PRIMARY KEY,
  request_context_json TEXT NOT NULL,
  response_decision TEXT NOT NULL,
  response_reason TEXT,
  winner_advertiser_id TEXT,
  winner_service TEXT,
  scout_score REAL,
  bid_price INTEGER,
  placement_type TEXT,
  auction_json TEXT,
  meta_json TEXT,
  created_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_placements_created ON placements(created_at);
CREATE INDEX IF NOT EXISTS idx_placements_winner ON placements(winner_advertiser_id);

CREATE TABLE IF NOT EXISTS scoutscore_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  advertiser_id TEXT NOT NULL,
  trust_score REAL,
  observed_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_scoutscore_adv ON scoutscore_history(advertiser_id);
CREATE INDEX IF NOT EXISTS idx_scoutscore_observed ON scoutscore_history(observed_at);

CREATE TABLE IF NOT EXISTS campaigns_local (
  campaign_id TEXT PRIMARY KEY,
  service TEXT,
  category TEXT,
  status TEXT,
  balance_usdc REAL,
  balance_spent_usdc REAL,
  trust_score REAL,
  x402_payer_wallet TEXT,
  bearer_token TEXT,
  created_at INTEGER,
  updated_at INTEGER,
  last_synced_at INTEGER
);
CREATE INDEX IF NOT EXISTS idx_campaigns_wallet ON campaigns_local(x402_payer_wallet);
CREATE INDEX IF NOT EXISTS idx_campaigns_category ON campaigns_local(category);

CREATE TABLE IF NOT EXISTS sync_state (
  table_name TEXT PRIMARY KEY,
  last_synced_at INTEGER NOT NULL,
  rows_synced INTEGER NOT NULL DEFAULT 0
);
`
