package sequence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAuthorizationIsTargetBoundSingleUseAndExpiryChecked(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store := NewStore(filepath.Join(dir, "auth.json"))
	now := time.Unix(100, 0)
	clock := func() time.Time { return now }
	a := NewAuthorizer(store, clock)
	plan := Plan{Target: "pve1", Actions: []Action{{Type: "key", Value: "Enter"}}, MaxDuration: time.Second}
	token, err := a.Authorize(plan, "pve1", true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Take(context.Background(), token, "pve2", plan); err == nil {
		t.Fatal("expected target mismatch")
	}
	if _, err := a.Take(context.Background(), token, "pve1", plan); err != nil {
		t.Fatal(err)
	}
	if _, err := a.Take(context.Background(), token, "pve1", plan); err == nil {
		t.Fatal("expected single-use rejection")
	}

	token, err = a.Authorize(plan, "pve1", true, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	now = time.Unix(102, 0)
	if _, err := a.Take(context.Background(), token, "pve1", plan); err == nil {
		t.Fatal("expected expiry rejection")
	}
}

func TestJournalRedactsSecrets(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	j := NewJournal(filepath.Join(dir, "journal.jsonl"))
	if err := j.Append(map[string]any{"target": "pve1", "token": "secret", "nested": map[string]any{"password": "pw"}}); err != nil {
		t.Fatal(err)
	}
	data, err := readJournal(j.path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) == "" || contains(string(data), "secret") || contains(string(data), "pw") {
		t.Fatalf("journal leaked secret: %s", data)
	}
}

func TestValidateSupportsCanonicalNonHardwareActionsAndLimits(t *testing.T) {
	valid := Plan{Target: " host-a ", Actions: []Action{
		{Type: "key", Value: "ControlLeft+AltLeft+Delete"},
		{Type: "hold_key", Value: "ShiftLeft", DurationMS: 5000},
		{Type: "release_all"},
		{Type: "mouse_move", X: 10, Y: -20},
		{Type: "wait", DurationMS: 1},
	}, MaxDuration: 30 * time.Second}
	if err := Validate(valid); err != nil {
		t.Fatalf("valid plan rejected: %v", err)
	}
	if err := Validate(Plan{Target: "x", Actions: []Action{{Type: "hold_key", Value: "A", DurationMS: 5001}}, MaxDuration: time.Second}); err == nil {
		t.Fatal("expected hold limit")
	}
	if err := Validate(Plan{Target: "x", Actions: []Action{{Type: "wait", DurationMS: 1}}, MaxDuration: time.Second, UnexpectedScreenPolicy: "retry"}); err == nil {
		t.Fatal("expected policy rejection")
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && index(s, sub) >= 0 }
func index(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
