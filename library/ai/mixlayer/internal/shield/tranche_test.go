// Copyright 2026 Cathryn Lavery and contributors. Licensed under Apache-2.0. See LICENSE.

package shield

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/mvanhorn/printing-press-library/library/ai/mixlayer/internal/store"
)

func TestSplitRecordsCarriesCSVHeader(t *testing.T) {
	input := "name,email\nCathryn Lavery,cathryn@example.com\nJane Smith,jane@example.com\n"
	tranches, err := SplitRecords(input, 45)
	if err != nil {
		t.Fatal(err)
	}
	if len(tranches) < 2 {
		t.Fatalf("len(tranches) = %d, want at least 2", len(tranches))
	}
	for _, tr := range tranches {
		if !strings.HasPrefix(tr.Text, "name,email\n") {
			t.Fatalf("tranche missing CSV header: %q", tr.Text)
		}
	}
}

func TestSplitRecordsPreservesLongNonCSVLines(t *testing.T) {
	input := strings.Repeat("a", 300*1024) + "\ntrailer\n"
	tranches, err := SplitRecords(input, 64*1024)
	if err != nil {
		t.Fatal(err)
	}
	var rebuilt strings.Builder
	for _, tr := range tranches {
		rebuilt.WriteString(tr.Text)
	}
	if rebuilt.String() != input {
		t.Fatalf("rebuilt corpus length = %d, want %d", rebuilt.Len(), len(input))
	}
}

func TestSplitOversizedPreservesUTF8(t *testing.T) {
	input := strings.Repeat("é", 20) + "\n"
	parts := splitOversized(input, 7)
	if len(parts) < 2 {
		t.Fatalf("len(parts) = %d, want multiple parts", len(parts))
	}
	for _, part := range parts {
		if !utf8.ValidString(part) {
			t.Fatalf("invalid utf8 part: %q", part)
		}
	}
}

func TestRedactUsesSharedVaultAcrossTranches(t *testing.T) {
	s, err := store.Open(t.TempDir() + "/data.db")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	first, err := Redact(ctx, s, "Cathryn Lavery <cathryn@example.com>", false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Redact(ctx, s, "Reminder for Cathryn Lavery", false)
	if err != nil {
		t.Fatal(err)
	}
	personToken := ""
	for _, entity := range first.Entities {
		if entity.Kind == "PERSON" {
			personToken = entity.Token
			break
		}
	}
	if personToken == "" || !strings.Contains(first.Text, personToken) || !strings.Contains(second.Text, personToken) {
		t.Fatalf("shared vault did not keep PERSON token consistent: token=%q first=%q second=%q", personToken, first.Text, second.Text)
	}
}
