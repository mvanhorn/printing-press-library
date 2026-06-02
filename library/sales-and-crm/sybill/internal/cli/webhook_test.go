// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Test the Svix webhook signature verification against a known-good signature.

package cli

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
	"time"
)

func signSvix(t *testing.T, secretB64, id, ts string, body []byte) string {
	t.Helper()
	key, err := base64.StdEncoding.DecodeString(secretB64)
	if err != nil {
		t.Fatalf("decode secret: %v", err)
	}
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + ts + "." + string(body)))
	return "v1," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func TestVerifySvixSignature(t *testing.T) {
	secretB64 := base64.StdEncoding.EncodeToString([]byte("super-secret-key-material"))
	now := time.Unix(1717320000, 0)
	id := "msg_2abc"
	ts := "1717320000"
	body := []byte(`{"event":"meeting.new_recording.v1"}`)

	good := signSvix(t, secretB64, id, ts, body)

	// Valid signature, secret with whsec_ prefix.
	if r := verifySvixSignature("whsec_"+secretB64, id, ts, good, body, 5*time.Minute, now); !r.Valid {
		t.Fatalf("expected valid, got %+v", r)
	}
	// Valid signature, bare secret (no prefix).
	if r := verifySvixSignature(secretB64, id, ts, good, body, 5*time.Minute, now); !r.Valid {
		t.Fatalf("expected valid (no prefix), got %+v", r)
	}
	// Tampered body must fail.
	if r := verifySvixSignature(secretB64, id, ts, good, []byte(`{"event":"tampered"}`), 5*time.Minute, now); r.Valid {
		t.Fatalf("tampered body should fail")
	}
	// Wrong secret must fail.
	other := base64.StdEncoding.EncodeToString([]byte("different-key"))
	if r := verifySvixSignature(other, id, ts, good, body, 5*time.Minute, now); r.Valid {
		t.Fatalf("wrong secret should fail")
	}
	// Stale timestamp must fail when tolerance is enforced.
	if r := verifySvixSignature(secretB64, id, ts, good, body, 1*time.Minute, now.Add(10*time.Minute)); r.Valid {
		t.Fatalf("stale timestamp should fail")
	}
	// Stale timestamp tolerated when tolerance disabled.
	if r := verifySvixSignature(secretB64, id, ts, good, body, 0, now.Add(10*time.Minute)); !r.Valid {
		t.Fatalf("tolerance=0 should skip timestamp check, got %+v", r)
	}
}
