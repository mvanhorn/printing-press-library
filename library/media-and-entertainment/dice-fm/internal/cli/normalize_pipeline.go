// Copyright 2026 vinny-pasceri. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/dice-fm/internal/store"
)

// classifyOpts controls the classify pipeline run.
type classifyOpts struct {
	// ClassifierVersion is stamped onto every row written by this run.
	ClassifierVersion int
	// Fuzzy enables a second-pass Jaro-Winkler clustering of near-duplicate
	// canonical names into a shared canonical ID. Off by default so the
	// primary path is fully deterministic.
	Fuzzy bool
}

// classifyResult summarises what the classify pipeline produced.
type classifyResult struct {
	// CanonicalCount is the number of distinct canonical entities written.
	CanonicalCount int
	// Matched is the number of distinct raw source values that were classified.
	Matched int
	// Unmatched is the number of distinct raw source values that could not be
	// classified and were stored with method="unmatched".
	Unmatched int
}

// mintCanonicalID derives a stable, deterministic canonical ID from the entity
// type and the already-canonicalized name. The ID is a SHA-1 hex digest
// prefixed by the entity type so IDs from different entity types never collide,
// even if the canonical name is identical. Truncated to prefix + 12 hex chars
// to keep IDs compact while retaining negligible collision probability for
// realistic catalog sizes.
func mintCanonicalID(entityType, canonicalName string) string {
	h := sha1.Sum([]byte(canonicalName))
	hex := fmt.Sprintf("%x", h)
	return fmt.Sprintf("%s:%s", entityType, hex[:12])
}

// classifyTiers reads distinct ticketType.name values from the tickets table,
// runs them through Layer-A canonicalization + Layer-B tier-axis extraction,
// and writes results to the normalization tables.
//
// Source values already stored with method="manual" are skipped so operator
// overrides survive re-runs.
func classifyTiers(ctx context.Context, s *store.Store, opts classifyOpts) (classifyResult, error) {
	raws, err := distinctTicketTypeNames(ctx, s.DB())
	if err != nil {
		return classifyResult{}, fmt.Errorf("reading ticket type names: %w", err)
	}

	// Build a map from canonical form to canonical ID so two raw values that
	// canonicalize identically share the same ID.
	canonToID := map[string]string{}
	var res classifyResult

	// Collect existing manual crosswalk entries so we can skip them.
	manual, err := manualCrosswalkSet(ctx, s, "ticket_type", "dice")
	if err != nil {
		return classifyResult{}, err
	}

	for _, raw := range raws {
		if manual[raw] {
			continue
		}
		canon := canonicalizeName(raw)
		cid, ok := canonToID[canon]
		if !ok {
			cid = mintCanonicalID("ticket_type", canon)
			canonToID[canon] = cid
		}

		axes := extractTierAxes(canon)
		if axes.Matched {
			if err := s.UpsertCanonicalEntity("ticket_type", cid, canon); err != nil {
				return classifyResult{}, fmt.Errorf("upsert canonical entity: %w", err)
			}
			if err := s.UpsertTierAttributes(cid, store.TierAttributesRow{
				CanonicalID:       cid,
				AccessClass:       axes.AccessClass,
				SalesStage:        axes.SalesStage,
				EntryWindowType:   axes.EntryWindowType,
				EntryWindowTime:   axes.EntryWindowTime,
				GroupSize:         axes.GroupSize,
				CompFlag:          axes.CompFlag,
				ClassifierVersion: opts.ClassifierVersion,
				Method:            "regex",
			}); err != nil {
				return classifyResult{}, fmt.Errorf("upsert tier attributes: %w", err)
			}
			if err := s.UpsertCrosswalk(store.CrosswalkRow{
				EntityType:        "ticket_type",
				SourceSystem:      "dice",
				SourceValue:       raw,
				CanonicalID:       cid,
				Method:            "regex",
				ClassifierVersion: opts.ClassifierVersion,
			}); err != nil {
				return classifyResult{}, fmt.Errorf("upsert crosswalk (matched): %w", err)
			}
			res.Matched++
		} else {
			if err := s.UpsertCrosswalk(store.CrosswalkRow{
				EntityType:        "ticket_type",
				SourceSystem:      "dice",
				SourceValue:       raw,
				CanonicalID:       cid,
				Method:            "unmatched",
				ClassifierVersion: opts.ClassifierVersion,
			}); err != nil {
				return classifyResult{}, fmt.Errorf("upsert crosswalk (unmatched): %w", err)
			}
			res.Unmatched++
		}
	}

	// Optional fuzzy pass: cluster near-duplicate canonical names and remap
	// their crosswalk entries to a shared representative canonical ID.
	if opts.Fuzzy && len(canonToID) > 1 {
		canonNames := make([]string, 0, len(canonToID))
		for cn := range canonToID {
			canonNames = append(canonNames, cn)
		}
		clusters := clusterNames(canonNames, 0.92)
		for _, cluster := range clusters {
			if len(cluster) < 2 {
				continue
			}
			// Use the first cluster member as the representative.
			repID := canonToID[cluster[0]]
			for _, cn := range cluster[1:] {
				old := canonToID[cn]
				if old == repID {
					continue
				}
				// Remap crosswalk rows pointing to old ID.
				rows, _ := s.ListCrosswalk("ticket_type", "dice")
				for _, r := range rows {
					if r.CanonicalID == old {
						_ = s.UpsertCrosswalk(store.CrosswalkRow{
							EntityType:        r.EntityType,
							SourceSystem:      r.SourceSystem,
							SourceValue:       r.SourceValue,
							CanonicalID:       repID,
							Method:            r.Method,
							ClassifierVersion: opts.ClassifierVersion,
						})
					}
				}
				canonToID[cn] = repID
			}
		}
	}

	res.CanonicalCount = countDistinctCanonicals(canonToID, opts.Fuzzy)
	return res, nil
}

// classifyVenues reads distinct venue names from the events table, runs them
// through Layer-A canonicalization + Layer-B venue-parts extraction, and writes
// results to the normalization tables.
//
// Source values already stored with method="manual" are skipped.
func classifyVenues(ctx context.Context, s *store.Store, opts classifyOpts) (classifyResult, error) {
	raws, err := distinctVenueNames(ctx, s.DB())
	if err != nil {
		return classifyResult{}, fmt.Errorf("reading venue names: %w", err)
	}

	canonToID := map[string]string{}
	var res classifyResult

	manual, err := manualCrosswalkSet(ctx, s, "venue", "dice")
	if err != nil {
		return classifyResult{}, err
	}

	for _, raw := range raws {
		if manual[raw] {
			continue
		}
		canon := canonicalizeName(raw)
		cid, ok := canonToID[canon]
		if !ok {
			cid = mintCanonicalID("venue", canon)
			canonToID[canon] = cid
		}

		parts := extractVenueParts(canon)
		if parts.Complex != "" {
			if err := s.UpsertCanonicalEntity("venue", cid, canon); err != nil {
				return classifyResult{}, fmt.Errorf("upsert canonical venue entity: %w", err)
			}
			if err := s.UpsertVenueAttributes(cid, store.VenueAttributesRow{
				CanonicalID:       cid,
				Complex:           parts.Complex,
				Room:              parts.Room,
				ClassifierVersion: opts.ClassifierVersion,
				Method:            "regex",
			}); err != nil {
				return classifyResult{}, fmt.Errorf("upsert venue attributes: %w", err)
			}
			if err := s.UpsertCrosswalk(store.CrosswalkRow{
				EntityType:        "venue",
				SourceSystem:      "dice",
				SourceValue:       raw,
				CanonicalID:       cid,
				Method:            "regex",
				ClassifierVersion: opts.ClassifierVersion,
			}); err != nil {
				return classifyResult{}, fmt.Errorf("upsert venue crosswalk: %w", err)
			}
			res.Matched++
		} else {
			if err := s.UpsertCrosswalk(store.CrosswalkRow{
				EntityType:        "venue",
				SourceSystem:      "dice",
				SourceValue:       raw,
				CanonicalID:       cid,
				Method:            "unmatched",
				ClassifierVersion: opts.ClassifierVersion,
			}); err != nil {
				return classifyResult{}, fmt.Errorf("upsert venue crosswalk (unmatched): %w", err)
			}
			res.Unmatched++
		}
	}

	res.CanonicalCount = countDistinctCanonicals(canonToID, false)
	return res, nil
}

// distinctTicketTypeNames returns the set of distinct ticketType.name values
// from the tickets resource table, using SQLite's json_extract.
func distinctTicketTypeNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT json_extract(data, '$.ticketType.name')
		 FROM resources
		 WHERE resource_type = 'tickets'
		   AND json_extract(data, '$.ticketType.name') IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// distinctVenueNames returns distinct venue names from the events resource
// table. DICE events carry venues as a JSON array, so we use json_each to
// expand the array and collect the name of each element.
func distinctVenueNames(ctx context.Context, db *sql.DB) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT DISTINCT json_extract(v.value, '$.name')
		 FROM resources r, json_each(json_extract(r.data, '$.venues')) v
		 WHERE r.resource_type = 'events'
		   AND json_extract(v.value, '$.name') IS NOT NULL`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		names = append(names, n)
	}
	return names, rows.Err()
}

// manualCrosswalkSet returns the set of source_value strings that have an
// existing method="manual" crosswalk row for the given entity type and source
// system. Used to gate re-classification so operator overrides survive a run.
func manualCrosswalkSet(_ context.Context, s *store.Store, entityType, sourceSystem string) (map[string]bool, error) {
	rows, err := s.ListCrosswalk(entityType, sourceSystem)
	if err != nil {
		return nil, fmt.Errorf("loading manual crosswalk: %w", err)
	}
	m := map[string]bool{}
	for _, r := range rows {
		if r.Method == "manual" {
			m[r.SourceValue] = true
		}
	}
	return m, nil
}

// countDistinctCanonicals returns the number of distinct canonical IDs in the
// map. When fuzzy is off, this equals the number of distinct canonical forms.
func countDistinctCanonicals(canonToID map[string]string, _ bool) int {
	seen := map[string]bool{}
	for _, id := range canonToID {
		seen[id] = true
	}
	return len(seen)
}
