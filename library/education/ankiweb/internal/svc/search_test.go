// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import "testing"

func TestBuildSearchRequest(t *testing.T) {
	req := BuildSearchRequest("casa")
	fields, err := Fields(req)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	if got := FirstString(fields, 1); got != "casa" {
		t.Errorf("query = %q", got)
	}
}

func TestDecodeSearchResults(t *testing.T) {
	// Two card results: {1:id, 2:text} and {1:id, nested {3:text}}.
	card := func(id uint64, build func([]byte) []byte) []byte {
		var m []byte
		m = appendVarintField(m, 1, id)
		m = build(m)
		var out []byte
		return appendMessageField(out, 1, m)
	}
	var buf []byte
	buf = append(buf, card(101, func(m []byte) []byte {
		return appendStringField(m, 2, "el gato negro")
	})...)
	buf = append(buf, card(202, func(m []byte) []byte {
		var inner []byte
		inner = appendStringField(inner, 3, "la casa grande")
		return appendMessageField(m, 7, inner)
	})...)

	cards, err := DecodeSearchResults(buf)
	if err != nil {
		t.Fatalf("DecodeSearchResults: %v", err)
	}
	if len(cards) != 2 {
		t.Fatalf("got %d cards, want 2", len(cards))
	}
	if cards[0].ID != "101" || cards[0].Snippet != "el gato negro" {
		t.Errorf("card0 = %+v", cards[0])
	}
	if cards[1].ID != "202" || cards[1].Snippet != "la casa grande" {
		t.Errorf("card1 = %+v (nested text not extracted)", cards[1])
	}
}
