// Copyright 2026 Jon Gouveia and contributors. Licensed under Apache-2.0. See LICENSE.

package fishaudio

import (
	"encoding/binary"
	"math"
	"strings"
	"testing"
)

func TestValidateModel(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		want     string
		wantWarn bool
		wantErr  bool
	}{
		{name: "empty falls back to the API default", in: "", want: DefaultModel},
		{name: "s1", in: "s1", want: "s1"},
		{name: "s2-pro", in: "s2-pro", want: "s2-pro"},
		{name: "s2.1-pro", in: "s2.1-pro", want: "s2.1-pro"},
		{name: "free tier", in: "s2.1-pro-free", want: "s2.1-pro-free"},
		{name: "deprecated speech-1.6 warns but is accepted", in: "speech-1.6", want: "speech-1.6", wantWarn: true},
		{name: "deprecated speech-1.5 warns but is accepted", in: "speech-1.5", want: "speech-1.5", wantWarn: true},
		{name: "missing dot is rejected", in: "s2.1pro", wantErr: true},
		{name: "unknown string is rejected", in: "turbo-v3", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, warn, err := ValidateModel(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateModel(%q) = %q, want an error", tc.in, got)
				}
				if !strings.Contains(err.Error(), "must be one of") {
					t.Fatalf("ValidateModel(%q) error = %v, want it to name the accepted set", tc.in, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateModel(%q) unexpected error %v", tc.in, err)
			}
			if got != tc.want {
				t.Fatalf("ValidateModel(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if (warn != "") != tc.wantWarn {
				t.Fatalf("ValidateModel(%q) warning = %q, wantWarn %v", tc.in, warn, tc.wantWarn)
			}
		})
	}
}

func TestIsS2Family(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"s1", false},
		{"s2-pro", true},
		{"s2.1-pro", true},
		{"s2.1-pro-free", true},
		{"speech-1.6", false},
	}
	for _, tc := range cases {
		if got := IsS2Family(tc.in); got != tc.want {
			t.Fatalf("IsS2Family(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestValidateClosedSets(t *testing.T) {
	if _, err := ValidateFormat("flac"); err == nil {
		t.Fatal("ValidateFormat(\"flac\") = nil error, want a rejection")
	}
	if got, err := ValidateFormat(""); err != nil || got != "mp3" {
		t.Fatalf("ValidateFormat(\"\") = %q, %v; want mp3, nil", got, err)
	}
	if _, err := ValidateLatency("instant"); err == nil {
		t.Fatal("ValidateLatency(\"instant\") = nil error, want a rejection")
	}
	if _, err := ValidateVisibility("secret"); err == nil {
		t.Fatal("ValidateVisibility(\"secret\") = nil error, want a rejection")
	}
	if got, err := ValidateVisibility(""); err != nil || got != "private" {
		t.Fatalf("ValidateVisibility(\"\") = %q, %v; want private, nil", got, err)
	}
}

func TestTTSCost(t *testing.T) {
	cases := []struct {
		name      string
		bytes     int
		model     string
		want      float64
		wantEquiv float64
	}{
		{name: "one million bytes on a paid model", bytes: 1_000_000, model: "s2.1-pro", want: 15.00, wantEquiv: 15.00},
		{name: "one million bytes on the free model", bytes: 1_000_000, model: FreeModel, want: 0, wantEquiv: 15.00},
		{name: "s1 is billed at the paid rate", bytes: 500_000, model: "s1", want: 7.50, wantEquiv: 7.50},
		{name: "no text costs nothing", bytes: 0, model: "s2.1-pro", want: 0, wantEquiv: 0},
		{name: "a negative count is clamped", bytes: -10, model: "s2.1-pro", want: 0, wantEquiv: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cost, equiv := TTSCost(tc.bytes, tc.model)
			if math.Abs(cost-tc.want) > 1e-9 {
				t.Fatalf("TTSCost(%d, %q) cost = %v, want %v", tc.bytes, tc.model, cost, tc.want)
			}
			if math.Abs(equiv-tc.wantEquiv) > 1e-9 {
				t.Fatalf("TTSCost(%d, %q) paid equivalent = %v, want %v", tc.bytes, tc.model, equiv, tc.wantEquiv)
			}
		})
	}
}

func TestASRCost(t *testing.T) {
	cases := []struct {
		name    string
		seconds float64
		want    float64
	}{
		{name: "one hour", seconds: 3600, want: 0.36},
		{name: "ten seconds", seconds: 10, want: 0.001},
		{name: "rounds to the nearest second", seconds: 10.4, want: 0.001},
		{name: "no audio costs nothing", seconds: 0, want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ASRCost(tc.seconds); math.Abs(got-tc.want) > 1e-6 {
				t.Fatalf("ASRCost(%v) = %v, want %v", tc.seconds, got, tc.want)
			}
		})
	}
}

func TestVoiceDesignCost(t *testing.T) {
	if got := VoiceDesignCost(3); math.Abs(got-0.03) > 1e-9 {
		t.Fatalf("VoiceDesignCost(3) = %v, want 0.03", got)
	}
}

// TestRequestHashIsOrderIndependent is the guarantee --skip-if-rendered rests
// on: the same render named through differently-ordered flags is one identity.
func TestRequestHashIsOrderIndependent(t *testing.T) {
	base := RenderRequest{
		Text: "Your table is ready.", VoiceID: "abc", Model: "s2.1-pro", Format: "mp3",
		MP3Bitrate: 128, Latency: "normal", Speed: 1.1, Volume: 0, Normalize: true,
		Temperature: 0.7, TopP: 0.7,
	}
	// Same fields, assembled in a different literal order.
	reordered := RenderRequest{
		TopP: 0.7, Temperature: 0.7, Normalize: true, Volume: 0, Speed: 1.1,
		Latency: "normal", MP3Bitrate: 128, Format: "mp3", Model: "s2.1-pro",
		VoiceID: "abc", Text: "Your table is ready.",
	}
	if base.Hash() != reordered.Hash() {
		t.Fatalf("Hash() differs across field order: %s vs %s", base.Hash(), reordered.Hash())
	}

	changed := base
	changed.Speed = 1.2
	if changed.Hash() == base.Hash() {
		t.Fatal("Hash() ignored a prosody change; --skip-if-rendered would reuse the wrong file")
	}
	changed = base
	changed.Model = "s1"
	if changed.Hash() == base.Hash() {
		t.Fatal("Hash() ignored a model change")
	}
	changed = base
	changed.ReferenceAudio = []byte{1, 2, 3}
	if changed.Hash() == base.Hash() {
		t.Fatal("Hash() ignored inline reference audio")
	}
}

func TestRenderRequestBodyAndEncoding(t *testing.T) {
	req := RenderRequest{Text: "hi", VoiceID: "abc", Model: "s2.1-pro", Format: "mp3", Speed: 1.2}
	if req.NeedsMsgpack() {
		t.Fatal("NeedsMsgpack() = true without reference audio")
	}
	body := req.Body()
	if body["reference_id"] != "abc" {
		t.Fatalf("Body()[reference_id] = %v, want abc", body["reference_id"])
	}
	prosody, ok := body["prosody"].(map[string]any)
	if !ok || prosody["speed"] != 1.2 {
		t.Fatalf("Body()[prosody] = %v, want speed 1.2", body["prosody"])
	}
	if req.BytesIn() != 2 {
		t.Fatalf("BytesIn() = %d, want 2", req.BytesIn())
	}

	multi := RenderRequest{Text: "hi", SpeakerVoiceIDs: []string{"a", "b"}, Format: "mp3"}
	ids, ok := multi.Body()["reference_id"].([]string)
	if !ok || len(ids) != 2 {
		t.Fatalf("Body()[reference_id] = %v, want the two-speaker array", multi.Body()["reference_id"])
	}

	zeroShot := RenderRequest{Text: "hi", ReferenceAudio: []byte{0x01}, Format: "mp3"}
	if !zeroShot.NeedsMsgpack() {
		t.Fatal("NeedsMsgpack() = false with inline reference audio; JSON cannot carry raw bytes")
	}
}

// buildWAV assembles a minimal RIFF/WAVE file with caller-chosen size fields,
// so the repair can be tested against both placeholder shapes.
func buildWAV(riffSize, dataSize uint32, payload []byte) []byte {
	out := make([]byte, 0, 44+len(payload))
	out = append(out, []byte("RIFF")...)
	out = binary.LittleEndian.AppendUint32(out, riffSize)
	out = append(out, []byte("WAVE")...)
	out = append(out, []byte("fmt ")...)
	out = binary.LittleEndian.AppendUint32(out, 16)
	out = append(out, make([]byte, 16)...)
	out = append(out, []byte("data")...)
	out = binary.LittleEndian.AppendUint32(out, dataSize)
	out = append(out, payload...)
	return out
}

func TestRepairWAVHeader(t *testing.T) {
	payload := make([]byte, 100)
	cases := []struct {
		name        string
		riff        uint32
		data        uint32
		wantChanged bool
	}{
		{name: "zeroed sizes are repaired", riff: 0, data: 0, wantChanged: true},
		{name: "streaming placeholder sizes are repaired", riff: 0xFFFFFFFF, data: 0xFFFFFFFF, wantChanged: true},
		{name: "correct sizes are left alone", riff: uint32(36 + len(payload)), data: uint32(len(payload)), wantChanged: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			in := buildWAV(tc.riff, tc.data, payload)
			out, changed := RepairWAVHeader(in)
			if changed != tc.wantChanged {
				t.Fatalf("RepairWAVHeader changed = %v, want %v", changed, tc.wantChanged)
			}
			riff, data, ok := WAVHeaderSizes(out)
			if !ok {
				t.Fatal("WAVHeaderSizes could not find the data chunk after repair")
			}
			if wantRiff := uint32(len(out) - 8); riff != wantRiff {
				t.Fatalf("RIFF size = %d, want %d", riff, wantRiff)
			}
			if data != uint32(len(payload)) {
				t.Fatalf("data size = %d, want %d", data, len(payload))
			}
		})
	}

	t.Run("a non-WAV payload is returned untouched", func(t *testing.T) {
		mp3 := []byte("ID3\x03\x00\x00\x00\x00\x00\x00mp3 payload")
		out, changed := RepairWAVHeader(mp3)
		if changed {
			t.Fatal("RepairWAVHeader reported a change on a non-WAV payload")
		}
		if string(out) != string(mp3) {
			t.Fatal("RepairWAVHeader modified a non-WAV payload")
		}
	})
}

func TestParseBatchInput(t *testing.T) {
	t.Run("plain lines", func(t *testing.T) {
		lines, err := ParseBatchInput("first line\n\n# a comment\nsecond line\n", false)
		if err != nil {
			t.Fatalf("ParseBatchInput error = %v", err)
		}
		if len(lines) != 2 {
			t.Fatalf("got %d lines, want 2: %+v", len(lines), lines)
		}
		if lines[0].Text != "first line" || lines[0].LineNo != 1 {
			t.Fatalf("first line = %+v", lines[0])
		}
		if lines[1].Text != "second line" || lines[1].LineNo != 4 {
			t.Fatalf("second line = %+v", lines[1])
		}
	})

	t.Run("JSONL with a per-line voice", func(t *testing.T) {
		lines, err := ParseBatchInput(`{"text":"hello","voice":"abc"}`+"\n"+`{"text":"there"}`, false)
		if err != nil {
			t.Fatalf("ParseBatchInput error = %v", err)
		}
		if len(lines) != 2 || lines[0].Voice != "abc" || lines[1].Voice != "" {
			t.Fatalf("lines = %+v", lines)
		}
	})

	t.Run("a JSONL record with no text is rejected", func(t *testing.T) {
		if _, err := ParseBatchInput(`{"voice":"abc"}`, false); err == nil {
			t.Fatal("ParseBatchInput accepted a record with no text")
		}
	})

	t.Run("dialogue turns", func(t *testing.T) {
		lines, err := ParseBatchInput("Alice: Hello there\nBob: Hi back\nAlice: Bye\n", true)
		if err != nil {
			t.Fatalf("ParseBatchInput error = %v", err)
		}
		if len(lines) != 3 {
			t.Fatalf("got %d lines, want 3", len(lines))
		}
		if lines[0].Speaker != "Alice" || lines[0].Text != "Hello there" {
			t.Fatalf("first turn = %+v", lines[0])
		}
		if lines[1].Speaker != "Bob" {
			t.Fatalf("second turn = %+v", lines[1])
		}
	})

	t.Run("a plain line under --dialogue is rejected", func(t *testing.T) {
		_, err := ParseBatchInput("just some prose\n", true)
		if err == nil {
			t.Fatal("ParseBatchInput accepted a non-dialogue line under --dialogue")
		}
		if !strings.Contains(err.Error(), "Speaker: text") {
			t.Fatalf("error = %v, want it to name the expected shape", err)
		}
	})
}

func TestParseSpeakerMap(t *testing.T) {
	got, err := ParseSpeakerMap([]string{"Alice=abc", "Bob=def"})
	if err != nil {
		t.Fatalf("ParseSpeakerMap error = %v", err)
	}
	if got["Alice"] != "abc" || got["Bob"] != "def" {
		t.Fatalf("ParseSpeakerMap = %v", got)
	}
	if _, err := ParseSpeakerMap([]string{"Alice"}); err == nil {
		t.Fatal("ParseSpeakerMap accepted a value with no =")
	}
}

func TestBuildDialogue(t *testing.T) {
	lines := []BatchLine{
		{LineNo: 1, Speaker: "Alice", Text: "Hello there"},
		{LineNo: 2, Speaker: "Bob", Text: "Hi back"},
		{LineNo: 3, Speaker: "Alice", Text: "Bye"},
	}
	d, err := BuildDialogue(lines, map[string]string{"Alice": "voice-a", "Bob": "voice-b"})
	if err != nil {
		t.Fatalf("BuildDialogue error = %v", err)
	}
	want := "<|speaker:0|>Hello there\n<|speaker:1|>Hi back\n<|speaker:0|>Bye"
	if d.Text != want {
		t.Fatalf("BuildDialogue text = %q, want %q", d.Text, want)
	}
	if len(d.ReferenceIDs) != 2 || d.ReferenceIDs[0] != "voice-a" || d.ReferenceIDs[1] != "voice-b" {
		t.Fatalf("BuildDialogue reference ids = %v", d.ReferenceIDs)
	}

	_, err = BuildDialogue(lines, map[string]string{"Alice": "voice-a"})
	if err == nil {
		t.Fatal("BuildDialogue accepted an unmapped speaker")
	}
	if !strings.Contains(err.Error(), "--speaker-map") {
		t.Fatalf("error = %v, want it to name the flag that fixes it", err)
	}
}

func TestBatchOutputName(t *testing.T) {
	if got := BatchOutputName(7, "wav"); got != "0007.wav" {
		t.Fatalf("BatchOutputName(7, wav) = %q, want 0007.wav", got)
	}
}

// TestBatchRowHash covers the per-file batch key: two output files of one
// render must not collide on the store's UNIQUE request_hash.
func TestBatchRowHash(t *testing.T) {
	req := RenderRequest{Text: "hello", VoiceID: "voice-a", Model: "s2.1-pro", Format: "mp3"}
	hash := req.Hash()
	cases := []struct {
		name  string
		left  string
		right string
		same  bool
	}{
		{name: "the same file is the same key", left: "out/0001.mp3", right: "out/0001.mp3", same: true},
		{name: "different files are different keys", left: "out/0001.mp3", right: "out/0003.mp3", same: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := BatchRowHash(hash, tc.left) == BatchRowHash(hash, tc.right)
			if got != tc.same {
				t.Fatalf("BatchRowHash equality = %v, want %v", got, tc.same)
			}
		})
	}
	if BatchRowHash(hash, "out/0001.mp3") == hash {
		t.Fatal("a batch row key must differ from the request hash it derives from")
	}
	other := RenderRequest{Text: "world", VoiceID: "voice-a", Model: "s2.1-pro", Format: "mp3"}
	if BatchRowHash(hash, "out/0001.mp3") == BatchRowHash(other.Hash(), "out/0001.mp3") {
		t.Fatal("two different renders written to one path share a key")
	}
}

// TestZeroShotBodyOmitsReferenceID guards the wire shape of a zero-shot render:
// references and reference_id must never both appear.
func TestZeroShotBodyOmitsReferenceID(t *testing.T) {
	zeroShot := RenderRequest{Text: "hello", Format: "mp3", ReferenceAudio: []byte{1, 2}, ReferenceText: "sample"}
	body := zeroShot.Body()
	if _, present := body["reference_id"]; present {
		t.Fatalf("zero-shot body carried reference_id: %v", body)
	}
	if _, present := body["references"]; !present {
		t.Fatalf("zero-shot body carried no references: %v", body)
	}

	saved := RenderRequest{Text: "hello", Format: "mp3", VoiceID: "voice-a"}
	savedBody := saved.Body()
	if savedBody["reference_id"] != "voice-a" {
		t.Fatalf("saved-voice body[reference_id] = %v, want voice-a", savedBody["reference_id"])
	}
	if _, present := savedBody["references"]; present {
		t.Fatalf("saved-voice body carried inline references: %v", savedBody)
	}
}

func TestBytesIn64(t *testing.T) {
	req := RenderRequest{Text: "hello"}
	if got := req.BytesIn64(); got != 5 {
		t.Fatalf("BytesIn64() = %d, want 5", got)
	}
}
