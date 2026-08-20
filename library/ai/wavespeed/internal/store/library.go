// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Generation is one recorded novel-command output in the library DB. Cost is
// in the WaveSpeed account's credit unit. ContentHash lets re-runs detect
// model non-determinism at stable paths.
type Generation struct {
	ID             string          `json:"id"`
	CreatedAt      string          `json:"created_at,omitempty"`
	Command        string          `json:"command,omitempty"`
	BrandProfileID string          `json:"brand_profile_id,omitempty"`
	BrandName      string          `json:"brand_name,omitempty"`
	PlatformTarget string          `json:"platform_target,omitempty"`
	ModelID        string          `json:"model_id,omitempty"`
	Prompt         string          `json:"prompt,omitempty"`
	AspectRatio    string          `json:"aspect_ratio,omitempty"`
	Seed           *int64          `json:"seed,omitempty"`
	Cost           float64         `json:"cost"`
	ContentHash    string          `json:"content_hash,omitempty"`
	Path           string          `json:"path,omitempty"`
	Status         string          `json:"status,omitempty"`
	Params         json.RawMessage `json:"params,omitempty"`
	Data           json.RawMessage `json:"data,omitempty"`
	Tags           []string        `json:"tags,omitempty"`
}

// GenerationFilter narrows ListGenerations. A zero value lists everything.
type GenerationFilter struct {
	Brand    string
	Platform string
	Model    string
	Tag      string
	Since    time.Time
	Limit    int
}

// BrandProfile is a stored brand profile. Data is the opaque profile body
// (style anchors, palette, voice, models, platforms, negative prompts).
type BrandProfile struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	CreatedAt string          `json:"created_at,omitempty"`
	UpdatedAt string          `json:"updated_at,omitempty"`
	Data      json.RawMessage `json:"data"`
}

// CostRow is one grouped rollup from CostReport.
type CostRow struct {
	Key       string  `json:"key"`
	Count     int     `json:"count"`
	TotalCost float64 `json:"total_cost"`
}

// RecordGeneration inserts (or replaces) a generation row. Writes are
// serialized by writeMu, matching the rest of the store's write discipline.
func (s *Store) RecordGeneration(g Generation) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	params := nullableJSON(g.Params)
	data := nullableJSON(g.Data)
	var seed any
	if g.Seed != nil {
		seed = *g.Seed
	}
	_, err := s.db.Exec(
		`INSERT INTO generations
			(id, command, brand_profile_id, brand_name, platform_target, model_id, prompt, aspect_ratio, seed, cost, content_hash, path, status, params, data)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
			command=excluded.command, brand_profile_id=excluded.brand_profile_id,
			brand_name=excluded.brand_name, platform_target=excluded.platform_target,
			model_id=excluded.model_id, prompt=excluded.prompt, aspect_ratio=excluded.aspect_ratio,
			seed=excluded.seed, cost=excluded.cost, content_hash=excluded.content_hash,
			path=excluded.path, status=excluded.status, params=excluded.params, data=excluded.data`,
		g.ID, g.Command, g.BrandProfileID, g.BrandName, g.PlatformTarget, g.ModelID,
		g.Prompt, g.AspectRatio, seed, g.Cost, g.ContentHash, g.Path, g.Status, params, data,
	)
	if err != nil {
		return fmt.Errorf("recording generation: %w", err)
	}
	return nil
}

// ListGenerations returns generations matching filter, newest first. Filters
// map onto the composite indexes created in migrateLibrary.
func (s *Store) ListGenerations(f GenerationFilter) ([]Generation, error) {
	var where []string
	var args []any
	if f.Brand != "" {
		where = append(where, "(g.brand_name = ? OR g.brand_profile_id = ?)")
		args = append(args, f.Brand, f.Brand)
	}
	if f.Platform != "" {
		where = append(where, "g.platform_target = ?")
		args = append(args, f.Platform)
	}
	if f.Model != "" {
		where = append(where, "g.model_id = ?")
		args = append(args, f.Model)
	}
	if !f.Since.IsZero() {
		where = append(where, "g.created_at >= ?")
		args = append(args, f.Since.UTC().Format(time.RFC3339))
	}
	join := ""
	if f.Tag != "" {
		join = "JOIN tag_links tl ON tl.generation_id = g.id JOIN tags t ON t.id = tl.tag_id"
		where = append(where, "t.name = ?")
		args = append(args, f.Tag)
	}
	query := "SELECT g.id, g.created_at, g.command, g.brand_profile_id, g.brand_name, g.platform_target, g.model_id, g.prompt, g.aspect_ratio, g.seed, g.cost, g.content_hash, g.path, g.status, g.params, g.data FROM generations g " + join
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY g.created_at DESC"
	if f.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", f.Limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing generations: %w", err)
	}
	defer rows.Close()
	return scanGenerations(rows)
}

// SearchGenerations runs an FTS5 MATCH against generation prompts. A malformed
// FTS5 query surfaces as an error the caller maps to a usage error.
func (s *Store) SearchGenerations(query string, limit int) ([]Generation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT g.id, g.created_at, g.command, g.brand_profile_id, g.brand_name, g.platform_target, g.model_id, g.prompt, g.aspect_ratio, g.seed, g.cost, g.content_hash, g.path, g.status, g.params, g.data
		 FROM generations g JOIN generations_fts f ON f.rowid = g.rowid
		 WHERE generations_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`, query, limit)
	if err != nil {
		return nil, fmt.Errorf("searching generations: %w", err)
	}
	defer rows.Close()
	return scanGenerations(rows)
}

// GetGeneration returns a single generation by id, with its tags populated.
func (s *Store) GetGeneration(id string) (Generation, error) {
	rows, err := s.db.Query(
		`SELECT g.id, g.created_at, g.command, g.brand_profile_id, g.brand_name, g.platform_target, g.model_id, g.prompt, g.aspect_ratio, g.seed, g.cost, g.content_hash, g.path, g.status, g.params, g.data
		 FROM generations g WHERE g.id = ?`, id)
	if err != nil {
		return Generation{}, fmt.Errorf("getting generation: %w", err)
	}
	defer rows.Close()
	gens, err := scanGenerations(rows)
	if err != nil {
		return Generation{}, err
	}
	if len(gens) == 0 {
		return Generation{}, sql.ErrNoRows
	}
	g := gens[0]
	tags, err := s.TagsFor(id)
	if err != nil {
		return g, err
	}
	g.Tags = tags
	return g, nil
}

func scanGenerations(rows *sql.Rows) ([]Generation, error) {
	out := []Generation{}
	for rows.Next() {
		var g Generation
		var seed sql.NullInt64
		var params, data sql.NullString
		var command, brandID, brandName, platform, model, prompt, aspect, hash, path, status sql.NullString
		if err := rows.Scan(&g.ID, &g.CreatedAt, &command, &brandID, &brandName, &platform,
			&model, &prompt, &aspect, &seed, &g.Cost, &hash, &path, &status, &params, &data); err != nil {
			return nil, fmt.Errorf("scanning generation: %w", err)
		}
		g.Command = command.String
		g.BrandProfileID = brandID.String
		g.BrandName = brandName.String
		g.PlatformTarget = platform.String
		g.ModelID = model.String
		g.Prompt = prompt.String
		g.AspectRatio = aspect.String
		g.ContentHash = hash.String
		g.Path = path.String
		g.Status = status.String
		if seed.Valid {
			v := seed.Int64
			g.Seed = &v
		}
		if params.Valid && params.String != "" {
			g.Params = json.RawMessage(params.String)
		}
		if data.Valid && data.String != "" {
			g.Data = json.RawMessage(data.String)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// AddTag attaches a tag to a generation, creating the tag if needed.
func (s *Store) AddTag(generationID, tag string) error {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return fmt.Errorf("tag must not be empty")
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO tags(name) VALUES (?)`, tag); err != nil {
		return fmt.Errorf("creating tag: %w", err)
	}
	var tagID int64
	if err := s.db.QueryRow(`SELECT id FROM tags WHERE name = ?`, tag).Scan(&tagID); err != nil {
		return fmt.Errorf("resolving tag: %w", err)
	}
	if _, err := s.db.Exec(`INSERT OR IGNORE INTO tag_links(generation_id, tag_id) VALUES (?, ?)`, generationID, tagID); err != nil {
		return fmt.Errorf("linking tag: %w", err)
	}
	return nil
}

// RemoveTag detaches a tag from a generation. Removing an absent tag is a
// no-op, not an error.
func (s *Store) RemoveTag(generationID, tag string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`DELETE FROM tag_links WHERE generation_id = ? AND tag_id = (SELECT id FROM tags WHERE name = ?)`,
		generationID, strings.TrimSpace(tag))
	if err != nil {
		return fmt.Errorf("removing tag: %w", err)
	}
	return nil
}

// TagsFor returns the tags attached to a generation, sorted.
func (s *Store) TagsFor(generationID string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT t.name FROM tags t JOIN tag_links tl ON tl.tag_id = t.id
		 WHERE tl.generation_id = ? ORDER BY t.name`, generationID)
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}
	defer rows.Close()
	tags := []string{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning tag: %w", err)
		}
		tags = append(tags, name)
	}
	return tags, rows.Err()
}

// UpsertBrandProfile inserts or updates a brand profile by name. id is used
// only on first insert; subsequent updates preserve the original id.
func (s *Store) UpsertBrandProfile(id, name string, data json.RawMessage) (BrandProfile, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.Exec(
		`INSERT INTO brand_profiles(id, name, data, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(name) DO UPDATE SET data=excluded.data, updated_at=CURRENT_TIMESTAMP`,
		id, name, []byte(data))
	if err != nil {
		return BrandProfile{}, fmt.Errorf("upserting brand profile: %w", err)
	}
	return s.getBrandProfileLocked(name)
}

// GetBrandProfile returns a brand profile by name, or sql.ErrNoRows.
func (s *Store) GetBrandProfile(name string) (BrandProfile, error) {
	return s.getBrandProfileLocked(name)
}

func (s *Store) getBrandProfileLocked(name string) (BrandProfile, error) {
	var p BrandProfile
	var data sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, created_at, updated_at, data FROM brand_profiles WHERE name = ?`, name).
		Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt, &data)
	if err != nil {
		return BrandProfile{}, err
	}
	if data.Valid {
		p.Data = json.RawMessage(data.String)
	}
	return p, nil
}

// ListBrandProfiles returns all brand profiles, alphabetical by name.
func (s *Store) ListBrandProfiles() ([]BrandProfile, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at, updated_at, data FROM brand_profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("listing brand profiles: %w", err)
	}
	defer rows.Close()
	out := []BrandProfile{}
	for rows.Next() {
		var p BrandProfile
		var data sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.CreatedAt, &p.UpdatedAt, &data); err != nil {
			return nil, fmt.Errorf("scanning brand profile: %w", err)
		}
		if data.Valid {
			p.Data = json.RawMessage(data.String)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// CostReport rolls up generation cost grouped by one dimension: "brand",
// "model", "platform", or "tag". Rows with no records in range yield an empty
// slice, not an error.
func (s *Store) CostReport(since time.Time, groupBy string) ([]CostRow, error) {
	var query string
	var args []any
	sinceClause := ""
	if !since.IsZero() {
		sinceClause = " WHERE g.created_at >= ?"
		args = append(args, since.UTC().Format(time.RFC3339))
	}
	switch groupBy {
	case "brand":
		query = `SELECT COALESCE(NULLIF(g.brand_name, ''), '(none)') AS k, COUNT(*), COALESCE(SUM(g.cost), 0) FROM generations g` + sinceClause + ` GROUP BY k ORDER BY 3 DESC`
	case "model":
		query = `SELECT COALESCE(NULLIF(g.model_id, ''), '(none)') AS k, COUNT(*), COALESCE(SUM(g.cost), 0) FROM generations g` + sinceClause + ` GROUP BY k ORDER BY 3 DESC`
	case "platform":
		query = `SELECT COALESCE(NULLIF(g.platform_target, ''), '(none)') AS k, COUNT(*), COALESCE(SUM(g.cost), 0) FROM generations g` + sinceClause + ` GROUP BY k ORDER BY 3 DESC`
	case "tag":
		tagSince := ""
		if !since.IsZero() {
			tagSince = " AND g.created_at >= ?"
		}
		query = `SELECT t.name AS k, COUNT(*), COALESCE(SUM(g.cost), 0)
			FROM generations g JOIN tag_links tl ON tl.generation_id = g.id JOIN tags t ON t.id = tl.tag_id
			WHERE 1=1` + tagSince + ` GROUP BY k ORDER BY 3 DESC`
	default:
		return nil, fmt.Errorf("unknown cost-report grouping %q (want brand|model|platform|tag)", groupBy)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("cost report: %w", err)
	}
	defer rows.Close()
	out := []CostRow{}
	for rows.Next() {
		var r CostRow
		if err := rows.Scan(&r.Key, &r.Count, &r.TotalCost); err != nil {
			return nil, fmt.Errorf("scanning cost row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func nullableJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}
