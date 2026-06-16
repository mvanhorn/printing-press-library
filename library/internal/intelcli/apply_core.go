package intelcli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ApplyModeDryRun = "dry-run"
const ApplyModeLiveApproved = "live-approved"

type ApplyPolicyBase struct {
	SchemaVersion    string `json:"schema_version,omitempty"`
	MaxChangesPerRun int    `json:"max_changes_per_run,omitempty"`
}

type ApplySnapshot struct {
	SchemaVersion string
	Home          string
	Profile       string
	AccountID     string
	Target        any
	StateKey      string
	State         any
}

type ApplyAuditEntry struct {
	SchemaVersion string `json:"schema_version"`
	At            time.Time
	Action        string
	AccountID     string
	Target        any
	Operation     any
	Inverse       any
	Status        string
	DraftPath     string
	Result        any
}

func LoadApplyPolicy[T any](path string, defaults T) (T, error) {
	p := defaults
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	if len(bytes.TrimSpace(b)) == 0 {
		return p, nil
	}
	return p, json.Unmarshal(b, &p)
}

func AllowlistSet(values ...[]string) map[string]bool {
	out := map[string]bool{}
	for _, list := range values {
		for _, value := range list {
			if strings.TrimSpace(value) != "" {
				out[strings.TrimSpace(value)] = true
			}
		}
	}
	return out
}

func ValidateMaxChanges(max int) error {
	if max <= 0 {
		return fmt.Errorf("refusing apply: --max-changes-per-run must be at least 1")
	}
	return nil
}

func EnforceChangeCap(planned, max int) error {
	if planned > max {
		return fmt.Errorf("refusing apply: %d planned changes exceeds max-changes-per-run cap %d", planned, max)
	}
	return nil
}

func ApplyMode(liveApproved bool) string {
	if liveApproved {
		return ApplyModeLiveApproved
	}
	return ApplyModeDryRun
}

func RequireTypedConfirm(action, got, want string) error {
	if strings.TrimSpace(got) != want {
		return fmt.Errorf("refusing live %s: typed confirmation must be exactly %q", action, want)
	}
	return nil
}

func AcquireApplyLock(home, namespace, accountID string) (func(), error) {
	dir := filepath.Join(home, "locks", namespace)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, SafeName(accountID)+".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("account %s is already locked for apply: %w", accountID, err)
	}
	_, _ = fmt.Fprintf(f, "pid=%d at=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	_ = f.Close()
	return func() { _ = os.Remove(path) }, nil
}

func WriteApplySnapshot(snapshot ApplySnapshot) (string, error) {
	dir := filepath.Join(snapshot.Home, "snapshots", "apply", SafeName(snapshot.Profile))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	now := time.Now().UTC()
	path := filepath.Join(dir, now.Format("20060102T150405.000000000")+"-"+SafeName(snapshot.AccountID)+".json")
	body := map[string]any{
		"schema_version": snapshot.SchemaVersion,
		"created_at":     now,
		"account_id":     snapshot.AccountID,
		"target":         snapshot.Target,
	}
	if snapshot.StateKey != "" {
		body[snapshot.StateKey] = snapshot.State
	}
	b, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
}

func AppendReversal(home, profile string, entry any) error {
	return AppendJSONL(filepath.Join(home, "reversals", SafeName(profile)+".jsonl"), entry)
}

func FindReversal[T any](home, profile, id string) (T, error) {
	var zero T
	path := filepath.Join(home, "reversals", SafeName(profile)+".jsonl")
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		var header struct {
			ID string `json:"id"`
		}
		if json.Unmarshal(sc.Bytes(), &header) != nil || header.ID != id {
			continue
		}
		var entry T
		if err := json.Unmarshal(sc.Bytes(), &entry); err != nil {
			return zero, err
		}
		return entry, nil
	}
	if err := sc.Err(); err != nil {
		return zero, err
	}
	return zero, fmt.Errorf("reversal %q not found", id)
}

func AppendApplyAudit(home string, entry ApplyAuditEntry) error {
	if entry.At.IsZero() {
		entry.At = time.Now().UTC()
	}
	body := map[string]any{
		"schema_version": entry.SchemaVersion,
		"at":             entry.At,
		"action":         entry.Action,
		"account_id":     entry.AccountID,
		"target":         entry.Target,
		"operation":      entry.Operation,
		"inverse":        entry.Inverse,
		"status":         entry.Status,
		"draft_path":     entry.DraftPath,
		"result":         entry.Result,
	}
	return AppendJSONL(filepath.Join(home, "audit", "apply.log"), body)
}

func AppendJSONL(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.Write(append(b, '\n')); err != nil {
		return err
	}
	return nil
}
