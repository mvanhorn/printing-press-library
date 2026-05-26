// Copyright 2026 justinwfu. Licensed under Apache-2.0. See LICENSE.

package cliutil

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"
)

// Decision is the outcome of a freshness check. It tells the caller what
// to do next without doing the refresh itself — EnsureFresh is pure
// decision logic so callers can compose it with their own refresh path.
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
		return "stale-api"
	case DecisionStaleShare:
		return "stale-share"
	case DecisionNoStore:
		return "no-store"
	default:
		return "unknown"
	}
}

// FreshnessMeta is the stable agent-facing metadata shape for commands that
// participate in machine-owned freshness. Generated JSON provenance envelopes
// attach this under meta.freshness.
type FreshnessMeta struct {
	Decision  string   `json:"decision"`
	Ran       bool     `json:"ran"`
	Reason    string   `json:"reason,omitempty"`
	Resources []string `json:"resources,omitempty"`
	ElapsedMS int64    `json:"elapsed_ms,omitempty"`
	Error     string   `json:"error,omitempty"`
	Source    string   `json:"source,omitempty"`
}

// Policy configures a freshness check. StaleAfter is the default threshold
// applied to any resource not listed in PerResource.
type Policy struct {
	StaleAfter   time.Duration
	PerResource  map[string]time.Duration
	EnvOptOut    string
	ShareEnabled bool
}

// EnsureFresh inspects sync_state for the requested resources and returns
// a Decision. A resource with no sync_state row or NULL last_synced_at is
// treated as stale; the worst-case decision across resources wins.
func EnsureFresh(ctx context.Context, db *sql.DB, resources []string, policy Policy) (Decision, error) {
	if policy.EnvOptOut != "" && os.Getenv(policy.EnvOptOut) == "1" {
		return DecisionFresh, nil
	}
	if db == nil {
		return DecisionNoStore, nil
	}
	if len(resources) == 0 {
		return DecisionFresh, nil
	}

	placeholders := make([]string, len(resources))
	args := make([]any, len(resources))
	for i, r := range resources {
		placeholders[i] = "?"
		args[i] = r
	}
	query := fmt.Sprintf(
		`SELECT resource_type, last_synced_at FROM sync_state WHERE resource_type IN (%s)`,
		strings.Join(placeholders, ","),
	)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return DecisionNoStore, nil
		}
		return DecisionFresh, fmt.Errorf("query sync_state: %w", err)
	}
	defer rows.Close()

	seen := make(map[string]bool, len(resources))
	anyStale := false
	for rows.Next() {
		var rtype string
		var lastSynced sql.NullTime
		if err := rows.Scan(&rtype, &lastSynced); err != nil {
			return DecisionFresh, fmt.Errorf("scan sync_state row: %w", err)
		}
		seen[rtype] = true
		if !lastSynced.Valid {
			anyStale = true
			continue
		}
		threshold := policy.StaleAfter
		if policy.PerResource != nil {
			if override, ok := policy.PerResource[rtype]; ok {
				threshold = override
			}
		}
		if threshold <= 0 {
			continue
		}
		if time.Since(lastSynced.Time) > threshold {
			anyStale = true
		}
	}
	if err := rows.Err(); err != nil {
		return DecisionFresh, fmt.Errorf("iterate sync_state rows: %w", err)
	}

	for _, r := range resources {
		if !seen[r] {
			anyStale = true
			break
		}
	}

	if !anyStale {
		return DecisionFresh, nil
	}
	if policy.ShareEnabled {
		return DecisionStaleShare, nil
	}
	return DecisionStaleAPI, nil
}
