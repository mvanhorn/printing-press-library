// Copyright 2026 drummerms and contributors. Licensed under Apache-2.0. See LICENSE.

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

// EnsureAVERUSASchema creates the AVer USA corpus tables.
//
// These live outside the generic `resources` table because the corpus is not a
// REST resource set: it is two scraped websites (the Salesforce support portal
// and the averusa.com catalog) plus a file layer, joined on a normalized model
// name and a keyword-classified doc type. Keeping them in a separate file
// (rather than appending to the emitted migration slice in store.go) is what
// lets `generate --force` preserve them.
//
// Safe to call repeatedly; every statement is IF NOT EXISTS.
func EnsureAVERUSASchema(ctx context.Context, db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS averusa_documents (
			url_name     TEXT PRIMARY KEY,
			title        TEXT NOT NULL DEFAULT '',
			doc_type     TEXT NOT NULL DEFAULT '',
			model        TEXT NOT NULL DEFAULT '',
			entity_id    TEXT NOT NULL DEFAULT '',
			pdf_url      TEXT NOT NULL DEFAULT '',
			has_file     INTEGER NOT NULL DEFAULT 0,
			file_size    INTEGER NOT NULL DEFAULT 0,
			published_at TEXT NOT NULL DEFAULT '',
			updated_at   TEXT NOT NULL DEFAULT '',
			body         TEXT NOT NULL DEFAULT '',
			last_checked TEXT NOT NULL DEFAULT '',
			last_status  INTEGER NOT NULL DEFAULT 0,
			synced_at    TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_averusa_documents_type ON averusa_documents(doc_type)`,
		`CREATE INDEX IF NOT EXISTS idx_averusa_documents_model ON averusa_documents(model)`,

		`CREATE TABLE IF NOT EXISTS averusa_products (
			slug          TEXT PRIMARY KEY,
			category      TEXT NOT NULL DEFAULT '',
			name          TEXT NOT NULL DEFAULT '',
			url           TEXT NOT NULL DEFAULT '',
			datasheet_url TEXT NOT NULL DEFAULT '',
			discontinued  INTEGER NOT NULL DEFAULT 0,
			synced_at     TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE INDEX IF NOT EXISTS idx_averusa_products_category ON averusa_products(category)`,
		`CREATE INDEX IF NOT EXISTS idx_averusa_products_discontinued ON averusa_products(discontinued)`,

		`CREATE TABLE IF NOT EXISTS averusa_spec_fields (
			model     TEXT NOT NULL,
			field     TEXT NOT NULL,
			value     TEXT NOT NULL DEFAULT '',
			synced_at TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (model, field)
		)`,

		// Harvest bookkeeping. `coverage` and `whats-new` read this to report
		// how much of each site actually parsed, so a silent extraction
		// regression is visible as a number instead of an empty result set.
		`CREATE TABLE IF NOT EXISTS averusa_harvest (
			source      TEXT PRIMARY KEY,
			attempted   INTEGER NOT NULL DEFAULT 0,
			succeeded   INTEGER NOT NULL DEFAULT 0,
			with_file   INTEGER NOT NULL DEFAULT 0,
			last_error  TEXT NOT NULL DEFAULT '',
			finished_at TEXT NOT NULL DEFAULT ''
		)`,

		`CREATE VIRTUAL TABLE IF NOT EXISTS averusa_documents_fts USING fts5(
			url_name UNINDEXED, doc_type UNINDEXED, model UNINDEXED, title, body
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS averusa_products_fts USING fts5(
			slug UNINDEXED, category UNINDEXED, name
		)`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			return fmt.Errorf("averusa schema: %w", err)
		}
	}
	return nil
}

// ---------- corpus queries ----------

// AVerUSADocument is one row of the harvested document catalog.
type AVerUSADocument struct {
	URLName    string `json:"url_name"`
	Title      string `json:"title"`
	DocType    string `json:"doc_type"`
	Model      string `json:"model,omitempty"`
	EntityID   string `json:"entity_id,omitempty"`
	PDFURL     string `json:"pdf_url,omitempty"`
	HasFile    bool   `json:"has_file"`
	FileSize   int64  `json:"file_size,omitempty"`
	Published  string `json:"published_at,omitempty"`
	Updated    string `json:"updated_at,omitempty"`
	LastStatus int    `json:"last_status,omitempty"`
	SyncedAt   string `json:"synced_at,omitempty"`
}

// AVerUSAProduct is one row of the harvested product catalog.
type AVerUSAProduct struct {
	Slug         string `json:"slug"`
	Category     string `json:"category"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	DatasheetURL string `json:"datasheet_url,omitempty"`
	Discontinued bool   `json:"discontinued"`
	SyncedAt     string `json:"synced_at,omitempty"`
}

// AVerUSASpecField is one extracted datasheet field.
type AVerUSASpecField struct {
	Model  string `json:"model"`
	Field  string `json:"field"`
	Value  string `json:"value"`
	Source string `json:"source,omitempty"`
}

// SearchAVERUSADocuments runs a full-text query across the harvested document
// catalog, optionally filtered by doc type. Missing tables (corpus not
// harvested yet) are skipped, not errors.
func (s *Store) SearchAVERUSADocuments(query, docType string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	matchQuery := ftsMatchQuery(query)
	if matchQuery == "" {
		return nil, nil
	}
	sqlQ := `SELECT d.url_name, d.title, d.doc_type, d.model, d.entity_id, d.has_file,
	            snippet(averusa_documents_fts, 1, '[', ']', ' ... ', 14)
	         FROM averusa_documents_fts
	         JOIN averusa_documents d ON d.url_name = averusa_documents_fts.url_name
	         WHERE averusa_documents_fts MATCH ?`
	args := []any{matchQuery}
	if docType != "" {
		sqlQ += ` AND d.doc_type = ?`
		args = append(args, docType)
	}
	sqlQ += ` ORDER BY rank LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(sqlQ, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := make([]json.RawMessage, 0, limit)
	for rows.Next() {
		var urlName, title, typ, model, entityID string
		var hasFile int
		var snip string
		if err := rows.Scan(&urlName, &title, &typ, &model, &entityID, &hasFile, &snip); err != nil {
			return nil, err
		}
		b, err := json.Marshal(map[string]any{
			"resource_type": "document",
			"url_name":      urlName,
			"title":         title,
			"doc_type":      typ,
			"model":         model,
			"entity_id":     entityID,
			"has_file":      hasFile == 1,
			"snippet":       snip,
		})
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// ListAVERUSADocuments returns catalog rows filtered by doc type and/or model,
// most recently updated first.
func (s *Store) ListAVERUSADocuments(docType, model string, limit int) ([]AVerUSADocument, error) {
	if limit <= 0 {
		limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	if docType != "" {
		where = append(where, "doc_type = ?")
		args = append(args, docType)
	}
	if model != "" {
		where = append(where, "model = ?")
		args = append(args, model)
	}
	args = append(args, limit)
	rows, err := s.db.Query(
		`SELECT url_name, title, doc_type, model, entity_id, pdf_url, has_file, file_size,
		        published_at, updated_at, last_status, synced_at
		 FROM averusa_documents WHERE `+strings.Join(where, " AND ")+
			` ORDER BY updated_at DESC, title LIMIT ?`, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []AVerUSADocument{}
	for rows.Next() {
		var d AVerUSADocument
		var hasFile int
		if err := rows.Scan(&d.URLName, &d.Title, &d.DocType, &d.Model, &d.EntityID,
			&d.PDFURL, &hasFile, &d.FileSize, &d.Published, &d.Updated, &d.LastStatus, &d.SyncedAt); err != nil {
			return nil, err
		}
		d.HasFile = hasFile == 1
		out = append(out, d)
	}
	return out, rows.Err()
}

// ListAVERUSAProducts returns catalog rows filtered by category (and
// optionally discontinued status), ordered by category then slug.
func (s *Store) ListAVERUSAProducts(category string, limit int) ([]AVerUSAProduct, error) {
	if limit <= 0 {
		limit = 200
	}
	where := []string{"1=1"}
	args := []any{}
	if category != "" {
		where = append(where, "category = ?")
		args = append(args, category)
	}
	args = append(args, limit)
	rows, err := s.db.Query(
		`SELECT slug, category, name, url, datasheet_url, discontinued, synced_at
		 FROM averusa_products WHERE `+strings.Join(where, " AND ")+
			` ORDER BY category, slug LIMIT ?`, args...)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []AVerUSAProduct{}
	for rows.Next() {
		var p AVerUSAProduct
		var disc int
		if err := rows.Scan(&p.Slug, &p.Category, &p.Name, &p.URL, &p.DatasheetURL, &disc, &p.SyncedAt); err != nil {
			return nil, err
		}
		p.Discontinued = disc == 1
		out = append(out, p)
	}
	return out, rows.Err()
}

// DocumentsForModel lists every catalog document whose extracted model matches.
func (s *Store) DocumentsForModel(model string, limit int) ([]AVerUSADocument, error) {
	return s.ListAVERUSADocuments("", model, limit)
}

// SpecFields returns the extracted datasheet fields for a model, ordered by
// field name. Returns nil (not an error) when the corpus is not harvested.
func (s *Store) SpecFields(model string) ([]AVerUSASpecField, error) {
	rows, err := s.db.Query(
		`SELECT field, value FROM averusa_spec_fields WHERE model = ? ORDER BY field`, model)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []AVerUSASpecField{}
	for rows.Next() {
		var f AVerUSASpecField
		f.Model = model
		if err := rows.Scan(&f.Field, &f.Value); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// AVerUSACoverageRow is one model row of the doc-type coverage matrix.
type AVerUSACoverageRow struct {
	Model    string   `json:"model"`
	Category string   `json:"category,omitempty"`
	Types    []string `json:"types"`
	Missing  []string `json:"missing"`
}

// KnownAVERUSADocTypes is the classification enum used across the CLI.
var KnownAVERUSADocTypes = []string{
	"user-manual", "spec-sheet", "white-paper", "quick-start",
	"software", "brochure", "comparison-chart", "article",
}

// CoverageAVERUSA returns the model x doc-type availability matrix for a
// category (empty = all products).
func (s *Store) CoverageAVERUSA(category string) ([]AVerUSACoverageRow, error) {
	prods, err := s.ListAVERUSAProducts(category, 500)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT model, doc_type FROM averusa_documents WHERE model != ''`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	have := map[string]map[string]bool{}
	for rows.Next() {
		var m, t string
		if err := rows.Scan(&m, &t); err != nil {
			return nil, err
		}
		if have[m] == nil {
			have[m] = map[string]bool{}
		}
		have[m][t] = true
	}
	out := make([]AVerUSACoverageRow, 0, len(prods))
	for _, p := range prods {
		row := AVerUSACoverageRow{Model: p.Slug, Category: p.Category}
		for _, t := range KnownAVERUSADocTypes {
			if have[p.Slug][t] {
				row.Types = append(row.Types, t)
			} else {
				row.Missing = append(row.Missing, t)
			}
		}
		out = append(out, row)
	}
	return out, nil
}

// AVerUSAHarvestRow is one source row of harvest bookkeeping.
type AVerUSAHarvestRow struct {
	Source     string `json:"source"`
	Attempted  int    `json:"attempted"`
	Succeeded  int    `json:"succeeded"`
	WithFile   int    `json:"with_file"`
	LastError  string `json:"last_error,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// HarvestAVERUSAStats returns the per-source harvest counts.
func (s *Store) HarvestAVERUSAStats() ([]AVerUSAHarvestRow, error) {
	rows, err := s.db.Query(
		`SELECT source, attempted, succeeded, with_file, last_error, finished_at
		 FROM averusa_harvest ORDER BY source`)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil
		}
		return nil, err
	}
	defer rows.Close()
	out := []AVerUSAHarvestRow{}
	for rows.Next() {
		var r AVerUSAHarvestRow
		if err := rows.Scan(&r.Source, &r.Attempted, &r.Succeeded, &r.WithFile, &r.LastError, &r.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// WhatChangedAVERUSA lists documents and products with an updated_at/synced_at
// newer than the cutoff, newest first. Used by whats-new.
func (s *Store) WhatChangedAVERUSA(cutoff string, limit int) (docs []AVerUSADocument, prods []AVerUSAProduct, err error) {
	if limit <= 0 {
		limit = 100
	}
	drows, err := s.db.Query(
		`SELECT url_name, title, doc_type, model, entity_id, pdf_url, has_file, file_size,
		        published_at, updated_at, last_status, synced_at
		 FROM averusa_documents WHERE updated_at > ? OR synced_at > ?
		 ORDER BY MAX(updated_at, synced_at) DESC LIMIT ?`,
		cutoff, cutoff, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	for drows.Next() {
		var d AVerUSADocument
		var hasFile int
		if err := drows.Scan(&d.URLName, &d.Title, &d.DocType, &d.Model, &d.EntityID,
			&d.PDFURL, &hasFile, &d.FileSize, &d.Published, &d.Updated, &d.LastStatus, &d.SyncedAt); err != nil {
			_ = drows.Close()
			return nil, nil, err
		}
		d.HasFile = hasFile == 1
		docs = append(docs, d)
	}
	if err := drows.Err(); err != nil {
		_ = drows.Close()
		return nil, nil, err
	}
	_ = drows.Close()

	prows, err := s.db.Query(
		`SELECT slug, category, name, url, datasheet_url, discontinued, synced_at
		 FROM averusa_products WHERE synced_at > ? ORDER BY synced_at DESC LIMIT ?`,
		cutoff, limit)
	if err != nil {
		if strings.Contains(err.Error(), "no such table") {
			return docs, nil, nil
		}
		return nil, nil, err
	}
	defer prows.Close()
	for prows.Next() {
		var p AVerUSAProduct
		var disc int
		if err := prows.Scan(&p.Slug, &p.Category, &p.Name, &p.URL, &p.DatasheetURL, &disc, &p.SyncedAt); err != nil {
			return nil, nil, err
		}
		p.Discontinued = disc == 1
		prods = append(prods, p)
	}
	return docs, prods, prows.Err()
}
