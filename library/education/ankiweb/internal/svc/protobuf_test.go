// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package svc

import "testing"

// putVarint appends a base-128 varint to b.
func putVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// tag builds a protobuf field tag byte/bytes for (fieldNum, wireType).
func tag(b []byte, num, wt int) []byte {
	return putVarint(b, uint64(num)<<3|uint64(wt))
}

// varintField appends a varint field.
func varintField(b []byte, num int, v uint64) []byte {
	b = tag(b, num, wireVarint)
	return putVarint(b, v)
}

// bytesField appends a length-delimited field.
func bytesField(b []byte, num int, payload []byte) []byte {
	b = tag(b, num, wireBytes)
	b = putVarint(b, uint64(len(payload)))
	return append(b, payload...)
}

func TestFields(t *testing.T) {
	var msg []byte
	msg = varintField(msg, 1, 150)
	msg = bytesField(msg, 2, []byte("hello"))
	msg = varintField(msg, 3, 1)

	fields, err := Fields(msg)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}

	cases := []struct {
		name string
		got  any
		want any
	}{
		{"field count", len(fields), 3},
		{"f1 num", fields[0].Num, 1},
		{"f1 wiretype", fields[0].WireType, wireVarint},
		{"f1 varint", fields[0].Varint, uint64(150)},
		{"f2 num", fields[1].Num, 2},
		{"f2 wiretype", fields[1].WireType, wireBytes},
		{"f2 bytes", string(fields[1].Bytes), "hello"},
		{"f3 varint", fields[2].Varint, uint64(1)},
		{"FirstString f2", FirstString(fields, 2), "hello"},
		{"FirstVarint f1", FirstVarint(fields, 1), uint64(150)},
		{"FirstVarint absent", FirstVarint(fields, 9), uint64(0)},
		{"FirstString absent", FirstString(fields, 9), ""},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestFieldsTruncated(t *testing.T) {
	// Tag for a length-delimited field claiming 5 bytes but only 2 follow.
	var msg []byte
	msg = tag(msg, 2, wireBytes)
	msg = putVarint(msg, 5)
	msg = append(msg, 'a', 'b')
	if _, err := Fields(msg); err == nil {
		t.Fatal("expected truncation error, got nil")
	}
}

func TestLongestString(t *testing.T) {
	var msg []byte
	msg = bytesField(msg, 1, []byte("short"))
	msg = bytesField(msg, 2, []byte("a much longer human readable description string"))
	msg = bytesField(msg, 3, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}) // binary, rejected
	fields, err := Fields(msg)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	if got := LongestString(fields); got != "a much longer human readable description string" {
		t.Errorf("LongestString = %q", got)
	}
}

func TestEncodersRoundTrip(t *testing.T) {
	// Build a message with varint, string, and nested-message fields, then
	// decode it back and assert every value survives.
	var inner []byte
	inner = appendVarintField(inner, 1, 1734789153563)
	inner = appendVarintField(inner, 2, 1740682870334)

	var msg []byte
	msg = appendStringField(msg, 1, "Front text")
	msg = appendStringField(msg, 1, "Back text")
	msg = appendStringField(msg, 2, "tag1 tag2")
	msg = appendMessageField(msg, 3, inner)

	fields, err := Fields(msg)
	if err != nil {
		t.Fatalf("Fields: %v", err)
	}
	strs := CollectBytes(fields, 1)
	if len(strs) != 2 || string(strs[0]) != "Front text" || string(strs[1]) != "Back text" {
		t.Errorf("repeated field 1 = %v", strs)
	}
	if got := FirstString(fields, 2); got != "tag1 tag2" {
		t.Errorf("tags = %q", got)
	}
	target := CollectBytes(fields, 3)
	if len(target) != 1 {
		t.Fatalf("expected one field-3 message, got %d", len(target))
	}
	tf, err := Fields(target[0])
	if err != nil {
		t.Fatalf("Fields(inner): %v", err)
	}
	if got := FirstVarint(tf, 1); got != 1734789153563 {
		t.Errorf("notetype_id = %d", got)
	}
	if got := FirstVarint(tf, 2); got != 1740682870334 {
		t.Errorf("deck_id = %d", got)
	}
}

func TestAppendRawVarint(t *testing.T) {
	for _, v := range []uint64{0, 1, 127, 128, 300, 16384, 1740682870334, 1<<63 + 1} {
		b := appendRawVarint(nil, v)
		got, n, err := readVarint(b, 0)
		if err != nil || n != len(b) || got != v {
			t.Errorf("roundtrip %d: got=%d n=%d err=%v", v, got, n, err)
		}
	}
}
