package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// migrateInventoryLinkIDs repairs inventory links written by older releases
// under their display name or slug. It leaves full VIN-bearing records and
// every other resource type untouched. The caller holds the migration lock.
func migrateInventoryLinkIDs(ctx context.Context, conn *sql.Conn) error {
	rows, err := conn.QueryContext(ctx, `SELECT id, data FROM resources WHERE resource_type = 'inventory'`)
	if err != nil {
		return err
	}
	type legacyLink struct {
		id, vin, data string
		fields        map[string]any
	}
	var links []legacyLink
	seenIDs := make(map[string]map[string]struct{})
	for rows.Next() {
		var id, data string
		if err := rows.Scan(&id, &data); err != nil {
			_ = rows.Close()
			return err
		}
		obj, err := DecodeJSONObject(json.RawMessage(data))
		if err != nil {
			continue
		}
		vin := inventoryLinkVIN(obj)
		if vin == "" {
			vin = ResourceIDString(obj["vin"])
		}
		if vin != "" {
			for _, key := range genericDescriptiveIDFieldFallbacks {
				if descriptiveID := ResourceIDString(obj[key]); descriptiveID != "" {
					if seenIDs[descriptiveID] == nil {
						seenIDs[descriptiveID] = make(map[string]struct{})
					}
					seenIDs[descriptiveID][vin] = struct{}{}
				}
			}
		}
		if isLegacyInventoryLinkID(id, obj) {
			links = append(links, legacyLink{id: id, vin: vin, data: data, fields: obj})
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, link := range links {
		if len(seenIDs[link.id]) > 1 {
			// A display name reused by several VINs has no safe target.
			continue
		}
		// An unscoped taught reference cannot be attributed to inventory.
		// Leave its old row in place rather than strand the learned target.
		var unscoped int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM search_learnings WHERE resource_id = ? AND COALESCE(resource_type, '') = ''`,
			link.id).Scan(&unscoped); err != nil {
			return fmt.Errorf("checking unscoped learned references: %w", err)
		}
		if unscoped > 0 {
			continue
		}
		var existingAlias string
		err := conn.QueryRowContext(ctx,
			`SELECT new_id FROM resource_id_aliases WHERE resource_type = 'inventory' AND old_id = ?`,
			link.id).Scan(&existingAlias)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("checking inventory ID alias: %w", err)
		}
		if existingAlias != "" && existingAlias != link.vin {
			continue
		}

		var targetData string
		err = conn.QueryRowContext(ctx,
			`SELECT data FROM resources WHERE resource_type = 'inventory' AND id = ?`, link.vin).Scan(&targetData)
		switch err {
		case nil:
			// A newer VIN-keyed record wins. Refresh the listing fields on a
			// full detail record without removing its vehicle fields.
			merged, err := mergeListingFieldsIntoDetail(link.vin, json.RawMessage(targetData), link.fields)
			if err != nil {
				return fmt.Errorf("preserving VIN inventory detail: %w", err)
			}
			if string(merged) != targetData {
				targetData = string(merged)
				if _, err := conn.ExecContext(ctx,
					`UPDATE resources SET data = ? WHERE resource_type = 'inventory' AND id = ?`, targetData, link.vin); err != nil {
					return fmt.Errorf("updating VIN inventory detail: %w", err)
				}
			}
			if _, err := conn.ExecContext(ctx,
				`DELETE FROM resources WHERE resource_type = 'inventory' AND id = ?`, link.id); err != nil {
				return fmt.Errorf("deleting legacy inventory link: %w", err)
			}
		case sql.ErrNoRows:
			// Preserve the original data and timestamps while changing the key.
			if _, err := conn.ExecContext(ctx,
				`UPDATE resources SET id = ? WHERE resource_type = 'inventory' AND id = ?`, link.vin, link.id); err != nil {
				return fmt.Errorf("rekeying legacy inventory link: %w", err)
			}
			targetData = link.data
		default:
			return fmt.Errorf("reading VIN-keyed inventory row: %w", err)
		}

		// Generic resources_fts is maintained manually, not by a trigger.
		if _, err := conn.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID("inventory", link.id)); err != nil {
			return fmt.Errorf("removing legacy inventory search row: %w", err)
		}
		if _, err := conn.ExecContext(ctx, `DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID("inventory", link.vin)); err != nil {
			return fmt.Errorf("replacing VIN inventory search row: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO resources_fts (rowid, id, resource_type, content) VALUES (?, ?, 'inventory', ?)`,
			ftsRowID("inventory", link.vin), link.vin, searchableResourceContent(json.RawMessage(targetData))); err != nil {
			return fmt.Errorf("indexing VIN inventory row: %w", err)
		}
		if _, err := conn.ExecContext(ctx,
			`INSERT INTO resource_id_aliases (resource_type, old_id, new_id)
			 VALUES ('inventory', ?, ?) ON CONFLICT (resource_type, old_id) DO NOTHING`,
			link.id, link.vin); err != nil {
			return fmt.Errorf("recording inventory ID alias: %w", err)
		}
	}
	return nil
}

func isLegacyInventoryLinkID(id string, obj map[string]any) bool {
	vin := inventoryLinkVIN(obj)
	if vin == "" || id == vin {
		return false
	}
	if _, hasVIN := obj["vin"]; hasVIN {
		return false
	}
	for _, key := range genericIDFieldFallbacks {
		if value := ResourceIDString(obj[key]); value != "" {
			return false
		}
	}
	if suffixIDFieldFallback("inventory", obj) != "" {
		return false
	}
	for _, key := range genericDescriptiveIDFieldFallbacks {
		if value := ResourceIDString(obj[key]); value != "" {
			if id == value {
				return true
			}
		}
	}
	return false
}

// A list sync carries only a link. Preserve a full detail response already
// cached under the same VIN while refreshing its current listing fields.
func mergeInventoryLinkWithDetailTx(tx *sql.Tx, resourceType, id string, incoming map[string]any, item json.RawMessage) (json.RawMessage, error) {
	if resourceType != "inventory" || inventoryLinkVIN(incoming) != id {
		return item, nil
	}
	if _, hasVIN := incoming["vin"]; hasVIN {
		return item, nil
	}
	var raw string
	err := tx.QueryRow(`SELECT data FROM resources WHERE resource_type = 'inventory' AND id = ?`, id).Scan(&raw)
	if err == sql.ErrNoRows {
		return item, nil
	}
	if err != nil {
		return nil, err
	}
	return mergeListingFieldsIntoDetail(id, json.RawMessage(raw), incoming)
}

func mergeListingFieldsIntoDetail(id string, existing json.RawMessage, link map[string]any) (json.RawMessage, error) {
	full, err := DecodeJSONObject(existing)
	if err != nil {
		return nil, err
	}
	if ResourceIDString(full["vin"]) != id {
		return existing, nil
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(existing, &fields); err != nil {
		return nil, err
	}
	for _, key := range []string{"url", "name", "slug", "text", "image", "rank"} {
		value, ok := link[key]
		if !ok || value == nil {
			continue
		}
		if key == "image" {
			if image, ok := value.(string); ok && image == "" {
				continue
			}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		fields[key] = raw
	}
	return json.Marshal(fields)
}
