package store

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

type Store struct {
	DB *sql.DB
}

const StoreSchemaVersion = "1"

func Open(dbPath string) (*Store, error) {
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(home, ".config", "homeassistant-pp-cli", "store.db")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{DB: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	query := `
CREATE TABLE IF NOT EXISTS states (
	entity_id TEXT PRIMARY KEY,
	state TEXT,
	attributes JSON,
	last_changed DATETIME,
	last_updated DATETIME
);
CREATE TABLE IF NOT EXISTS sync_state (
	key TEXT PRIMARY KEY,
	value TEXT
);
CREATE VIRTUAL TABLE IF NOT EXISTS states_fts USING fts5(
	entity_id,
	state,
	attributes,
	content='states',
	content_rowid='rowid'
);
CREATE TRIGGER IF NOT EXISTS states_ai AFTER INSERT ON states BEGIN
	INSERT INTO states_fts(rowid, entity_id, state, attributes) VALUES (new.rowid, new.entity_id, new.state, new.attributes);
END;
CREATE TRIGGER IF NOT EXISTS states_ad AFTER DELETE ON states BEGIN
	INSERT INTO states_fts(states_fts, rowid, entity_id, state, attributes) VALUES ('delete', old.rowid, old.entity_id, old.state, old.attributes);
END;
CREATE TRIGGER IF NOT EXISTS states_au AFTER UPDATE ON states BEGIN
	INSERT INTO states_fts(states_fts, rowid, entity_id, state, attributes) VALUES ('delete', old.rowid, old.entity_id, old.state, old.attributes);
	INSERT INTO states_fts(rowid, entity_id, state, attributes) VALUES (new.rowid, new.entity_id, new.state, new.attributes);
END;
`
	_, err := s.DB.Exec(query)
	if err != nil {
		return err
	}
	_, err = s.DB.Exec("PRAGMA user_version = " + StoreSchemaVersion)
	return err
}

type State struct {
	EntityID    string          `json:"entity_id"`
	State       string          `json:"state"`
	Attributes  json.RawMessage `json:"attributes"` // Stored as JSON string
	LastChanged string          `json:"last_changed"`
	LastUpdated string          `json:"last_updated"`
}

func (s *Store) UpsertStateBatch(states []State) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
INSERT INTO states (entity_id, state, attributes, last_changed, last_updated)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(entity_id) DO UPDATE SET
	state=excluded.state,
	attributes=excluded.attributes,
	last_changed=excluded.last_changed,
	last_updated=excluded.last_updated
`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, st := range states {
		attrBytes, err := st.Attributes.MarshalJSON()
		if err != nil {
			attrBytes = []byte("{}")
		}
		if string(attrBytes) == "null" {
			attrBytes = []byte("{}")
		}
		_, err = stmt.Exec(st.EntityID, st.State, string(attrBytes), st.LastChanged, st.LastUpdated)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) SearchStates(query string) ([]State, error) {
	stmt := `
SELECT s.entity_id, s.state, s.attributes, s.last_changed, s.last_updated
FROM states s
JOIN states_fts f ON s.rowid = f.rowid
WHERE states_fts MATCH ?
ORDER BY rank
`
	rows, err := s.DB.Query(stmt, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []State
	for rows.Next() {
		var st State
		var attrs string
		if err := rows.Scan(&st.EntityID, &st.State, &attrs, &st.LastChanged, &st.LastUpdated); err != nil {
			return nil, err
		}
		st.Attributes = json.RawMessage(attrs)
		results = append(results, st)
	}
	return results, nil
}

func (s *Store) GetState(entityID string) (*State, error) {
	row := s.DB.QueryRow(`SELECT entity_id, state, attributes, last_changed, last_updated FROM states WHERE entity_id = ?`, entityID)
	var st State
	var attrs string
	if err := row.Scan(&st.EntityID, &st.State, &attrs, &st.LastChanged, &st.LastUpdated); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	st.Attributes = json.RawMessage(attrs)
	return &st, nil
}

func (s *Store) GetAllStates() ([]State, error) {
	stmt := `
SELECT entity_id, state, attributes, last_changed, last_updated
FROM states
ORDER BY entity_id
`
	rows, err := s.DB.Query(stmt)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []State
	for rows.Next() {
		var st State
		var attrs string
		if err := rows.Scan(&st.EntityID, &st.State, &attrs, &st.LastChanged, &st.LastUpdated); err != nil {
			return nil, err
		}
		st.Attributes = json.RawMessage(attrs)
		results = append(results, st)
	}
	return results, nil
}

func (s *Store) StateCount() (int, error) {
	var count int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM states`).Scan(&count)
	return count, err
}

// SetMeta is an alias for SaveSyncState for backward compatibility
func (s *Store) SetMeta(key, value string) error {
	return s.SaveSyncState(key, value)
}

// SaveSyncState records a piece of metadata in the sync_state table.
func (s *Store) SaveSyncState(key, value string) error {
	_, err := s.DB.Exec(`INSERT INTO sync_state (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// GetMeta is an alias for GetSyncState for backward compatibility
func (s *Store) GetMeta(key string) (string, error) {
	return s.GetSyncState(key)
}

// GetSyncState retrieves a piece of metadata from the sync_state table.
func (s *Store) GetSyncState(key string) (string, error) {
	var value string
	err := s.DB.QueryRow(`SELECT value FROM sync_state WHERE key = ?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

// SaveSyncCursor records a pagination cursor.
func (s *Store) SaveSyncCursor(resource, cursor string) error {
	return s.SaveSyncState("cursor:"+resource, cursor)
}

// GetSyncCursor retrieves a pagination cursor.
func (s *Store) GetSyncCursor(resource string) (string, error) {
	return s.GetSyncState("cursor:" + resource)
}

