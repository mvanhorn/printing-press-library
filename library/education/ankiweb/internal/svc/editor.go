// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import "strconv"

// NoteType is one Anki note type (model) available for adding notes.
type NoteType struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AddDeck is a deck that notes can be added to. (Named AddDeck to avoid
// colliding with the shared-deck and MyDeck types elsewhere in the package.)
type AddDeck struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// AddInfo is the decoded /svc/editor/get-info-for-adding response: the note
// types and decks the user can add to, the currently-selected defaults, and the
// field names of the default note type.
type AddInfo struct {
	NoteTypes         []NoteType `json:"note_types"`
	Decks             []AddDeck  `json:"decks"`
	DefaultDeckID     string     `json:"default_deck_id"`
	DefaultNoteTypeID string     `json:"default_note_type_id"`
	DefaultFields     []string   `json:"default_fields"` // field names of the default note type (e.g. Front, Back)
}

// DecodeAddInfo parses the get-info-for-adding protobuf. Wire shape (reverse
// engineered, see manuscripts editor-add-report.md):
//
//	field 1 (repeated msg) note types {1: id, 2: name}
//	field 2 (repeated msg) decks      {1: id, 2: name}
//	field 3 (varint)       default deck id
//	field 4 (varint)       default note type id
//	field 5 (repeated msg) default note type field defs {2: field name}
func DecodeAddInfo(buf []byte) (AddInfo, error) {
	fields, err := Fields(buf)
	var info AddInfo
	for _, f := range fields {
		switch f.Num {
		case 1:
			if f.WireType == wireBytes {
				if nt, ok := decodeIDName(f.Bytes); ok {
					info.NoteTypes = append(info.NoteTypes, NoteType{ID: nt.id, Name: nt.name})
				}
			}
		case 2:
			if f.WireType == wireBytes {
				if dk, ok := decodeIDName(f.Bytes); ok {
					info.Decks = append(info.Decks, AddDeck{ID: dk.id, Name: dk.name})
				}
			}
		case 3:
			if f.WireType == wireVarint {
				info.DefaultDeckID = strconv.FormatUint(f.Varint, 10)
			}
		case 4:
			if f.WireType == wireVarint {
				info.DefaultNoteTypeID = strconv.FormatUint(f.Varint, 10)
			}
		case 5:
			if f.WireType == wireBytes {
				sub, _ := Fields(f.Bytes)
				if name := FirstString(sub, 2); name != "" {
					info.DefaultFields = append(info.DefaultFields, name)
				}
			}
		}
	}
	return info, err
}

type idName struct {
	id   string
	name string
}

// decodeIDName extracts {1: id varint, 2: name string} from a sub-message.
func decodeIDName(buf []byte) (idName, bool) {
	sub, _ := Fields(buf)
	var r idName
	if v := FirstVarint(sub, 1); v != 0 {
		r.id = strconv.FormatUint(v, 10)
	}
	r.name = FirstString(sub, 2)
	if r.id == "" && r.name == "" {
		return r, false
	}
	return r, true
}

// BuildAddNoteRequest encodes the /svc/editor/add-or-update request body:
//
//	field 1 (repeated string) ordered note field values
//	field 2 (string)          space-separated tags
//	field 3 (msg)             {1: notetype_id, 2: deck_id}
func BuildAddNoteRequest(notetypeID, deckID uint64, fieldValues []string, tags string) []byte {
	var b []byte
	for _, v := range fieldValues {
		b = appendStringField(b, 1, v)
	}
	b = appendStringField(b, 2, tags)
	var target []byte
	target = appendVarintField(target, 1, notetypeID)
	target = appendVarintField(target, 2, deckID)
	b = appendMessageField(b, 3, target)
	return b
}
