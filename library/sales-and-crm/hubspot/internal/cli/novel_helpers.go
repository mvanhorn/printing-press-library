// Copyright 2026 sdhilip200. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/hubspot/internal/store"
	"math"
	"sort"
	"strings"
	"time"
)

// parseSinceFlag accepts both a duration shorthand ("90d", "1w", "36h") and
// a calendar date ("2026-01-01") and returns the corresponding lookback
// duration measured from `now`. Empty input returns a zero duration with
// ok=true so callers can treat "since unset" as "no time-bound filter".
func parseSinceFlag(s string, now time.Time) (time.Duration, bool, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return 0, true, nil
	}
	if d, err := cliutil.ParseDurationLoose(t); err == nil {
		return d, true, nil
	}
	if dt, err := time.Parse("2006-01-02", t); err == nil {
		if dt.After(now) {
			return 0, false, fmt.Errorf("--since %q is in the future", t)
		}
		return now.Sub(dt), true, nil
	}
	return 0, false, fmt.Errorf("--since %q: expected duration like 30d or date like 2026-01-01", t)
}

// median returns the median of a numeric slice. Returns 0 for empty input;
// callers gate on len(values)>0 when zero is a meaningful baseline.
func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2
	}
	return sorted[mid]
}

// streamContacts iterates every row in the typed crm table, decoding each
// into a contactProps and invoking visit. Iteration short-circuits when
// visit returns a non-nil error. Streaming (rather than slurping all rows
// into a slice) keeps memory bounded for large contact lists; the duplicate-
// suspects command paginates explicitly and does not use this helper.
//
// parseErrors counts rows that scanned cleanly from SQLite but failed
// contactProps decoding; callers surface the number in their result envelope
// so silently skipped rows don't disappear from observability.
func streamContacts(ctx context.Context, db *store.Store, visit func(contactProps) error) (scanned int, parseErrors int, err error) {
	rows, err := db.QueryContext(ctx, `SELECT "id", "data" FROM "crm" ORDER BY "id"`)
	if err != nil {
		return 0, 0, fmt.Errorf("querying crm: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return scanned, parseErrors, err
		}
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return scanned, parseErrors, fmt.Errorf("scanning crm row: %w", err)
		}
		scanned++
		p, perr := parseContactProps(id, json.RawMessage(data))
		if perr != nil {
			parseErrors++
			continue
		}
		if err := visit(p); err != nil {
			return scanned, parseErrors, err
		}
	}
	if err := rows.Err(); err != nil {
		return scanned, parseErrors, fmt.Errorf("iterating crm rows: %w", err)
	}
	return scanned, parseErrors, nil
}

// snapshotRetention bounds how far back snapshot rows are kept. score-drift's
// default --window is 14d and daily-digest's default --since is 1d, and both
// commands look at most 2*window back. 90d covers 2*window for any reasonable
// window up to ~45 days, so analytics queries see what they need while the
// snapshot tables stop growing without bound. Operators with shorter windows
// effectively get more retention; operators with longer windows than 45 days
// should also raise this constant.
const snapshotRetention = 90 * 24 * time.Hour

// pruneOldSnapshots deletes contact_score_snapshots and
// contact_engagement_snapshots rows older than snapshotRetention. Idempotent
// and cheap (idx_*_snapshots_at indexes the snapshot_at column).
func pruneOldSnapshots(ctx context.Context, db *store.Store, now time.Time) error {
	cutoff := now.Add(-snapshotRetention).UTC().Format(time.RFC3339Nano)
	if _, err := db.DB().ExecContext(ctx, `DELETE FROM contact_score_snapshots WHERE snapshot_at < ?`, cutoff); err != nil {
		return fmt.Errorf("prune score snapshots: %w", err)
	}
	if _, err := db.DB().ExecContext(ctx, `DELETE FROM contact_engagement_snapshots WHERE snapshot_at < ?`, cutoff); err != nil {
		return fmt.Errorf("prune engagement snapshots: %w", err)
	}
	return nil
}

// snapshotScores writes the current score-related columns for every contact
// into contact_score_snapshots so subsequent calls can compute deltas.
// Returns the number of rows inserted.
func snapshotScores(ctx context.Context, db *store.Store, now time.Time) (int, error) {
	contacts, err := loadContactsForSnapshot(ctx, db)
	if err != nil {
		return 0, err
	}
	if len(contacts) == 0 {
		return 0, nil
	}
	if err := pruneOldSnapshots(ctx, db, now); err != nil {
		return 0, err
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin snapshot tx: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO contact_score_snapshots
		(contact_id, snapshot_at, hubspotscore, lifecyclestage, hs_lead_status, hubspot_owner_id)
		VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare snapshot insert: %w", err)
	}
	defer stmt.Close()
	stamp := now.UTC().Format(time.RFC3339Nano)
	count := 0
	for _, p := range contacts {
		var score sql.NullFloat64
		if p.HubspotScoreSet {
			score = sql.NullFloat64{Float64: p.HubspotScore, Valid: true}
		}
		if _, err := stmt.ExecContext(ctx,
			p.ID, stamp, score,
			nullableString(p.LifecycleStage),
			nullableString(p.LeadStatus),
			nullableString(p.OwnerID),
		); err != nil {
			return count, fmt.Errorf("insert snapshot %s: %w", p.ID, err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("commit snapshot: %w", err)
	}
	return count, nil
}

// snapshotEngagement writes the current engagement-related columns for every
// contact into contact_engagement_snapshots.
func snapshotEngagement(ctx context.Context, db *store.Store, now time.Time) (int, error) {
	contacts, err := loadContactsForSnapshot(ctx, db)
	if err != nil {
		return 0, err
	}
	if len(contacts) == 0 {
		return 0, nil
	}
	if err := pruneOldSnapshots(ctx, db, now); err != nil {
		return 0, err
	}
	tx, err := db.DB().BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin engagement snapshot tx: %w", err)
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO contact_engagement_snapshots
		(contact_id, snapshot_at, hs_email_open, hs_email_click, hs_last_activity_date)
		VALUES (?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("prepare engagement snapshot insert: %w", err)
	}
	defer stmt.Close()
	stamp := now.UTC().Format(time.RFC3339Nano)
	count := 0
	for _, p := range contacts {
		var lastAct sql.NullString
		if p.LastActivitySet {
			lastAct = sql.NullString{String: p.LastActivityDate.UTC().Format(time.RFC3339), Valid: true}
		}
		if _, err := stmt.ExecContext(ctx,
			p.ID, stamp, p.EmailOpen, p.EmailClick, lastAct,
		); err != nil {
			return count, fmt.Errorf("insert engagement snapshot %s: %w", p.ID, err)
		}
		count++
	}
	if err := tx.Commit(); err != nil {
		return count, fmt.Errorf("commit engagement snapshot: %w", err)
	}
	return count, nil
}

// loadContactsForSnapshot slurps every contact into memory once. The crm
// table is bounded by a HubSpot portal's contact count and snapshots run
// inside a single transaction, so a slice (rather than a streaming cursor)
// keeps the prepared statement loop simple.
func loadContactsForSnapshot(ctx context.Context, db *store.Store) ([]contactProps, error) {
	var contacts []contactProps
	_, _, err := streamContacts(ctx, db, func(p contactProps) error {
		contacts = append(contacts, p)
		return nil
	})
	return contacts, err
}

func nullableString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

// priorScoreSnapshot is the score value observed at some earlier point in
// time. Drift commands compare a current contactProps to one of these.
type priorScoreSnapshot struct {
	contactID      string
	snapshotAt     time.Time
	score          float64
	scoreSet       bool
	lifecycleStage string
	leadStatus     string
	ownerID        string
}

// loadPriorScoreSnapshots returns the oldest snapshot for each contact whose
// snapshot_at falls inside [now-2*window, now-window]. That window picks the
// earliest sample at least `window` ago but no more than `2*window` ago, so
// the comparison is stable across runs even when score-drift is invoked
// twice in quick succession.
func loadPriorScoreSnapshots(ctx context.Context, db *store.Store, now time.Time, window time.Duration) (map[string]priorScoreSnapshot, error) {
	if window <= 0 {
		return nil, fmt.Errorf("window must be positive")
	}
	upper := now.Add(-window).UTC().Format(time.RFC3339Nano)
	lower := now.Add(-2 * window).UTC().Format(time.RFC3339Nano)
	rows, err := db.QueryContext(ctx, `
		SELECT s.contact_id, s.snapshot_at, s.hubspotscore, s.lifecyclestage, s.hs_lead_status, s.hubspot_owner_id
		FROM contact_score_snapshots s
		JOIN (
			SELECT contact_id, MIN(snapshot_at) AS first_at
			FROM contact_score_snapshots
			WHERE snapshot_at >= ? AND snapshot_at <= ?
			GROUP BY contact_id
		) f ON f.contact_id = s.contact_id AND f.first_at = s.snapshot_at
	`, lower, upper)
	if err != nil {
		return nil, fmt.Errorf("loading prior snapshots: %w", err)
	}
	defer rows.Close()
	out := map[string]priorScoreSnapshot{}
	for rows.Next() {
		var (
			id        string
			at        string
			score     sql.NullFloat64
			lifecycle sql.NullString
			status    sql.NullString
			owner     sql.NullString
		)
		if err := rows.Scan(&id, &at, &score, &lifecycle, &status, &owner); err != nil {
			return nil, fmt.Errorf("scan snapshot: %w", err)
		}
		parsed, _ := parseHubSpotTime(at)
		out[id] = priorScoreSnapshot{
			contactID:      id,
			snapshotAt:     parsed,
			score:          score.Float64,
			scoreSet:       score.Valid,
			lifecycleStage: lifecycle.String,
			leadStatus:     status.String,
			ownerID:        owner.String,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating prior snapshots: %w", err)
	}
	return out, nil
}

// scoreDriftItem is the per-contact result row emitted by score-drift and
// embedded in daily-digest's score_movers section.
type scoreDriftItem struct {
	ID         string  `json:"id"`
	Email      string  `json:"email,omitempty"`
	Name       string  `json:"name,omitempty"`
	ScoreThen  float64 `json:"score_then"`
	ScoreNow   float64 `json:"score_now"`
	DriftPct   float64 `json:"drift_pct"`
	SnapshotAt string  `json:"snapshot_at,omitempty"`
}

// computeScoreDrift compares each current contact's score to its earliest
// snapshot inside the window. When direction is "drop" it keeps contacts
// whose drift <= -thresholdPct; "any" returns all contacts with a valid
// prior snapshot. Results are sorted by absolute drift descending; ties
// break alphabetically on id for a deterministic ordering across runs.
//
// Returns (items, scanned, parseErrors, err).
func computeScoreDrift(ctx context.Context, db *store.Store, now time.Time, window time.Duration, thresholdPct float64, direction string) ([]scoreDriftItem, int, int, error) {
	prior, err := loadPriorScoreSnapshots(ctx, db, now, window)
	if err != nil {
		return nil, 0, 0, err
	}
	if len(prior) == 0 {
		return []scoreDriftItem{}, 0, 0, nil
	}
	items := make([]scoreDriftItem, 0)
	scanned, parseErrors, err := streamContacts(ctx, db, func(p contactProps) error {
		snap, ok := prior[p.ID]
		if !ok {
			return nil
		}
		// Split the legacy `!snap.scoreSet || snap.score == 0` guard so the
		// two cases are distinguishable in code review and (eventually) in
		// observability output.
		if !snap.scoreSet {
			return nil
		}
		if snap.score == 0 {
			// Drift can't be expressed as a percentage when the denominator
			// is zero; skip rather than emit Inf/NaN.
			return nil
		}
		if !p.HubspotScoreSet {
			return nil
		}
		drift := (p.HubspotScore - snap.score) / snap.score * 100
		switch direction {
		case "drop":
			if drift > -thresholdPct {
				return nil
			}
		case "any":
			if math.Abs(drift) < thresholdPct {
				return nil
			}
		}
		items = append(items, scoreDriftItem{
			ID:         p.ID,
			Email:      p.Email,
			Name:       contactDisplayName(p),
			ScoreThen:  snap.score,
			ScoreNow:   p.HubspotScore,
			DriftPct:   roundTo(drift, 2),
			SnapshotAt: snap.snapshotAt.UTC().Format(time.RFC3339),
		})
		return nil
	})
	if err != nil {
		return nil, scanned, parseErrors, err
	}
	sort.Slice(items, func(i, j int) bool {
		ai, aj := math.Abs(items[i].DriftPct), math.Abs(items[j].DriftPct)
		if ai != aj {
			return ai > aj
		}
		return items[i].ID < items[j].ID
	})
	return items, scanned, parseErrors, nil
}

// staleVIPItem mirrors stale-but-valuable's per-row output shape. Lifted out
// so daily-digest can reuse the same compute path without duplicating the
// scoring formula.
type staleVIPItem struct {
	ID                 string  `json:"id"`
	Email              string  `json:"email,omitempty"`
	Name               string  `json:"name,omitempty"`
	HubspotScore       float64 `json:"hubspotscore"`
	TotalRevenue       float64 `json:"total_revenue"`
	NumAssociatedDeals int64   `json:"num_associated_deals"`
	LastActivityAt     string  `json:"last_activity_at,omitempty"`
	SilentDays         int     `json:"silent_days"`
	ValueScore         float64 `json:"value_score"`
}

// computeStaleVIPs runs the stale-but-valuable predicate over the crm table:
// hubspotscore >= scoreMin AND last activity older than silentDays. Items
// are ranked by value_score = score + 10*deals + revenue/1000; ties break
// alphabetically on id so two equal value scores order deterministically.
//
// Returns (items, scanned, parseErrors, err).
func computeStaleVIPs(ctx context.Context, db *store.Store, now time.Time, scoreMin float64, silentDays int) ([]staleVIPItem, int, int, error) {
	items := make([]staleVIPItem, 0)
	scanned, parseErrors, err := streamContacts(ctx, db, func(p contactProps) error {
		if !p.HubspotScoreSet || p.HubspotScore < scoreMin {
			return nil
		}
		if !p.LastActivitySet {
			return nil
		}
		silent := int(now.Sub(p.LastActivityDate).Hours() / 24)
		if silent < silentDays {
			return nil
		}
		value := p.HubspotScore + float64(p.NumAssociatedDeals)*10 + p.TotalRevenue/1000
		items = append(items, staleVIPItem{
			ID:                 p.ID,
			Email:              p.Email,
			Name:               contactDisplayName(p),
			HubspotScore:       p.HubspotScore,
			TotalRevenue:       p.TotalRevenue,
			NumAssociatedDeals: p.NumAssociatedDeals,
			LastActivityAt:     p.LastActivityDate.UTC().Format(time.RFC3339),
			SilentDays:         silent,
			ValueScore:         roundTo(value, 2),
		})
		return nil
	})
	if err != nil {
		return nil, scanned, parseErrors, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ValueScore != items[j].ValueScore {
			return items[i].ValueScore > items[j].ValueScore
		}
		return items[i].ID < items[j].ID
	})
	return items, scanned, parseErrors, nil
}

// decayItem is engagement-decay's per-row output shape; shared with daily-digest.
type decayItem struct {
	ID             string `json:"id"`
	Email          string `json:"email,omitempty"`
	Name           string `json:"name,omitempty"`
	PriorOpens     int64  `json:"prior_opens"`
	PriorClicks    int64  `json:"prior_clicks"`
	LastActivityAt string `json:"last_activity_at,omitempty"`
	SilentDays     int    `json:"silent_days"`
}

// computeDecay finds contacts who used to engage (hs_email_open >=
// minPriorOpens) but whose last activity is older than `cutoff`. Contacts
// who have never recorded a last activity also qualify when their open
// count meets the threshold — silence is the dominant signal.
//
// Returns (items, scanned, parseErrors, err).
func computeDecay(ctx context.Context, db *store.Store, now time.Time, window time.Duration, minPriorOpens int64) ([]decayItem, int, int, error) {
	cutoff := now.Add(-window)
	items := make([]decayItem, 0)
	scanned, parseErrors, err := streamContacts(ctx, db, func(p contactProps) error {
		if p.EmailOpen < minPriorOpens {
			return nil
		}
		if p.LastActivitySet && p.LastActivityDate.After(cutoff) {
			return nil
		}
		silent := -1
		lastStr := ""
		if p.LastActivitySet {
			silent = int(now.Sub(p.LastActivityDate).Hours() / 24)
			lastStr = p.LastActivityDate.UTC().Format(time.RFC3339)
		}
		items = append(items, decayItem{
			ID:             p.ID,
			Email:          p.Email,
			Name:           contactDisplayName(p),
			PriorOpens:     p.EmailOpen,
			PriorClicks:    p.EmailClick,
			LastActivityAt: lastStr,
			SilentDays:     silent,
		})
		return nil
	})
	if err != nil {
		return nil, scanned, parseErrors, err
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].PriorOpens != items[j].PriorOpens {
			return items[i].PriorOpens > items[j].PriorOpens
		}
		// Older silence (or unset) ranks first.
		if items[i].LastActivityAt == "" && items[j].LastActivityAt != "" {
			return true
		}
		if items[j].LastActivityAt == "" && items[i].LastActivityAt != "" {
			return false
		}
		if items[i].LastActivityAt != items[j].LastActivityAt {
			return items[i].LastActivityAt < items[j].LastActivityAt
		}
		return items[i].ID < items[j].ID
	})
	return items, scanned, parseErrors, nil
}

// roundTo rounds f to the given number of decimal places. Used for output-
// facing floats so JSON consumers don't see noisy trailing digits.
func roundTo(f float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Round(f*shift) / shift
}

// limitInt clamps a slice length without panicking on negative inputs.
func limitInt[T any](items []T, limit int) []T {
	if limit < 0 {
		limit = 0
	}
	if len(items) > limit {
		return items[:limit]
	}
	return items
}

// canonicalLifecycleStages enumerates the HubSpot-supplied stage values the
// lifecycle-stuck command accepts via --stage, plus "other" as an escape
// hatch for custom stages.
var canonicalLifecycleStages = []string{
	"lead", "marketingqualifiedlead", "salesqualifiedlead",
	"opportunity", "customer", "evangelist", "other",
}

// stageWeight maps a HubSpot lifecycle stage to the workload weight used by
// owner-overload. Custom stages fall back to the "other" weight.
func stageWeight(stage string) float64 {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "lead":
		return 1
	case "marketingqualifiedlead":
		return 2
	case "salesqualifiedlead":
		return 4
	case "opportunity":
		return 8
	case "customer":
		return 4
	case "":
		return 1
	default:
		return 1
	}
}

// stagePromotionItem is the per-row output shape for daily-digest's
// stage_promotions section.
type stagePromotionItem struct {
	ID         string `json:"id"`
	Email      string `json:"email,omitempty"`
	Name       string `json:"name,omitempty"`
	FromStage  string `json:"from_stage"`
	ToStage    string `json:"to_stage"`
	SnapshotAt string `json:"snapshot_at,omitempty"`
}

// computeStagePromotions inspects snapshots and current contactProps to
// surface lifecycle transitions. Like computeScoreDrift, it uses the oldest
// snapshot in [now-2*window, now-window] as the "before" value.
//
// Returns (items, parseErrors, err).
func computeStagePromotions(ctx context.Context, db *store.Store, now time.Time, window time.Duration) ([]stagePromotionItem, int, error) {
	prior, err := loadPriorScoreSnapshots(ctx, db, now, window)
	if err != nil {
		return nil, 0, err
	}
	items := make([]stagePromotionItem, 0)
	if len(prior) == 0 {
		return items, 0, nil
	}
	_, parseErrors, err := streamContacts(ctx, db, func(p contactProps) error {
		snap, ok := prior[p.ID]
		if !ok {
			return nil
		}
		from := strings.ToLower(strings.TrimSpace(snap.lifecycleStage))
		to := strings.ToLower(strings.TrimSpace(p.LifecycleStage))
		if from == to {
			return nil
		}
		if from == "" && to == "" {
			return nil
		}
		items = append(items, stagePromotionItem{
			ID:         p.ID,
			Email:      p.Email,
			Name:       contactDisplayName(p),
			FromStage:  from,
			ToStage:    to,
			SnapshotAt: snap.snapshotAt.UTC().Format(time.RFC3339),
		})
		return nil
	})
	return items, parseErrors, err
}

// shortLifecycleLabel turns the canonical HubSpot lifecycle stage string
// into a short label suitable for the by_stage breakdown emitted by
// owner-overload.
func shortLifecycleLabel(stage string) string {
	switch strings.ToLower(strings.TrimSpace(stage)) {
	case "lead":
		return "lead"
	case "marketingqualifiedlead":
		return "mql"
	case "salesqualifiedlead":
		return "sql"
	case "opportunity":
		return "opportunity"
	case "customer":
		return "customer"
	case "evangelist":
		return "evangelist"
	case "":
		return "unknown"
	default:
		return "other"
	}
}
