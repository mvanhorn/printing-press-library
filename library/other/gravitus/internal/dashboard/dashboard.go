// Package dashboard writes LiftingSession records to the training dashboard's
// SQLite database (dev.db) in the exact format Prisma expects.
package dashboard

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// ExerciseSet is one set within an exercise.
type ExerciseSet struct {
	Reps      int
	WeightLbs float64
}

// ExerciseEntry is one exercise block in a session.
type ExerciseEntry struct {
	Name string
	Sets []ExerciseSet
}

// LiftingSession matches the normalized Prisma LiftingSession model.
// Exercises are written to the Exercise/ExerciseSet tables, not stored as JSON.
type LiftingSession struct {
	ID        string
	Date      time.Time
	Title     string
	Exercises []ExerciseEntry
	Notes     string
	Source    string
	CreatedAt time.Time
}

// OpenDB opens the dashboard SQLite database. The caller is responsible for
// calling Close when done. Sharing one *sql.DB across all Upsert/ExistsOnDate
// calls in a sync loop avoids the per-call open/close overhead.
func OpenDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening dashboard db %s: %w", dbPath, err)
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	return db, nil
}

// Upsert writes a LiftingSession (and its exercises) to the dashboard's SQLite
// database using the same (date, source) unique constraint that Prisma enforces.
func Upsert(db *sql.DB, s LiftingSession) error {
	if s.Source == "" {
		s.Source = "gravitus"
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now().UTC()
	}
	if s.ID == "" {
		s.ID = newCUID()
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var sessionID string
	err = tx.QueryRow(
		`SELECT id FROM LiftingSession WHERE date = ? AND source = ?`,
		s.Date.UTC().Format("2006-01-02T15:04:05.000Z"), s.Source,
	).Scan(&sessionID)

	if err == sql.ErrNoRows {
		sessionID = s.ID
		_, err = tx.Exec(
			`INSERT INTO LiftingSession (id, date, title, notes, source, createdAt)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			sessionID,
			s.Date.UTC().Format("2006-01-02T15:04:05.000Z"),
			s.Title,
			nullStr(s.Notes),
			s.Source,
			s.CreatedAt.UTC().Format("2006-01-02T15:04:05.000Z"),
		)
		if err != nil {
			return fmt.Errorf("inserting lifting session: %w", err)
		}
	} else if err == nil {
		_, err = tx.Exec(
			`UPDATE LiftingSession SET title = ?, notes = ? WHERE id = ?`,
			s.Title,
			nullStr(s.Notes),
			sessionID,
		)
		if err != nil {
			return fmt.Errorf("updating lifting session: %w", err)
		}
		// Delete existing exercises so we can replace them wholesale.
		// ExerciseSet rows cascade-delete via the FK.
		if _, err = tx.Exec(`DELETE FROM Exercise WHERE sessionId = ?`, sessionID); err != nil {
			return fmt.Errorf("deleting existing exercises: %w", err)
		}
	} else {
		return fmt.Errorf("checking existing session: %w", err)
	}

	// Insert exercises and their sets.
	for i, ex := range s.Exercises {
		exID := newCUID()
		if _, err = tx.Exec(
			`INSERT INTO Exercise (id, sessionId, name, "order") VALUES (?, ?, ?, ?)`,
			exID, sessionID, ex.Name, i,
		); err != nil {
			return fmt.Errorf("inserting exercise %q: %w", ex.Name, err)
		}

		for j, set := range ex.Sets {
			if _, err = tx.Exec(
				`INSERT INTO ExerciseSet (id, exerciseId, reps, weightLbs, "order") VALUES (?, ?, ?, ?, ?)`,
				newCUID(), exID, set.Reps, set.WeightLbs, j,
			); err != nil {
				return fmt.Errorf("inserting set %d for exercise %q: %w", j, ex.Name, err)
			}
		}
	}

	return tx.Commit()
}

// ExistsOnDate returns true if a LiftingSession with (date, source) already exists.
func ExistsOnDate(db *sql.DB, date time.Time, source string) (bool, error) {
	if source == "" {
		source = "gravitus"
	}

	var id string
	err := db.QueryRow(
		`SELECT id FROM LiftingSession WHERE date = ? AND source = ?`,
		date.UTC().Format("2006-01-02T15:04:05.000Z"), source,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// TableExists checks whether the LiftingSession table is present in the database.
// Used by doctor to verify the dashboard DB is valid.
func TableExists(db *sql.DB) (bool, error) {
	var name string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='LiftingSession'`,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// newCUID generates a 25-character ID matching Prisma's cuid() format:
// lowercase 'c' followed by 24 lowercase hex characters.
func newCUID() string {
	return "c" + strings.ReplaceAll(uuid.New().String(), "-", "")[:24]
}
