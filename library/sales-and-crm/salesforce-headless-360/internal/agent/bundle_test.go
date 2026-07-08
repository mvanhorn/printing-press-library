package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

// fixedSigner is an inline signer for tests so we don't depend on on-disk
// key generation.
type fixedSigner struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
	kid  string
}

func (s fixedSigner) Sign(payload []byte) ([]byte, error) { return ed25519.Sign(s.priv, payload), nil }
func (s fixedSigner) KID() string                         { return s.kid }

// TestAssembleRoundtrip assembles a bundle, saves the public key record,
// and verifies the bundle. The round-trip is the primary contract.
func TestAssembleRoundtrip(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	s := fixedSigner{priv: priv, pub: pub, kid: "test-kid-1"}

	// Save a matching public-key record to the keystore so verify can find it
	rec := trust.KeyRecord{KID: "test-kid-1", OrgAlias: "test", Source: "local-generated"}
	// Inline-encode the PEM to avoid round-tripping through FileSigner
	rec.PublicKeyPEM = encodePubPEM(t, pub)
	trust.SaveKeyRecord(rec)

	m := Manifest{
		Account:  &Account{ID: "001ABC", Name: "Acme Corp"},
		Contacts: []Contact{{ID: "003AAA", FirstName: "Ada", LastName: "Lovelace"}},
	}
	opts := AssembleOptions{
		OrgAlias:    "test",
		OrgID:       "00D000000000001",
		UserID:      "005000000000001",
		QueryWindow: "P90D",
		TraceID:     "01JEHXTEST",
		SourcesUsed: []string{"local"},
	}
	bundle, err := Assemble(m, opts, s)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if bundle.Signature == "" {
		t.Fatal("expected signature")
	}
	if bundle.Envelope.Aud != "agent-context" {
		t.Fatalf("aud mismatch: %s", bundle.Envelope.Aud)
	}
	if bundle.Envelope.ExpiresAt.Before(time.Now()) {
		t.Fatal("expected future expiry")
	}

	// Persist to disk and verify via VerifyBundle
	path := tmp + "/bundle.json"
	writeBundle(t, path, bundle)

	result, err := VerifyBundle(path, VerifyOptions{})
	if err != nil {
		t.Fatalf("VerifyBundle: %v", err)
	}
	if !result.SignatureOK {
		t.Fatal("signature not ok")
	}
	if result.Issuer != "00D000000000001" {
		t.Fatalf("issuer mismatch: %s", result.Issuer)
	}
	if result.Audience != "agent-context" {
		t.Fatalf("aud mismatch: %s", result.Audience)
	}
}

// TestVerifyStrict_ExpiredFails checks --strict rejects expired bundles.
func TestVerifyStrict_ExpiredFails(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	s := fixedSigner{priv: priv, pub: pub, kid: "kid-expired"}

	rec := trust.KeyRecord{KID: "kid-expired", OrgAlias: "test", Source: "local-generated", PublicKeyPEM: encodePubPEM(t, pub)}
	trust.SaveKeyRecord(rec)

	m := Manifest{Account: &Account{ID: "001X"}}
	opts := AssembleOptions{
		OrgID:   "00D000000000002",
		UserID:  "005000000000002",
		TraceID: "t1",
		TTL:     -1 * time.Hour, // already expired
	}
	bundle, _ := Assemble(m, opts, s)
	path := tmp + "/bundle.json"
	writeBundle(t, path, bundle)

	if _, err := VerifyBundle(path, VerifyOptions{Strict: true}); err == nil {
		t.Fatal("expected strict verify to fail on expired bundle")
	}
}

// TestDryRunSummary is a smoke test for the dry-run path.
func TestDryRunSummary(t *testing.T) {
	m := Manifest{
		Account:  &Account{ID: "001"},
		Contacts: []Contact{{ID: "003A"}, {ID: "003B"}},
	}
	env := Envelope{Redactions: map[string]int{"PII": 2}, SourcesUsed: []string{"local"}}
	out := DryRunSummary(m, env)
	if !contains(out, "DRY RUN") || !contains(out, "contacts=2") || !contains(out, "PII") {
		t.Fatalf("dry-run summary missing expected content:\n%s", out)
	}
}

func contains(s, sub string) bool { return indexOf(s, sub) >= 0 }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
