// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import "testing"

func TestDecodeAddInfo(t *testing.T) {
	idName := func(num int, id uint64, name string) []byte {
		var m []byte
		m = appendVarintField(m, 1, id)
		m = appendStringField(m, 2, name)
		var out []byte
		return appendMessageField(out, num, m)
	}
	fieldDef := func(name string) []byte {
		var fd []byte
		fd = appendStringField(fd, 2, name)
		var out []byte
		return appendMessageField(out, 5, fd)
	}

	var buf []byte
	buf = append(buf, idName(1, 1734789153562, "Basic")...)
	buf = append(buf, idName(1, 1734789153563, "Basic (and reversed card)")...)
	buf = append(buf, idName(2, 1, "Default")...)
	buf = append(buf, idName(2, 1740682870334, "Words & phrases")...)
	buf = appendVarintField(buf, 3, 1740682870334) // default deck
	buf = appendVarintField(buf, 4, 1734789153563) // default note type
	buf = append(buf, fieldDef("Front")...)
	buf = append(buf, fieldDef("Back")...)

	info, err := DecodeAddInfo(buf)
	if err != nil {
		t.Fatalf("DecodeAddInfo: %v", err)
	}
	if len(info.NoteTypes) != 2 || info.NoteTypes[1].Name != "Basic (and reversed card)" {
		t.Errorf("note types = %+v", info.NoteTypes)
	}
	if len(info.Decks) != 2 || info.Decks[1].Name != "Words & phrases" || info.Decks[1].ID != "1740682870334" {
		t.Errorf("decks = %+v", info.Decks)
	}
	if info.DefaultDeckID != "1740682870334" {
		t.Errorf("default deck = %q", info.DefaultDeckID)
	}
	if info.DefaultNoteTypeID != "1734789153563" {
		t.Errorf("default note type = %q", info.DefaultNoteTypeID)
	}
	if len(info.DefaultFields) != 2 || info.DefaultFields[0] != "Front" || info.DefaultFields[1] != "Back" {
		t.Errorf("fields = %v", info.DefaultFields)
	}
}

func TestBuildAddNoteRequest(t *testing.T) {
	req := BuildAddNoteRequest(1734789153563, 1740682870334, []string{"Front text", "Back text"}, "tag1 tag2")
	fields, err := Fields(req)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	vals := CollectBytes(fields, 1)
	if len(vals) != 2 || string(vals[0]) != "Front text" || string(vals[1]) != "Back text" {
		t.Errorf("field values = %v", vals)
	}
	if FirstString(fields, 2) != "tag1 tag2" {
		t.Errorf("tags = %q", FirstString(fields, 2))
	}
	target := CollectBytes(fields, 3)
	if len(target) != 1 {
		t.Fatalf("expected target message, got %d", len(target))
	}
	tf, _ := Fields(target[0])
	if FirstVarint(tf, 1) != 1734789153563 {
		t.Errorf("notetype_id = %d", FirstVarint(tf, 1))
	}
	if FirstVarint(tf, 2) != 1740682870334 {
		t.Errorf("deck_id = %d", FirstVarint(tf, 2))
	}
}
