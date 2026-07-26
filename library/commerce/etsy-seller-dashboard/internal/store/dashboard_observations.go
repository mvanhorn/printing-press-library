// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// DashboardObservation is one prepared read-only dashboard snapshot.
type DashboardObservation struct {
	ID    string
	Label string
	Data  json.RawMessage
}

// ReplaceDashboardObservations atomically replaces selected observation
// labels. Any decode or insert error rolls the deletion back.
func (s *Store) ReplaceDashboardObservations(
	ctx context.Context,
	resource string,
	labels []string,
	observations []DashboardObservation,
) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	domainTable := map[string]string{
		"marketplace-insights": "marketplace_insights",
		"offsite-ads":          "offsite_ads",
		"promotions":           "promotions",
	}[resource]
	for _, label := range labels {
		if domainTable != "" {
			query := fmt.Sprintf(
				`DELETE FROM %s WHERE id IN (
					SELECT id FROM resources
					WHERE resource_type = ? AND json_extract(data, '$._observation_type') = ?
				)`,
				domainTable,
			)
			if _, err := tx.ExecContext(ctx, query, resource, label); err != nil {
				return fmt.Errorf("clearing prior %s observations: %w", label, err)
			}
		}
		if _, err := tx.ExecContext(
			ctx,
			`DELETE FROM resources
			 WHERE resource_type = ? AND json_extract(data, '$._observation_type') = ?`,
			resource,
			label,
		); err != nil {
			return fmt.Errorf("clearing prior %s observations: %w", label, err)
		}
	}

	for _, observation := range observations {
		if err := s.upsertDashboardObservationTx(tx, resource, observation); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) upsertDashboardObservationTx(
	tx *sql.Tx,
	resource string,
	observation DashboardObservation,
) error {
	obj, err := DecodeJSONObject(observation.Data)
	if err != nil {
		return fmt.Errorf("decoding %s observation: %w", observation.Label, err)
	}
	identifier := observation.ID
	if identifier == "" {
		identifier = extractObjectID(obj)
	}
	if identifier == "" {
		return fmt.Errorf("missing id for %s observation", observation.Label)
	}

	switch resource {
	case "marketplace-insights":
		storageID := resourceStorageID(resource, identifier, obj)
		if err := s.upsertGenericResourceTx(tx, resource, storageID, observation.Data); err != nil {
			return err
		}
		return s.upsertMarketplaceInsightsTx(tx, storageID, obj, observation.Data)
	case "offsite-ads":
		storageID := resourceStorageID(resource, identifier, obj)
		if err := s.upsertGenericResourceTx(tx, resource, storageID, observation.Data); err != nil {
			return err
		}
		return s.upsertOffsiteAdsTx(tx, storageID, obj, observation.Data)
	case "promotions":
		storageID := resourceStorageID(resource, identifier, obj)
		if err := s.upsertGenericResourceTx(tx, resource, storageID, observation.Data); err != nil {
			return err
		}
		return s.upsertPromotionsTx(tx, storageID, obj, observation.Data)
	default:
		return s.upsertGenericResourceTx(tx, resource, identifier, observation.Data)
	}
}
