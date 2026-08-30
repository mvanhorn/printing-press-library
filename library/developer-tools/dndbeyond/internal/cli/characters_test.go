// Copyright 2026 Matthew Martin and contributors. Licensed under Apache-2.0.

package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNormalizeCharacterSnapshotOmitsPrivateFields(t *testing.T) {
	input := []byte(`{
		"action":"get-character",
		"character": {
			"name":"Moss Lantern",
			"id":"123456",
			"type":"Character",
			"description":"private narrative",
			"race":"Elf",
			"level":"5",
			"classes":{"Wizard":"5"},
			"abilities":{"str":{"score":8,"modifier":-1},"dex":{"score":14,"modifier":2}},
			"hp":27,
			"max-hp":32,
			"email":"redacted-value",
			"backstory":"private text",
		"actions":[{"name":"Ray of Frost","dice":"1d8","description":"A cold ray","playerEmail":"redacted-value","unexpected":"drop me"}]
		}
	}`)
	out, err := normalizeCharacterSnapshot(input)
	if err != nil {
		t.Fatalf("normalizeCharacterSnapshot() error = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode normalized output: %v", err)
	}
	if got["read_only"] != true {
		t.Fatalf("read_only = %#v, want true", got["read_only"])
	}
	text := string(out)
	for _, forbidden := range []string{"redacted-value", "private text", "private narrative", "email", "backstory", "unexpected"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("normalized output contains forbidden value/key %q: %s", forbidden, text)
		}
	}
	character, ok := got["character"].(map[string]any)
	if !ok || character["name"] != "Moss Lantern" || character["race"] != "Elf" {
		t.Fatalf("normalized character = %#v, missing core fields", got["character"])
	}
}

func TestReadCharacterSnapshotRequiresOneInput(t *testing.T) {
	if _, err := readCharacterSnapshot("", false, strings.NewReader("{}")); err == nil {
		t.Fatal("readCharacterSnapshot() with missing file should fail")
	}
}

func TestCharacterSnapshotFromPDFFormJSON(t *testing.T) {
	input := []byte(`{
		"forms": [{
			"textfield": [
				{"id":"character-name","value":"Moss Lantern"},
				{"id":"race","value":"Elf"},
				{"id":"class","value":"Wizard"},
				{"id":"level","value":"5"},
				{"id":"hp","value":"27"},
				{"id":"email","value":"redacted-value"},
				{"id":"Description","value":"private description"},
				{"id":"Phone","value":"555-0100"},
				{"id":"Address","value":"123 Main Street"},
				{"id":"SSN","value":"123-45-6789"},
				{"id":"Date of Birth","value":"1990-01-01"},
				{"id":"personality-traits","value":"private narrative"},
				{"id":"ray-of-frost","value":"1d8 cold"}
			]
		}]
	}`)
	out, err := characterSnapshotFromPDFFormJSON(input)
	if err != nil {
		t.Fatalf("characterSnapshotFromPDFFormJSON() error = %v", err)
	}
	normalized, err := normalizeCharacterSnapshotWithFormat(out, "D&D Beyond exported PDF form")
	if err != nil {
		t.Fatalf("normalizeCharacterSnapshotWithFormat() error = %v", err)
	}
	text := string(normalized)
	for _, forbidden := range []string{"redacted-value", "email", "private description", "Description", "555-0100", "123 Main Street", "123-45-6789", "1990-01-01", "Phone", "Address", "SSN", "Date of Birth", "private narrative", "personality-traits"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("normalized PDF output contains forbidden value/key %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"Moss Lantern", "Elf", "Wizard", "1d8 cold", "D\\u0026D Beyond exported PDF form"} {
		if !strings.Contains(text, required) {
			t.Errorf("normalized PDF output missing %q: %s", required, text)
		}
	}
}

func TestCharacterSnapshotFromPDFWidgetFields(t *testing.T) {
	out, err := characterSnapshotFromPDFWidgetFields([]pdfWidgetField{
		{Label: "CharacterName", Value: "Moss Lantern"},
		{Label: "CLASS  LEVEL", Value: "Wizard 5"},
		{Label: "STR", Value: "8"},
		{Label: "PLAYER NAME", Value: "redacted-value"},
		{Label: "Description", Value: "private description"},
		{Label: "Phone", Value: "555-0100"},
		{Label: "Address", Value: "123 Main Street"},
		{Label: "SSN", Value: "123-45-6789"},
		{Label: "Date of Birth", Value: "1990-01-01"},
		{Label: "Backstory", Value: "private narrative"},
		{Label: "Wpn Name", Value: "Ray of Frost"},
	})
	if err != nil {
		t.Fatalf("characterSnapshotFromPDFWidgetFields() error = %v", err)
	}
	normalized, err := normalizeCharacterSnapshotWithFormat(out, "D&D Beyond exported PDF form")
	if err != nil {
		t.Fatalf("normalizeCharacterSnapshotWithFormat() error = %v", err)
	}
	text := string(normalized)
	for _, forbidden := range []string{"redacted-value", "PLAYER NAME", "private description", "Description", "555-0100", "123 Main Street", "123-45-6789", "1990-01-01", "Phone", "Address", "SSN", "Date of Birth", "private narrative", "Backstory"} {
		if strings.Contains(text, forbidden) {
			t.Errorf("normalized widget output contains forbidden value/key %q: %s", forbidden, text)
		}
	}
	for _, required := range []string{"Moss Lantern", "Wizard 5", "Ray of Frost"} {
		if !strings.Contains(text, required) {
			t.Errorf("normalized widget output missing %q: %s", required, text)
		}
	}
}
