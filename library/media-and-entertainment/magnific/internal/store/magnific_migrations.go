package store

import (
	"context"
	"database/sql"
	"sync"
)

var magnificMigrationOnce sync.Once
var magnificMigrationErr error

// EnsureMagnificTables creates the durable tables the novel-feature commands
// rely on: magnific_prompts (saved/replayable templates + history), magnific_tasks
// (unified async-task ledger across all model endpoints), magnific_assets
// (downloaded outputs), magnific_stock_library (local index of downloaded
// icons/videos/resources), plus FTS5 indices for full-text search. The call
// is idempotent via sync.Once so commands can invoke it lazily without
// coordinating with the generator's migration slice in store.go.
func EnsureMagnificTables(ctx context.Context, db *sql.DB) error {
	magnificMigrationOnce.Do(func() {
		stmts := []string{
			// magnific_tasks: every async-task POST writes a row here.
			// task_id is the API's task identifier; model is the slug used
			// to route polling to the correct GET endpoint; endpoint stores
			// the literal API path so reconcile/wait can re-poll without
			// re-deriving from a model table.
			`CREATE TABLE IF NOT EXISTS magnific_tasks (
				task_id TEXT PRIMARY KEY,
				model TEXT NOT NULL,
				endpoint TEXT NOT NULL,
				status TEXT NOT NULL DEFAULT 'IN_PROGRESS',
				prompt TEXT,
				credit_cost REAL,
				tag TEXT,
				output_url TEXT,
				error_message TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				completed_at DATETIME
			)`,
			`CREATE INDEX IF NOT EXISTS idx_magnific_tasks_model ON magnific_tasks(model)`,
			`CREATE INDEX IF NOT EXISTS idx_magnific_tasks_status ON magnific_tasks(status)`,
			`CREATE INDEX IF NOT EXISTS idx_magnific_tasks_created ON magnific_tasks(created_at)`,

			// magnific_prompts: reusable templates AND prompt history.
			// "name" non-null = saved template; null = ad-hoc history entry.
			`CREATE TABLE IF NOT EXISTS magnific_prompts (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT,
				prompt TEXT NOT NULL,
				model TEXT,
				params TEXT,
				task_id TEXT,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(name)
			)`,
			`CREATE INDEX IF NOT EXISTS idx_magnific_prompts_task ON magnific_prompts(task_id)`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS magnific_prompts_fts USING fts5(
				prompt, model, name,
				content='magnific_prompts', content_rowid='id'
			)`,
			`CREATE TRIGGER IF NOT EXISTS magnific_prompts_ai AFTER INSERT ON magnific_prompts BEGIN
				INSERT INTO magnific_prompts_fts(rowid, prompt, model, name)
				VALUES (new.id, new.prompt, COALESCE(new.model,''), COALESCE(new.name,''));
			END`,
			`CREATE TRIGGER IF NOT EXISTS magnific_prompts_ad AFTER DELETE ON magnific_prompts BEGIN
				INSERT INTO magnific_prompts_fts(magnific_prompts_fts, rowid, prompt, model, name)
				VALUES ('delete', old.id, old.prompt, COALESCE(old.model,''), COALESCE(old.name,''));
			END`,

			// magnific_assets: downloaded outputs from generation/upscale/etc.
			// id is a content-derived hash; local_path is the file on disk.
			`CREATE TABLE IF NOT EXISTS magnific_assets (
				id TEXT PRIMARY KEY,
				task_id TEXT,
				model TEXT,
				local_path TEXT NOT NULL,
				orientation TEXT,
				tag TEXT,
				size_bytes INTEGER,
				downloaded_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE INDEX IF NOT EXISTS idx_magnific_assets_task ON magnific_assets(task_id)`,
			`CREATE INDEX IF NOT EXISTS idx_magnific_assets_tag ON magnific_assets(tag)`,

			// magnific_stock_library: local index of downloaded stock icons,
			// videos, resources keyed by stock asset id.
			`CREATE TABLE IF NOT EXISTS magnific_stock_library (
				id TEXT PRIMARY KEY,
				kind TEXT NOT NULL,
				title TEXT,
				local_path TEXT,
				tags TEXT,
				indexed_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`,
			`CREATE VIRTUAL TABLE IF NOT EXISTS magnific_stock_library_fts USING fts5(
				title, tags, kind,
				content='magnific_stock_library', content_rowid='rowid'
			)`,
			`CREATE TRIGGER IF NOT EXISTS magnific_stock_library_ai AFTER INSERT ON magnific_stock_library BEGIN
				INSERT INTO magnific_stock_library_fts(rowid, title, tags, kind)
				VALUES (new.rowid, COALESCE(new.title,''), COALESCE(new.tags,''), new.kind);
			END`,
		}

		conn, err := db.Conn(ctx)
		if err != nil {
			magnificMigrationErr = err
			return
		}
		defer conn.Close()

		for _, stmt := range stmts {
			if _, err := conn.ExecContext(ctx, stmt); err != nil {
				magnificMigrationErr = err
				return
			}
		}
	})
	return magnificMigrationErr
}
