// Copyright 2026 Ricardo Cabral and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored, not generator output. See
// .printing-press-patches/unifi-console-keyed-mirror.json.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

// consoleKeyMarker extends the existing NUL-composite storage key, so
// BareResourceID and the reconcile logic keep working unchanged: the bare id is
// still everything before the first NUL.
const consoleKeyMarker = "\x00console="

// ConsoleIDField is stamped into every console-hosted row so SQL and JSON
// consumers can filter without decoding the storage key.
const ConsoleIDField = "_console_id"

var activeConsole atomic.Value

// SetActiveConsole scopes this process's mirror reads and writes to one console.
// A package variable rather than a Store field because resourceStorageID is a
// free function with many generated call sites, and one CLI process only ever
// targets one console. Fan-out commands read live and never write the mirror.
func SetActiveConsole(key string) { activeConsole.Store(strings.TrimSpace(key)) }

// ActiveConsole returns the scope, or "" when none is set.
func ActiveConsole() string {
	if v, ok := activeConsole.Load().(string); ok {
		return v
	}
	return ""
}

// IsConsoleHostedResource: Site Manager resources (hosts, sites, devices,
// sd-wan-configs, carrier, mobility) are account-wide and stay unkeyed.
func IsConsoleHostedResource(resourceType string) bool {
	return resourceType == "network" || resourceType == "protect" || resourceType == "innerspace" ||
		strings.HasPrefix(resourceType, "network-") || strings.HasPrefix(resourceType, "network_") ||
		strings.HasPrefix(resourceType, "protect-") || strings.HasPrefix(resourceType, "protect_") ||
		strings.HasPrefix(resourceType, "innerspace-") || strings.HasPrefix(resourceType, "innerspace_")
}

func consoleKeyedStorageID(resourceType, storageID string) string {
	console := ActiveConsole()
	if console == "" || !IsConsoleHostedResource(resourceType) {
		return storageID
	}
	return storageID + consoleKeyMarker + console
}

// ConsoleOfStorageID returns the console a row belongs to, "" for unkeyed rows.
func ConsoleOfStorageID(storageID string) string {
	if i := strings.Index(storageID, consoleKeyMarker); i >= 0 {
		return storageID[i+len(consoleKeyMarker):]
	}
	return ""
}

// StripConsoleKey removes only the console suffix, keeping any parent component.
func StripConsoleKey(storageID string) string {
	if i := strings.Index(storageID, consoleKeyMarker); i >= 0 {
		return storageID[:i]
	}
	return storageID
}

// InActiveConsole keeps unkeyed rows (Site Manager, and mirrors written with no
// console) visible everywhere, and hides rows of other consoles only when a
// scope is set. With no scope, cross-console views see everything.
func InActiveConsole(storageID string) bool {
	active := ActiveConsole()
	if active == "" {
		return true
	}
	console := ConsoleOfStorageID(storageID)
	return console == "" || console == active
}

// RowInActiveConsole is InActiveConsole for callers that hold row JSON, not its key.
func RowInActiveConsole(data json.RawMessage) bool {
	active := ActiveConsole()
	if active == "" {
		return true
	}
	var row struct {
		Console string `json:"_console_id"`
	}
	if json.Unmarshal(data, &row) != nil || row.Console == "" {
		return true
	}
	return row.Console == active
}

func stampConsoleID(storageID string, data json.RawMessage) json.RawMessage {
	console := ConsoleOfStorageID(storageID)
	if console == "" {
		return data
	}
	var obj map[string]json.RawMessage
	if json.Unmarshal(data, &obj) != nil || obj == nil {
		return data
	}
	encoded, err := json.Marshal(console)
	if err != nil {
		return data
	}
	obj[ConsoleIDField] = encoded
	out, err := json.Marshal(obj)
	if err != nil {
		return data
	}
	return out
}

// GetInActiveConsole resolves a bare API id to its row. Go-side matching, because
// SQLite string functions truncate at the NUL inside composite keys.
func (s *Store) GetInActiveConsole(resourceType, id string) (json.RawMessage, error) {
	if data, err := s.Get(resourceType, id); err == nil {
		return data, nil
	}
	ids, err := s.ListIDs(resourceType)
	if err != nil {
		return nil, err
	}
	for _, storageID := range ids {
		if BareResourceID(storageID) == id && InActiveConsole(storageID) {
			return s.Get(resourceType, storageID)
		}
	}
	return nil, sql.ErrNoRows
}

// DeleteResource removes one entity from the generic table, FTS and typed table
// after a confirmed live delete. Scoped to the active console and, when parentID
// is given, to that parent, because the same bare id can exist under both.
func (s *Store) DeleteResource(resourceType, id, parentID string) (int, error) {
	ids, err := s.ListIDs(resourceType)
	if err != nil {
		return 0, err
	}
	victims := make([]string, 0, 1)
	for _, storageID := range ids {
		if BareResourceID(storageID) != id || !InActiveConsole(storageID) {
			continue
		}
		if parentID != "" {
			rest := StripConsoleKey(storageID)
			if i := strings.IndexByte(rest, 0); i >= 0 && rest[i+1:] != parentID {
				continue
			}
		}
		victims = append(victims, storageID)
	}
	if len(victims) == 0 {
		return 0, nil
	}
	s.lockForWrite()
	defer s.unlockAfterWrite()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	typedTable, hasTyped := TypedListTable(resourceType)
	for _, storageID := range victims {
		if _, err := tx.Exec(`DELETE FROM resources WHERE resource_type = ? AND id = ?`, resourceType, storageID); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM resources_fts WHERE rowid = ?`, ftsRowID(resourceType, storageID)); err != nil {
			fmt.Fprintf(os.Stderr, "warning: FTS index cleanup failed for %s: %v\n", resourceType, err)
		}
		if hasTyped {
			if _, err := tx.Exec(fmt.Sprintf(`DELETE FROM "%s" WHERE id = ?`, typedTable), storageID); err != nil {
				fmt.Fprintf(os.Stderr, "warning: typed table cleanup failed for %s: %v\n", typedTable, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(victims), nil
}
