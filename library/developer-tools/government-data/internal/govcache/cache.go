// Package govcache stores immutable, explicitly scoped catalog snapshots.
package govcache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/govdata"
)

func Init(ctx context.Context, db *sql.DB) error {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS gov_snapshots(id TEXT PRIMARY KEY, scope_json TEXT NOT NULL, captured_at TEXT NOT NULL); CREATE TABLE IF NOT EXISTS gov_snapshot_records(snapshot_id TEXT NOT NULL REFERENCES gov_snapshots(id),source TEXT NOT NULL,dataset_id TEXT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(snapshot_id,source,dataset_id));`)
	return err
}
func Save(ctx context.Context, db *sql.DB, s govdata.Snapshot) error {
	if strings.TrimSpace(s.ID) == "" || s.CapturedAt == "" {
		return fmt.Errorf("snapshot requires a nonempty ID and captured_at timestamp")
	}
	if err := Init(ctx, db); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, d := range s.Datasets {
		if d.Source != "california" && d.Source != "federal" {
			return fmt.Errorf("snapshot accepts catalog sources only, got %q", d.Source)
		}
		if d.ID == "" {
			return fmt.Errorf("dataset ID is empty")
		}
		k := Key(d)
		if seen[k] {
			return fmt.Errorf("duplicate snapshot dataset %s", k)
		}
		seen[k] = true
	}
	header := s
	header.Datasets = nil
	b, err := json.Marshal(header)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `INSERT INTO gov_snapshots(id,scope_json,captured_at) VALUES(?,?,?)`, s.ID, string(b), s.CapturedAt); err != nil {
		return fmt.Errorf("save snapshot %q (IDs are immutable; choose a new --save ID): %w", s.ID, err)
	}
	for _, d := range s.Datasets {
		b, err = json.Marshal(d)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO gov_snapshot_records(snapshot_id,source,dataset_id,payload) VALUES(?,?,?,?)`, s.ID, d.Source, d.ID, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit()
}
func Load(ctx context.Context, db *sql.DB, id string) (govdata.Snapshot, error) {
	s := govdata.Snapshot{}
	var b string
	if err := db.QueryRowContext(ctx, `SELECT scope_json FROM gov_snapshots WHERE id=?`, id).Scan(&b); err != nil {
		return s, fmt.Errorf("snapshot %q unavailable; create it with 'catalog find water --source both --save %s': %w", id, id, err)
	}
	if err := json.Unmarshal([]byte(b), &s); err != nil {
		return s, err
	}
	s.Datasets = []govdata.Dataset{}
	rows, err := db.QueryContext(ctx, `SELECT payload FROM gov_snapshot_records WHERE snapshot_id=? ORDER BY source,dataset_id`, id)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return s, err
		}
		var d govdata.Dataset
		if err = json.Unmarshal([]byte(raw), &d); err != nil {
			return s, err
		}
		s.Datasets = append(s.Datasets, d)
	}
	return s, rows.Err()
}
func List(ctx context.Context, db *sql.DB) ([]govdata.Snapshot, error) {
	rows, err := db.QueryContext(ctx, `SELECT scope_json FROM gov_snapshots ORDER BY captured_at DESC,id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []govdata.Snapshot{}
	for rows.Next() {
		var raw string
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		var s govdata.Snapshot
		if err = json.Unmarshal([]byte(raw), &s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func Find(ctx context.Context, db *sql.DB, q string, limit int) ([]govdata.Dataset, error) {
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be 1..1000")
	}
	rows, err := db.QueryContext(ctx, `SELECT r.payload FROM gov_snapshot_records r JOIN gov_snapshots s ON s.id=r.snapshot_id ORDER BY s.captured_at DESC,r.source,r.dataset_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []govdata.Dataset{}
	seen := map[string]bool{}
	for rows.Next() {
		var b string
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var d govdata.Dataset
		if err = json.Unmarshal([]byte(b), &d); err != nil {
			return nil, err
		}
		k := Key(d)
		if seen[k] {
			continue
		}
		seen[k] = true
		if strings.Contains(strings.ToLower(d.Title+" "+d.Description+" "+d.ID), strings.ToLower(q)) {
			out = append(out, d)
			if len(out) == limit {
				break
			}
		}
	}
	return out, rows.Err()
}
func Key(d govdata.Dataset) string { return d.Source + ":" + d.ID }

// Fingerprint canonicalizes raw JSON keys; if unavailable it fingerprints normalized metadata.
func Fingerprint(d govdata.Dataset) (string, error) {
	var v any = d
	if len(d.Raw) > 0 {
		decoder := json.NewDecoder(bytes.NewReader(d.Raw))
		decoder.UseNumber()
		if err := decoder.Decode(&v); err != nil {
			return "", fmt.Errorf("invalid metadata for %s: %w", Key(d), err)
		}
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

type Reference struct {
	Source     string `json:"source"`
	DatasetID  string `json:"dataset_id"`
	ResourceID string `json:"resource_id"`
}
type Shared struct {
	URL        string      `json:"url"`
	References []Reference `json:"references"`
}

func Overlap(s govdata.Snapshot) []Shared {
	by := map[string][]Reference{}
	for _, d := range s.Datasets {
		for _, r := range d.Resources {
			if r.URL != "" {
				by[r.URL] = append(by[r.URL], Reference{d.Source, d.ID, r.ID})
			}
		}
	}
	out := []Shared{}
	for u, rs := range by {
		sources := map[string]bool{}
		for _, r := range rs {
			sources[r.Source] = true
		}
		if len(sources) > 1 {
			sort.Slice(rs, func(i, j int) bool {
				return rs[i].Source+rs[i].DatasetID+rs[i].ResourceID < rs[j].Source+rs[j].DatasetID+rs[j].ResourceID
			})
			out = append(out, Shared{u, rs})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].URL < out[j].URL })
	return out
}

type FieldChange struct {
	Field  string `json:"field"`
	Before any    `json:"before"`
	After  any    `json:"after"`
}
type Change struct {
	Dataset string        `json:"dataset"`
	Kind    string        `json:"kind"`
	Fields  []FieldChange `json:"fields,omitempty"`
}

func Diff(a, b govdata.Snapshot) []Change {
	am, bm := map[string]govdata.Dataset{}, map[string]govdata.Dataset{}
	keys := map[string]bool{}
	for _, d := range a.Datasets {
		am[Key(d)] = d
		keys[Key(d)] = true
	}
	for _, d := range b.Datasets {
		bm[Key(d)] = d
		keys[Key(d)] = true
	}
	ks := []string{}
	for k := range keys {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	out := []Change{}
	for _, k := range ks {
		x, xok := am[k]
		y, yok := bm[k]
		if !xok {
			out = append(out, Change{Dataset: k, Kind: "added_to_snapshot"})
			continue
		}
		if !yok {
			out = append(out, Change{Dataset: k, Kind: "absent_from_snapshot"})
			continue
		}
		xm, ym := metadata(x), metadata(y)
		fs := []string{}
		fieldSet := map[string]bool{}
		for f := range xm {
			fieldSet[f] = true
		}
		for f := range ym {
			fieldSet[f] = true
		}
		for f := range fieldSet {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		changes := []FieldChange{}
		for _, f := range fs {
			xb, _ := json.Marshal(xm[f])
			yb, _ := json.Marshal(ym[f])
			if string(xb) != string(yb) {
				changes = append(changes, FieldChange{f, xm[f], ym[f]})
			}
		}
		if len(changes) > 0 {
			out = append(out, Change{k, "metadata_changed", changes})
		}
	}
	return out
}
func metadata(d govdata.Dataset) map[string]any {
	resources := append([]govdata.Resource{}, d.Resources...)
	sort.Slice(resources, func(i, j int) bool { return resources[i].ID+resources[i].URL < resources[j].ID+resources[j].URL })
	m := map[string]any{"title": d.Title, "description": d.Description, "publisher": d.Publisher, "license": d.License, "modified": d.Modified, "resources": resources}
	if len(d.Raw) > 0 {
		if h, err := Fingerprint(d); err == nil {
			m["upstream_metadata_sha256"] = h
		}
	}
	return m
}

type Finding struct {
	Dataset    string `json:"dataset"`
	ResourceID string `json:"resource_id,omitempty"`
	Code       string `json:"code"`
}

func Audit(s govdata.Snapshot) []Finding {
	out := []Finding{}
	for _, d := range s.Datasets {
		fields := []struct{ k, v string }{{"title", d.Title}, {"publisher", d.Publisher}, {"license", d.License}, {"modified", d.Modified}}
		for _, f := range fields {
			if strings.TrimSpace(f.v) == "" {
				out = append(out, Finding{Dataset: Key(d), Code: "missing_" + f.k})
			}
		}
		if len(d.Resources) == 0 {
			out = append(out, Finding{Dataset: Key(d), Code: "missing_resources"})
		}
		for _, r := range d.Resources {
			if strings.TrimSpace(r.URL) == "" {
				out = append(out, Finding{Key(d), r.ID, "missing_resource_url"})
			}
			if strings.TrimSpace(r.Format) == "" {
				out = append(out, Finding{Key(d), r.ID, "missing_resource_format"})
			}
		}
	}
	return out
}

type FormatCount struct {
	Source    string `json:"source"`
	Format    string `json:"format"`
	Resources int    `json:"resources"`
	Datasets  int    `json:"datasets"`
}

func Coverage(s govdata.Snapshot) []FormatCount {
	type count struct {
		n  int
		ds map[string]bool
	}
	m := map[string]*count{}
	for _, d := range s.Datasets {
		rs := d.Resources
		if len(rs) == 0 {
			rs = []govdata.Resource{{}}
		}
		for _, r := range rs {
			f := strings.ToUpper(strings.TrimSpace(r.Format))
			if f == "" {
				f = "UNDECLARED"
			}
			k := d.Source + "\n" + f
			if m[k] == nil {
				m[k] = &count{ds: map[string]bool{}}
			}
			if len(d.Resources) > 0 {
				m[k].n++
			}
			m[k].ds[d.ID] = true
		}
	}
	ks := []string{}
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	out := []FormatCount{}
	for _, k := range ks {
		a := strings.SplitN(k, "\n", 2)
		out = append(out, FormatCount{a[0], a[1], m[k].n, len(m[k].ds)})
	}
	return out
}

type Locked struct {
	Source         string `json:"source"`
	DatasetID      string `json:"dataset_id"`
	ResourceID     string `json:"resource_id"`
	URL            string `json:"url"`
	MetadataSHA256 string `json:"metadata_sha256"`
}

func Lock(s govdata.Snapshot, keys []string) ([]Locked, error) {
	selected := map[string]bool{}
	for _, k := range keys {
		selected[k] = false
	}
	out := []Locked{}
	for _, d := range s.Datasets {
		for _, r := range d.Resources {
			k := Key(d) + ":" + r.ID
			if _, ok := selected[k]; !ok {
				continue
			}
			if r.URL == "" {
				return nil, fmt.Errorf("selected resource %s has no URL", k)
			}
			h, err := Fingerprint(d)
			if err != nil {
				return nil, err
			}
			out = append(out, Locked{d.Source, d.ID, r.ID, r.URL, h})
			selected[k] = true
		}
	}
	for k, ok := range selected {
		if !ok {
			return nil, fmt.Errorf("selected resource %q absent from snapshot; use source:dataset-id:resource-id from catalog resources", k)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Source+out[i].DatasetID+out[i].ResourceID < out[j].Source+out[j].DatasetID+out[j].ResourceID
	})
	return out, nil
}
