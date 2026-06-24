package fares

import (
	"database/sql"
	"fmt"
)

// FeedData holds all parsed RJFAF feed records for a single bulk load.
type FeedData struct {
	Locations    []Location
	Flows        []Flow
	Fares        []Fare
	NDF          []NonDerivableFare
	Tickets      []TicketType
	Railcards    []Railcard
	Clusters     []ClusterMember
	GroupMembers []GroupMember
	Restrictions []Restriction
}

// EnsureSchema creates the rjf_* tables and their indexes if they do not
// already exist. Safe to call repeatedly on the same database.
func EnsureSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS rjf_locations (
			nlc        TEXT,
			crs        TEXT,
			name       TEXT,
			start_date TEXT,
			end_date   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_locations_crs ON rjf_locations(crs)`,

		`CREATE TABLE IF NOT EXISTS rjf_flows (
			flow_id    TEXT,
			origin_nlc TEXT,
			dest_nlc   TEXT,
			route      TEXT,
			direction  TEXT,
			toc        TEXT,
			start_date TEXT,
			end_date   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_flows_od ON rjf_flows(origin_nlc, dest_nlc)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_flows_id ON rjf_flows(flow_id)`,

		`CREATE TABLE IF NOT EXISTS rjf_fares (
			flow_id          TEXT,
			ticket_code      TEXT,
			pence            INTEGER,
			restriction_code TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_fares_flow ON rjf_fares(flow_id)`,

		`CREATE TABLE IF NOT EXISTS rjf_ndf (
			origin_nlc       TEXT,
			dest_nlc         TEXT,
			route            TEXT,
			ticket_code      TEXT,
			pence            INTEGER,
			restriction_code TEXT,
			start_date       TEXT,
			end_date         TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_ndf_od ON rjf_ndf(origin_nlc, dest_nlc)`,

		`CREATE TABLE IF NOT EXISTS rjf_ticket_types (
			code         TEXT PRIMARY KEY,
			description  TEXT,
			ticket_class TEXT,
			ticket_type  TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS rjf_railcards (
			code         TEXT PRIMARY KEY,
			description  TEXT,
			min_pence    INTEGER,
			discount_pct INTEGER
		)`,

		`CREATE TABLE IF NOT EXISTS rjf_clusters (
			cluster_id TEXT,
			member_nlc TEXT,
			start_date TEXT,
			end_date   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_clusters_member ON rjf_clusters(member_nlc)`,

		`CREATE TABLE IF NOT EXISTS rjf_group_members (
			member_nlc TEXT,
			group_nlc  TEXT,
			end_date   TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_rjf_group_members_member ON rjf_group_members(member_nlc)`,

		`CREATE TABLE IF NOT EXISTS rjf_restrictions (
			code        TEXT PRIMARY KEY,
			description TEXT
		)`,

		`CREATE TABLE IF NOT EXISTS rjf_meta (
			id            INTEGER PRIMARY KEY CHECK(id=1),
			sequence      TEXT,
			last_modified TEXT,
			publish_date  TEXT,
			synced_at     TEXT
		)`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("fares: EnsureSchema: %w", err)
		}
	}
	return nil
}

// Load replaces all rows in the 9 rjf_* data tables (not rjf_meta) with
// the contents of data. The operation runs inside a single transaction;
// on any error the transaction is rolled back and the tables are left
// unchanged.
func Load(db *sql.DB, data *FeedData) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("fares: Load: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	dataTables := []string{
		"rjf_locations",
		"rjf_flows",
		"rjf_fares",
		"rjf_ndf",
		"rjf_ticket_types",
		"rjf_railcards",
		"rjf_clusters",
		"rjf_group_members",
		"rjf_restrictions",
	}
	for _, t := range dataTables {
		if _, err := tx.Exec("DELETE FROM " + t); err != nil {
			return fmt.Errorf("fares: Load: delete %s: %w", t, err)
		}
	}

	if err := insertLocations(tx, data.Locations); err != nil {
		return err
	}
	if err := insertFlows(tx, data.Flows); err != nil {
		return err
	}
	if err := insertFares(tx, data.Fares); err != nil {
		return err
	}
	if err := insertNDF(tx, data.NDF); err != nil {
		return err
	}
	if err := insertTicketTypes(tx, data.Tickets); err != nil {
		return err
	}
	if err := insertRailcards(tx, data.Railcards); err != nil {
		return err
	}
	if err := insertClusters(tx, data.Clusters); err != nil {
		return err
	}
	if err := insertGroupMembers(tx, data.GroupMembers); err != nil {
		return err
	}
	if err := insertRestrictions(tx, data.Restrictions); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("fares: Load: commit: %w", err)
	}
	return nil
}

func insertLocations(tx *sql.Tx, rows []Location) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO rjf_locations(nlc,crs,name,start_date,end_date) VALUES(?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_locations: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.NLC, r.CRS, r.Name, r.StartDate, r.EndDate); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_locations: %w", err)
		}
	}
	return nil
}

func insertFlows(tx *sql.Tx, rows []Flow) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO rjf_flows(flow_id,origin_nlc,dest_nlc,route,direction,toc,start_date,end_date) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_flows: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.FlowID, r.OriginNLC, r.DestNLC, r.Route, r.Direction, r.TOC, r.StartDate, r.EndDate); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_flows: %w", err)
		}
	}
	return nil
}

func insertFares(tx *sql.Tx, rows []Fare) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO rjf_fares(flow_id,ticket_code,pence,restriction_code) VALUES(?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_fares: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.FlowID, r.TicketCode, r.Pence, r.RestrictionCode); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_fares: %w", err)
		}
	}
	return nil
}

func insertNDF(tx *sql.Tx, rows []NonDerivableFare) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO rjf_ndf(origin_nlc,dest_nlc,route,ticket_code,pence,restriction_code,start_date,end_date) VALUES(?,?,?,?,?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_ndf: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.OriginNLC, r.DestNLC, r.Route, r.TicketCode, r.Pence, r.RestrictionCode, r.StartDate, r.EndDate); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_ndf: %w", err)
		}
	}
	return nil
}

func insertTicketTypes(tx *sql.Tx, rows []TicketType) error {
	if len(rows) == 0 {
		return nil
	}
	// The RJFAF feed carries multiple dated rows per ticket code; only the
	// display description varies across them, so last-write-wins is correct.
	stmt, err := tx.Prepare(`INSERT INTO rjf_ticket_types(code,description,ticket_class,ticket_type) VALUES(?,?,?,?) ON CONFLICT(code) DO UPDATE SET description=excluded.description, ticket_class=excluded.ticket_class, ticket_type=excluded.ticket_type`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_ticket_types: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Code, r.Description, r.TicketClass, r.TicketType); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_ticket_types: %w", err)
		}
	}
	return nil
}

func insertRailcards(tx *sql.Tx, rows []Railcard) error {
	if len(rows) == 0 {
		return nil
	}
	// Railcard codes repeat across dated rows in the feed; last-write-wins.
	stmt, err := tx.Prepare(`INSERT INTO rjf_railcards(code,description,min_pence,discount_pct) VALUES(?,?,?,?) ON CONFLICT(code) DO UPDATE SET description=excluded.description, min_pence=excluded.min_pence, discount_pct=excluded.discount_pct`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_railcards: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Code, r.Description, r.MinPence, r.DiscountPct); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_railcards: %w", err)
		}
	}
	return nil
}

func insertClusters(tx *sql.Tx, rows []ClusterMember) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO rjf_clusters(cluster_id,member_nlc,start_date,end_date) VALUES(?,?,?,?)`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_clusters: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.ClusterID, r.MemberNLC, r.StartDate, r.EndDate); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_clusters: %w", err)
		}
	}
	return nil
}

func insertGroupMembers(tx *sql.Tx, rows []GroupMember) error {
	if len(rows) == 0 {
		return nil
	}
	stmt, err := tx.Prepare(`INSERT INTO rjf_group_members(member_nlc,group_nlc,end_date) VALUES(?,?,?)`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_group_members: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.MemberNLC, r.GroupNLC, r.EndDate); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_group_members: %w", err)
		}
	}
	return nil
}

func insertRestrictions(tx *sql.Tx, rows []Restriction) error {
	if len(rows) == 0 {
		return nil
	}
	// Restriction codes repeat across dated rows in the feed; last-write-wins.
	stmt, err := tx.Prepare(`INSERT INTO rjf_restrictions(code,description) VALUES(?,?) ON CONFLICT(code) DO UPDATE SET description=excluded.description`)
	if err != nil {
		return fmt.Errorf("fares: Load: prepare rjf_restrictions: %w", err)
	}
	defer stmt.Close()
	for _, r := range rows {
		if _, err := stmt.Exec(r.Code, r.Description); err != nil {
			return fmt.Errorf("fares: Load: insert rjf_restrictions: %w", err)
		}
	}
	return nil
}

// WriteMeta upserts the single metadata row (id=1) into rjf_meta.
// The caller is responsible for setting FeedMeta.SyncedAt before calling.
func WriteMeta(db *sql.DB, m FeedMeta) error {
	_, err := db.Exec(
		`INSERT INTO rjf_meta(id,sequence,last_modified,publish_date,synced_at) VALUES(1,?,?,?,?)
		 ON CONFLICT(id) DO UPDATE SET
		   sequence=excluded.sequence,
		   last_modified=excluded.last_modified,
		   publish_date=excluded.publish_date,
		   synced_at=excluded.synced_at`,
		m.Sequence, m.LastModified, m.PublishDate, m.SyncedAt,
	)
	if err != nil {
		return fmt.Errorf("fares: WriteMeta: %w", err)
	}
	return nil
}

// ReadMeta returns the stored feed metadata. If no row exists it returns
// (FeedMeta{}, false, nil). On any other error it returns (FeedMeta{}, false, err).
func ReadMeta(db *sql.DB) (FeedMeta, bool, error) {
	var m FeedMeta
	err := db.QueryRow(
		`SELECT sequence,last_modified,publish_date,synced_at FROM rjf_meta WHERE id=1`,
	).Scan(&m.Sequence, &m.LastModified, &m.PublishDate, &m.SyncedAt)
	if err == sql.ErrNoRows {
		return FeedMeta{}, false, nil
	}
	if err != nil {
		return FeedMeta{}, false, fmt.Errorf("fares: ReadMeta: %w", err)
	}
	return m, true, nil
}
