// Copyright 2026 Hunter Veltri and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

// Scryfall alternate route keys are indexed separately from the primary UUID.
// A key can point to more than one card, so callers must still reject ambiguous
// matches. All values are folded for lookup and verified against source JSON.
func upsertScryfallAlternateKeys(tx *sql.Tx, resourceType, id string, data json.RawMessage) error {
	if _, err := tx.Exec(`DELETE FROM resource_alt_keys WHERE resource_type = ? AND resource_id = ?`, resourceType, id); err != nil {
		return err
	}
	object, err := DecodeJSONObject(data)
	if err != nil {
		return nil
	}
	add := func(kind, value string) error {
		if value == "" {
			return nil
		}
		_, err := tx.Exec(`INSERT OR IGNORE INTO resource_alt_keys(resource_type,kind,value,resource_id) VALUES(?,?,?,?)`, resourceType, kind, strings.ToLower(value), id)
		return err
	}
	if resourceType == "cards" {
		for _, field := range []string{"name", "arena_id", "mtgo_id", "tcgplayer_id", "cardmarket_id"} {
			if err := add(field, ResourceIDString(object[field])); err != nil {
				return err
			}
		}
		if values, ok := object["multiverse_ids"].([]any); ok {
			for _, value := range values {
				if err := add("multiverse_ids", ResourceIDString(value)); err != nil {
					return err
				}
			}
		}
		if set, collector := ResourceIDString(object["set"]), ResourceIDString(object["collector_number"]); set != "" && collector != "" {
			if err := add("set_collector", set+"\x00"+collector); err != nil {
				return err
			}
		}
	} else if resourceType == "sets" {
		for _, field := range []string{"code", "mtgo_code"} {
			if err := add("code", ResourceIDString(object[field])); err != nil {
				return err
			}
		}
		if err := add("tcgplayer_id", ResourceIDString(object["tcgplayer_id"])); err != nil {
			return err
		}
	}
	return nil
}

// Backfill legacy mirrors once on upgrade. INSERT OR IGNORE keeps repeated
// aliases within one resource harmless, and the existing transaction makes the
// schema version and index contents visible atomically to readers.
func migrateScryfallAlternateKeys(ctx context.Context, conn *sql.Conn) error {
	statements := []string{
		`INSERT OR IGNORE INTO resource_alt_keys(resource_type,kind,value,resource_id)
		 SELECT r.resource_type,
		  CASE WHEN r.resource_type = 'sets' AND j.key IN ('code','mtgo_code') THEN 'code' ELSE j.key END,
		  lower(CAST(j.value AS TEXT)), r.id
		 FROM resources r, json_each(CASE WHEN json_valid(r.data) THEN r.data ELSE '{}' END) j
		 WHERE ((r.resource_type = 'cards' AND j.key IN ('name','arena_id','mtgo_id','tcgplayer_id','cardmarket_id'))
		     OR (r.resource_type = 'sets' AND j.key IN ('code','mtgo_code','tcgplayer_id')))
		   AND j.type IN ('text','integer','real') AND CAST(j.value AS TEXT) <> ''`,
		`INSERT OR IGNORE INTO resource_alt_keys(resource_type,kind,value,resource_id)
		 SELECT 'cards','multiverse_ids',lower(CAST(j.value AS TEXT)),r.id
		 FROM resources r, json_each(CASE WHEN json_valid(r.data) THEN
		  CASE WHEN json_type(r.data,'$.multiverse_ids') = 'array' THEN json_extract(r.data,'$.multiverse_ids') ELSE '[]' END
		  ELSE '[]' END) j
		 WHERE r.resource_type = 'cards' AND j.type IN ('text','integer','real') AND CAST(j.value AS TEXT) <> ''`,
		`INSERT OR IGNORE INTO resource_alt_keys(resource_type,kind,value,resource_id)
		 SELECT 'cards','set_collector',lower(CAST(json_extract(d.data,'$.set') AS TEXT)) || char(0) || lower(CAST(json_extract(d.data,'$.collector_number') AS TEXT)),d.id
		 FROM (SELECT id, CASE WHEN json_valid(data) THEN data ELSE '{}' END AS data FROM resources WHERE resource_type = 'cards') d
		 WHERE json_type(d.data,'$.set') = 'text' AND json_type(d.data,'$.collector_number') IN ('text','integer','real')
		  AND json_extract(d.data,'$.set') <> '' AND CAST(json_extract(d.data,'$.collector_number') AS TEXT) <> ''`,
	}
	for _, statement := range statements {
		if _, err := conn.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}
