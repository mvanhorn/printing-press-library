package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS grants (
    opportunity_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    agency TEXT,
    cfda_number TEXT,
    close_date TEXT,
    post_date TEXT,
    eligibility TEXT,
    synopsis TEXT,
    award_floor REAL,
    award_ceiling REAL,
    raw_json TEXT,
    synced_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sam_opportunities (
    notice_id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    agency TEXT,
    sub_tier TEXT,
    naics_code TEXT,
    set_aside TEXT,
    response_deadline TEXT,
    posted_date TEXT,
    description TEXT,
    solicitation_number TEXT,
    raw_json TEXT,
    synced_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS awards (
    award_id TEXT PRIMARY KEY,
    recipient_name TEXT,
    agency TEXT,
    cfda_number TEXT,
    amount REAL,
    start_date TEXT,
    end_date TEXT,
    description TEXT,
    raw_json TEXT,
    synced_at TEXT DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sync_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source TEXT NOT NULL,
    records_synced INTEGER,
    synced_at TEXT DEFAULT (datetime('now')),
    status TEXT,
    error TEXT
);

CREATE VIRTUAL TABLE IF NOT EXISTS grants_fts USING fts5(
    title, synopsis, agency, eligibility,
    content=grants, content_rowid=rowid
);

CREATE VIRTUAL TABLE IF NOT EXISTS sam_fts USING fts5(
    title, description, agency,
    content=sam_opportunities, content_rowid=rowid
);

CREATE VIRTUAL TABLE IF NOT EXISTS awards_fts USING fts5(
    description, recipient_name, agency,
    content=awards, content_rowid=rowid
);

CREATE TRIGGER IF NOT EXISTS grants_ai AFTER INSERT ON grants BEGIN
    INSERT INTO grants_fts(rowid, title, synopsis, agency, eligibility)
    VALUES (new.rowid, new.title, new.synopsis, new.agency, new.eligibility);
END;

CREATE TRIGGER IF NOT EXISTS grants_ad AFTER DELETE ON grants BEGIN
    INSERT INTO grants_fts(grants_fts, rowid, title, synopsis, agency, eligibility)
    VALUES ('delete', old.rowid, old.title, old.synopsis, old.agency, old.eligibility);
END;

CREATE TRIGGER IF NOT EXISTS grants_au AFTER UPDATE ON grants BEGIN
    INSERT INTO grants_fts(grants_fts, rowid, title, synopsis, agency, eligibility)
    VALUES ('delete', old.rowid, old.title, old.synopsis, old.agency, old.eligibility);
    INSERT INTO grants_fts(rowid, title, synopsis, agency, eligibility)
    VALUES (new.rowid, new.title, new.synopsis, new.agency, new.eligibility);
END;

CREATE TRIGGER IF NOT EXISTS sam_ai AFTER INSERT ON sam_opportunities BEGIN
    INSERT INTO sam_fts(rowid, title, description, agency)
    VALUES (new.rowid, new.title, new.description, new.agency);
END;

CREATE TRIGGER IF NOT EXISTS sam_ad AFTER DELETE ON sam_opportunities BEGIN
    INSERT INTO sam_fts(sam_fts, rowid, title, description, agency)
    VALUES ('delete', old.rowid, old.title, old.description, old.agency);
END;

CREATE TRIGGER IF NOT EXISTS sam_au AFTER UPDATE ON sam_opportunities BEGIN
    INSERT INTO sam_fts(sam_fts, rowid, title, description, agency)
    VALUES ('delete', old.rowid, old.title, old.description, old.agency);
    INSERT INTO sam_fts(rowid, title, description, agency)
    VALUES (new.rowid, new.title, new.description, new.agency);
END;

CREATE TRIGGER IF NOT EXISTS awards_ai AFTER INSERT ON awards BEGIN
    INSERT INTO awards_fts(rowid, description, recipient_name, agency)
    VALUES (new.rowid, new.description, new.recipient_name, new.agency);
END;

CREATE TRIGGER IF NOT EXISTS awards_ad AFTER DELETE ON awards BEGIN
    INSERT INTO awards_fts(awards_fts, rowid, description, recipient_name, agency)
    VALUES ('delete', old.rowid, old.description, old.recipient_name, old.agency);
END;

CREATE TRIGGER IF NOT EXISTS awards_au AFTER UPDATE ON awards BEGIN
    INSERT INTO awards_fts(awards_fts, rowid, description, recipient_name, agency)
    VALUES ('delete', old.rowid, old.description, old.recipient_name, old.agency);
    INSERT INTO awards_fts(rowid, description, recipient_name, agency)
    VALUES (new.rowid, new.description, new.recipient_name, new.agency);
END;
`

// DB wraps a SQLite connection.
type DB struct {
	conn *sql.DB
	Path string
}

// Open opens (or creates) the SQLite database and applies the schema.
func Open(path string) (*DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	conn, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	conn.SetMaxOpenConns(1)

	if err := applySchema(conn); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}

	return &DB{conn: conn, Path: path}, nil
}

func applySchema(conn *sql.DB) error {
	_, err := conn.Exec(schema)
	return err
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

// Conn returns the underlying sql.DB for direct queries.
func (d *DB) Conn() *sql.DB {
	return d.conn
}

// HealthCheck verifies the DB is accessible and FTS5 is functional.
func (d *DB) HealthCheck() error {
	var name string
	if err := d.conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='grants' LIMIT 1").Scan(&name); err != nil {
		return fmt.Errorf("grants table missing: %w", err)
	}

	// Test FTS5
	if _, err := d.conn.Exec("INSERT INTO grants_fts(grants_fts) VALUES('integrity-check')"); err != nil {
		return fmt.Errorf("FTS5 integrity check failed: %w", err)
	}

	return nil
}

// SyncLogEntry represents a single sync operation log entry.
type SyncLogEntry struct {
	ID             int64
	Source         string
	RecordsSynced  int
	SyncedAt       string
	Status         string
	Error          string
}

// LogSync writes a sync operation result to the sync_log table.
func (d *DB) LogSync(source string, records int, status, errMsg string) error {
	_, err := d.conn.Exec(
		`INSERT INTO sync_log (source, records_synced, status, error) VALUES (?, ?, ?, ?)`,
		source, records, status, errMsg,
	)
	return err
}

// LastSync returns the most recent sync entry for the given source.
func (d *DB) LastSync(source string) (*SyncLogEntry, error) {
	row := d.conn.QueryRow(
		`SELECT id, source, records_synced, synced_at, status, COALESCE(error,'') FROM sync_log WHERE source=? ORDER BY id DESC LIMIT 1`,
		source,
	)
	var e SyncLogEntry
	if err := row.Scan(&e.ID, &e.Source, &e.RecordsSynced, &e.SyncedAt, &e.Status, &e.Error); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &e, nil
}

// Grant represents a grants.gov opportunity record.
type Grant struct {
	OpportunityID string
	Title         string
	Agency        string
	CFDANumber    string
	CloseDate     string
	PostDate      string
	Eligibility   string
	Synopsis      string
	AwardFloor    float64
	AwardCeiling  float64
	RawJSON       string
}

// UpsertGrant inserts or replaces a grant record.
func (d *DB) UpsertGrant(g *Grant) error {
	_, err := d.conn.Exec(`
		INSERT INTO grants (opportunity_id, title, agency, cfda_number, close_date, post_date, eligibility, synopsis, award_floor, award_ceiling, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(opportunity_id) DO UPDATE SET
			title=excluded.title, agency=excluded.agency, cfda_number=excluded.cfda_number,
			close_date=excluded.close_date, post_date=excluded.post_date, eligibility=excluded.eligibility,
			synopsis=excluded.synopsis, award_floor=excluded.award_floor, award_ceiling=excluded.award_ceiling,
			raw_json=excluded.raw_json, synced_at=datetime('now')`,
		g.OpportunityID, g.Title, g.Agency, g.CFDANumber, g.CloseDate, g.PostDate,
		g.Eligibility, g.Synopsis, g.AwardFloor, g.AwardCeiling, g.RawJSON,
	)
	return err
}

// SearchGrants performs FTS5 full-text search on grants.
func (d *DB) SearchGrants(query string, limit int) ([]*Grant, error) {
	rows, err := d.conn.Query(`
		SELECT g.opportunity_id, g.title, g.agency, g.cfda_number, g.close_date, g.post_date,
		       g.eligibility, g.synopsis, g.award_floor, g.award_ceiling, g.raw_json
		FROM grants g
		JOIN grants_fts f ON g.rowid = f.rowid
		WHERE grants_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGrants(rows)
}

// GetGrant fetches a single grant by opportunity ID.
func (d *DB) GetGrant(id string) (*Grant, error) {
	row := d.conn.QueryRow(`
		SELECT opportunity_id, title, agency, cfda_number, close_date, post_date,
		       eligibility, synopsis, award_floor, award_ceiling, raw_json
		FROM grants WHERE opportunity_id=?`, id)
	g := &Grant{}
	err := row.Scan(&g.OpportunityID, &g.Title, &g.Agency, &g.CFDANumber, &g.CloseDate,
		&g.PostDate, &g.Eligibility, &g.Synopsis, &g.AwardFloor, &g.AwardCeiling, &g.RawJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return g, err
}

// GrantsClosingWithin returns grants closing within the next N days.
func (d *DB) GrantsClosingWithin(days int) ([]*Grant, error) {
	rows, err := d.conn.Query(`
		SELECT opportunity_id, title, agency, cfda_number, close_date, post_date,
		       eligibility, synopsis, award_floor, award_ceiling, raw_json
		FROM grants
		WHERE close_date BETWEEN datetime('now') AND datetime('now', '+'||?||' days')
		ORDER BY close_date ASC`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGrants(rows)
}

func scanGrants(rows *sql.Rows) ([]*Grant, error) {
	var results []*Grant
	for rows.Next() {
		g := &Grant{}
		if err := rows.Scan(&g.OpportunityID, &g.Title, &g.Agency, &g.CFDANumber, &g.CloseDate,
			&g.PostDate, &g.Eligibility, &g.Synopsis, &g.AwardFloor, &g.AwardCeiling, &g.RawJSON); err != nil {
			return nil, err
		}
		results = append(results, g)
	}
	return results, rows.Err()
}

// SAMOpportunity represents a SAM.gov contract notice.
type SAMOpportunity struct {
	NoticeID             string
	Title                string
	Agency               string
	SubTier              string
	NAICSCode            string
	SetAside             string
	ResponseDeadline     string
	PostedDate           string
	Description          string
	SolicitationNumber   string
	RawJSON              string
}

// UpsertSAM inserts or replaces a SAM opportunity.
func (d *DB) UpsertSAM(s *SAMOpportunity) error {
	_, err := d.conn.Exec(`
		INSERT INTO sam_opportunities (notice_id, title, agency, sub_tier, naics_code, set_aside, response_deadline, posted_date, description, solicitation_number, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(notice_id) DO UPDATE SET
			title=excluded.title, agency=excluded.agency, sub_tier=excluded.sub_tier,
			naics_code=excluded.naics_code, set_aside=excluded.set_aside,
			response_deadline=excluded.response_deadline, posted_date=excluded.posted_date,
			description=excluded.description, solicitation_number=excluded.solicitation_number,
			raw_json=excluded.raw_json, synced_at=datetime('now')`,
		s.NoticeID, s.Title, s.Agency, s.SubTier, s.NAICSCode, s.SetAside,
		s.ResponseDeadline, s.PostedDate, s.Description, s.SolicitationNumber, s.RawJSON,
	)
	return err
}

// SearchSAM performs FTS5 search on SAM opportunities.
func (d *DB) SearchSAM(query string, limit int) ([]*SAMOpportunity, error) {
	rows, err := d.conn.Query(`
		SELECT s.notice_id, s.title, s.agency, s.sub_tier, s.naics_code, s.set_aside,
		       s.response_deadline, s.posted_date, s.description, s.solicitation_number, s.raw_json
		FROM sam_opportunities s
		JOIN sam_fts f ON s.rowid = f.rowid
		WHERE sam_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSAM(rows)
}

// GetSAM fetches a single SAM opportunity by notice ID.
func (d *DB) GetSAM(id string) (*SAMOpportunity, error) {
	row := d.conn.QueryRow(`
		SELECT notice_id, title, agency, sub_tier, naics_code, set_aside,
		       response_deadline, posted_date, description, solicitation_number, raw_json
		FROM sam_opportunities WHERE notice_id=?`, id)
	s := &SAMOpportunity{}
	err := row.Scan(&s.NoticeID, &s.Title, &s.Agency, &s.SubTier, &s.NAICSCode, &s.SetAside,
		&s.ResponseDeadline, &s.PostedDate, &s.Description, &s.SolicitationNumber, &s.RawJSON)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

// SAMBySetAside returns opportunities filtered by set-aside code.
func (d *DB) SAMBySetAside(setAside string, limit int) ([]*SAMOpportunity, error) {
	rows, err := d.conn.Query(`
		SELECT notice_id, title, agency, sub_tier, naics_code, set_aside,
		       response_deadline, posted_date, description, solicitation_number, raw_json
		FROM sam_opportunities
		WHERE set_aside = ?
		ORDER BY response_deadline ASC
		LIMIT ?`, setAside, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanSAM(rows)
}

func scanSAM(rows *sql.Rows) ([]*SAMOpportunity, error) {
	var results []*SAMOpportunity
	for rows.Next() {
		s := &SAMOpportunity{}
		if err := rows.Scan(&s.NoticeID, &s.Title, &s.Agency, &s.SubTier, &s.NAICSCode, &s.SetAside,
			&s.ResponseDeadline, &s.PostedDate, &s.Description, &s.SolicitationNumber, &s.RawJSON); err != nil {
			return nil, err
		}
		results = append(results, s)
	}
	return results, rows.Err()
}

// Award represents a USASpending.gov award record.
type Award struct {
	AwardID       string
	RecipientName string
	Agency        string
	CFDANumber    string
	Amount        float64
	StartDate     string
	EndDate       string
	Description   string
	RawJSON       string
}

// UpsertAward inserts or replaces an award record.
func (d *DB) UpsertAward(a *Award) error {
	_, err := d.conn.Exec(`
		INSERT INTO awards (award_id, recipient_name, agency, cfda_number, amount, start_date, end_date, description, raw_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(award_id) DO UPDATE SET
			recipient_name=excluded.recipient_name, agency=excluded.agency, cfda_number=excluded.cfda_number,
			amount=excluded.amount, start_date=excluded.start_date, end_date=excluded.end_date,
			description=excluded.description, raw_json=excluded.raw_json, synced_at=datetime('now')`,
		a.AwardID, a.RecipientName, a.Agency, a.CFDANumber, a.Amount,
		a.StartDate, a.EndDate, a.Description, a.RawJSON,
	)
	return err
}

// SearchAwards performs FTS5 search on awards.
func (d *DB) SearchAwards(query string, limit int) ([]*Award, error) {
	rows, err := d.conn.Query(`
		SELECT a.award_id, a.recipient_name, a.agency, a.cfda_number, a.amount,
		       a.start_date, a.end_date, a.description, a.raw_json
		FROM awards a
		JOIN awards_fts f ON a.rowid = f.rowid
		WHERE awards_fts MATCH ?
		ORDER BY rank
		LIMIT ?`, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAwards(rows)
}

// AwardsByRecipient returns awards for a given recipient (fuzzy name).
func (d *DB) AwardsByRecipient(name string, limit int) ([]*Award, error) {
	rows, err := d.conn.Query(`
		SELECT award_id, recipient_name, agency, cfda_number, amount,
		       start_date, end_date, description, raw_json
		FROM awards
		WHERE recipient_name LIKE ?
		ORDER BY amount DESC
		LIMIT ?`, "%"+name+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAwards(rows)
}

// AwardsByAgency returns awards for a given agency.
func (d *DB) AwardsByAgency(agency string, limit int) ([]*Award, error) {
	rows, err := d.conn.Query(`
		SELECT award_id, recipient_name, agency, cfda_number, amount,
		       start_date, end_date, description, raw_json
		FROM awards
		WHERE agency LIKE ?
		ORDER BY amount DESC
		LIMIT ?`, "%"+agency+"%", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAwards(rows)
}

func scanAwards(rows *sql.Rows) ([]*Award, error) {
	var results []*Award
	for rows.Next() {
		a := &Award{}
		if err := rows.Scan(&a.AwardID, &a.RecipientName, &a.Agency, &a.CFDANumber, &a.Amount,
			&a.StartDate, &a.EndDate, &a.Description, &a.RawJSON); err != nil {
			return nil, err
		}
		results = append(results, a)
	}
	return results, rows.Err()
}

// ZombieGrant represents a grant closing soon with no award history.
type ZombieGrant struct {
	Grant
	DaysUntilClose int
}

// ZombieGrants returns grants closing within N days that have no matching award records by CFDA.
func (d *DB) ZombieGrants(days int) ([]*ZombieGrant, error) {
	rows, err := d.conn.Query(`
		SELECT g.opportunity_id, g.title, g.agency, g.cfda_number, g.close_date, g.post_date,
		       g.eligibility, g.synopsis, g.award_floor, g.award_ceiling, g.raw_json,
		       CAST(julianday(g.close_date) - julianday('now') AS INTEGER) as days_left
		FROM grants g
		LEFT JOIN awards a ON g.cfda_number = a.cfda_number AND g.cfda_number != ''
		WHERE g.close_date BETWEEN datetime('now') AND datetime('now', '+'||?||' days')
		  AND a.award_id IS NULL
		ORDER BY g.close_date ASC`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []*ZombieGrant
	for rows.Next() {
		z := &ZombieGrant{}
		if err := rows.Scan(
			&z.OpportunityID, &z.Title, &z.Agency, &z.CFDANumber, &z.CloseDate, &z.PostDate,
			&z.Eligibility, &z.Synopsis, &z.AwardFloor, &z.AwardCeiling, &z.RawJSON,
			&z.DaysUntilClose,
		); err != nil {
			return nil, err
		}
		results = append(results, z)
	}
	return results, rows.Err()
}

// AgencyProfile holds aggregated agency intelligence.
type AgencyProfile struct {
	Agency         string
	OpenGrants     []*Grant
	OpenSAM        []*SAMOpportunity
	TotalAwarded   float64
	AwardCount     int
	TopRecipients  []string
}

// GetAgencyProfile assembles a full agency intelligence card.
func (d *DB) GetAgencyProfile(agency string) (*AgencyProfile, error) {
	profile := &AgencyProfile{Agency: agency}

	// Open grants
	grants, err := d.conn.Query(`
		SELECT opportunity_id, title, agency, cfda_number, close_date, post_date,
		       eligibility, synopsis, award_floor, award_ceiling, raw_json
		FROM grants WHERE agency LIKE ? AND close_date >= datetime('now')
		ORDER BY close_date ASC LIMIT 10`, "%"+agency+"%")
	if err != nil {
		return nil, err
	}
	defer grants.Close()
	profile.OpenGrants, err = scanGrants(grants)
	if err != nil {
		return nil, err
	}

	// Open SAM notices
	sam, err := d.conn.Query(`
		SELECT notice_id, title, agency, sub_tier, naics_code, set_aside,
		       response_deadline, posted_date, description, solicitation_number, raw_json
		FROM sam_opportunities WHERE agency LIKE ?
		ORDER BY response_deadline ASC LIMIT 10`, "%"+agency+"%")
	if err != nil {
		return nil, err
	}
	defer sam.Close()
	profile.OpenSAM, err = scanSAM(sam)
	if err != nil {
		return nil, err
	}

	// Award totals
	row := d.conn.QueryRow(`SELECT COALESCE(SUM(amount),0), COUNT(*) FROM awards WHERE agency LIKE ?`, "%"+agency+"%")
	if err := row.Scan(&profile.TotalAwarded, &profile.AwardCount); err != nil {
		return nil, err
	}

	// Top recipients
	recips, err := d.conn.Query(`
		SELECT recipient_name, SUM(amount) as total
		FROM awards WHERE agency LIKE ?
		GROUP BY recipient_name ORDER BY total DESC LIMIT 5`, "%"+agency+"%")
	if err != nil {
		return nil, err
	}
	defer recips.Close()
	for recips.Next() {
		var name string
		var total float64
		if err := recips.Scan(&name, &total); err != nil {
			return nil, err
		}
		profile.TopRecipients = append(profile.TopRecipients, fmt.Sprintf("%s ($%.0f)", name, total))
	}

	return profile, recips.Err()
}
