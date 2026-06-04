package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type AutomationAudit struct {
	ID        string          `json:"id"`
	Command   string          `json:"command"`
	Mode      string          `json:"mode"`
	Report    string          `json:"report,omitempty"`
	PlanCount int             `json:"plan_count"`
	Payload   json.RawMessage `json:"payload,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}

func (s *Store) AppendAutomationAudit(ctx context.Context, command, mode, reportPath string, planCount int, payload json.RawMessage) (AutomationAudit, error) {
	if command == "" {
		return AutomationAudit{}, fmt.Errorf("automation audit command is required")
	}
	if mode == "" {
		mode = "dry_run"
	}
	if !json.Valid(payload) {
		return AutomationAudit{}, fmt.Errorf("automation audit payload must be valid JSON")
	}
	createdAt := time.Now().UTC()
	id := automationAuditID(command, mode, reportPath, createdAt, payload)
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if _, err := s.db.ExecContext(ctx,
		`INSERT INTO automation_audit (id, command, mode, report_path, plan_count, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, command, mode, reportPath, planCount, string(payload), createdAt.Format(time.RFC3339),
	); err != nil {
		return AutomationAudit{}, fmt.Errorf("inserting automation audit: %w", err)
	}
	return AutomationAudit{ID: id, Command: command, Mode: mode, Report: reportPath, PlanCount: planCount, Payload: payload, CreatedAt: createdAt}, nil
}

func (s *Store) ListAutomationAudits(ctx context.Context, limit int) ([]AutomationAudit, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, command, mode, COALESCE(report_path, ''), plan_count, payload, created_at
		 FROM automation_audit ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("listing automation audits: %w", err)
	}
	defer rows.Close()
	var out []AutomationAudit
	for rows.Next() {
		var item AutomationAudit
		var payload, createdAt string
		if err := rows.Scan(&item.ID, &item.Command, &item.Mode, &item.Report, &item.PlanCount, &payload, &createdAt); err != nil {
			return nil, fmt.Errorf("scanning automation audit: %w", err)
		}
		item.Payload = json.RawMessage(payload)
		item.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("reading automation audits: %w", err)
	}
	return out, nil
}

func automationAuditID(command, mode, reportPath string, createdAt time.Time, payload json.RawMessage) string {
	h := sha256.New()
	_, _ = h.Write([]byte(command))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(mode))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(reportPath))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(createdAt.Format(time.RFC3339Nano)))
	_, _ = h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))[:16]
}
