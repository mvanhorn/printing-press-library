package trust

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type AuditEvent struct {
	ID        int64     `json:"id,omitempty"`
	Event     string    `json:"event"`
	KID       string    `json:"kid"`
	OrgAlias  string    `json:"org"`
	Source    string    `json:"source"`
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

type SetupAuditEntry struct {
	Action      string    `json:"action"`
	Display     string    `json:"display"`
	CreatedDate time.Time `json:"created_date"`
}

type BundleAuditHealth struct {
	Status    string `json:"status"`
	Total     int64  `json:"total"`
	OK        int64  `json:"ok"`
	Failed    int64  `json:"failed"`
	Pending   int64  `json:"pending"`
	LastError string `json:"last_error,omitempty"`
}

func RecordAuditEvent(event, kid, orgAlias, source, detail string) error {
	db, err := openAuditDB()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := ensureAuditSchema(db); err != nil {
		return err
	}
	_, err = db.Exec(
		`INSERT INTO trust_audit(event, kid, org_alias, source, detail, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		event, kid, orgAlias, source, detail, time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func LocalAuditEvents() ([]AuditEvent, error) {
	db, err := openAuditDB()
	if err != nil {
		return nil, err
	}
	defer db.Close()
	if err := ensureAuditSchema(db); err != nil {
		return nil, err
	}
	rows, err := db.Query(`SELECT id, event, kid, org_alias, source, detail, created_at FROM trust_audit ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AuditEvent
	for rows.Next() {
		var event AuditEvent
		var created string
		if err := rows.Scan(&event.ID, &event.Event, &event.KID, &event.OrgAlias, &event.Source, &event.Detail, &created); err != nil {
			return nil, err
		}
		event.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		events = append(events, event)
	}
	return events, rows.Err()
}

func FetchSetupAuditTrail(c OrgClient, kids []string) ([]SetupAuditEntry, error) {
	if c == nil {
		return nil, fmt.Errorf("org client required")
	}
	q := "SELECT Action, Display, CreatedDate FROM SetupAuditTrail WHERE CreatedDate = LAST_N_DAYS:30"
	raw, err := c.Get("/services/data/"+APIVersion+"/tooling/query", map[string]string{"q": q})
	if err != nil {
		return nil, err
	}
	var payload struct {
		Records []struct {
			Action      string `json:"Action"`
			Display     string `json:"Display"`
			CreatedDate string `json:"CreatedDate"`
		} `json:"records"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	var entries []SetupAuditEntry
	for _, record := range payload.Records {
		if !auditRecordMatchesKids(record.Display, kids) {
			continue
		}
		created, _ := time.Parse(time.RFC3339, record.CreatedDate)
		entries = append(entries, SetupAuditEntry{
			Action:      record.Action,
			Display:     record.Display,
			CreatedDate: created,
		})
	}
	return entries, nil
}

func LocalBundleAuditHealth(db *sql.DB) (BundleAuditHealth, error) {
	if db == nil {
		return BundleAuditHealth{Status: "unknown"}, nil
	}
	health := BundleAuditHealth{Status: "unknown"}
	rows, err := db.Query(`SELECT write_status, COUNT(*) FROM bundle_audit_local GROUP BY write_status`)
	if err != nil {
		return health, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return health, err
		}
		health.Total += count
		switch status {
		case "ok":
			health.OK = count
		case "failed":
			health.Failed = count
		case "pending":
			health.Pending = count
		}
	}
	if err := rows.Err(); err != nil {
		return health, err
	}
	_ = db.QueryRow(`SELECT COALESCE(remote_error, '') FROM bundle_audit_local WHERE write_status = 'failed' ORDER BY generated_at DESC LIMIT 1`).Scan(&health.LastError)
	switch {
	case health.Failed > 0:
		health.Status = "red"
	case health.Pending > 0:
		health.Status = "yellow"
	case health.Total > 0:
		health.Status = "green"
	default:
		health.Status = "unknown"
	}
	return health, nil
}

func auditRecordMatchesKids(display string, kids []string) bool {
	if strings.Contains(display, "SF360_Bundle_Key") {
		return true
	}
	for _, kid := range kids {
		if kid != "" && strings.Contains(display, kid) {
			return true
		}
	}
	return false
}

func openAuditDB() (*sql.DB, error) {
	dir, err := keystoreDir()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return sql.Open("sqlite", filepath.Join(dir, "trust_audit.sqlite"))
}

func ensureAuditSchema(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS trust_audit (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		event TEXT NOT NULL,
		kid TEXT NOT NULL,
		org_alias TEXT NOT NULL,
		source TEXT NOT NULL,
		detail TEXT,
		created_at TEXT NOT NULL
	)`)
	return err
}
