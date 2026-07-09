// Package june implements June's cloud command protocol: the Ed25519-signed
// WebSocket envelope, SRP-6a pairing, and telemetry decoding. The wire format is
// reverse-engineered and verified against a real oven; see the plan and the
// golden vectors in testdata/vectors.json. The oven silently drops any command
// whose signature is not the exact 72-byte BLAKE2b(pubkey,8)||Ed25519 form, so
// byte-exact serialization is load-bearing and covered by signer_test.go.
package june

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

// Message codes (companion -> oven).
const (
	CodeKeepalive = 11011
	CodePreheat   = 11002
	CodeTemp      = 11005
	CodeTimer     = 11006
	CodeCancel    = 11004
)

// Envelope is serialized with its fields in exactly this order; the order is
// part of the signed bytes. Do not reorder, and do not marshal via a map.
type Envelope struct {
	V           int             `json:"v"`
	MessageCode int             `json:"message_code"`
	Order       int64           `json:"order"`
	Time        int64           `json:"time"`
	Signature   string          `json:"signature"`
	DeviceName  string          `json:"device_name"`
	DeviceID    string          `json:"device_id"`
	Data        json.RawMessage `json:"data"`
	Target      target          `json:"target"`
}

type target struct {
	ID string `json:"id"`
}

// Signer holds the Ed25519 identity the oven trusts (derived from the 32-byte
// seed captured at pairing) plus the strictly-increasing order counter.
type Signer struct {
	priv        ed25519.PrivateKey
	fingerprint []byte // 8-byte BLAKE2b of the public key

	mu        sync.Mutex
	lastOrder int64
}

// NewSigner builds a Signer from a 32-byte Ed25519 seed.
func NewSigner(seed []byte) (*Signer, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("june: seed must be %d bytes, got %d", ed25519.SeedSize, len(seed))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub := priv.Public().(ed25519.PublicKey)
	fp := fingerprint8(pub)
	return &Signer{priv: priv, fingerprint: fp}, nil
}

// fingerprint8 is libsodium crypto_generichash(pub, 8) — BLAKE2b with an 8-byte
// digest, not a truncation of BLAKE2b-512.
func fingerprint8(pub []byte) []byte {
	h, err := blake2b.New(8, nil)
	if err != nil {
		// New only errors on invalid size/key; 8 with no key is always valid.
		panic(err)
	}
	h.Write(pub)
	return h.Sum(nil)
}

// nextOrder returns a strictly-increasing order the oven echoes as request_order.
func (s *Signer) nextOrder(now int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := now & 0x7fffffff
	if o <= s.lastOrder {
		o = s.lastOrder + 1
	}
	s.lastOrder = o
	return o
}

// compactJSON marshals v with no whitespace and HTML escaping disabled, matching
// Python's json.dumps(separators=(",",":"), ensure_ascii=False).
func compactJSON(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// canonicalBytes returns the exact bytes that get signed: the envelope with
// signature set to "".
func canonicalBytes(env Envelope) ([]byte, error) {
	env.Signature = ""
	return compactJSON(env)
}

// sign fills env.Signature with base64(fingerprint||Ed25519(canonicalBytes)) and
// returns the wire frame to send.
func (s *Signer) sign(env Envelope) (string, error) {
	canon, err := canonicalBytes(env)
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(s.priv, canon)
	wire := make([]byte, 0, len(s.fingerprint)+len(sig))
	wire = append(wire, s.fingerprint...)
	wire = append(wire, sig...)
	env.Signature = base64.StdEncoding.EncodeToString(wire)
	frame, err := compactJSON(env)
	if err != nil {
		return "", err
	}
	return string(frame), nil
}

// Frame builds and signs a command frame. data must already be compact,
// ordered JSON (use json.RawMessage("{}") for an empty payload). now is epoch
// milliseconds; order is derived from it. It returns the wire string plus the
// order so callers can match the oven's ack (request_order).
func (s *Signer) Frame(code int, data json.RawMessage, deviceName, deviceID, ovenID string, now int64) (frame string, order int64, err error) {
	order = s.nextOrder(now)
	env := Envelope{
		V:           2,
		MessageCode: code,
		Order:       order,
		Time:        now,
		Signature:   "",
		DeviceName:  deviceName,
		DeviceID:    deviceID,
		Data:        data,
		Target:      target{ID: ovenID},
	}
	frame, err = s.sign(env)
	return frame, order, err
}

// NowMillis is the current epoch-millisecond clock used for live frames.
func NowMillis() int64 { return time.Now().UnixMilli() }
