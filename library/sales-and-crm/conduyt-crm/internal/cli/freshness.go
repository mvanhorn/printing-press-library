// Copyright 2026 conduyt. Licensed under Apache-2.0. See LICENSE.

// PATCH: moved from internal/cliutil/ to internal/cli/ to survive generator regen
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Decision int

const (
	DecisionFresh Decision = iota
	DecisionStaleAPI
	DecisionStaleShare
	DecisionNoStore
)

func (d Decision) String() string {
	switch d {
	case DecisionFresh:
		return "fresh"
	case DecisionStaleAPI:
		return "stale_api"
	case DecisionStaleShare:
		return "stale_share"
	case DecisionNoStore:
		return "no_store"
	default:
		return "unknown"
	}
}

type Policy struct {
	StaleAfter   time.Duration
	PerResource  map[string]time.Duration
	EnvOptOut    string
	ShareEnabled bool
}

type FreshnessMeta struct {
	Decision  string   `json:"decision"`
	Reason    string   `json:"reason,omitempty"`
	Resources []string `json:"resources,omitempty"`
	Source    string   `json:"source,omitempty"`
	Ran       bool     `json:"ran,omitempty"`
	ElapsedMS int64    `json:"elapsed_ms,omitempty"`
	Error     string   `json:"error,omitempty"`
}

const StoreSchemaVersion = 3

func EnsureFresh(ctx context.Context, db *sql.DB, resources []string, policy Policy) (Decision, error) {
	if db == nil {
		return DecisionNoStore, nil
	}
	var tableName string
	err := db.QueryRowContext(ctx, "SELECT name FROM sqlite_master WHERE type='table' AND name='sync_state'").Scan(&tableName)
	if err != nil {
		return DecisionNoStore, nil
	}

	for _, res := range resources {
		staleAfter := policy.StaleAfter
		if override, ok := policy.PerResource[res]; ok {
			staleAfter = override
		}
		var lastSynced sql.NullString
		err := db.QueryRowContext(ctx,
			"SELECT last_synced_at FROM sync_state WHERE resource_type = ?", res,
		).Scan(&lastSynced)
		if err != nil || !lastSynced.Valid {
			return DecisionStaleAPI, nil
		}
		ts, err := time.Parse(time.RFC3339, lastSynced.String)
		if err != nil {
			return DecisionStaleAPI, fmt.Errorf("parse last_synced_at for %s: %w", res, err)
		}
		if time.Since(ts) > staleAfter {
			return DecisionStaleAPI, nil
		}
	}
	return DecisionFresh, nil
}

