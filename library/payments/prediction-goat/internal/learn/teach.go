// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// teach.go implements teach-time resource-shape validation: a soft
// guardrail that emits warnings when an LLM teaches against a
// parent-shape Kalshi event for a query carrying a specific entity
// AND a strictly-better child market exists, or when the resource
// being taught has no entity overlap with the query (the "wrong team
// teach" anti-pattern).
//
// The validator NEVER blocks a teach. Warnings land in teach.log for
// offline review; learnings list --warnings surfaces them. Calls are
// best-effort -- a DB-side failure mid-validation is swallowed and the
// teach still returns success, because the teach call itself has
// already succeeded by the time the validator runs.
//
// Why a new file in the learn package (vs extending recall.go or
// living next to internal/cli/teach.go): the validator wants the same
// child-market lookup logic the recall path uses (findKalshiChildForEntity
// in recall.go), so it belongs in the learn package rather than the cli
// package. Keeping it in its own file lets the cli-side teach hook stay
// a one-line call into learn, and lets a future generator template lift
// this file alongside match.go and recall.go without dragging the cli
// surface.
//
// Per the U6 section of
// docs/plans/2026-05-23-002-feat-prediction-goat-smart-learning-plan.md.
//
// TODO(prediction-goat U6 deferred): add a Polymarket-side parent
// detector. Polymarket events carry negRisk multi-outcome groups but
// the prediction-goat schema does not yet ship a clean "parent event"
// classifier akin to IsKalshiParentTicker. When that lands, extend
// ValidateResourceShape with a parallel branch for resource_type
// "events" + child-market lookup against `markets WHERE event_id=...`.
// See the Deferred section of the U6 plan.

package learn

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/payments/prediction-goat/internal/learn/entities"
)

// Warning is one teach-time validation finding. The shape is the JSON
// envelope written to teach.log per the U6 contract and consumed by
// `learnings list --warnings`. Adding a new field is backward-compatible
// for the LLM consumer; renaming an existing JSON tag is not.
type Warning struct {
	// Code is the stable warning identifier the LLM keys on
	// (e.g., "parent_event_when_child_exists"). Matches the warning
	// constants in match.go so the same vocabulary is used at teach
	// time and recall time.
	Code string `json:"code"`

	// Resource is the resource_id the teach call recorded -- the
	// thing the warning is ABOUT.
	Resource string `json:"resource"`

	// Detail is a human-readable explanation of why the warning
	// fired. Stable enough to grep for, but not parsed by callers.
	Detail string `json:"detail"`

	// Suggested optionally names a strictly-better target the LLM
	// could have taught against instead (e.g., the matching child
	// ticker). Empty when the warning describes a quality issue
	// with no concrete substitute.
	Suggested string `json:"suggested,omitempty"`
}

// ValidateResourceShape inspects the resource being taught against
// the query's entities, returning a slice of warnings. It NEVER
// returns an error: the teach itself has already succeeded and a
// validator failure shouldn't propagate.
//
// Rules (v1):
//
//  1. parent_event_when_child_exists -- resource_type=="kalshi_events"
//     (or resourceID matches IsKalshiParentTicker and resource_type is
//     empty/permissive) AND the query has entities AND a child market
//     under the parent has a yes_sub_title matching one of those
//     entities. Names the matching child ticker via Suggested. This is
//     the USA-vs-KXMENWORLDCUP-26 flagship case from the U6 plan.
//
//  2. no_entity_overlap -- query has entities AND the resource (via
//     ResourceEntities) has entities AND no overlap. Catches "LLM
//     taught the wrong team" cases. Soft warning: a categorical-pattern
//     teach with no entities on the query side is fine.
//
// The validator reads from the DB only when needed (child-market
// lookup, resource-entity fetch). Both reads are cheap: the
// resources table is keyed by (resource_type, id), and the child
// scan is bounded by the per-event subset size (tens to a few
// hundred rows typical).
func ValidateResourceShape(ctx context.Context, db *sql.DB, query, resourceID, resourceType string) []Warning {
	if db == nil || resourceID == "" {
		return nil
	}

	// Extract query entities once. The prediction-goat config gives
	// us Kalshi/Polymarket ticker recognition plus the prediction-
	// market stopword vocabulary so domain content tokens
	// (odds/win/etc) don't get mis-classified as entities.
	parsed := entities.Extract(query, DefaultPredictionGoatConfig())
	queryEntities := parsed.Entities

	var warnings []Warning

	// Rule 1: parent-event-when-child-exists.
	//
	// The trigger fires when both:
	//   - The resource looks like a Kalshi parent event (explicit
	//     resource_type, or shape-detected via IsKalshiParentTicker
	//     when the caller didn't supply a type).
	//   - The query carries entities (a parent teach for a generic
	//     "what's the overall market" query is legitimate -- it's
	//     ONLY a warning when the LLM is asking about a specific
	//     entity that a child market already covers).
	parentLike := resourceType == "kalshi_events" ||
		(resourceType == "" && IsKalshiParentTicker(resourceID))
	if parentLike && len(queryEntities) > 0 {
		if child := findKalshiChildForEntity(ctx, db, resourceID, queryEntities); child != "" {
			warnings = append(warnings, Warning{
				Code:     WarningParentEventWhenChildExists,
				Resource: resourceID,
				Detail: fmt.Sprintf(
					"taught against parent event %s for a query carrying entities %v; child market %s matches the query entity and is a better target",
					resourceID, queryEntities, child,
				),
				Suggested: child,
			})
		}
	}

	// Rule 2: no_entity_overlap.
	//
	// Only fires when BOTH sides carry entities and they don't
	// overlap. Both-empty / one-empty cases are handled by
	// ClassifyEntityMatch returning "partial" (legitimate
	// categorical teach) and are intentionally not warned-on.
	if len(queryEntities) > 0 {
		resourceEntities := lookupResourceEntities(ctx, db, resourceType, resourceID)
		if len(resourceEntities) > 0 {
			match := ClassifyEntityMatch(queryEntities, resourceEntities)
			if match == EntityMatchMismatch {
				warnings = append(warnings, Warning{
					Code:     "no_entity_overlap",
					Resource: resourceID,
					Detail: fmt.Sprintf(
						"query entities %v do not overlap with resource entities %v; verify this is the right resource for the query",
						queryEntities, resourceEntities,
					),
				})
			}
		}
		// When resourceEntities is empty (resource not in store, or
		// has no entity-bearing fields for its type), we cannot
		// validate. That's not a warning -- the LLM may have taught
		// against a resource that hasn't been synced yet, or against
		// a categorical hub page. Silence is the right default.
	}

	return warnings
}

// lookupResourceEntities reads the resource from the local `resources`
// table and runs the same field-extraction logic ResourceEntities uses
// on the recall path. Returns nil on lookup miss or empty payload.
//
// When resourceType is empty (older teach calls that didn't supply
// one), we walk the price-bearing tables in the same priority order
// resolveLearnedHit uses in the CLI layer. The first non-empty match
// wins.
func lookupResourceEntities(ctx context.Context, db *sql.DB, resourceType, resourceID string) []string {
	if resourceID == "" {
		return nil
	}

	candidates := []string{resourceType}
	if resourceType == "" {
		// Probe in the same priority order resolveLearnedHit uses,
		// so an unannotated teach gets the same entity-extraction
		// chance the recall path would give it.
		candidates = []string{
			"kalshi_markets", "markets", "kalshi_events",
			"events", "kalshi_series", "tags",
		}
	}

	for _, rt := range candidates {
		if rt == "" {
			continue
		}
		var data string
		err := db.QueryRowContext(ctx,
			`SELECT data FROM resources WHERE resource_type = ? AND id = ?`,
			rt, resourceID,
		).Scan(&data)
		if err != nil {
			continue
		}
		if ents := ResourceEntities(rt, []byte(data)); len(ents) > 0 {
			return ents
		}
	}
	return nil
}
