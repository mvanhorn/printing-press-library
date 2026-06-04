// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

// Package svc speaks AnkiWeb's /svc/ protobuf API directly. AnkiWeb serves
// these endpoints as raw protobuf (application/octet-stream) with no public
// .proto files, so this package hand-decodes the wire format: varints,
// length-delimited bytes, and fixed-width fields. No protobuf runtime
// dependency is used.
package svc

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// Wire types per the protobuf encoding spec.
const (
	wireVarint  = 0
	wireFixed64 = 1
	wireBytes   = 2
	wireFixed32 = 5
)

// errTruncated signals the buffer ended mid-field.
var errTruncated = errors.New("svc: truncated protobuf message")

// Field is one decoded top-level (or nested) protobuf field. Exactly one of
// the value accessors is meaningful, depending on WireType.
type Field struct {
	Num      int    // field number (tag >> 3)
	WireType int    // 0=varint, 1=fixed64, 2=bytes, 5=fixed32
	Varint   uint64 // populated when WireType==wireVarint
	Bytes    []byte // populated when WireType==wireBytes
	Fixed64  uint64 // populated when WireType==wireFixed64
	Fixed32  uint32 // populated when WireType==wireFixed32
}

// readVarint decodes a base-128 varint starting at buf[pos], returning the
// value and the number of bytes consumed.
func readVarint(buf []byte, pos int) (uint64, int, error) {
	var x uint64
	var shift uint
	start := pos
	for {
		if pos >= len(buf) {
			return 0, 0, errTruncated
		}
		b := buf[pos]
		pos++
		if shift >= 64 {
			return 0, 0, fmt.Errorf("svc: varint overflow at offset %d", start)
		}
		x |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			break
		}
		shift += 7
	}
	return x, pos - start, nil
}

// Fields decodes every top-level field of a protobuf message. Decoding stops
// at the first malformed field and returns what was parsed so far plus the
// error; callers that want best-effort behavior can ignore the error and use
// the returned slice.
func Fields(buf []byte) ([]Field, error) {
	var out []Field
	pos := 0
	for pos < len(buf) {
		tag, n, err := readVarint(buf, pos)
		if err != nil {
			return out, err
		}
		pos += n
		fieldNum := int(tag >> 3)
		wt := int(tag & 0x7)
		f := Field{Num: fieldNum, WireType: wt}
		switch wt {
		case wireVarint:
			v, n, err := readVarint(buf, pos)
			if err != nil {
				return out, err
			}
			pos += n
			f.Varint = v
		case wireBytes:
			length, n, err := readVarint(buf, pos)
			if err != nil {
				return out, err
			}
			pos += n
			end := pos + int(length)
			if end < pos || end > len(buf) {
				return out, errTruncated
			}
			f.Bytes = buf[pos:end]
			pos = end
		case wireFixed64:
			if pos+8 > len(buf) {
				return out, errTruncated
			}
			f.Fixed64 = binary.LittleEndian.Uint64(buf[pos : pos+8])
			pos += 8
		case wireFixed32:
			if pos+4 > len(buf) {
				return out, errTruncated
			}
			f.Fixed32 = binary.LittleEndian.Uint32(buf[pos : pos+4])
			pos += 4
		default:
			return out, fmt.Errorf("svc: unsupported wire type %d for field %d", wt, fieldNum)
		}
		out = append(out, f)
	}
	return out, nil
}

// FirstString returns the bytes of the first field with the given number as a
// string, or "" when absent.
func FirstString(fields []Field, num int) string {
	for _, f := range fields {
		if f.Num == num && f.WireType == wireBytes {
			return string(f.Bytes)
		}
	}
	return ""
}

// FirstVarint returns the value of the first varint field with the given
// number, or 0 when absent.
func FirstVarint(fields []Field, num int) uint64 {
	for _, f := range fields {
		if f.Num == num && f.WireType == wireVarint {
			return f.Varint
		}
	}
	return 0
}

// CollectBytes returns every length-delimited field with the given number.
func CollectBytes(fields []Field, num int) [][]byte {
	var out [][]byte
	for _, f := range fields {
		if f.Num == num && f.WireType == wireBytes {
			out = append(out, f.Bytes)
		}
	}
	return out
}

// LongestString returns the longest decodable string among the top-level
// length-delimited fields. Used to heuristically pull a description out of a
// message whose exact field layout we cannot fully map.
func LongestString(fields []Field) string {
	best := ""
	for _, f := range fields {
		if f.WireType != wireBytes {
			continue
		}
		s := string(f.Bytes)
		if isMostlyText(s) && len(s) > len(best) {
			best = s
		}
	}
	return best
}

// --- Encoding helpers ---
//
// AnkiWeb's /svc/ write endpoints (e.g. /svc/editor/add-or-update) take a
// protobuf request body. These helpers build that wire format by hand, mirroring
// the decoder above. They append to a caller-owned slice so a message can be
// assembled field by field.

// appendRawVarint appends v as a base-128 varint.
func appendRawVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// appendTag appends a field tag (field number + wire type).
func appendTag(b []byte, num, wireType int) []byte {
	return appendRawVarint(b, uint64(num)<<3|uint64(wireType))
}

// appendVarintField appends a varint field (wire type 0).
func appendVarintField(b []byte, num int, v uint64) []byte {
	b = appendTag(b, num, wireVarint)
	return appendRawVarint(b, v)
}

// appendBytesField appends a length-delimited field (wire type 2).
func appendBytesField(b []byte, num int, data []byte) []byte {
	b = appendTag(b, num, wireBytes)
	b = appendRawVarint(b, uint64(len(data)))
	return append(b, data...)
}

// appendStringField appends a string as a length-delimited field.
func appendStringField(b []byte, num int, s string) []byte {
	return appendBytesField(b, num, []byte(s))
}

// appendMessageField appends an already-encoded sub-message as a
// length-delimited field.
func appendMessageField(b []byte, num int, msg []byte) []byte {
	return appendBytesField(b, num, msg)
}

// isMostlyText reports whether s looks like human-readable UTF-8 text rather
// than a packed sub-message. Sub-messages decode to strings full of control
// bytes; we reject anything with a high ratio of non-printable runes.
func isMostlyText(s string) bool {
	if s == "" {
		return false
	}
	bad := 0
	for _, r := range s {
		if r == '\n' || r == '\t' || r == '\r' {
			continue
		}
		if r < 0x20 || r == 0xFFFD {
			bad++
		}
	}
	return bad*5 < len(s)
}
