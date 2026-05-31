// Copyright 2026 Chirantan Rajhans and contributors. Licensed under Apache-2.0. See LICENSE.

package store

// substackCreatorMigrations are the bespoke per-resource tables ported
// verbatim from the substack-creator CLI's store schema. The modern
// substack store keeps all synced data in the generic `resources` table,
// but the ported portfolio/authoring novel commands (posts twin/best/pair/
// pairs, portfolio, grep, schedule board) query columnar `posts` and
// `publications` tables directly via raw SQL. These CREATE TABLE IF NOT
// EXISTS statements ensure those queries resolve against an empty (but
// present) table until a future sync populates them, instead of failing
// with "no such table".
//
// This file is the source of truth for these columnar schemas in the merged
// CLI — substack-creator is a separate repo with no write-back path here. If a
// schema changes, edit it here and record the change in .printing-press-patches.json.
var substackCreatorMigrations = []string{
	`CREATE TABLE IF NOT EXISTS drafts (
		id TEXT PRIMARY KEY,
		data JSON NOT NULL,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		title TEXT,
		subtitle TEXT,
		body TEXT,
		publication_id TEXT,
		section_id TEXT,
		audience TEXT,
		scheduled_at TEXT,
		last_edited TEXT,
		paywall_markers INTEGER,
		url TEXT,
		expires_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_drafts_publication_id ON drafts(publication_id)`,
	`CREATE INDEX IF NOT EXISTS idx_drafts_section_id ON drafts(section_id)`,
	`CREATE TABLE IF NOT EXISTS notes (
		id TEXT PRIMARY KEY,
		data JSON NOT NULL,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		body TEXT,
		author_handle TEXT,
		posted_at TEXT,
		like_count INTEGER,
		restack_count INTEGER,
		reply_count INTEGER,
		parent_id TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_notes_parent_id ON notes(parent_id)`,
	`CREATE TABLE IF NOT EXISTS comments (
		id TEXT PRIMARY KEY,
		data JSON NOT NULL,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		post_id TEXT,
		body TEXT,
		author_handle TEXT,
		author_name TEXT,
		posted_at TEXT,
		reaction_count INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id)`,
	`CREATE TABLE IF NOT EXISTS posts (
		id TEXT PRIMARY KEY,
		data JSON NOT NULL,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		slug TEXT,
		title TEXT,
		subtitle TEXT,
		publication_id TEXT,
		publish_date TEXT,
		post_date TEXT,
		audience TEXT,
		type TEXT,
		body_html TEXT,
		body_markdown TEXT,
		word_count INTEGER,
		paywalled INTEGER,
		scheduled_at TEXT,
		canonical_url TEXT,
		post_id TEXT,
		views INTEGER,
		opens INTEGER,
		clicks INTEGER,
		likes INTEGER,
		comments INTEGER,
		restacks INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_posts_publication_id ON posts(publication_id)`,
	`CREATE INDEX IF NOT EXISTS idx_posts_post_id ON posts(post_id)`,
	`CREATE TABLE IF NOT EXISTS publications (
		id TEXT PRIMARY KEY,
		data JSON NOT NULL,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		name TEXT,
		subdomain TEXT,
		custom_domain TEXT,
		hero_image TEXT,
		subscriber_count INTEGER,
		paid_subscriber_count INTEGER,
		description TEXT
	)`,
	`CREATE TABLE IF NOT EXISTS subscribers (
		id TEXT PRIMARY KEY,
		data JSON NOT NULL,
		synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		publication_id TEXT,
		free_count INTEGER,
		paid_count INTEGER,
		founding_count INTEGER,
		total INTEGER,
		url TEXT,
		expires_at TEXT,
		row_count INTEGER,
		email TEXT,
		name TEXT,
		tier TEXT,
		status TEXT,
		subscribed_at TEXT
	)`,
	`CREATE INDEX IF NOT EXISTS idx_subscribers_publication_id ON subscribers(publication_id)`,
}
