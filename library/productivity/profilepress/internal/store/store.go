package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"profilepress-pp-cli/internal/message"
	"profilepress-pp-cli/internal/packet"
	"profilepress-pp-cli/internal/profile"
)

type Store struct{ db *sql.DB }

func DefaultPath() string {
	if v := os.Getenv("PROFILEPRESS_DB"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "profilepress", "profilepress.db")
}

func Open(path string) (*Store, error) {
	if path == "" {
		path = DefaultPath()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

func NewID(prefix string) string { return fmt.Sprintf("%s_%s", prefix, uuid.NewString()) }

func (s *Store) CreateSnapshot(source string, sections []profile.Section) (profile.Snapshot, error) {
	snap := profile.Snapshot{ID: NewID("snap"), CreatedAt: time.Now().UTC(), Source: source, Sections: sections}
	b, err := json.Marshal(sections)
	if err != nil {
		return snap, err
	}
	_, err = s.db.Exec(`INSERT INTO snapshots(id, created_at, source, sections_json) VALUES(?,?,?,?)`, snap.ID, snap.CreatedAt.Format(time.RFC3339), snap.Source, string(b))
	return snap, err
}

func (s *Store) GetSnapshot(id string) (profile.Snapshot, error) {
	var snap profile.Snapshot
	var created, sections string
	err := s.db.QueryRow(`SELECT id, created_at, source, sections_json FROM snapshots WHERE id=?`, id).Scan(&snap.ID, &created, &snap.Source, &sections)
	if err != nil {
		return snap, err
	}
	snap.CreatedAt, _ = time.Parse(time.RFC3339, created)
	if err := json.Unmarshal([]byte(sections), &snap.Sections); err != nil {
		return snap, err
	}
	return snap, nil
}

func (s *Store) LatestSnapshot() (profile.Snapshot, error) {
	var id string
	if err := s.db.QueryRow(`SELECT id FROM snapshots ORDER BY created_at DESC LIMIT 1`).Scan(&id); err != nil {
		return profile.Snapshot{}, err
	}
	return s.GetSnapshot(id)
}

func (s *Store) CreatePacket(p packet.Packet) (packet.Packet, error) {
	if p.ID == "" {
		p.ID = NewID("pkt")
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now().UTC()
	}
	if p.Status == "" {
		p.Status = "proposed"
	}
	risk, err := json.Marshal(p.RiskNotes)
	if err != nil {
		return p, err
	}
	changes, err := json.Marshal(p.Changes)
	if err != nil {
		return p, err
	}
	_, err = s.db.Exec(`INSERT INTO packets(id, snapshot_id, created_at, opportunity, opportunity_id, status, risk_notes_json, changes_json) VALUES(?,?,?,?,?,?,?,?)`, p.ID, p.SnapshotID, p.CreatedAt.Format(time.RFC3339), p.Opportunity, p.OpportunityID, p.Status, string(risk), string(changes))
	return p, err
}

func (s *Store) GetPacket(id string) (packet.Packet, error) {
	var p packet.Packet
	var created, risk, changes string
	err := s.db.QueryRow(`SELECT id, snapshot_id, created_at, opportunity, opportunity_id, status, risk_notes_json, changes_json FROM packets WHERE id=?`, id).Scan(&p.ID, &p.SnapshotID, &created, &p.Opportunity, &p.OpportunityID, &p.Status, &risk, &changes)
	if err != nil {
		return p, err
	}
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	if err := json.Unmarshal([]byte(risk), &p.RiskNotes); err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(changes), &p.Changes); err != nil {
		return p, err
	}
	return p, nil
}

func (s *Store) LatestPacket() (packet.Packet, error) {
	var id string
	if err := s.db.QueryRow(`SELECT id FROM packets ORDER BY created_at DESC LIMIT 1`).Scan(&id); err != nil {
		return packet.Packet{}, err
	}
	return s.GetPacket(id)
}

func (s *Store) AddApplyLog(log packet.ApplyLog) (packet.ApplyLog, error) {
	if log.ID == "" {
		log.ID = NewID("apply")
	}
	if log.CreatedAt.IsZero() {
		log.CreatedAt = time.Now().UTC()
	}
	dry := 0
	if log.DryRun {
		dry = 1
	}
	_, err := s.db.Exec(`INSERT INTO apply_logs(id, packet_id, created_at, privacy_status, sensitive_status, result, dry_run, confirmation_source) VALUES(?,?,?,?,?,?,?,?)`, log.ID, log.PacketID, log.CreatedAt.Format(time.RFC3339), log.PrivacyStatus, log.SensitiveStatus, log.Result, dry, log.ConfirmationSource)
	return log, err
}

func (s *Store) CreateMessageDraft(d message.Draft) (message.Draft, error) {
	if d.ID == "" {
		d.ID = NewID("msg")
	}
	if d.CreatedAt.IsZero() {
		d.CreatedAt = time.Now().UTC()
	}
	if d.Status == "" {
		d.Status = "draft"
	}
	_, err := s.db.Exec(`INSERT INTO message_drafts(id, created_at, recipient, body, status, source_note, sent_at, send_mode, confirm_text) VALUES(?,?,?,?,?,?,?,?,?)`, d.ID, d.CreatedAt.Format(time.RFC3339), d.To, d.Body, d.Status, d.SourceNote, timeString(d.SentAt), d.SendMode, d.ConfirmText)
	return d, err
}

func (s *Store) GetMessageDraft(id string) (message.Draft, error) {
	var d message.Draft
	var created, sent string
	err := s.db.QueryRow(`SELECT id, created_at, recipient, body, status, source_note, sent_at, send_mode, confirm_text FROM message_drafts WHERE id=?`, id).Scan(&d.ID, &created, &d.To, &d.Body, &d.Status, &d.SourceNote, &sent, &d.SendMode, &d.ConfirmText)
	if err != nil {
		return d, err
	}
	d.CreatedAt, _ = time.Parse(time.RFC3339, created)
	if sent != "" {
		d.SentAt, _ = time.Parse(time.RFC3339, sent)
	}
	return d, nil
}

func (s *Store) LatestMessageDraft() (message.Draft, error) {
	var id string
	if err := s.db.QueryRow(`SELECT id FROM message_drafts ORDER BY created_at DESC LIMIT 1`).Scan(&id); err != nil {
		return message.Draft{}, err
	}
	return s.GetMessageDraft(id)
}

func (s *Store) ListMessageDrafts(limit int) ([]message.Draft, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.Query(`SELECT id FROM message_drafts ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var drafts []message.Draft
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		d, err := s.GetMessageDraft(id)
		if err != nil {
			return nil, err
		}
		drafts = append(drafts, d)
	}
	return drafts, rows.Err()
}

func (s *Store) MarkMessageSent(id, mode, confirm string) (message.Draft, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE message_drafts SET status=?, sent_at=?, send_mode=?, confirm_text=? WHERE id=?`, "sent", now.Format(time.RFC3339), mode, confirm, id)
	if err != nil {
		return message.Draft{}, err
	}
	return s.GetMessageDraft(id)
}

func timeString(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
