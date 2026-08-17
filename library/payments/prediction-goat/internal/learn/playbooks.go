// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

// playbooks.go hosts the in-process Playbook types and slot-resolution
// primitive for the learning subsystem's hand-authored choreography
// layer. A playbook attaches a structured tool-call sequence (steps,
// entity_slots, expected_tool_calls) plus free-text notes to a query
// family; the store-layer table learning_playbooks (U5) persists the
// row, U7 surfaces it on the recall envelope, and U8's teach-playbook
// CLI command writes new rows.
//
// PATCH(learn-loop-backport U6): part of the ESPN learn-loop cascade
// backport. See docs/plans/2026-05-25-001-feat-prediction-goat-learn-
// loop-backport-plan.md. Ported verbatim from ESPN PR #851 HEAD
// 9bb0a40a (library/media-and-entertainment/espn/internal/learn/
// playbooks.go), with import paths adapted and ResolveSlots' candidate
// pool restricted to normalized.Entities per Greptile round 3 finding.

package learn

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

// Playbook is the on-disk shape of a stored CLI choreography for a
// query family. Steps are linear; no branching or conditionals in v1.
// Either Cmd or ClientSide is non-empty on each step.
type Playbook struct {
	QueryFamilyExamples []string       `json:"query_family_examples,omitempty"`
	Steps               []PlaybookStep `json:"steps"`
	EntitySlots         []string       `json:"entity_slots,omitempty"`
	ExpectedToolCalls   int            `json:"expected_tool_calls,omitempty"`
}

// PlaybookStep is one entry in the choreography. Mutually exclusive
// shapes:
//   - Cmd: CLI command string with entity slots like "{country.canonical}",
//     replayed against the printed CLI. Optional pagination hint.
//   - ClientSide: post-process the previous step's result (rank_by,
//     filter, etc.). Args carry the parameters.
type PlaybookStep struct {
	Cmd        string         `json:"cmd,omitempty"`
	ClientSide string         `json:"client_side,omitempty"`
	Args       map[string]any `json:"args,omitempty"`
	Purpose    string         `json:"purpose,omitempty"`
	Pagination string         `json:"pagination,omitempty"`
}

// ResolvedPlaybook wraps a Playbook with the per-call slot resolution
// map: $COUNTRY -> {token, canonical}. Unresolvable slots stay absent
// from the map. The recall envelope embeds this (U7).
type ResolvedPlaybook struct {
	Playbook      Playbook                  `json:"playbook"`
	SlotsResolved map[string]map[string]any `json:"slots_resolved,omitempty"`
	Notes         string                    `json:"notes,omitempty"`
	QueryFamily   string                    `json:"query_family"`
	Confidence    int                       `json:"confidence,omitempty"`
}

// ParsePlaybookFile reads a JSON playbook file from disk and returns
// the parsed Playbook. Empty/malformed files return an error with the
// file path embedded.
func ParsePlaybookFile(path string) (Playbook, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Playbook{}, fmt.Errorf("read playbook %s: %w", path, err)
	}
	return ParsePlaybook(data, path)
}

// ParsePlaybook decodes a JSON byte slice into a Playbook. label is
// used only in error messages (file path, "inline", etc.).
func ParsePlaybook(data []byte, label string) (Playbook, error) {
	data = []byte(strings.TrimSpace(string(data)))
	if len(data) == 0 {
		return Playbook{}, fmt.Errorf("parse playbook %s: empty", label)
	}
	var p Playbook
	if err := json.Unmarshal(data, &p); err != nil {
		return Playbook{}, fmt.Errorf("parse playbook %s: %w", label, err)
	}
	return p, nil
}

// MarshalPlaybook returns the canonical JSON form of a Playbook for
// storage in learning_playbooks.playbook_json.
func MarshalPlaybook(p Playbook) (string, error) {
	out, err := json.Marshal(p)
	if err != nil {
		return "", fmt.Errorf("marshal playbook: %w", err)
	}
	return string(out), nil
}

// QueryFamily derives the structural family key from a NormalizedQuery.
// Used at both teach time (UpsertPlaybook) and recall time (lookup),
// so the same key resolves consistently across the two paths.
//
// Today the family is simply NonEntityNormalized — Normalize +
// PromoteEntities already strips entities and stopwords and produces a
// sorted-joined token bag, which is what "structural shape" means. The
// function exists so future refinements (e.g., pluralization folding,
// lemmatization) have a single place to land without rewriting teach
// and recall in lockstep.
func QueryFamily(normalized NormalizedQuery) string {
	return normalized.NonEntityNormalized
}

// ResolveSlots walks the entity_slots declared on a Playbook and
// resolves each one against the current query's entities + canonical
// resolver. Returns a map keyed by slot name (e.g., "$COUNTRY") to a
// metadata map ({"token": "portugal", "canonical": "Portugal"}).
//
// Slots that don't match any query entity stay absent from the map —
// the agent reads "I have $COUNTRY bound to {...} but no $TOURNAMENT
// bound, the user must have meant differently" and decides.
//
// The resolver is the same EntityResolver interface PromoteEntities
// uses, so this composes cleanly with the existing recall path.
//
// CRITICAL (Greptile PR #851 round 3): the candidate pool is
// restricted to normalized.Entities only. Including the union with
// strings.Fields(normalized.NonEntityNormalized) would let a
// non-entity token that coincidentally resolves through entity_lookups
// (a secondary-alias collision the extractor didn't promote) win a
// slot intended for a real entity — e.g. a "$COUNTRY" slot getting
// bound to "ppg" if "ppg" were ever added as a secondary alias of a
// different canonical. The docstring contract says "Slots that don't
// match any query entity stay absent"; restricting the pool to
// normalized.Entities matches that intent.
func ResolveSlots(p Playbook, normalized NormalizedQuery, r EntityResolver) map[string]map[string]any {
	if len(p.EntitySlots) == 0 || r == nil {
		return nil
	}
	out := make(map[string]map[string]any, len(p.EntitySlots))
	// Slot binding must only consider tokens classified as entities
	// (after PromoteEntities). Do NOT include NonEntityNormalized
	// tokens here — see the function-level docstring for the
	// Greptile-found contract this enforces.
	queryTokens := append([]string(nil), normalized.Entities...)
	sort.Strings(queryTokens)

	// Build a working set of unmatched tokens; mark off as slots claim them.
	unclaimed := make(map[string]bool, len(queryTokens))
	for _, t := range queryTokens {
		unclaimed[t] = true
	}

	for _, slot := range p.EntitySlots {
		// Take the first unclaimed token that resolves to a canonical.
		// Slots in the playbook order are bound to entities in
		// normalized-token order; multi-slot playbooks (e.g.,
		// "$HOME vs $AWAY") need explicit ordering by their author.
		for _, tok := range queryTokens {
			if !unclaimed[tok] {
				continue
			}
			cans := r.Resolve(tok)
			if len(cans) == 0 {
				continue
			}
			canonical := cans[0]
			if len(cans) > 1 {
				// Multiple canonicals; pick the first deterministically.
				sort.Strings(cans)
				canonical = cans[0]
			}
			out[slot] = map[string]any{
				"token":     tok,
				"canonical": canonical,
			}
			unclaimed[tok] = false
			break
		}
	}
	return out
}
