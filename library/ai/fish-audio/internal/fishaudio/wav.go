// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package fishaudio

import (
	"encoding/binary"
	"math"
)

// unknownSize is the streaming-writer placeholder some encoders leave in the
// RIFF and data chunk size fields. Zero is the other placeholder.
const unknownSize uint32 = 0xFFFFFFFF

// RepairWAVHeader rewrites the RIFF and `data` chunk sizes of a WAV payload
// when the server streamed the file and left them at 0 or 0xFFFFFFFF.
//
// Fish Audio returns WAV as a chunked stream, so the frame count is not known
// when the header is written. Players that seek by the declared frame count
// (pygame, several browser decoders) then treat the file as empty. The audio
// itself is fine, so the fix is arithmetic on two 32-bit fields, not a
// re-encode.
//
// It returns the payload and whether anything was changed. Any input that is
// not a RIFF/WAVE file is returned untouched.
func RepairWAVHeader(data []byte) ([]byte, bool) {
	if len(data) < 12 {
		return data, false
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return data, false
	}
	// The RIFF and `data` size fields are 32 bits. A payload larger than that
	// cannot be described by a WAV header, so repairing it would write a wrong
	// number. Leave such a payload untouched instead.
	if uint64(len(data)) > math.MaxUint32 {
		return data, false
	}
	out := make([]byte, len(data))
	copy(out, data)
	changed := false

	riffSize := binary.LittleEndian.Uint32(out[4:8])
	// #nosec G115 -- len(out) is bounded above by math.MaxUint32, so the
	// conversion cannot overflow.
	wantRiff := uint32(len(out) - 8)
	if riffSize == 0 || riffSize == unknownSize || riffSize != wantRiff {
		binary.LittleEndian.PutUint32(out[4:8], wantRiff)
		changed = true
	}

	// Walk the chunk list. A bogus size on a non-final chunk would derail the
	// walk, so the loop stops at the first `data` chunk, which is the one the
	// bug affects and is always last in a streamed file.
	pos := 12
	for pos+8 <= len(out) {
		id := string(out[pos : pos+4])
		size := binary.LittleEndian.Uint32(out[pos+4 : pos+8])
		if id == "data" {
			// #nosec G115 -- len(out) is bounded above by math.MaxUint32, so the
			// conversion cannot overflow.
			wantData := uint32(len(out) - (pos + 8))
			if size == 0 || size == unknownSize || size > wantData {
				binary.LittleEndian.PutUint32(out[pos+4:pos+8], wantData)
				changed = true
			}
			break
		}
		if size == 0 || size == unknownSize {
			// Cannot advance past a chunk of unknown length.
			break
		}
		advance := int(size)
		if advance%2 == 1 {
			advance++ // RIFF chunks are word-aligned.
		}
		pos += 8 + advance
	}
	if !changed {
		return data, false
	}
	return out, true
}

// WAVHeaderSizes reports the declared RIFF size and `data` chunk size. It is
// the read side of RepairWAVHeader and exists so tests and `--json` output can
// state what the header claims without decoding the audio.
func WAVHeaderSizes(data []byte) (riffSize uint32, dataSize uint32, ok bool) {
	if len(data) < 12 || string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return 0, 0, false
	}
	riffSize = binary.LittleEndian.Uint32(data[4:8])
	pos := 12
	for pos+8 <= len(data) {
		id := string(data[pos : pos+4])
		size := binary.LittleEndian.Uint32(data[pos+4 : pos+8])
		if id == "data" {
			return riffSize, size, true
		}
		if size == 0 || size == unknownSize {
			break
		}
		advance := int(size)
		if advance%2 == 1 {
			advance++
		}
		pos += 8 + advance
	}
	return riffSize, 0, false
}
