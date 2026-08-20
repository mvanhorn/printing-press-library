// Copyright 2026 Derik Parkinson and contributors. Licensed under Apache-2.0. See LICENSE.

package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte(content), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return dir
}

func TestLoadValidManifest(t *testing.T) {
	t.Parallel()
	dir := writeManifest(t, `
calendars:
  - account: personal
    id: derik@example.com
    role: write
    default_write: true
  - account: personal
    id: family@group.calendar.google.com
    role: read
  - account: work
    id: derik@work.example.com
    role: Read
    note: scope-limited
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(m.Calendars) != 3 {
		t.Fatalf("want 3 entries, got %d", len(m.Calendars))
	}
	if m.Calendars[2].Role != RoleRead {
		t.Errorf("role must normalize to lowercase, got %q", m.Calendars[2].Role)
	}
	if err := m.Validate([]string{"personal", "work"}); err != nil {
		t.Errorf("Validate with known accounts: %v", err)
	}
	order, grouped := m.ByAccount()
	if len(order) != 2 || order[0] != "personal" || order[1] != "work" {
		t.Errorf("ByAccount order = %v, want [personal work]", order)
	}
	if len(grouped["personal"]) != 2 {
		t.Errorf("personal group = %d entries, want 2", len(grouped["personal"]))
	}
}

func TestLoadMissingFileIsActionable(t *testing.T) {
	t.Parallel()
	_, err := Load(t.TempDir())
	if err == nil {
		t.Fatalf("missing calendars.yaml must error")
	}
	msg := err.Error()
	if !strings.Contains(msg, FileName) || !strings.Contains(msg, "emit-skeleton") {
		t.Errorf("missing-file error must name the file and the skeleton recovery path, got: %s", msg)
	}
}

func TestLoadDuplicateAccountIDPairErrors(t *testing.T) {
	t.Parallel()
	dir := writeManifest(t, `
calendars:
  - {account: personal, id: derik@example.com, role: write}
  - {account: personal, id: derik@example.com, role: read}
`)
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Errorf("duplicate (account,id) must error, got: %v", err)
	}
}

func TestLoadSameIDDifferentAccountsAllowed(t *testing.T) {
	t.Parallel()
	dir := writeManifest(t, `
calendars:
  - {account: personal, id: shared@group.calendar.google.com, role: read}
  - {account: work, id: shared@group.calendar.google.com, role: write}
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("same id under different accounts is legal: %v", err)
	}
	if got := len(m.Find("shared@group.calendar.google.com")); got != 2 {
		t.Errorf("Find = %d entries, want 2", got)
	}
}

func TestLoadBadRoleErrors(t *testing.T) {
	t.Parallel()
	dir := writeManifest(t, `
calendars:
  - {account: personal, id: derik@example.com, role: admin}
`)
	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "unknown role") {
		t.Errorf("bad role must error, got: %v", err)
	}
}

func TestLoadEmptyManifestErrors(t *testing.T) {
	t.Parallel()
	dir := writeManifest(t, "calendars: []\n")
	if _, err := Load(dir); err == nil || !strings.Contains(err.Error(), "no calendars") {
		t.Errorf("empty manifest must error, got: %v", err)
	}
}

func TestValidateUnknownAccountErrors(t *testing.T) {
	t.Parallel()
	dir := writeManifest(t, `
calendars:
  - {account: mystery, id: derik@example.com, role: read}
`)
	m, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	err = m.Validate([]string{"personal", "work"})
	if err == nil {
		t.Fatalf("unknown account must fail validation")
	}
	if !strings.Contains(err.Error(), "mystery") || !strings.Contains(err.Error(), "personal") {
		t.Errorf("error must name the bad account and the known ones, got: %v", err)
	}
	if _, err := LoadValidated(dir, []string{"personal"}); err == nil {
		t.Errorf("LoadValidated must propagate validation failure")
	}
}
