// Copyright 2026 user. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/ai/cartesia/internal/store"
)

func TestSQL_RejectsNonSelect(t *testing.T) {
	rejected := []string{
		"DELETE FROM agents",
		"UPDATE agents SET name = 'x'",
		"INSERT INTO agents (id) VALUES ('x')",
		"DROP TABLE agents",
	}
	flags := &rootFlags{}
	cmd := newSQLCmd(flags)
	for _, stmt := range rejected {
		err := cmd.RunE(cmd, []string{stmt})
		if err == nil {
			t.Errorf("expected rejection for %q, got nil", stmt)
		} else if !strings.Contains(err.Error(), "only SELECT") {
			t.Errorf("expected 'only SELECT' in error, got %v", err)
		}
	}
}

func TestSQL_RunsSelect(t *testing.T) {
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "data.db")
	s, err := store.Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s.Close()

	flags := &rootFlags{asJSON: true}
	cmd := newSQLCmd(flags)
	cmd.Flags().Set("db", dbPath)
	cmd.SetContext(context.Background())
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	if err := cmd.RunE(cmd, []string{"SELECT 1 AS one"}); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"one"`) {
		t.Fatalf("expected 'one' column in output, got %s", out)
	}
}
