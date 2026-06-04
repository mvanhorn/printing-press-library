package research

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"
)

const createSnapshotTableSQL = `
CREATE TABLE IF NOT EXISTS research_snapshots (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	scope_kind TEXT NOT NULL,
	scope_value TEXT NOT NULL,
	resources TEXT NOT NULL,
	fetched_at TEXT NOT NULL,
	fresh_for_seconds INTEGER NOT NULL,
	raw_records TEXT NOT NULL,
	evidence TEXT NOT NULL,
	coverage TEXT NOT NULL,
	warnings TEXT NOT NULL
)`

type SnapshotStore struct {
	db *store.Store
}

func NewSnapshotStore(db *store.Store) *SnapshotStore {
	return &SnapshotStore{db: db}
}

func (s *SnapshotStore) Save(ctx context.Context, snapshot Snapshot) error {
	if err := s.ensureSchema(ctx); err != nil {
		return err
	}

	resources, err := json.Marshal(snapshot.Resources)
	if err != nil {
		return fmt.Errorf("marshal resources: %w", err)
	}
	rawRecords, err := json.Marshal(snapshot.RawRecords)
	if err != nil {
		return fmt.Errorf("marshal raw records: %w", err)
	}
	evidence, err := json.Marshal(snapshot.Evidence)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}
	coverage, err := json.Marshal(snapshot.Coverage)
	if err != nil {
		return fmt.Errorf("marshal coverage: %w", err)
	}
	warnings, err := json.Marshal(snapshot.Warnings)
	if err != nil {
		return fmt.Errorf("marshal warnings: %w", err)
	}

	_, err = s.db.DB().ExecContext(ctx, `
		INSERT INTO research_snapshots (
			scope_kind,
			scope_value,
			resources,
			fetched_at,
			fresh_for_seconds,
			raw_records,
			evidence,
			coverage,
			warnings
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(snapshot.Scope.Kind),
		snapshot.Scope.Value,
		string(resources),
		snapshot.FetchedAt.UTC().Format(time.RFC3339),
		int64(snapshot.FreshFor.Seconds()),
		string(rawRecords),
		string(evidence),
		string(coverage),
		string(warnings),
	)
	if err != nil {
		return fmt.Errorf("save research snapshot: %w", err)
	}

	return nil
}

func (s *SnapshotStore) List(ctx context.Context, scope ResearchScope, limit int) ([]Snapshot, error) {
	if err := s.ensureSchema(ctx); err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.DB().QueryContext(ctx, `
		SELECT
			id,
			scope_kind,
			scope_value,
			resources,
			fetched_at,
			fresh_for_seconds,
			raw_records,
			evidence,
			coverage,
			warnings
		FROM research_snapshots
		WHERE scope_kind = ? AND scope_value = ?
		ORDER BY fetched_at DESC, id DESC
		LIMIT ?`,
		string(scope.Kind),
		scope.Value,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list research snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []Snapshot
	for rows.Next() {
		snapshot, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate research snapshots: %w", err)
	}

	return snapshots, nil
}

func (s *SnapshotStore) ensureSchema(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("research snapshot store has no database")
	}
	if _, err := s.db.DB().ExecContext(ctx, createSnapshotTableSQL); err != nil {
		return fmt.Errorf("create research snapshot table: %w", err)
	}
	return nil
}

type snapshotScanner interface {
	Scan(dest ...any) error
}

func scanSnapshot(scanner snapshotScanner) (Snapshot, error) {
	var (
		snapshot        Snapshot
		scopeKind       string
		resourcesJSON   string
		fetchedAt       string
		freshForSeconds int64
		rawRecordsJSON  string
		evidenceJSON    string
		coverageJSON    string
		warningsJSON    string
	)

	err := scanner.Scan(
		&snapshot.ID,
		&scopeKind,
		&snapshot.Scope.Value,
		&resourcesJSON,
		&fetchedAt,
		&freshForSeconds,
		&rawRecordsJSON,
		&evidenceJSON,
		&coverageJSON,
		&warningsJSON,
	)
	if err != nil {
		return Snapshot{}, fmt.Errorf("scan research snapshot: %w", err)
	}

	snapshot.Scope.Kind = ScopeKind(scopeKind)
	parsedFetchedAt, err := time.Parse(time.RFC3339, fetchedAt)
	if err != nil {
		return Snapshot{}, fmt.Errorf("parse fetched_at: %w", err)
	}
	snapshot.FetchedAt = parsedFetchedAt
	snapshot.FreshFor = time.Duration(freshForSeconds) * time.Second

	if err := json.Unmarshal([]byte(resourcesJSON), &snapshot.Resources); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal resources: %w", err)
	}
	if err := json.Unmarshal([]byte(rawRecordsJSON), &snapshot.RawRecords); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal raw records: %w", err)
	}
	if err := json.Unmarshal([]byte(evidenceJSON), &snapshot.Evidence); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal evidence: %w", err)
	}
	if err := json.Unmarshal([]byte(coverageJSON), &snapshot.Coverage); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal coverage: %w", err)
	}
	if err := json.Unmarshal([]byte(warningsJSON), &snapshot.Warnings); err != nil {
		return Snapshot{}, fmt.Errorf("unmarshal warnings: %w", err)
	}

	return snapshot, nil
}
