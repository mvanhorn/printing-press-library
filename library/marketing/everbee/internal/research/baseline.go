// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package research

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrNoBaseline means no snapshot has been saved for this seed yet. It is a
// normal first-run state, not a failure, and callers say so plainly.
var ErrNoBaseline = errors.New("no baseline saved for this seed")

// Baseline is a saved point-in-time snapshot of a niche, used to answer the one
// question EverBee cannot: what changed since last time? EverBee exposes no
// history at all, so week-over-week movement is only knowable locally.
type Baseline struct {
	Seed        string      `json:"seed"`
	SavedAt     time.Time   `json:"saved_at"`
	Demand      int         `json:"demand"`
	Competition int         `json:"competition"`
	Opportunity float64     `json:"opportunity_score"`
	Confidence  float64     `json:"confidence"`
	PriceBand   PriceBand   `json:"price_band"`
	Evidence    EvidenceSet `json:"evidence"`
}

// EnsureSchema creates the baseline table. It is safe to call on every command
// and is the only place this package writes DDL.
func EnsureSchema(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS research_baselines (
    seed        TEXT PRIMARY KEY,
    saved_at    TEXT NOT NULL,
    payload     TEXT NOT NULL
)`)
	if err != nil {
		return fmt.Errorf("creating research_baselines table: %w", err)
	}
	return nil
}

// SaveBaseline records the current verdict as the comparison point for future
// drift runs, replacing any previous snapshot for the same seed.
func SaveBaseline(ctx context.Context, db *sql.DB, v *Verdict) error {
	if err := EnsureSchema(ctx, db); err != nil {
		return err
	}
	b := Baseline{
		Seed:        v.Seed,
		SavedAt:     Now().UTC(),
		Demand:      v.Demand,
		Competition: v.Competition,
		Opportunity: v.Opportunity,
		Confidence:  v.Confidence,
		PriceBand:   v.PriceBand,
		Evidence:    v.Evidence,
	}
	payload, err := json.Marshal(b)
	if err != nil {
		return fmt.Errorf("encoding baseline: %w", err)
	}
	_, err = db.ExecContext(ctx,
		`INSERT INTO research_baselines (seed, saved_at, payload) VALUES (?, ?, ?)
		 ON CONFLICT(seed) DO UPDATE SET saved_at = excluded.saved_at, payload = excluded.payload`,
		b.Seed, b.SavedAt.Format(time.RFC3339), string(payload))
	if err != nil {
		return fmt.Errorf("saving baseline for %q: %w", v.Seed, err)
	}
	return nil
}

// LoadBaseline reads the saved snapshot for a seed. Returns ErrNoBaseline when
// none exists so the caller can say "save one first" rather than inventing a
// zero-valued comparison.
func LoadBaseline(ctx context.Context, db *sql.DB, seed string) (*Baseline, error) {
	if err := EnsureSchema(ctx, db); err != nil {
		return nil, err
	}
	var payload string
	err := db.QueryRowContext(ctx, `SELECT payload FROM research_baselines WHERE seed = ?`, seed).Scan(&payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoBaseline
	}
	if err != nil {
		return nil, fmt.Errorf("loading baseline for %q: %w", seed, err)
	}
	var b Baseline
	if err := json.Unmarshal([]byte(payload), &b); err != nil {
		return nil, fmt.Errorf("decoding baseline for %q: %w", seed, err)
	}
	return &b, nil
}

// Drift is the movement of a niche between a saved baseline and now. Both
// timestamps travel with it so the window being compared is never ambiguous.
type Drift struct {
	Seed             string       `json:"seed"`
	BaselineAt       time.Time    `json:"baseline_at"`
	CurrentAt        time.Time    `json:"current_at"`
	WindowHours      float64      `json:"window_hours"`
	DemandDelta      int          `json:"demand_delta"`
	CompetitionDelta int          `json:"competition_delta"`
	OpportunityDelta float64      `json:"opportunity_delta"`
	MedianPriceDelta float64      `json:"median_price_delta"`
	Baseline         Baseline     `json:"baseline"`
	Current          Baseline     `json:"current"`
	Warnings         []string     `json:"warnings"`
	Provenance       []Provenance `json:"provenance"`
}

// DiffBaseline computes the movement between a saved baseline and a fresh verdict.
func DiffBaseline(b *Baseline, v *Verdict) *Drift {
	now := Now().UTC()
	cur := Baseline{
		Seed:        v.Seed,
		SavedAt:     now,
		Demand:      v.Demand,
		Competition: v.Competition,
		Opportunity: v.Opportunity,
		Confidence:  v.Confidence,
		PriceBand:   v.PriceBand,
		Evidence:    v.Evidence,
	}
	d := &Drift{
		Seed:             v.Seed,
		BaselineAt:       b.SavedAt,
		CurrentAt:        now,
		WindowHours:      round2(now.Sub(b.SavedAt).Hours()),
		DemandDelta:      v.Demand - b.Demand,
		CompetitionDelta: v.Competition - b.Competition,
		OpportunityDelta: round2(v.Opportunity - b.Opportunity),
		MedianPriceDelta: round2(v.PriceBand.Median - b.PriceBand.Median),
		Baseline:         *b,
		Current:          cur,
		Warnings:         []string{},
		Provenance:       v.Provenance,
	}
	// A drift computed against thin evidence on either side is not trustworthy,
	// and saying so is cheaper than a caller acting on noise.
	if b.Evidence.TotalEvidence() == 0 || v.Evidence.TotalEvidence() == 0 {
		d.Warnings = append(d.Warnings,
			"one side of this comparison has no relevant evidence; the deltas describe missing data, not market movement.")
	}
	if d.WindowHours < 1 {
		d.Warnings = append(d.Warnings,
			"the baseline is less than an hour old; movement over this window is unlikely to be meaningful.")
	}
	return d
}
