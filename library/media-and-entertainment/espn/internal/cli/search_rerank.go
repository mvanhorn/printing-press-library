// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/espn/internal/store"
)

// searchApplier adapts the rerank Apply engine to ESPN's search command,
// which returns three raw-JSON slices (events / news / general resources)
// rather than a single typed slice. One instance of searchApplier is
// created per group and Apply is called three times so each slice is
// independently reranked under the same query pattern.
//
// items is a pointer so MoveToFront / RemoveHit / InsertLearnedHit /
// ReplaceHit can splice the underlying slice in place and the caller's
// slice header observes the mutation.
type searchApplier struct {
	ctx   context.Context
	db    *store.Store
	kind  string
	items *[]json.RawMessage
}

// rawHitID parses the JSON id field from a result. ESPN's events / news
// / general slices all carry an "id" key (string-shaped in raw JSON, may
// be either a JSON string or a JSON number depending on which upstream
// table fed it). We canonicalize to a string by trimming surrounding
// quotes so "12345" and 12345 compare equal.
func rawHitID(raw json.RawMessage) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	id, ok := obj["id"]
	if !ok {
		return ""
	}
	s := strings.TrimSpace(string(id))
	s = strings.Trim(s, `"`)
	return s
}

// matchesKind returns true when the learning's resource_type is either
// empty (no type recorded -- match any group) or equals the group tag
// this applier was constructed with.
func (a *searchApplier) matchesKind(rt string) bool {
	return rt == "" || rt == a.kind
}

func (a *searchApplier) HasHit(rt, rid string) bool {
	if !a.matchesKind(rt) {
		return false
	}
	for _, raw := range *a.items {
		if rawHitID(raw) == rid {
			return true
		}
	}
	return false
}

func (a *searchApplier) MoveToFront(rt, rid string) {
	if !a.matchesKind(rt) {
		return
	}
	idx := -1
	items := *a.items
	for i, raw := range items {
		if rawHitID(raw) == rid {
			idx = i
			break
		}
	}
	if idx <= 0 {
		return
	}
	hit := items[idx]
	items = append(items[:idx], items[idx+1:]...)
	items = append([]json.RawMessage{hit}, items...)
	*a.items = items
}

func (a *searchApplier) InsertLearnedHit(h store.LearnedHit) error {
	// Soft-skip: this applier only owns one group. When a learning
	// targets a different resource_type, let the next applier instance
	// claim it. Returning ErrApplierSkip prevents Apply from counting
	// this rule as applied or surfacing a spurious warning.
	if !a.matchesKind(h.ResourceType) {
		return store.ErrApplierSkip
	}
	raw, ok := resolveSearchLearnedHit(a.ctx, a.db, h)
	if !ok {
		// Resource not in local store: nothing to insert. Returning the
		// sentinel keeps Apply from counting this as an applied change
		// (which would mislead hintForApplied into "low_confidence" when
		// the bundle was untouched).
		return store.ErrApplierSkip
	}
	items := append([]json.RawMessage{raw}, *a.items...)
	*a.items = items
	return nil
}

func (a *searchApplier) RemoveHit(rt, rid string) {
	if !a.matchesKind(rt) {
		return
	}
	items := *a.items
	for i, raw := range items {
		if rawHitID(raw) == rid {
			*a.items = append(items[:i], items[i+1:]...)
			return
		}
	}
}

func (a *searchApplier) ReplaceHit(srcType, srcID, dstType, dstID string) error {
	if a.matchesKind(srcType) {
		items := *a.items
		for i, raw := range items {
			if rawHitID(raw) == srcID {
				rawDst, ok := resolveSearchLearnedHit(a.ctx, a.db, store.LearnedHit{
					ResourceType: dstType,
					ResourceID:   dstID,
				})
				if !ok {
					return fmt.Errorf("alias target not found in local DB")
				}
				items[i] = rawDst
				*a.items = items
				return nil
			}
		}
	}
	// Src not present in this group. Only fall through to InsertLearnedHit
	// when the alias destination belongs to this group; otherwise return
	// the sentinel so other applier instances get a chance to handle it
	// without inflating Apply's counter.
	if !a.matchesKind(dstType) {
		return store.ErrApplierSkip
	}
	return a.InsertLearnedHit(store.LearnedHit{ResourceType: dstType, ResourceID: dstID})
}

// resolveSearchLearnedHit looks up the raw JSON for a learning whose
// resource the upstream search bundle missed. ESPN's store.Get is keyed
// by (resource_type, id) against the generic resources table; when a
// learning carries an empty resource_type we probe the priority order
// events / news / resources, returning the first non-empty payload.
func resolveSearchLearnedHit(ctx context.Context, db *store.Store, h store.LearnedHit) (json.RawMessage, bool) {
	_ = ctx
	if strings.TrimSpace(h.ResourceID) == "" {
		return nil, false
	}
	candidates := []string{h.ResourceType}
	if h.ResourceType == "" {
		candidates = []string{"events", "news", "resources"}
	}
	for _, rt := range candidates {
		raw, err := db.Get(rt, h.ResourceID)
		if err != nil {
			continue
		}
		if len(raw) == 0 {
			continue
		}
		return raw, true
	}
	return nil, false
}

// applyLearningsForSearch runs the rerank Apply engine against ESPN's
// three search-result slices (events / news / general). Each slice is
// reranked independently under a fresh searchApplier instance bound to
// the corresponding group tag and a pointer to the underlying slice.
// Returns the combined count of rules that visibly touched any group
// and whether any high-confidence boost fired across all three.
//
// Errors from any individual Apply call are logged via writeTeachErrLog
// and skipped — the loop is soft-failing so a broken read against one
// group doesn't suppress reranking of the other two.
func applyLearningsForSearch(
	ctx context.Context,
	db *store.Store,
	query string,
	eventsPtr, newsPtr, generalPtr *[]json.RawMessage,
) (applied int, hasHigh bool) {
	groups := []struct {
		kind  string
		items *[]json.RawMessage
	}{
		{kind: "events", items: eventsPtr},
		{kind: "news", items: newsPtr},
		{kind: "general", items: generalPtr},
	}
	for _, g := range groups {
		ap := &searchApplier{ctx: ctx, db: db, kind: g.kind, items: g.items}
		res, err := db.Apply(ctx, query, ap)
		if err != nil {
			writeTeachErrLog(fmt.Sprintf("apply learnings for search %q (%s): %v", query, g.kind, err))
			continue
		}
		for _, w := range res.Warnings {
			writeTeachErrLog(fmt.Sprintf("apply (search/%s): %s", g.kind, w))
		}
		applied += res.Count
		if res.HasHighConfidence {
			hasHigh = true
		}
	}
	return applied, hasHigh
}

// hintForApplied returns the teach-hint envelope value mirroring
// prediction-goat's contract:
//
//   - "none"           — a high-confidence rerank fired; no further teach needed
//   - "low_confidence" — some rerank fired but no high-confidence boost
//   - "empty"          — the store had no relevant learnings for this query
func hintForApplied(applied int, hasHigh bool) string {
	switch {
	case hasHigh:
		return "none"
	case applied > 0:
		return "low_confidence"
	default:
		return "empty"
	}
}
