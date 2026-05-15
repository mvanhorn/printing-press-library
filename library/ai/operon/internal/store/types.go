// Copyright 2026 yaooooooooooooooo. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel store — not generated.

package store

// DemandEntry mirrors the public projection returned by GET /demand. The
// store carries first_seen_at / last_seen_at columns so freshness queries
// (demand stale, demand health) can run without contacting the API.
type DemandEntry struct {
	ID          string
	Service     string
	ServiceType string
	Category    string
	Description string
	Domain      string
	Assets      []string
	Type        string
	FirstSeenAt int64 // unix ms
	LastSeenAt  int64 // unix ms
}

// Placement captures a single auction round end-to-end: the request that
// triggered it, the decision, the winner's commercial terms, and the full
// auction ranking object so `auction explain` and `placement replay` can
// reconstruct context without re-querying the API.
type Placement struct {
	ID                 string
	RequestContextJSON string
	ResponseDecision   string
	ResponseReason     string
	WinnerAdvertiserID string
	WinnerService      string
	ScoutScore         *float64
	BidPrice           *int64
	PlacementType      string
	AuctionJSON        string
	MetaJSON           string
	CreatedAt          int64
}

// ScoutScorePoint is one row of trust history for a single advertiser.
type ScoutScorePoint struct {
	AdvertiserID string
	TrustScore   *float64
	ObservedAt   int64
}

// CampaignLocal mirrors GET /x402/campaign/{id} for offline inspection.
// BearerToken is stored locally only — the API never returns it on read.
type CampaignLocal struct {
	CampaignID       string
	Service          string
	Category         string
	Status           string
	BalanceUSDC      *float64
	BalanceSpentUSDC *float64
	TrustScore       *float64
	X402PayerWallet  string
	BearerToken      string
	CreatedAt        *int64
	UpdatedAt        *int64
	LastSyncedAt     int64
}
