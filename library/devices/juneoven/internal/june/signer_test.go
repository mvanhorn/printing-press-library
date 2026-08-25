package june

import (
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// vectors mirrors the fields of testdata/vectors.json this package verifies.
// The fixtures are generated from the verified Python reference (gen_vectors.py)
// with fixed synthetic inputs; the Go port must reproduce them byte-for-byte.
type vectors struct {
	Inputs struct {
		SeedHex    string `json:"seed_hex"`
		DeviceID   string `json:"device_id"`
		OvenID     string `json:"oven_id"`
		DeviceName string `json:"device_name"`
		Order      int64  `json:"order"`
		Time       int64  `json:"time"`
	} `json:"inputs"`
	SignKeepalive signVector `json:"sign_keepalive_11011"`
	SignPreheat   signVector `json:"sign_preheat_11002"`
}

type signVector struct {
	CanonicalBytes   string `json:"canonical_bytes"`
	FingerprintHex   string `json:"fingerprint_hex"`
	WireSignatureB64 string `json:"wire_signature_b64"`
	WireSignatureLen int    `json:"wire_signature_len"`
}

func loadVectors(t *testing.T) vectors {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "vectors.json"))
	if err != nil {
		t.Fatalf("read vectors: %v", err)
	}
	var v vectors
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatalf("parse vectors: %v", err)
	}
	return v
}

func newTestSigner(t *testing.T, seedHex string) *Signer {
	t.Helper()
	seed, err := hex.DecodeString(seedHex)
	if err != nil {
		t.Fatalf("decode seed: %v", err)
	}
	s, err := NewSigner(seed)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}
	return s
}

// buildEnvelope constructs an envelope with an explicit order/time (the vectors
// use fixed values, not clock-derived ones).
func buildEnvelope(v vectors, code int, data json.RawMessage) Envelope {
	return Envelope{
		V:           2,
		MessageCode: code,
		Order:       v.Inputs.Order,
		Time:        v.Inputs.Time,
		Signature:   "",
		DeviceName:  v.Inputs.DeviceName,
		DeviceID:    v.Inputs.DeviceID,
		Data:        data,
		Target:      target{ID: v.Inputs.OvenID},
	}
}

func checkSignVector(t *testing.T, name string, s *Signer, env Envelope, want signVector) {
	t.Helper()

	// Canonical (pre-signature) bytes must match the Python reference exactly.
	canon, err := canonicalBytes(env)
	if err != nil {
		t.Fatalf("%s: canonical bytes: %v", name, err)
	}
	if string(canon) != want.CanonicalBytes {
		t.Errorf("%s canonical bytes mismatch:\n got: %s\nwant: %s", name, canon, want.CanonicalBytes)
	}

	// 8-byte key fingerprint.
	if got := hex.EncodeToString(s.fingerprint); got != want.FingerprintHex {
		t.Errorf("%s fingerprint mismatch: got %s want %s", name, got, want.FingerprintHex)
	}

	// Full 72-byte wire signature, standard base64.
	frame, err := s.sign(env)
	if err != nil {
		t.Fatalf("%s: sign: %v", name, err)
	}
	var signed Envelope
	if err := json.Unmarshal([]byte(frame), &signed); err != nil {
		t.Fatalf("%s: reparse signed frame: %v", name, err)
	}
	if signed.Signature != want.WireSignatureB64 {
		t.Errorf("%s wire signature mismatch:\n got: %s\nwant: %s", name, signed.Signature, want.WireSignatureB64)
	}
}

func TestSignKeepaliveVector(t *testing.T) {
	v := loadVectors(t)
	s := newTestSigner(t, v.Inputs.SeedHex)
	env := buildEnvelope(v, CodeKeepalive, json.RawMessage("{}"))
	checkSignVector(t, "keepalive", s, env, v.SignKeepalive)
}

func TestSignPreheatVector(t *testing.T) {
	v := loadVectors(t)
	s := newTestSigner(t, v.Inputs.SeedHex)
	// Nested data key order (primitive_type, temperature_cavity) is inside the
	// signed bytes, so it is built from an ordered struct, never a map.
	data, err := compactJSON(struct {
		PrimitiveType     string `json:"primitive_type"`
		TemperatureCavity int    `json:"temperature_cavity"`
	}{"bake", 176667})
	if err != nil {
		t.Fatalf("build preheat data: %v", err)
	}
	env := buildEnvelope(v, CodePreheat, data)
	checkSignVector(t, "preheat", s, env, v.SignPreheat)
}

func TestOrderStrictlyIncreasing(t *testing.T) {
	s := newTestSigner(t, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	// Two frames minted in the same millisecond must get distinct, increasing orders.
	const now = int64(1700000000000)
	o1 := s.nextOrder(now)
	o2 := s.nextOrder(now)
	if o2 <= o1 {
		t.Errorf("order not strictly increasing: o1=%d o2=%d", o1, o2)
	}
}
