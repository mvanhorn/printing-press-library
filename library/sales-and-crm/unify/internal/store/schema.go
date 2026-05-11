package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Object holds the API's object descriptor as we cache it.
type Object struct {
	APIName     string         `json:"api_name"`
	Provider    string         `json:"provider"`
	Category    string         `json:"category"`
	DisplayName string         `json:"display_name,omitempty"`
	Description string         `json:"description,omitempty"`
	Raw         map[string]any `json:"-"`
}

// Attribute holds the API's attribute descriptor.
type Attribute struct {
	ObjectName  string         `json:"object_name"`
	APIName     string         `json:"api_name"`
	Type        string         `json:"type,omitempty"`
	DisplayName string         `json:"display_name,omitempty"`
	Description string         `json:"description,omitempty"`
	IsUnique    bool           `json:"is_unique,omitempty"`
	Raw         map[string]any `json:"-"`
}

// AttributeOption holds the API's attribute-option descriptor.
type AttributeOption struct {
	ObjectName    string         `json:"object_name"`
	AttributeName string         `json:"attribute_name"`
	APIName       string         `json:"api_name"`
	DisplayName   string         `json:"display_name,omitempty"`
	Raw           map[string]any `json:"-"`
}

// UpsertObject writes an object descriptor to the objects table.
func (s *Store) UpsertObject(ctx context.Context, o Object) error {
	blob, err := json.Marshal(o.Raw)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT OR REPLACE INTO objects (api_name, provider, category, display_name, description, data, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.APIName, o.Provider, o.Category, o.DisplayName, o.Description, string(blob), time.Now().Unix())
	return err
}

// UpsertAttribute writes an attribute descriptor.
func (s *Store) UpsertAttribute(ctx context.Context, a Attribute) error {
	blob, err := json.Marshal(a.Raw)
	if err != nil {
		return err
	}
	unique := 0
	if a.IsUnique {
		unique = 1
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT OR REPLACE INTO attributes (object_name, api_name, type, display_name, description, is_unique, data, synced_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ObjectName, a.APIName, a.Type, a.DisplayName, a.Description, unique, string(blob), time.Now().Unix())
	return err
}

// UpsertAttributeOption writes an attribute-option descriptor.
func (s *Store) UpsertAttributeOption(ctx context.Context, o AttributeOption) error {
	blob, err := json.Marshal(o.Raw)
	if err != nil {
		return err
	}
	_, err = s.DB.ExecContext(ctx,
		`INSERT OR REPLACE INTO attribute_options (object_name, attribute_name, option_name, display_name, data, synced_at) VALUES (?, ?, ?, ?, ?, ?)`,
		o.ObjectName, o.AttributeName, o.APIName, o.DisplayName, string(blob), time.Now().Unix())
	return err
}

// ListObjects returns every cached object.
func (s *Store) ListObjects(ctx context.Context) ([]Object, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT api_name, provider, category, display_name, description, data FROM objects ORDER BY api_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Object
	for rows.Next() {
		var o Object
		var blob string
		if err := rows.Scan(&o.APIName, &o.Provider, &o.Category, &o.DisplayName, &o.Description, &blob); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(blob), &o.Raw)
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListAttributes returns every cached attribute for one object.
func (s *Store) ListAttributes(ctx context.Context, objectName string) ([]Attribute, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT object_name, api_name, type, display_name, description, is_unique, data FROM attributes WHERE object_name = ? ORDER BY api_name`, objectName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attribute
	for rows.Next() {
		var a Attribute
		var unique int
		var blob string
		if err := rows.Scan(&a.ObjectName, &a.APIName, &a.Type, &a.DisplayName, &a.Description, &unique, &blob); err != nil {
			return nil, err
		}
		a.IsUnique = unique == 1
		_ = json.Unmarshal([]byte(blob), &a.Raw)
		out = append(out, a)
	}
	return out, rows.Err()
}

// Snapshot writes a full schema dump (objects + attributes + options) to
// schema_snapshots. Returns the new snapshot id.
func (s *Store) Snapshot(ctx context.Context, label string) (int64, error) {
	objects, err := s.ListObjects(ctx)
	if err != nil {
		return 0, err
	}
	doc := map[string]any{
		"taken_at": time.Now().UTC().Format(time.RFC3339),
		"objects":  objects,
		"attrs":    map[string][]Attribute{},
	}
	attrByObj := map[string][]Attribute{}
	for _, o := range objects {
		attrs, err := s.ListAttributes(ctx, o.APIName)
		if err != nil {
			return 0, err
		}
		attrByObj[o.APIName] = attrs
	}
	doc["attrs"] = attrByObj
	blob, err := json.Marshal(doc)
	if err != nil {
		return 0, err
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO schema_snapshots (label, taken_at, data) VALUES (?, ?, ?)`, label, time.Now().Unix(), string(blob))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// SnapshotRow is one row of the schema_snapshots table.
type SnapshotRow struct {
	ID      int64           `json:"id"`
	Label   string          `json:"label,omitempty"`
	TakenAt int64           `json:"taken_at"`
	Data    json.RawMessage `json:"data"`
}

// LatestSnapshots returns the N most-recent snapshots, newest first.
func (s *Store) LatestSnapshots(ctx context.Context, n int) ([]SnapshotRow, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, IFNULL(label,''), taken_at, data FROM schema_snapshots ORDER BY taken_at DESC, id DESC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SnapshotRow
	for rows.Next() {
		var r SnapshotRow
		var data string
		if err := rows.Scan(&r.ID, &r.Label, &r.TakenAt, &data); err != nil {
			return nil, err
		}
		r.Data = json.RawMessage(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetSnapshot returns one snapshot by id.
func (s *Store) GetSnapshot(ctx context.Context, id int64) (*SnapshotRow, error) {
	var r SnapshotRow
	var data string
	err := s.DB.QueryRowContext(ctx, `SELECT id, IFNULL(label,''), taken_at, data FROM schema_snapshots WHERE id = ?`, id).Scan(&r.ID, &r.Label, &r.TakenAt, &data)
	if err != nil {
		return nil, fmt.Errorf("snapshot %d: %w", id, err)
	}
	r.Data = json.RawMessage(data)
	return &r, nil
}
