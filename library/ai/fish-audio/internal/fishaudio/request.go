// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package fishaudio

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
)

// RenderRequest is the resolved, validated shape of one TTS render. The CLI
// layer fills it from flags; this package turns it into a request body and a
// stable identity hash.
type RenderRequest struct {
	Text          string
	VoiceID       string
	Model         string
	Format        string
	MP3Bitrate    int
	OpusBitrate   int
	SampleRate    int
	Latency       string
	Speed         float64
	Volume        float64
	Normalize     bool
	Temperature   float64
	TopP          float64
	ChunkLength   int
	ReferenceText string
	// ReferenceAudio is inline zero-shot reference audio. When set the
	// request must be encoded as MessagePack, because JSON cannot carry raw
	// bytes.
	ReferenceAudio []byte
	// SpeakerVoiceIDs carries the ordered reference_id array for a
	// multi-speaker dialogue request. When it has entries it replaces
	// VoiceID on the wire.
	SpeakerVoiceIDs []string
}

// BytesIn is the billable input size: the UTF-8 byte length of the text.
func (r RenderRequest) BytesIn() int { return len([]byte(r.Text)) }

// BytesIn64 is BytesIn widened for the store, whose columns are INTEGER.
func (r RenderRequest) BytesIn64() int64 { return int64(r.BytesIn()) }

// Hash returns the stable identity of this render. Two invocations that name
// the same fields in a different flag order produce the same hash, so
// `--skip-if-rendered` can recognize a repeat. Reference audio contributes its
// own digest rather than its bytes so the hash stays cheap.
func (r RenderRequest) Hash() string {
	fields := map[string]string{
		"text":         r.Text,
		"voice":        r.VoiceID,
		"model":        r.Model,
		"format":       r.Format,
		"mp3_bitrate":  strconv.Itoa(r.MP3Bitrate),
		"opus_bitrate": strconv.Itoa(r.OpusBitrate),
		"sample_rate":  strconv.Itoa(r.SampleRate),
		"latency":      r.Latency,
		"speed":        strconv.FormatFloat(r.Speed, 'f', 4, 64),
		"volume":       strconv.FormatFloat(r.Volume, 'f', 4, 64),
		"normalize":    strconv.FormatBool(r.Normalize),
		"temperature":  strconv.FormatFloat(r.Temperature, 'f', 4, 64),
		"top_p":        strconv.FormatFloat(r.TopP, 'f', 4, 64),
		"chunk_length": strconv.Itoa(r.ChunkLength),
		"ref_text":     r.ReferenceText,
	}
	if len(r.ReferenceAudio) > 0 {
		sum := sha256.Sum256(r.ReferenceAudio)
		fields["ref_audio"] = hex.EncodeToString(sum[:])
	}
	for i, id := range r.SpeakerVoiceIDs {
		fields["speaker_"+strconv.Itoa(i)] = id
	}
	sum := sha256.Sum256([]byte(canonicalKV(fields)))
	return hex.EncodeToString(sum[:])
}

// NeedsMsgpack reports whether the request must go out as
// application/msgpack. Inline reference audio is raw bytes, which JSON cannot
// carry; every other render encodes as JSON.
func (r RenderRequest) NeedsMsgpack() bool { return len(r.ReferenceAudio) > 0 }

// Body builds the wire body for POST /v1/tts. Only fields the caller set are
// included, so the server's own defaults still apply to everything else.
func (r RenderRequest) Body() map[string]any {
	body := map[string]any{
		"text":   r.Text,
		"format": r.Format,
	}
	if len(r.SpeakerVoiceIDs) > 0 {
		body["reference_id"] = r.SpeakerVoiceIDs
	} else if r.VoiceID != "" {
		body["reference_id"] = r.VoiceID
	}
	if r.MP3Bitrate > 0 {
		body["mp3_bitrate"] = r.MP3Bitrate
	}
	if r.OpusBitrate != 0 {
		body["opus_bitrate"] = r.OpusBitrate
	}
	if r.SampleRate > 0 {
		body["sample_rate"] = r.SampleRate
	}
	if r.Latency != "" {
		body["latency"] = r.Latency
	}
	if r.Speed != 0 || r.Volume != 0 {
		prosody := map[string]any{}
		if r.Speed != 0 {
			prosody["speed"] = r.Speed
		}
		if r.Volume != 0 {
			prosody["volume"] = r.Volume
		}
		body["prosody"] = prosody
	}
	body["normalize"] = r.Normalize
	if r.Temperature != 0 {
		body["temperature"] = r.Temperature
	}
	if r.TopP != 0 {
		body["top_p"] = r.TopP
	}
	if r.ChunkLength > 0 {
		body["chunk_length"] = r.ChunkLength
	}
	if len(r.ReferenceAudio) > 0 {
		body["references"] = []map[string]any{{
			"audio": r.ReferenceAudio,
			"text":  r.ReferenceText,
		}}
	}
	return body
}

// BatchRowHash derives the render-log identity of one batch output file.
//
// render_log.request_hash is UNIQUE, so a batch containing the same line twice
// would collapse into a single row and under-report spend. A batch row is
// therefore keyed per output file: the same audio written to two paths is two
// rows, both traceable back to the one request that produced them through the
// requestHash prefix.
func BatchRowHash(requestHash, outputPath string) string {
	sum := sha256.Sum256([]byte(requestHash + "\x00" + outputPath))
	return hex.EncodeToString(sum[:])
}
