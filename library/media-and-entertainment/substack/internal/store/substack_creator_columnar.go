// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.
// PATCH: substack-creator columnar data layer — ported from the substack-creator
// CLI's typed Upsert<Resource>Tx column-extraction methods so the merged CLI's
// portfolio/authoring novel commands (portfolio, posts best/twin/pair[s], grep,
// schedule board, subs churn/cross-sell) have populated columnar tables to query.
//
// The substack base store keeps all synced data in the generic `resources`
// table; the columnar `posts`/`publications`/`subscribers`/`notes`/`comments`/
// `drafts` tables are created (empty) by substack_creator_migrations.go. This
// file adds the typed upsert methods that fill them, multi-publication: every
// public Upsert<Resource> method accepts an explicit publicationID that is
// stamped into the row so cross-publication commands accumulate per-pub data.
//
// Column lists here MUST match the CREATE TABLE statements in
// substack_creator_migrations.go exactly. Recorded in .printing-press-patches.json.

package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// columnarItem pairs a decoded+enriched object with its original JSON payload.
// The multi-publication sync decodes each API item once, enriches it (stamping
// publication_id, mapping handle->identity, classifying tier), then hands the
// pair here so the typed columns and the generic resources/FTS rows stay
// consistent. data is what lands in the `data` JSON column and the FTS index;
// obj is what the column extractors read via lookupFieldValue.
type columnarItem struct {
	obj  map[string]any
	data json.RawMessage
}

// upsertColumnarBatch runs the generic resources insert + a typed-table insert
// for every item inside a single transaction. typedFn is one of the
// upsert<Resource>Tx helpers below. Mirrors the substack-creator Upsert<Resource>
// shape (generic + typed in one tx) but batched so a per-publication fetch of
// hundreds of rows is one transaction, not one-per-row.
func (s *Store) upsertColumnarBatch(resourceType string, items []columnarItem, typedFn func(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("starting %s columnar batch: %w", resourceType, err)
	}
	defer tx.Rollback()

	stored := 0
	for _, item := range items {
		id := extractObjectID(item.obj)
		if id == "" {
			continue
		}
		if err := s.upsertGenericResourceTx(tx, resourceType, id, item.data); err != nil {
			return stored, fmt.Errorf("upserting %s/%s (generic): %w", resourceType, id, err)
		}
		if err := typedFn(tx, id, item.obj, item.data); err != nil {
			return stored, fmt.Errorf("upserting %s/%s (typed): %w", resourceType, id, err)
		}
		stored++
	}
	return stored, tx.Commit()
}

// ---------------------------------------------------------------------------
// Public per-publication upsert entry points. The multi-publication sync calls
// these with a pre-built slice and the owning publication's id; each stamps
// publication_id so accumulation across publications is keyed correctly.
// ---------------------------------------------------------------------------

// UpsertPublication upserts a single owned-publication row. Unlike the other
// resources this is a per-pub identity record, not a list, so it takes the
// already-decoded object directly.
func (s *Store) UpsertPublication(obj map[string]any, data json.RawMessage) error {
	_, err := s.upsertColumnarBatch("publications", []columnarItem{{obj: obj, data: data}}, s.upsertCreatorPublicationsTx)
	return err
}

// UpsertPostsForPublication stamps publicationID into every post object and
// upserts the batch into the columnar posts table (+ generic resources + FTS).
func (s *Store) UpsertPostsForPublication(publicationID string, objs []map[string]any) (int, error) {
	return s.upsertColumnarBatch("posts", stampItems(publicationID, objs), s.upsertCreatorPostsTx)
}

// UpsertDraftsForPublication stamps publicationID into every draft and upserts.
func (s *Store) UpsertDraftsForPublication(publicationID string, objs []map[string]any) (int, error) {
	return s.upsertColumnarBatch("drafts", stampItems(publicationID, objs), s.upsertCreatorDraftsTx)
}

// UpsertSubscribersForPublication stamps publicationID into every subscriber
// row and upserts into the columnar subscribers table.
func (s *Store) UpsertSubscribersForPublication(publicationID string, objs []map[string]any) (int, error) {
	return s.upsertColumnarBatch("subscribers", stampItems(publicationID, objs), s.upsertCreatorSubscribersTx)
}

// UpsertNotesForPublication upserts notes (notes are author-scoped, not
// publication-keyed in the schema, so publicationID is informational only and
// not stamped onto a column that doesn't exist on the notes table).
func (s *Store) UpsertNotesForPublication(publicationID string, objs []map[string]any) (int, error) {
	return s.upsertColumnarBatch("notes", wrapItems(objs), s.upsertCreatorNotesTx)
}

// UpsertCommentsForPublication upserts comments.
func (s *Store) UpsertCommentsForPublication(publicationID string, objs []map[string]any) (int, error) {
	return s.upsertColumnarBatch("comments", wrapItems(objs), s.upsertCreatorCommentsTx)
}

// stampItems sets publication_id on each object (only when not already present
// with a usable value) and marshals the enriched object back to JSON so the
// generic resources row and FTS index carry the stamp too.
func stampItems(publicationID string, objs []map[string]any) []columnarItem {
	out := make([]columnarItem, 0, len(objs))
	for _, obj := range objs {
		if publicationID != "" {
			obj["publication_id"] = publicationID
		}
		data, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		out = append(out, columnarItem{obj: obj, data: data})
	}
	return out
}

// wrapItems marshals objects without stamping a publication_id (for tables
// whose schema has no publication_id column).
func wrapItems(objs []map[string]any) []columnarItem {
	out := make([]columnarItem, 0, len(objs))
	for _, obj := range objs {
		data, err := json.Marshal(obj)
		if err != nil {
			continue
		}
		out = append(out, columnarItem{obj: obj, data: data})
	}
	return out
}

// ---------------------------------------------------------------------------
// Typed-table column extractors. Ported from substack-creator's upsert<X>Tx
// helpers (store.go), with the column lists matched to this repo's
// substack_creator_migrations.go table definitions. lookupFieldValue and
// extractObjectID are the lift store's own helpers (store.go).
// ---------------------------------------------------------------------------

func (s *Store) upsertCreatorPublicationsTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO publications (id, data, synced_at, name, subdomain, custom_domain, hero_image, subscriber_count, paid_subscriber_count, description)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, name = excluded.name, subdomain = excluded.subdomain, custom_domain = excluded.custom_domain, hero_image = excluded.hero_image, subscriber_count = excluded.subscriber_count, paid_subscriber_count = excluded.paid_subscriber_count, description = excluded.description`,
		id,
		string(data),
		time.Now(),
		lookupFieldValue(obj, "name"),
		lookupFieldValue(obj, "subdomain"),
		lookupFieldValue(obj, "custom_domain"),
		lookupFieldValue(obj, "hero_image"),
		lookupFieldValue(obj, "subscriber_count"),
		lookupFieldValue(obj, "paid_subscriber_count"),
		lookupFieldValue(obj, "description"),
	); err != nil {
		return fmt.Errorf("insert into publications: %w", err)
	}
	return nil
}

func (s *Store) upsertCreatorPostsTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO posts (id, data, synced_at, slug, title, subtitle, publication_id, publish_date, post_date, audience, type, body_html, body_markdown, word_count, paywalled, scheduled_at, canonical_url, post_id, views, opens, clicks, likes, comments, restacks)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, slug = excluded.slug, title = excluded.title, subtitle = excluded.subtitle, publication_id = excluded.publication_id, publish_date = excluded.publish_date, post_date = excluded.post_date, audience = excluded.audience, type = excluded.type, body_html = excluded.body_html, body_markdown = excluded.body_markdown, word_count = excluded.word_count, paywalled = excluded.paywalled, scheduled_at = excluded.scheduled_at, canonical_url = excluded.canonical_url, post_id = excluded.post_id, views = excluded.views, opens = excluded.opens, clicks = excluded.clicks, likes = excluded.likes, comments = excluded.comments, restacks = excluded.restacks`,
		id,
		string(data),
		time.Now(),
		lookupFieldValue(obj, "slug"),
		lookupFieldValue(obj, "title"),
		lookupFieldValue(obj, "subtitle"),
		lookupFieldValue(obj, "publication_id"),
		lookupFieldValue(obj, "publish_date"),
		lookupFieldValue(obj, "post_date"),
		lookupFieldValue(obj, "audience"),
		lookupFieldValue(obj, "type"),
		lookupFieldValue(obj, "body_html"),
		lookupFieldValue(obj, "body_markdown"),
		lookupFieldValue(obj, "word_count"),
		lookupFieldValue(obj, "paywalled"),
		lookupFieldValue(obj, "scheduled_at"),
		lookupFieldValue(obj, "canonical_url"),
		lookupFieldValue(obj, "post_id"),
		lookupFieldValue(obj, "views"),
		lookupFieldValue(obj, "opens"),
		lookupFieldValue(obj, "clicks"),
		lookupFieldValue(obj, "likes"),
		lookupFieldValue(obj, "comments"),
		lookupFieldValue(obj, "restacks"),
	); err != nil {
		return fmt.Errorf("insert into posts: %w", err)
	}
	return nil
}

func (s *Store) upsertCreatorDraftsTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO drafts (id, data, synced_at, title, subtitle, body, publication_id, section_id, audience, scheduled_at, last_edited, paywall_markers, url, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, title = excluded.title, subtitle = excluded.subtitle, body = excluded.body, publication_id = excluded.publication_id, section_id = excluded.section_id, audience = excluded.audience, scheduled_at = excluded.scheduled_at, last_edited = excluded.last_edited, paywall_markers = excluded.paywall_markers, url = excluded.url, expires_at = excluded.expires_at`,
		id,
		string(data),
		time.Now(),
		lookupFieldValue(obj, "title"),
		lookupFieldValue(obj, "subtitle"),
		lookupFieldValue(obj, "body"),
		lookupFieldValue(obj, "publication_id"),
		lookupFieldValue(obj, "section_id"),
		lookupFieldValue(obj, "audience"),
		lookupFieldValue(obj, "scheduled_at"),
		lookupFieldValue(obj, "last_edited"),
		lookupFieldValue(obj, "paywall_markers"),
		lookupFieldValue(obj, "url"),
		lookupFieldValue(obj, "expires_at"),
	); err != nil {
		return fmt.Errorf("insert into drafts: %w", err)
	}
	return nil
}

func (s *Store) upsertCreatorSubscribersTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO subscribers (id, data, synced_at, publication_id, free_count, paid_count, founding_count, total, url, expires_at, row_count, email, name, tier, status, subscribed_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, publication_id = excluded.publication_id, free_count = excluded.free_count, paid_count = excluded.paid_count, founding_count = excluded.founding_count, total = excluded.total, url = excluded.url, expires_at = excluded.expires_at, row_count = excluded.row_count, email = excluded.email, name = excluded.name, tier = excluded.tier, status = excluded.status, subscribed_at = excluded.subscribed_at`,
		id,
		string(data),
		time.Now(),
		lookupFieldValue(obj, "publication_id"),
		lookupFieldValue(obj, "free_count"),
		lookupFieldValue(obj, "paid_count"),
		lookupFieldValue(obj, "founding_count"),
		lookupFieldValue(obj, "total"),
		lookupFieldValue(obj, "url"),
		lookupFieldValue(obj, "expires_at"),
		lookupFieldValue(obj, "row_count"),
		lookupFieldValue(obj, "email"),
		lookupFieldValue(obj, "name"),
		lookupFieldValue(obj, "tier"),
		lookupFieldValue(obj, "status"),
		lookupFieldValue(obj, "subscribed_at"),
	); err != nil {
		return fmt.Errorf("insert into subscribers: %w", err)
	}
	return nil
}

func (s *Store) upsertCreatorNotesTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO notes (id, data, synced_at, body, author_handle, posted_at, like_count, restack_count, reply_count, parent_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, body = excluded.body, author_handle = excluded.author_handle, posted_at = excluded.posted_at, like_count = excluded.like_count, restack_count = excluded.restack_count, reply_count = excluded.reply_count, parent_id = excluded.parent_id`,
		id,
		string(data),
		time.Now(),
		lookupFieldValue(obj, "body"),
		lookupFieldValue(obj, "author_handle"),
		lookupFieldValue(obj, "posted_at"),
		lookupFieldValue(obj, "like_count"),
		lookupFieldValue(obj, "restack_count"),
		lookupFieldValue(obj, "reply_count"),
		lookupFieldValue(obj, "parent_id"),
	); err != nil {
		return fmt.Errorf("insert into notes: %w", err)
	}
	return nil
}

func (s *Store) upsertCreatorCommentsTx(tx *sql.Tx, id string, obj map[string]any, data json.RawMessage) error {
	if _, err := tx.Exec(
		`INSERT INTO comments (id, data, synced_at, post_id, body, author_handle, author_name, posted_at, reaction_count)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, post_id = excluded.post_id, body = excluded.body, author_handle = excluded.author_handle, author_name = excluded.author_name, posted_at = excluded.posted_at, reaction_count = excluded.reaction_count`,
		id,
		string(data),
		time.Now(),
		lookupFieldValue(obj, "post_id"),
		lookupFieldValue(obj, "body"),
		lookupFieldValue(obj, "author_handle"),
		lookupFieldValue(obj, "author_name"),
		lookupFieldValue(obj, "posted_at"),
		lookupFieldValue(obj, "reaction_count"),
	); err != nil {
		return fmt.Errorf("insert into comments: %w", err)
	}
	return nil
}
