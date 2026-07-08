package schemas

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

type testSigner struct {
	priv ed25519.PrivateKey
	kid  string
}

func (s testSigner) Sign(payload []byte) ([]byte, error) { return ed25519.Sign(s.priv, payload), nil }
func (s testSigner) KID() string                         { return s.kid }
func (s testSigner) PublicKeyPEM() string                { return "" }

func TestEnvelopeValidatesAcmeFixture(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	jws, err := trust.SignJWS(testSigner{priv: priv, kid: "kid-schema"}, []byte(`{"manifest_sha256":"`+strings64("a")+`"}`))
	if err != nil {
		t.Fatalf("sign jws: %v", err)
	}
	bundle := map[string]any{
		"$schema": "pp-salesforce-360/bundle/v1",
		"envelope": map[string]any{
			"org_id":              "00DACME",
			"user_id":             "005ACME",
			"generated_at":        time.Now().UTC().Format(time.RFC3339),
			"sources_used":        []string{"composite_graph"},
			"sources_unavailable": []string{},
			"redactions":          map[string]int{},
		},
		"manifest": map[string]any{
			"account": map[string]any{"id": "001ACME0001", "name": "Acme Manufacturing"},
			"files": []map[string]any{{
				"name":                  "contract.pdf",
				"sha256":                strings64("b"),
				"sf_content_version_id": "068ACME0001",
				"size_bytes":            12,
			}},
		},
		"signature": jws,
	}
	data, err := json.Marshal(bundle)
	if err != nil {
		t.Fatalf("marshal bundle: %v", err)
	}
	if err := ValidateBundle(data); err != nil {
		t.Fatalf("ValidateBundle: %v", err)
	}
}

func TestEnvelopeRejectsMissingSignatureKID(t *testing.T) {
	bundle := map[string]any{
		"envelope": map[string]any{
			"org_id":       "00D",
			"user_id":      "005",
			"generated_at": time.Now().UTC().Format(time.RFC3339),
			"sources_used": []string{"local"},
		},
		"manifest":  map[string]any{"account": map[string]any{"id": "001"}},
		"signature": "not-a-jws",
	}
	data, _ := json.Marshal(bundle)
	if err := ValidateBundle(data); err == nil {
		t.Fatal("expected schema validation error")
	}
}

func strings64(s string) string {
	out := ""
	for len(out) < 64 {
		out += s
	}
	return out[:64]
}
