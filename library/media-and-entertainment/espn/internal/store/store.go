// Package store provides local SQLite persistence for espn-pp-cli.
// Uses modernc.org/sqlite (pure Go, no CGO) for zero-dependency cross-compilation.
// FTS5 full-text search indexes are created for searchable content.
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// IsUUID returns true if the input looks like a UUID.
func IsUUID(s string) bool {
	return uuidPattern.MatchString(s)
}

// StoreSchemaVersion is the on-disk schema version this binary understands.
// It is stamped into SQLite's PRAGMA user_version on fresh databases and
// checked on every open. Learn-enabled CLIs advance to v6 for the
// canonical learn-loop tables ported from prediction-goat.
const StoreSchemaVersion = 6

type Store struct {
	db *sql.DB
	// writeMu serializes all DB writes. Read paths bypass the lock and run
	// concurrently against WAL.
	writeMu sync.Mutex
	path    string
}

// Open opens or creates the SQLite store at dbPath using the background
// context. Prefer OpenWithContext from a Cobra command so SIGINT during
// a slow migration interrupts the open instead of stranding the caller.
func Open(dbPath string) (*Store, error) {
	return OpenWithContext(context.Background(), dbPath)
}

// OpenWithContext opens or creates the SQLite store at dbPath. The
// context is honored by the migration path.
func OpenWithContext(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)&_pragma=temp_store(MEMORY)&_pragma=mmap_size(268435456)")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// WAL mode + 2 connections allows one read cursor open while a second
	// query executes. Writes are still serialized by SQLite's WAL lock.
	db.SetMaxOpenConns(2)

	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// Path returns the on-disk path of the backing SQLite file.
func (s *Store) Path() string {
	return s.path
}

// DB exposes the underlying *sql.DB for callers that need to run ad-hoc
// queries (e.g., learn-loop helpers). Callers must not call Close on the
// returned handle.
func (s *Store) DB() *sql.DB {
	return s.db
}

// SchemaVersion reads PRAGMA user_version, which is stamped by migrate().
// A zero value means the database predates the schema-version gate.
func (s *Store) SchemaVersion() (int, error) {
	var v int
	if err := s.db.QueryRow(`PRAGMA user_version`).Scan(&v); err != nil {
		return 0, fmt.Errorf("read user_version: %w", err)
	}
	return v, nil
}

func (s *Store) migrate(ctx context.Context) error {
	migrations := []string{
		// Generic resources table (kept for backward compat)
		`CREATE TABLE IF NOT EXISTS resources (
			id TEXT PRIMARY KEY,
			resource_type TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_resources_type ON resources(resource_type)`,
		`CREATE INDEX IF NOT EXISTS idx_resources_synced ON resources(synced_at)`,
		`CREATE TABLE IF NOT EXISTS sync_state (
			resource_type TEXT PRIMARY KEY,
			last_cursor TEXT,
			last_synced_at DATETIME,
			total_count INTEGER DEFAULT 0
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS resources_fts USING fts5(
			id, resource_type, content, tokenize='porter unicode61'
		)`,

		// ── Domain-specific tables ──

		// events table (26 columns)
		`CREATE TABLE IF NOT EXISTS events (
			id TEXT PRIMARY KEY,
			sport TEXT NOT NULL,
			league TEXT NOT NULL,
			name TEXT,
			short_name TEXT,
			date TEXT,
			season_year INTEGER,
			season_type INTEGER,
			week INTEGER,
			status TEXT,
			completed INTEGER DEFAULT 0,
			home_team_id TEXT,
			home_team_abbr TEXT,
			home_team_name TEXT,
			home_score TEXT,
			home_winner INTEGER DEFAULT 0,
			away_team_id TEXT,
			away_team_abbr TEXT,
			away_team_name TEXT,
			away_score TEXT,
			away_winner INTEGER DEFAULT 0,
			venue_name TEXT,
			venue_city TEXT,
			broadcast TEXT,
			attendance INTEGER,
			neutral_site INTEGER DEFAULT 0,
			notes TEXT,
			data JSON NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_events_sport_league ON events(sport, league)`,
		`CREATE INDEX IF NOT EXISTS idx_events_date ON events(date)`,
		`CREATE INDEX IF NOT EXISTS idx_events_home_team ON events(home_team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_away_team ON events(away_team_id)`,
		`CREATE INDEX IF NOT EXISTS idx_events_season ON events(season_year, season_type)`,
		`CREATE INDEX IF NOT EXISTS idx_events_status ON events(status)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS events_fts USING fts5(
			name, short_name, home_team_name, away_team_name, venue_name, notes,
			tokenize='porter unicode61'
		)`,

		// teams_domain table
		`CREATE TABLE IF NOT EXISTS teams_domain (
			id TEXT NOT NULL,
			sport TEXT NOT NULL,
			league TEXT NOT NULL,
			slug TEXT,
			abbreviation TEXT,
			display_name TEXT,
			short_name TEXT,
			location TEXT,
			color TEXT,
			logo_url TEXT,
			is_active INTEGER DEFAULT 1,
			data JSON NOT NULL,
			PRIMARY KEY (id, sport, league)
		)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS teams_fts USING fts5(
			display_name, short_name, location, abbreviation,
			tokenize='porter unicode61'
		)`,

		// news_domain table
		`CREATE TABLE IF NOT EXISTS news_domain (
			id INTEGER PRIMARY KEY,
			sport TEXT,
			league TEXT,
			type TEXT,
			headline TEXT,
			description TEXT,
			byline TEXT,
			published TEXT,
			last_modified TEXT,
			premium INTEGER DEFAULT 0,
			web_url TEXT,
			data JSON NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_news_published ON news_domain(published)`,
		`CREATE INDEX IF NOT EXISTS idx_news_sport_league ON news_domain(sport, league)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS news_fts USING fts5(
			headline, description, byline,
			tokenize='porter unicode61'
		)`,

		// standings table
		`CREATE TABLE IF NOT EXISTS standings (
			team_id TEXT NOT NULL,
			sport TEXT NOT NULL,
			league TEXT NOT NULL,
			season INTEGER NOT NULL,
			wins INTEGER DEFAULT 0,
			losses INTEGER DEFAULT 0,
			ties INTEGER DEFAULT 0,
			win_pct REAL DEFAULT 0,
			points_for INTEGER DEFAULT 0,
			points_against INTEGER DEFAULT 0,
			differential INTEGER DEFAULT 0,
			streak TEXT,
			playoff_seed INTEGER,
			clincher TEXT,
			division_record TEXT,
			conference_record TEXT,
			games_behind TEXT,
			data JSON NOT NULL,
			PRIMARY KEY (team_id, sport, league, season)
		)`,

		// sync_state_v2 table
		`CREATE TABLE IF NOT EXISTS sync_state_v2 (
			sport TEXT NOT NULL,
			league TEXT NOT NULL,
			resource TEXT NOT NULL,
			last_sync TEXT,
			last_date TEXT,
			cursor TEXT,
			PRIMARY KEY (sport, league, resource)
		)`,

		// Legacy promoted tables (kept for backward compat)
		`CREATE TABLE IF NOT EXISTS scoreboard (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS teams (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS news (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS summary (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS rankings (
			id TEXT PRIMARY KEY,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// CLI Printing Press: learn migrations
		//
		// search_learnings: LLM-driven per-query reranking. Populated by
		// the `teach` command (silent, backgrounded by the LLM after a
		// successful response) and read by the rerank layer to
		// boost/hide/alias hits on subsequent queries. See learnings.go
		// for the full semantics. Per-user table; stays small.
		//
		// query_entities: JSON array of case-preserving entity tokens
		// extracted from query_pattern at teach time. Used by the recall
		// match validator to reject cross-entity matches that would
		// otherwise score high on non-entity Jaccard.
		`CREATE TABLE IF NOT EXISTS search_learnings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_pattern TEXT NOT NULL,
			query_entities TEXT,
			venue TEXT,
			resource_type TEXT,
			resource_id TEXT NOT NULL,
			action TEXT NOT NULL,
			alias_target TEXT,
			source TEXT NOT NULL,
			confidence INTEGER DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			notes TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_learn_query ON search_learnings(query_pattern)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_learn_unique ON search_learnings(query_pattern, resource_id, action)`,
		// entity_lookups: canonical-to-value reference data for the
		// pattern substitution engine in internal/learn/patterns. Seeded
		// at migration time by the consumer (e.g., a CLI may register
		// country codes, sports team abbreviations, etc.); per-user
		// additions land via the `teach-lookup` CLI command with
		// source='taught'. PK is the (kind, canonical, value) triple so
		// multiple aliases under the same kind coexist without
		// collision.
		`CREATE TABLE IF NOT EXISTS entity_lookups (
			kind TEXT NOT NULL,
			canonical TEXT NOT NULL,
			value TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'seeded',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (kind, canonical, value)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entity_lookup_canonical ON entity_lookups(canonical)`,
		`CREATE INDEX IF NOT EXISTS idx_entity_lookup_kind ON entity_lookups(kind)`,
		// search_patterns: inferred and taught templates for the
		// generalization layer in internal/learn/patterns. Each row
		// encodes a query_template with one {entity[:kind]} slot and a
		// resource_template that names how the entity substitutes into
		// the resource ID. Extract() writes "inferred" rows whenever
		// two or more search_learnings rows share a structural shape;
		// the teach-pattern CLI command writes "taught" rows directly
		// for explicit template authorship.
		//
		// Idempotency leans on idx_patterns_unique: a re-Extract pass
		// over the same source learnings re-asserts the same
		// (query_template, resource_template, strategy) triple, which
		// bumps confidence and refreshes last_observed_at on the
		// existing row rather than spawning a duplicate.
		`CREATE TABLE IF NOT EXISTS search_patterns (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_template TEXT NOT NULL,
			resource_template TEXT NOT NULL,
			resource_type TEXT NOT NULL,
			venue TEXT,
			strategy TEXT NOT NULL,
			entity_kind TEXT NOT NULL,
			confidence INTEGER NOT NULL DEFAULT 2,
			source TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME,
			example_query TEXT,
			example_resource TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_patterns_query_template ON search_patterns(query_template)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_patterns_unique ON search_patterns(query_template, resource_template, strategy)`,
		// learning_playbooks: keyed on the structural query family (all
		// entities stripped, see learn.queryFamily). One row per family
		// holds the optional structured playbook (ordered CLI command
		// sequence with entity slots) and the optional free-text notes
		// (gotchas, workarounds the CLI surface doesn't expose). Either
		// field may be empty; non-empty in both is the strongest signal.
		//
		// Read at recall time by query_family; surfaces to the agent
		// alongside the existing per-resource hits so a future inquiry
		// of the same shape can skip rediscovery of the choreography.
		`CREATE TABLE IF NOT EXISTS learning_playbooks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			query_family TEXT NOT NULL,
			playbook_json TEXT,
			notes_text TEXT,
			source TEXT NOT NULL DEFAULT 'taught',
			confidence INTEGER NOT NULL DEFAULT 2,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_observed_at DATETIME
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_playbooks_family ON learning_playbooks(query_family)`,
	}

	// Two-pass migration. Pass 1: CREATE TABLE IF NOT EXISTS statements
	// only — these are idempotent (no-op on existing tables, create
	// fresh on first install). Pass 2 (interleaved): reconcile each
	// existing table's columns against canonical via ALTER TABLE ADD
	// COLUMN, so subsequent index creation against newer columns
	// doesn't fail on stale user DBs. Pass 3: remaining migrations
	// (indexes, virtual tables) — now safe to run.
	createTables, others := partitionCreateTableStatements(migrations)
	for _, m := range createTables {
		if _, err := s.db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	if err := s.reconcileSchema(ctx, createTables); err != nil {
		return fmt.Errorf("reconcile schema: %w", err)
	}
	for _, m := range others {
		if _, err := s.db.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("migration failed: %w", err)
		}
	}
	// Stamp the schema version. On a fresh DB this writes the current
	// StoreSchemaVersion; on an already-stamped DB this is a no-op
	// write of the same value.
	if _, err := s.db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, StoreSchemaVersion)); err != nil {
		return fmt.Errorf("stamp user_version: %w", err)
	}
	return nil
}

// partitionCreateTableStatements splits a flat migrations slice into
// CREATE TABLE statements (which establish column shape and can no-op
// on existing tables) and everything else (CREATE INDEX, CREATE VIRTUAL
// TABLE, etc.). Order within each partition is preserved; CREATE
// VIRTUAL TABLE goes into the "others" bucket because it doesn't
// participate in column-shape reconciliation.
func partitionCreateTableStatements(migrations []string) (createTables, others []string) {
	for _, m := range migrations {
		stripped := strings.TrimLeftFunc(m, func(r rune) bool {
			return r == ' ' || r == '\t' || r == '\n'
		})
		upper := strings.ToUpper(stripped)
		if strings.HasPrefix(upper, "CREATE TABLE ") || strings.HasPrefix(upper, "CREATE TABLE\n") || strings.HasPrefix(upper, "CREATE TABLE\t") {
			createTables = append(createTables, m)
		} else {
			others = append(others, m)
		}
	}
	return createTables, others
}

// reconcileSchema heals stale-schema user DBs by adding canonical
// columns that are missing. The canonical column shape is derived
// from the SAME migration list passed in (running the CREATE TABLE
// statements against an in-memory SQLite DB) — so any future column
// addition to a CREATE TABLE statement automatically flows into
// reconciliation without a separate declaration to keep in sync.
//
// SQLite limitations honored:
//   - PRIMARY KEY columns cannot be added via ALTER. If a canonical
//     PK column is missing on disk, return an error explaining the
//     user can recover by removing the DB file.
//   - NOT NULL columns without defaults will fail at ALTER time if
//     the table has rows. The error message is propagated as-is.
//
// Forward-compatible: columns present on disk but not in canonical
// are left alone (no DROP COLUMN). Type-mismatched columns aren't
// auto-repaired — destructive repair belongs to the operator, not
// the migration.
func (s *Store) reconcileSchema(ctx context.Context, createTables []string) error {
	canon, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return fmt.Errorf("open canonical reference: %w", err)
	}
	defer canon.Close()
	for _, m := range createTables {
		if _, err := canon.ExecContext(ctx, m); err != nil {
			return fmt.Errorf("canonical reference build: %w", err)
		}
	}
	canonRows, err := canon.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`)
	if err != nil {
		return fmt.Errorf("list canonical tables: %w", err)
	}
	var canonicalTables []string
	for canonRows.Next() {
		var name string
		if err := canonRows.Scan(&name); err != nil {
			canonRows.Close()
			return fmt.Errorf("scan canonical table: %w", err)
		}
		canonicalTables = append(canonicalTables, name)
	}
	if err := canonRows.Err(); err != nil {
		canonRows.Close()
		return fmt.Errorf("iterate canonical tables: %w", err)
	}
	canonRows.Close()
	for _, table := range canonicalTables {
		var userTableName string
		err := s.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&userTableName)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return fmt.Errorf("check user table %s: %w", table, err)
		}
		canonCols, err := pragmaColumns(ctx, canon, table)
		if err != nil {
			return fmt.Errorf("canonical columns for %s: %w", table, err)
		}
		userCols, err := pragmaColumns(ctx, s.db, table)
		if err != nil {
			return fmt.Errorf("user columns for %s: %w", table, err)
		}
		for _, col := range canonCols {
			if _, present := userCols[col.name]; present {
				continue
			}
			if col.pk {
				return fmt.Errorf("table %s on disk is missing canonical primary-key column %s; SQLite cannot add a primary key via ALTER TABLE. Remove %s to start with a fresh DB", table, col.name, s.path)
			}
			alter := fmt.Sprintf(`ALTER TABLE %s ADD COLUMN %s %s`, table, col.name, col.spec)
			if _, err := s.db.ExecContext(ctx, alter); err != nil {
				return fmt.Errorf("reconcile %s.%s: %w (consider removing %s if recovery is acceptable)", table, col.name, err, s.path)
			}
		}
	}
	return nil
}

// colInfo captures the shape of a column as PRAGMA table_info reports
// it. spec is the ALTER-suitable type + nullability + default fragment
// (e.g., "INTEGER NOT NULL DEFAULT 0" or "TEXT"); pk is true when the
// column is part of the table's primary key.
type colInfo struct {
	name string
	spec string
	pk   bool
}

// pragmaColumns returns a name->colInfo map for the given table using
// PRAGMA table_info. The map is suitable for diffing two schemas:
// missing keys in one direction are columns to add (with ALTER), missing
// keys in the other direction are columns the operator added and we
// should leave alone (forward-compat).
func pragmaColumns(ctx context.Context, db *sql.DB, table string) (map[string]colInfo, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return nil, fmt.Errorf("pragma table_info(%s): %w", table, err)
	}
	defer rows.Close()
	out := make(map[string]colInfo)
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan pragma row: %w", err)
		}
		spec := typ
		if notnull == 1 {
			spec += " NOT NULL"
		}
		if dflt.Valid {
			spec += " DEFAULT " + dflt.String
		}
		out[name] = colInfo{name: name, spec: spec, pk: pk > 0}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pragma table_info(%s): %w", table, err)
	}
	return out, nil
}

// ── Domain Upsert Methods ──

// UpsertEvent parses an ESPN event JSON and extracts all columns.
func (s *Store) UpsertEvent(sport, league string, data json.RawMessage) error {
	var ev map[string]any
	if err := json.Unmarshal(data, &ev); err != nil {
		return fmt.Errorf("parsing event: %w", err)
	}

	id := jsonStr(ev, "id")
	if id == "" {
		return fmt.Errorf("event missing id")
	}

	name := jsonStr(ev, "name")
	shortName := jsonStr(ev, "shortName")
	date := jsonStr(ev, "date")

	// Season info
	var seasonYear, seasonType, week int
	if season, ok := ev["season"].(map[string]any); ok {
		seasonYear = jsonInt(season, "year")
		seasonType = jsonInt(season, "type")
	}
	if weekObj, ok := ev["week"].(map[string]any); ok {
		week = jsonInt(weekObj, "number")
	}

	// Status
	var status string
	var completed int
	if statusObj, ok := ev["status"].(map[string]any); ok {
		if typeObj, ok2 := statusObj["type"].(map[string]any); ok2 {
			status = jsonStr(typeObj, "state")
			if jsonBool(typeObj, "completed") {
				completed = 1
			}
		}
	}

	// Competitors (home/away)
	var homeTeamID, homeTeamAbbr, homeTeamName, homeScore string
	var homeWinner int
	var awayTeamID, awayTeamAbbr, awayTeamName, awayScore string
	var awayWinner int
	var venueName, venueCity, broadcast string
	var attendance int
	var neutralSite int

	// Notes (e.g., "Super Bowl LX", "World Series - Game 1", "NBA Finals - Game 3")
	var notes string
	if comps, ok := ev["competitions"].([]any); ok && len(comps) > 0 {
		if comp, ok := comps[0].(map[string]any); ok {
			if notesArr, ok := comp["notes"].([]any); ok {
				var parts []string
				for _, n := range notesArr {
					if nm, ok := n.(map[string]any); ok {
						if h := jsonStr(nm, "headline"); h != "" {
							parts = append(parts, h)
						}
					}
				}
				notes = strings.Join(parts, "; ")
			}
		}
	}

	if comps, ok := ev["competitions"].([]any); ok && len(comps) > 0 {
		comp, _ := comps[0].(map[string]any)
		if comp != nil {
			// Venue
			if venue, ok := comp["venue"].(map[string]any); ok {
				venueName = jsonStr(venue, "fullName")
				if addr, ok := venue["address"].(map[string]any); ok {
					venueCity = jsonStr(addr, "city")
				}
			}
			attendance = jsonInt(comp, "attendance")
			neutralSite = boolToInt(jsonBool(comp, "neutralSite"))

			// Broadcast
			if broadcasts, ok := comp["broadcasts"].([]any); ok && len(broadcasts) > 0 {
				if b, ok := broadcasts[0].(map[string]any); ok {
					if names, ok := b["names"].([]any); ok && len(names) > 0 {
						broadcast = fmt.Sprintf("%v", names[0])
					}
				}
			}

			// Competitors
			if competitors, ok := comp["competitors"].([]any); ok {
				for _, c := range competitors {
					team, _ := c.(map[string]any)
					if team == nil {
						continue
					}
					homeAway := jsonStr(team, "homeAway")
					teamID := jsonStr(team, "id")
					score := jsonStr(team, "score")
					winner := boolToInt(jsonBool(team, "winner"))

					var abbr, teamName string
					if t, ok := team["team"].(map[string]any); ok {
						abbr = jsonStr(t, "abbreviation")
						teamName = jsonStr(t, "displayName")
					}

					if homeAway == "home" {
						homeTeamID = teamID
						homeTeamAbbr = abbr
						homeTeamName = teamName
						homeScore = score
						homeWinner = winner
					} else {
						awayTeamID = teamID
						awayTeamAbbr = abbr
						awayTeamName = teamName
						awayScore = score
						awayWinner = winner
					}
				}
			}
		}
	}

	_, err := s.db.Exec(`INSERT INTO events (
		id, sport, league, name, short_name, date,
		season_year, season_type, week,
		status, completed,
		home_team_id, home_team_abbr, home_team_name, home_score, home_winner,
		away_team_id, away_team_abbr, away_team_name, away_score, away_winner,
		venue_name, venue_city, broadcast, attendance, neutral_site,
		notes, data
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON CONFLICT(id) DO UPDATE SET
		sport=excluded.sport, league=excluded.league, name=excluded.name,
		short_name=excluded.short_name, date=excluded.date,
		season_year=excluded.season_year, season_type=excluded.season_type, week=excluded.week,
		status=excluded.status, completed=excluded.completed,
		home_team_id=excluded.home_team_id, home_team_abbr=excluded.home_team_abbr,
		home_team_name=excluded.home_team_name, home_score=excluded.home_score, home_winner=excluded.home_winner,
		away_team_id=excluded.away_team_id, away_team_abbr=excluded.away_team_abbr,
		away_team_name=excluded.away_team_name, away_score=excluded.away_score, away_winner=excluded.away_winner,
		venue_name=excluded.venue_name, venue_city=excluded.venue_city,
		broadcast=excluded.broadcast, attendance=excluded.attendance, neutral_site=excluded.neutral_site,
		notes=excluded.notes, data=excluded.data`,
		id, sport, league, name, shortName, date,
		seasonYear, seasonType, week,
		status, completed,
		homeTeamID, homeTeamAbbr, homeTeamName, homeScore, homeWinner,
		awayTeamID, awayTeamAbbr, awayTeamName, awayScore, awayWinner,
		venueName, venueCity, broadcast, attendance, neutralSite,
		notes, string(data),
	)
	if err != nil {
		return fmt.Errorf("upserting event %s: %w", id, err)
	}

	// Update FTS
	s.db.Exec(`DELETE FROM events_fts WHERE rowid = (SELECT rowid FROM events WHERE id = ?)`, id)
	s.db.Exec(`INSERT INTO events_fts (rowid, name, short_name, home_team_name, away_team_name, venue_name, notes)
		SELECT rowid, name, short_name, home_team_name, away_team_name, venue_name, notes FROM events WHERE id = ?`, id)

	return nil
}

// UpsertTeamDomain parses an ESPN team JSON.
func (s *Store) UpsertTeamDomain(sport, league string, data json.RawMessage) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing team: %w", err)
	}

	// ESPN wraps team data: {"team": {...}} in the teams array
	teamData := raw
	if t, ok := raw["team"].(map[string]any); ok {
		teamData = t
		// Re-marshal just the team object for storage
		if b, err := json.Marshal(t); err == nil {
			data = json.RawMessage(b)
		}
	}

	id := jsonStr(teamData, "id")
	if id == "" {
		return fmt.Errorf("team missing id")
	}

	slug := jsonStr(teamData, "slug")
	abbreviation := jsonStr(teamData, "abbreviation")
	displayName := jsonStr(teamData, "displayName")
	shortName := jsonStr(teamData, "shortName")
	location := jsonStr(teamData, "location")
	color := jsonStr(teamData, "color")
	isActive := boolToInt(jsonBool(teamData, "isActive"))

	var logoURL string
	if logos, ok := teamData["logos"].([]any); ok && len(logos) > 0 {
		if logo, ok := logos[0].(map[string]any); ok {
			logoURL = jsonStr(logo, "href")
		}
	}

	_, err := s.db.Exec(`INSERT INTO teams_domain (id, sport, league, slug, abbreviation, display_name, short_name, location, color, logo_url, is_active, data)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id, sport, league) DO UPDATE SET
			slug=excluded.slug, abbreviation=excluded.abbreviation,
			display_name=excluded.display_name, short_name=excluded.short_name,
			location=excluded.location, color=excluded.color,
			logo_url=excluded.logo_url, is_active=excluded.is_active,
			data=excluded.data`,
		id, sport, league, slug, abbreviation, displayName, shortName, location, color, logoURL, isActive, string(data),
	)
	if err != nil {
		return fmt.Errorf("upserting team %s: %w", id, err)
	}

	// Update FTS
	s.db.Exec(`DELETE FROM teams_fts WHERE rowid IN (SELECT rowid FROM teams_domain WHERE id = ? AND sport = ? AND league = ?)`, id, sport, league)
	s.db.Exec(`INSERT INTO teams_fts (rowid, display_name, short_name, location, abbreviation)
		SELECT rowid, display_name, short_name, location, abbreviation FROM teams_domain WHERE id = ? AND sport = ? AND league = ?`, id, sport, league)

	return nil
}

// UpsertNewsDomain parses an ESPN news article JSON.
func (s *Store) UpsertNewsDomain(sport, league string, data json.RawMessage) error {
	var article map[string]any
	if err := json.Unmarshal(data, &article); err != nil {
		return fmt.Errorf("parsing news: %w", err)
	}

	// ESPN uses numeric IDs for articles
	idRaw := article["id"]
	var id int
	switch v := idRaw.(type) {
	case float64:
		id = int(v)
	case string:
		fmt.Sscanf(v, "%d", &id)
	}
	if id == 0 {
		// Generate a hash-based ID from headline
		headline := jsonStr(article, "headline")
		if headline == "" {
			return fmt.Errorf("news article missing id and headline")
		}
		id = int(hashString(headline))
	}

	headline := jsonStr(article, "headline")
	description := jsonStr(article, "description")
	byline := jsonStr(article, "byline")
	published := jsonStr(article, "published")
	lastModified := jsonStr(article, "lastModified")
	articleType := jsonStr(article, "type")
	premium := boolToInt(jsonBool(article, "premium"))

	var webURL string
	if links, ok := article["links"].(map[string]any); ok {
		if web, ok := links["web"].(map[string]any); ok {
			webURL = jsonStr(web, "href")
		}
	}

	_, err := s.db.Exec(`INSERT INTO news_domain (id, sport, league, type, headline, description, byline, published, last_modified, premium, web_url, data)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
			sport=excluded.sport, league=excluded.league, type=excluded.type,
			headline=excluded.headline, description=excluded.description, byline=excluded.byline,
			published=excluded.published, last_modified=excluded.last_modified,
			premium=excluded.premium, web_url=excluded.web_url, data=excluded.data`,
		id, sport, league, articleType, headline, description, byline, published, lastModified, premium, webURL, string(data),
	)
	if err != nil {
		return fmt.Errorf("upserting news %d: %w", id, err)
	}

	// Update FTS
	s.db.Exec(`DELETE FROM news_fts WHERE rowid = ?`, id)
	s.db.Exec(`INSERT INTO news_fts (rowid, headline, description, byline) VALUES (?,?,?,?)`,
		id, headline, description, byline)

	return nil
}

// ── Query Methods ──

// SearchEvents performs FTS5 search on events.
func (s *Store) SearchEvents(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT e.data FROM events e
		 JOIN events_fts f ON e.rowid = f.rowid
		 WHERE events_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

// SearchNews performs FTS5 search on news articles.
func (s *Store) SearchNews(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT n.data FROM news_domain n
		 JOIN news_fts f ON n.rowid = f.rowid
		 WHERE news_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

// ListEvents returns events filtered by sport/league/team with optional completed filter.
func (s *Store) ListEvents(sport, league, teamAbbr string, limit int, completedOnly bool) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `SELECT data FROM events WHERE 1=1`
	var args []any

	if sport != "" {
		query += ` AND sport = ?`
		args = append(args, sport)
	}
	if league != "" {
		query += ` AND league = ?`
		args = append(args, league)
	}
	if teamAbbr != "" {
		query += ` AND (home_team_abbr = ? OR away_team_abbr = ?)`
		args = append(args, strings.ToUpper(teamAbbr), strings.ToUpper(teamAbbr))
	}
	if completedOnly {
		query += ` AND completed = 1`
	}
	query += ` ORDER BY date DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

// QueryRaw executes a raw SQL query and returns results as maps.
func (s *Store) QueryRaw(query string) ([]map[string]any, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(cols))
		valuePtrs := make([]any, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}
		row := make(map[string]any)
		for i, col := range cols {
			val := values[i]
			// Convert []byte to string for readability
			if b, ok := val.([]byte); ok {
				row[col] = string(b)
			} else {
				row[col] = val
			}
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// ── Count Methods ──

func (s *Store) EventCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&count)
	return count, err
}

func (s *Store) TeamCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM teams_domain`).Scan(&count)
	return count, err
}

func (s *Store) NewsCount() (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM news_domain`).Scan(&count)
	return count, err
}

// ── Sync State V2 ──

func (s *Store) GetSyncStateV2(sport, league, resource string) (lastSync, lastDate, cursor string) {
	s.db.QueryRow(
		`SELECT COALESCE(last_sync,''), COALESCE(last_date,''), COALESCE(cursor,'') FROM sync_state_v2 WHERE sport = ? AND league = ? AND resource = ?`,
		sport, league, resource,
	).Scan(&lastSync, &lastDate, &cursor)
	return
}

func (s *Store) SaveSyncStateV2(sport, league, resource, lastDate string) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state_v2 (sport, league, resource, last_sync, last_date, cursor)
		 VALUES (?, ?, ?, ?, ?, '')
		 ON CONFLICT(sport, league, resource) DO UPDATE SET
			last_sync = excluded.last_sync, last_date = excluded.last_date`,
		sport, league, resource, time.Now().UTC().Format(time.RFC3339), lastDate,
	)
	return err
}

// ── Legacy Methods (preserved for generic sync/search) ──

func (s *Store) upsertGenericResourceTx(tx *sql.Tx, resourceType, id string, data json.RawMessage) error {
	_, err := tx.Exec(
		`INSERT INTO resources (id, resource_type, data, synced_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, synced_at = excluded.synced_at, updated_at = excluded.updated_at`,
		id, resourceType, string(data), time.Now(), time.Now(),
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(`DELETE FROM resources_fts WHERE id = ?`, id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: FTS index cleanup failed: %v\n", err)
	}

	_, err = tx.Exec(
		`INSERT INTO resources_fts (id, resource_type, content)
		 VALUES (?, ?, ?)`,
		id, resourceType, string(data),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: FTS index update failed: %v\n", err)
	}

	return nil
}

func (s *Store) Upsert(resourceType, id string, data json.RawMessage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := s.upsertGenericResourceTx(tx, resourceType, id, data); err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Store) Get(resourceType, id string) (json.RawMessage, error) {
	var data string
	err := s.db.QueryRow(
		`SELECT data FROM resources WHERE resource_type = ? AND id = ?`,
		resourceType, id,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}

func (s *Store) List(resourceType string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.Query(
		`SELECT data FROM resources WHERE resource_type = ? ORDER BY updated_at DESC LIMIT ?`,
		resourceType, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

func (s *Store) Search(query string, limit int) ([]json.RawMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT r.data FROM resources r
		 JOIN resources_fts f ON r.id = f.id
		 WHERE resources_fts MATCH ?
		 ORDER BY rank
		 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []json.RawMessage
	for rows.Next() {
		var data string
		if err := rows.Scan(&data); err != nil {
			return nil, err
		}
		results = append(results, json.RawMessage(data))
	}
	return results, rows.Err()
}

func extractObjectID(obj map[string]any) string {
	for _, key := range []string{"id", "ID", "uuid", "slug", "name"} {
		if v, ok := obj[key]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func lookupFieldValue(obj map[string]any, snakeKey string) any {
	if v, ok := obj[snakeKey]; ok {
		return v
	}
	parts := strings.Split(snakeKey, "_")
	for i := 1; i < len(parts); i++ {
		if parts[i] == "" {
			continue
		}
		parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
	}
	if v, ok := obj[strings.Join(parts, "")]; ok {
		return v
	}
	return nil
}

func (s *Store) UpsertBatch(resourceType string, items []json.RawMessage) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("starting batch transaction: %w", err)
	}
	defer tx.Rollback()

	for _, item := range items {
		var obj map[string]any
		if err := json.Unmarshal(item, &obj); err != nil {
			continue
		}
		id := fmt.Sprintf("%v", lookupFieldValue(obj, "id"))
		if id == "" || id == "<nil>" {
			continue
		}

		_, err := tx.Exec(
			`INSERT OR REPLACE INTO resources (id, resource_type, data, synced_at, updated_at)
			 VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			id, resourceType, string(item),
		)
		if err != nil {
			return fmt.Errorf("upserting %s/%s: %w", resourceType, id, err)
		}

		if _, ftsErr := tx.Exec(`DELETE FROM resources_fts WHERE id = ?`, id); ftsErr != nil {
			fmt.Fprintf(os.Stderr, "warning: FTS index cleanup failed for %s/%s: %v\n", resourceType, id, ftsErr)
		}
		if _, ftsErr := tx.Exec(
			`INSERT INTO resources_fts (id, resource_type, content) VALUES (?, ?, ?)`,
			id, resourceType, string(item),
		); ftsErr != nil {
			fmt.Fprintf(os.Stderr, "warning: FTS index update failed for %s/%s: %v\n", resourceType, id, ftsErr)
		}
	}

	return tx.Commit()
}

func (s *Store) SaveSyncState(resourceType, cursor string, count int) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, ?, ?)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = excluded.last_cursor,
		 last_synced_at = excluded.last_synced_at, total_count = excluded.total_count`,
		resourceType, cursor, time.Now(), count,
	)
	return err
}

func (s *Store) GetSyncState(resourceType string) (cursor string, lastSynced time.Time, count int, err error) {
	err = s.db.QueryRow(
		`SELECT last_cursor, last_synced_at, total_count FROM sync_state WHERE resource_type = ?`,
		resourceType,
	).Scan(&cursor, &lastSynced, &count)
	if err == sql.ErrNoRows {
		return "", time.Time{}, 0, nil
	}
	return
}

func (s *Store) SaveSyncCursor(resourceType, cursor string) error {
	_, err := s.db.Exec(
		`INSERT INTO sync_state (resource_type, last_cursor, last_synced_at, total_count)
		 VALUES (?, ?, CURRENT_TIMESTAMP, 0)
		 ON CONFLICT(resource_type) DO UPDATE SET last_cursor = ?, last_synced_at = CURRENT_TIMESTAMP`,
		resourceType, cursor, cursor,
	)
	return err
}

func (s *Store) GetSyncCursor(resourceType string) string {
	var cursor sql.NullString
	s.db.QueryRow("SELECT last_cursor FROM sync_state WHERE resource_type = ?", resourceType).Scan(&cursor)
	if cursor.Valid {
		return cursor.String
	}
	return ""
}

func (s *Store) GetLastSyncedAt(resourceType string) string {
	var ts sql.NullString
	s.db.QueryRow("SELECT last_synced_at FROM sync_state WHERE resource_type = ?", resourceType).Scan(&ts)
	if ts.Valid {
		return ts.String
	}
	return ""
}

func (s *Store) ClearSyncCursors() error {
	_, err := s.db.Exec("DELETE FROM sync_state")
	return err
}

func (s *Store) Query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(query, args...)
}

func (s *Store) Count(resourceType string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM resources WHERE resource_type = ?`,
		resourceType,
	).Scan(&count)
	return count, err
}

func (s *Store) Status() (map[string]int, error) {
	rows, err := s.db.Query(
		`SELECT resource_type, COUNT(*) FROM resources GROUP BY resource_type ORDER BY resource_type`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	status := make(map[string]int)
	for rows.Next() {
		var rt string
		var count int
		if err := rows.Scan(&rt, &count); err != nil {
			return nil, err
		}
		status[rt] = count
	}
	return status, rows.Err()
}

func (s *Store) ResolveByName(resourceType string, input string, matchFields ...string) (string, error) {
	if IsUUID(input) {
		return input, nil
	}

	var matches []string
	for _, field := range matchFields {
		query := fmt.Sprintf(
			`SELECT id FROM resources WHERE resource_type = ? AND LOWER(json_extract(data, '$.%s')) = LOWER(?)`,
			field,
		)
		rows, err := s.db.Query(query, resourceType, input)
		if err != nil {
			continue
		}
		for rows.Next() {
			var id string
			if rows.Scan(&id) == nil {
				found := false
				for _, m := range matches {
					if m == id {
						found = true
						break
					}
				}
				if !found {
					matches = append(matches, id)
				}
			}
		}
		rows.Close()
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("%s %q not found in local store. Run 'sync' first, or use the UUID directly", resourceType, input)
	case 1:
		return matches[0], nil
	default:
		hint := matches[0]
		if len(matches) > 5 {
			hint = strings.Join(matches[:5], ", ") + "..."
		} else {
			hint = strings.Join(matches, ", ")
		}
		return "", fmt.Errorf("ambiguous: %q matches %d %s entries (%s). Use the exact UUID instead", input, len(matches), resourceType, hint)
	}
}

// ── Helpers ──

func jsonStr(obj map[string]any, key string) string {
	if v, ok := obj[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprintf("%v", v)
	}
	return ""
}

func jsonInt(obj map[string]any, key string) int {
	if v, ok := obj[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}

func jsonBool(obj map[string]any, key string) bool {
	if v, ok := obj[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func hashString(s string) uint32 {
	var h uint32
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	if h == 0 {
		h = 1
	}
	return h
}
