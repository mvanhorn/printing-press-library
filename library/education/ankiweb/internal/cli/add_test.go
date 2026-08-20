// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"testing"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
)

func testInfo() svc.AddInfo {
	return svc.AddInfo{
		NoteTypes: []svc.NoteType{
			{ID: "10", Name: "Basic"},
			{ID: "11", Name: "Basic (and reversed card)"},
			{ID: "12", Name: "Cloze"},
		},
		Decks: []svc.AddDeck{
			{ID: "1", Name: "Default"},
			{ID: "99", Name: "Words & phrases"},
		},
		DefaultNoteTypeID: "11",
		DefaultDeckID:     "99",
		DefaultFields:     []string{"Front", "Back"},
	}
}

func TestResolveNoteTypeDefaultAndByName(t *testing.T) {
	info := testInfo()
	if nt, err := resolveNoteType(info, ""); err != nil || nt.ID != "11" {
		t.Errorf("default note type = %+v err=%v", nt, err)
	}
	if nt, err := resolveNoteType(info, "cloze"); err != nil || nt.ID != "12" {
		t.Errorf("by-name note type = %+v err=%v", nt, err)
	}
	if _, err := resolveNoteType(info, "nope"); err == nil {
		t.Error("expected error for unknown note type")
	}
}

func TestResolveDeckDefaultNameID(t *testing.T) {
	info := testInfo()
	if d, err := resolveDeck(info, ""); err != nil || d.ID != "99" {
		t.Errorf("default deck = %+v err=%v", d, err)
	}
	if d, err := resolveDeck(info, "Default"); err != nil || d.ID != "1" {
		t.Errorf("by-name deck = %+v err=%v", d, err)
	}
	if d, err := resolveDeck(info, "99"); err != nil || d.Name != "Words & phrases" {
		t.Errorf("by-id deck = %+v err=%v", d, err)
	}
	if _, err := resolveDeck(info, "ghost"); err == nil {
		t.Error("expected error for unknown deck")
	}
}

func TestResolveFieldValuesPositional(t *testing.T) {
	info := testInfo()
	nt, _ := resolveNoteType(info, "")
	values, fm, err := resolveFieldValues(info, nt, []string{"Bonjour", "Hello"}, nil)
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(values) != 2 || values[0] != "Bonjour" || values[1] != "Hello" {
		t.Errorf("values=%v", values)
	}
	if fm["Front"] != "Bonjour" || fm["Back"] != "Hello" {
		t.Errorf("field map=%v", fm)
	}
}

func TestResolveFieldValuesNamedReordered(t *testing.T) {
	info := testInfo()
	nt, _ := resolveNoteType(info, "") // default => Front,Back order known
	// Provide in reverse order; expect reordered to Front,Back.
	values, _, err := resolveFieldValues(info, nt, nil, []string{"Back=Hello", "Front=Bonjour"})
	if err != nil {
		t.Fatalf("err=%v", err)
	}
	if len(values) != 2 || values[0] != "Bonjour" || values[1] != "Hello" {
		t.Errorf("expected reordered [Bonjour Hello], got %v", values)
	}
}

func TestResolveFieldValuesBadFlag(t *testing.T) {
	info := testInfo()
	nt, _ := resolveNoteType(info, "")
	if _, _, err := resolveFieldValues(info, nt, nil, []string{"noequals"}); err == nil {
		t.Error("expected error for --field without =")
	}
}
