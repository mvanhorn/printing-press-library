// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// Typed design-data tables for the awwwards mirror. Hand-authored extension
// beside the generated store; lazily initialized by the commands that use it.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/awwwards/internal/awwwards"
)

const awwwardsSchema = `
CREATE TABLE IF NOT EXISTS aw_sites (
	slug TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL DEFAULT 0,
	thumbnail TEXT NOT NULL DEFAULT '',
	card_type TEXT NOT NULL DEFAULT '',
	external_url TEXT NOT NULL DEFAULT '',
	award TEXT NOT NULL DEFAULT '',
	score_overall REAL,
	score_design REAL,
	score_usability REAL,
	score_creativity REAL,
	score_content REAL,
	first_seen_filter TEXT NOT NULL DEFAULT '',
	detail_synced_at INTEGER NOT NULL DEFAULT 0,
	mirrored_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_aw_sites_created ON aw_sites(created_at);
CREATE INDEX IF NOT EXISTS idx_aw_sites_score ON aw_sites(score_overall);

CREATE TABLE IF NOT EXISTS aw_site_tags (
	slug TEXT NOT NULL,
	tag TEXT NOT NULL,
	PRIMARY KEY (slug, tag)
);
CREATE INDEX IF NOT EXISTS idx_aw_tags_tag ON aw_site_tags(tag);

CREATE TABLE IF NOT EXISTS aw_palette (
	slug TEXT NOT NULL,
	hex TEXT NOT NULL,
	position INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (slug, hex)
);

CREATE TABLE IF NOT EXISTS aw_jury (
	slug TEXT NOT NULL,
	juror TEXT NOT NULL,
	profile TEXT NOT NULL DEFAULT '',
	country TEXT NOT NULL DEFAULT '',
	design REAL, usability REAL, creativity REAL, content REAL, overall REAL,
	PRIMARY KEY (slug, juror)
);

CREATE TABLE IF NOT EXISTS aw_credits (
	slug TEXT NOT NULL,
	username TEXT NOT NULL,
	display_name TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (slug, username)
);
CREATE INDEX IF NOT EXISTS idx_aw_credits_user ON aw_credits(username);

CREATE TABLE IF NOT EXISTS aw_elements (
	id TEXT PRIMARY KEY,
	etype TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL DEFAULT '',
	image TEXT NOT NULL DEFAULT '',
	created_at INTEGER NOT NULL DEFAULT 0,
	username TEXT NOT NULL DEFAULT '',
	inspiration_slug TEXT NOT NULL DEFAULT '',
	parent_slug TEXT NOT NULL DEFAULT '',
	external_url TEXT NOT NULL DEFAULT '',
	tags_json TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX IF NOT EXISTS idx_aw_elements_type ON aw_elements(etype);
CREATE INDEX IF NOT EXISTS idx_aw_elements_parent ON aw_elements(parent_slug);
`

// EnsureAwwwardsTables creates the typed mirror tables when absent.
func (s *Store) EnsureAwwwardsTables(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, awwwardsSchema)
	if err != nil {
		return fmt.Errorf("creating awwwards mirror tables: %w", err)
	}
	// Additive column for mirrors created before external_url existed; only
	// the expected duplicate-column error on current schemas is ignored.
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE aw_elements ADD COLUMN external_url TEXT NOT NULL DEFAULT ''`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column name") {
			return fmt.Errorf("adding aw_elements.external_url: %w", err)
		}
	}
	return nil
}

// HasAwwwardsMirror reports whether any cards have been mirrored.
func (s *Store) HasAwwwardsMirror(ctx context.Context) bool {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM aw_sites`).Scan(&n)
	return err == nil && n > 0
}

// UpsertAwCards stores listing cards (and their tags) in one transaction.
// filter records the listing filter segment the cards were seen under ("" for
// the unfiltered feed); it is only set when the row is first created.
func (s *Store) UpsertAwCards(ctx context.Context, cards []awwwards.Card, filter string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin mirror tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	siteStmt, err := tx.PrepareContext(ctx, `
		INSERT INTO aw_sites (slug, title, created_at, thumbnail, card_type, first_seen_filter, mirrored_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title = excluded.title,
			created_at = excluded.created_at,
			thumbnail = excluded.thumbnail,
			card_type = excluded.card_type,
			mirrored_at = excluded.mirrored_at`)
	if err != nil {
		return fmt.Errorf("prepare site upsert: %w", err)
	}
	defer siteStmt.Close()
	tagStmt, err := tx.PrepareContext(ctx, `INSERT OR IGNORE INTO aw_site_tags (slug, tag) VALUES (?, ?)`)
	if err != nil {
		return fmt.Errorf("prepare tag upsert: %w", err)
	}
	defer tagStmt.Close()

	for _, c := range cards {
		if c.Type == "element" || c.Slug == "" {
			continue
		}
		if _, err := siteStmt.ExecContext(ctx, c.Slug, c.Title, c.CreatedAt, c.Thumbnail(), c.Type, filter, now); err != nil {
			return fmt.Errorf("upsert site %q: %w", c.Slug, err)
		}
		for _, t := range c.Tags {
			if _, err := tagStmt.ExecContext(ctx, c.Slug, t); err != nil {
				return fmt.Errorf("upsert tag %q on %q: %w", t, c.Slug, err)
			}
		}
	}
	return tx.Commit()
}

// UpsertAwDetail stores a parsed detail page: scores, palette, jury, credits, tags.
func (s *Store) UpsertAwDetail(ctx context.Context, d awwwards.Detail) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin detail tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	now := time.Now().Unix()
	// Nullability comes from parse success, not score value: only treat the
	// scores as real when at least one dimension parsed (> 0; awwwards scores
	// are 1-10, so all-zero means the score block was absent or unparsed).
	hasScores := d.Scores.Design > 0 || d.Scores.Usability > 0 || d.Scores.Creativity > 0 || d.Scores.Content > 0
	scoreVal := func(f float64) any {
		if !hasScores {
			return nil
		}
		return f
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO aw_sites (slug, title, external_url, award, score_overall, score_design, score_usability, score_creativity, score_content, detail_synced_at, mirrored_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(slug) DO UPDATE SET
			title = CASE WHEN excluded.title != '' THEN excluded.title ELSE aw_sites.title END,
			external_url = excluded.external_url,
			award = excluded.award,
			score_overall = excluded.score_overall,
			score_design = excluded.score_design,
			score_usability = excluded.score_usability,
			score_creativity = excluded.score_creativity,
			score_content = excluded.score_content,
			detail_synced_at = excluded.detail_synced_at`,
		d.Slug, d.Title, d.ExternalURL, d.Award,
		scoreVal(d.Scores.Overall), scoreVal(d.Scores.Design), scoreVal(d.Scores.Usability),
		scoreVal(d.Scores.Creativity), scoreVal(d.Scores.Content), now, now); err != nil {
		return fmt.Errorf("upsert detail %q: %w", d.Slug, err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM aw_palette WHERE slug = ?`, d.Slug); err != nil {
		return fmt.Errorf("clear palette %q: %w", d.Slug, err)
	}
	for i, hex := range d.Palette {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO aw_palette (slug, hex, position) VALUES (?, ?, ?)`, d.Slug, hex, i); err != nil {
			return fmt.Errorf("upsert palette %q: %w", d.Slug, err)
		}
	}

	for _, v := range d.Jury {
		var ds, us, cs, ct, ov any
		if len(v.Scores) >= 5 {
			ds, us, cs, ct, ov = v.Scores[0], v.Scores[1], v.Scores[2], v.Scores[3], v.Scores[4]
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO aw_jury (slug, juror, profile, country, design, usability, creativity, content, overall)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(slug, juror) DO UPDATE SET
				profile = excluded.profile, country = excluded.country,
				design = excluded.design, usability = excluded.usability,
				creativity = excluded.creativity, content = excluded.content, overall = excluded.overall`,
			d.Slug, v.Name, v.Profile, v.Country, ds, us, cs, ct, ov); err != nil {
			return fmt.Errorf("upsert jury %q on %q: %w", v.Name, d.Slug, err)
		}
	}

	for _, c := range d.Credits {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO aw_credits (slug, username, display_name) VALUES (?, ?, ?)
			ON CONFLICT(slug, username) DO UPDATE SET display_name = excluded.display_name`,
			d.Slug, c.Username, c.DisplayName); err != nil {
			return fmt.Errorf("upsert credit %q on %q: %w", c.Username, d.Slug, err)
		}
	}

	for _, t := range d.Tags {
		if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO aw_site_tags (slug, tag) VALUES (?, ?)`, d.Slug, t.Label); err != nil {
			return fmt.Errorf("upsert detail tag %q on %q: %w", t.Label, d.Slug, err)
		}
	}

	return tx.Commit()
}

// UpsertAwElements stores element cards for one element type.
func (s *Store) UpsertAwElements(ctx context.Context, cards []awwwards.Card, etype string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin elements tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, c := range cards {
		if c.Type != "element" {
			continue
		}
		username := ""
		if c.User != nil {
			username = c.User.Username
		}
		tagsJSON, err := json.Marshal(c.Tags)
		if err != nil {
			tagsJSON = []byte("[]")
		}
		parent := awwwards.ParentSlugForElement(c.InspirationSlug, etype)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO aw_elements (id, etype, title, image, created_at, username, inspiration_slug, parent_slug, external_url, tags_json)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(id) DO UPDATE SET
				etype = excluded.etype, title = excluded.title, image = excluded.image,
				created_at = excluded.created_at, username = excluded.username,
				inspiration_slug = excluded.inspiration_slug, parent_slug = excluded.parent_slug,
				external_url = excluded.external_url, tags_json = excluded.tags_json`,
			string(c.ID), etype, c.Title, c.Thumbnail(), c.CreatedAt, username, c.InspirationSlug, parent, c.ExternalURL, string(tagsJSON)); err != nil {
			return fmt.Errorf("upsert element %q: %w", c.ID, err)
		}
	}
	return tx.Commit()
}

// SlugsNeedingDetail returns mirrored site slugs with no detail sync yet,
// newest first, capped at limit.
func (s *Store) SlugsNeedingDetail(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT slug FROM aw_sites
		WHERE detail_synced_at = 0
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying slugs needing detail: %w", err)
	}
	defer rows.Close()
	var slugs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scanning slug: %w", err)
		}
		slugs = append(slugs, s)
	}
	return slugs, rows.Err()
}

// ElementParentsNeedingDetail returns derived element parent slugs that have
// no detail-synced site row yet, capped at limit.
func (s *Store) ElementParentsNeedingDetail(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT parent_slug FROM aw_elements
		WHERE parent_slug != ''
		  AND parent_slug NOT IN (SELECT slug FROM aw_sites WHERE detail_synced_at > 0)
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying element parents needing detail: %w", err)
	}
	defer rows.Close()
	var slugs []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, fmt.Errorf("scanning parent slug: %w", err)
		}
		slugs = append(slugs, s)
	}
	return slugs, rows.Err()
}
