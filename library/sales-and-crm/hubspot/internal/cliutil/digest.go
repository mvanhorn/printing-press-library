// Copyright 2026 Damien Stevens and contributors. Licensed under Apache-2.0. See LICENSE.

// Dry-run → digest → confirm gating for large mutation batches.
//
// The pattern: any command that mutates more than Threshold rows must run
// twice. The first call (dry-run) hashes the plan, stores it in the local
// SQLite cache with a TTL, and tells the user what to re-run. The second
// call (confirm) supplies the digest and the row count; the gate verifies
// both match what's stored, then waves the mutation through. A mismatch
// (count off, digest unknown, expired) fails closed — the user re-runs the
// dry-run from scratch.
//
// Stable digests rely on canonical JSON serialization (sorted keys, no
// extra whitespace) so that two semantically-identical plans always produce
// the same hex digest regardless of map-iteration order or formatting.

package cliutil

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// PendingDigestStore is the subset of the store API the digest gate needs.
// Defined here so cliutil doesn't import store directly (which would create a
// cycle — store may want to use cliutil helpers later) and so tests can
// substitute a memory-backed fake.
type PendingDigestStore interface {
	PutPendingDigest(ctx context.Context, digest, command string, plan []byte, rowCount int, expires time.Duration) error
	GetPendingDigest(ctx context.Context, digest string) (*PendingDigest, error)
	PurgeExpiredDigests(ctx context.Context) error
}

// PendingDigest is the type the store returns from GetPendingDigest. Mirrors
// the row shape; defined in cliutil so the gate can consume it without
// importing store (avoids cycles).
type PendingDigest struct {
	Digest    string
	Command   string
	PlanJSON  []byte
	RowCount  int
	CreatedAt time.Time
	ExpiresAt time.Time
}

// MakeDigest computes blast-<hex> from a stable serialization of the plan.
//
// Stability: the plan input is re-marshalled through canonicalJSON before
// hashing so two semantically-identical Go maps with different iteration
// orders produce the same digest. command is included verbatim — different
// commands hash to different digests even with identical row payloads, so
// a `contacts bulk-update` digest cannot be redeemed against a hypothetical
// `companies bulk-update` confirm call.
//
// The "blast-" prefix is operator-readable in shell history and unambiguous
// at a glance ("that's a destructive-operation token, not an arbitrary
// UUID").
func MakeDigest(command string, plan []byte) string {
	h := sha256.New()
	h.Write([]byte(command))
	h.Write([]byte{0}) // separator so command + plan don't collide with plan + command
	canonical, err := canonicalJSON(plan)
	if err != nil {
		// Fall back to raw bytes — caller already passed JSON, so this is
		// nearly impossible. Stable in the degenerate case at least.
		canonical = plan
	}
	h.Write(canonical)
	return "blast-" + hex.EncodeToString(h.Sum(nil))[:16]
}

// canonicalJSON re-marshals data with sorted keys and no extra whitespace,
// so identical-content payloads with different formatting hash identically.
// Returns the input unchanged on parse failure (degenerate input).
func canonicalJSON(data []byte) ([]byte, error) {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return data, err
	}
	return marshalCanonical(v)
}

func marshalCanonical(v any) ([]byte, error) {
	switch x := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf := []byte{'{'}
		for i, k := range keys {
			if i > 0 {
				buf = append(buf, ',')
			}
			kb, err := json.Marshal(k)
			if err != nil {
				return nil, err
			}
			buf = append(buf, kb...)
			buf = append(buf, ':')
			vb, err := marshalCanonical(x[k])
			if err != nil {
				return nil, err
			}
			buf = append(buf, vb...)
		}
		buf = append(buf, '}')
		return buf, nil
	case []any:
		buf := []byte{'['}
		for i, item := range x {
			if i > 0 {
				buf = append(buf, ',')
			}
			vb, err := marshalCanonical(item)
			if err != nil {
				return nil, err
			}
			buf = append(buf, vb...)
		}
		buf = append(buf, ']')
		return buf, nil
	default:
		return json.Marshal(v)
	}
}

// DigestGate is the contract every >Threshold-row mutation goes through.
// Construct with a Threshold (typically 100) and TTL (typically 5 minutes);
// invoke Evaluate from the command handler.
type DigestGate struct {
	Threshold int
	TTL       time.Duration
}

// DefaultGate returns the gate every novel bulk-mutation command should use
// unless it explicitly overrides. 100 rows is HubSpot's batch limit and a
// reasonable "is this big enough to require a second look?" heuristic; 5
// minutes is long enough for the operator to read the dry-run output and
// short enough that a stale digest can't be redeemed by an unrelated
// later session.
func DefaultGate() DigestGate {
	return DigestGate{Threshold: 100, TTL: 5 * time.Minute}
}

// GateOutcome distinguishes the three terminal states for the caller.
type GateOutcome int

const (
	// GateProceedBelowThreshold: row count is at/under the threshold so
	// the gate is bypassed entirely. The caller should print a one-line
	// warning and execute the mutation.
	GateProceedBelowThreshold GateOutcome = iota
	// GateDryRunPersisted: large batch + no confirm — the gate stored
	// the digest and the caller should print {digest, count, instructions}
	// and exit 0.
	GateDryRunPersisted
	// GateProceedConfirmed: large batch + matching confirm — the caller
	// should execute the mutation.
	GateProceedConfirmed
)

// Evaluate routes a planned mutation through the gate.
//
//   - Plans at/under Threshold rows: returns (GateProceedBelowThreshold, digest, count, nil).
//     The digest is computed for logging but the gate is bypassed; the
//     caller may proceed immediately.
//   - Plans over Threshold with empty providedDigest: persists the plan
//     under its computed digest, returns (GateDryRunPersisted, digest, count, nil).
//     The caller should NOT execute the mutation; print the digest +
//     instructions and exit 0.
//   - Plans over Threshold with non-empty providedDigest: looks up the
//     stored plan. Returns (GateProceedConfirmed, digest, count, nil) on
//     match. Returns a non-nil error on mismatch (count off, digest
//     unknown, expired) — caller should surface the error and exit non-zero.
//
// The planRowCount is always returned for the caller's logging / output
// envelope, even on error paths where it might be zero.
func (g DigestGate) Evaluate(
	ctx context.Context,
	db PendingDigestStore,
	command, providedDigest string,
	providedConfirm int,
	plan []byte,
	planRowCount int,
) (GateOutcome, string, int, error) {
	threshold := g.Threshold
	if threshold <= 0 {
		threshold = 100
	}
	ttl := g.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	computed := MakeDigest(command, plan)

	if planRowCount <= threshold {
		return GateProceedBelowThreshold, computed, planRowCount, nil
	}

	// Always best-effort purge before either branch. Lazy GC keeps the
	// table small without a separate sweeper.
	_ = db.PurgeExpiredDigests(ctx)

	if providedDigest == "" {
		if err := db.PutPendingDigest(ctx, computed, command, plan, planRowCount, ttl); err != nil {
			return GateDryRunPersisted, computed, planRowCount, fmt.Errorf("persisting dry-run digest: %w", err)
		}
		return GateDryRunPersisted, computed, planRowCount, nil
	}

	// Confirm path: look up the stored digest.
	stored, err := db.GetPendingDigest(ctx, providedDigest)
	if err != nil {
		return GateProceedConfirmed, computed, planRowCount, fmt.Errorf("looking up digest: %w", err)
	}
	if stored == nil {
		return GateProceedConfirmed, computed, planRowCount, fmt.Errorf("digest %q expired or unknown — re-run with --dry-run first", providedDigest)
	}
	if stored.Command != command {
		return GateProceedConfirmed, computed, planRowCount, fmt.Errorf("digest %q was created for command %q, not %q", providedDigest, stored.Command, command)
	}
	if stored.RowCount != providedConfirm {
		return GateProceedConfirmed, computed, planRowCount, fmt.Errorf("digest mismatch: stored row count %d, your --confirm was %d", stored.RowCount, providedConfirm)
	}
	if stored.RowCount != planRowCount {
		return GateProceedConfirmed, computed, planRowCount, fmt.Errorf("plan changed since dry-run: stored %d rows, this run has %d — re-run with --dry-run", stored.RowCount, planRowCount)
	}
	if computed != providedDigest {
		return GateProceedConfirmed, computed, planRowCount, fmt.Errorf("plan content changed since dry-run: stored digest %q, this run computed %q — re-run with --dry-run", providedDigest, computed)
	}
	return GateProceedConfirmed, computed, planRowCount, nil
}
