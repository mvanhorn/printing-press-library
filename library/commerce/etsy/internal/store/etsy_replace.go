package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// ReplaceParentPartition replaces supplied parent partitions in one transaction.
// IDs are compared in Go because composite storage IDs can contain NUL bytes.
func (s *Store) ReplaceParentPartition(resourceType, typedTable string, parentIDs []string, items []json.RawMessage) (stored, extractFailures, typedFailures, deleted int, err error) {
	if len(parentIDs) == 0 {
		return
	}
	parents := make(map[string]bool, len(parentIDs))
	for _, id := range parentIDs {
		parents[id] = true
	}
	// etsy-ext: Narrow ordinary partitions in SQL; retain decoding for NUL parents.
	fullScan := false
	for _, id := range parentIDs {
		fullScan = fullScan || strings.ContainsRune(id, '\x00')
	}
	stored, extractFailures, typedFailures, err = s.upsertBatchDetailedPre(resourceType, items, func(tx *sql.Tx) error {
		var victims []string
		seen := make(map[string]bool)
		collect := func(query string, args ...any) error {
			rows, e := tx.Query(query, args...)
			if e != nil {
				return e
			}
			defer rows.Close()
			for rows.Next() {
				var id, parent string
				if fullScan {
					var raw []byte
					if e = rows.Scan(&id, &raw); e != nil {
						return e
					}
					obj, decodeErr := DecodeJSONObject(raw)
					if decodeErr != nil {
						continue
					}
					parent = fmt.Sprint(obj["parent_id"])
				} else if e = rows.Scan(&id, &parent); e != nil {
					return e
				}
				// SQLite text comparisons can admit candidates requiring an exact Go check.
				if parents[parent] && !seen[id] {
					seen[id] = true
					victims = append(victims, id)
				}
			}
			return rows.Err()
		}
		var e error
		if fullScan {
			e = collect("SELECT id,data FROM resources WHERE resource_type=?", resourceType)
		} else {
			for start := 0; start < len(parentIDs); start += 500 {
				end := start + 500
				if end > len(parentIDs) {
					end = len(parentIDs)
				}
				args := []any{resourceType}
				for _, id := range parentIDs[start:end] {
					args = append(args, id)
				}
				query := "SELECT id, CAST(json_extract(data,'$.parent_id') AS TEXT) FROM resources WHERE resource_type=? AND CASE WHEN json_valid(data) THEN CAST(json_extract(data,'$.parent_id') AS TEXT) END IN (" + strings.TrimSuffix(strings.Repeat("?,", end-start), ",") + ")"
				if e = collect(query, args...); e != nil {
					break
				}
			}
		}
		if e != nil {
			return e
		}
		for _, id := range victims {
			if typedTable != "" {
				// Quote the table identifier; callers supply the typed projection name.
				table := "\""
				for _, c := range typedTable {
					if c == '"' {
						table += "\""
					}
					table += string(c)
				}
				table += "\""
				if _, e = tx.Exec("DELETE FROM "+table+" WHERE id=?", id); e != nil {
					return e
				}
			}
			if _, e = tx.Exec("DELETE FROM resources_fts WHERE rowid=?", ftsRowID(resourceType, id)); e != nil {
				return e
			}
			if _, e = tx.Exec("DELETE FROM resources WHERE resource_type=? AND id=?", resourceType, id); e != nil {
				return e
			}
			deleted++
		}
		return nil
	})
	if err != nil {
		deleted = 0
	}
	return
}
