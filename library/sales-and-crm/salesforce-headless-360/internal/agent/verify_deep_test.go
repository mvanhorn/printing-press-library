package agent

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestVerifyDeepRejectsTamperedContentVersionBytes(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	s := fixedSigner{priv: priv, pub: pub, kid: "kid-deep"}
	if err := trust.SaveKeyRecord(trust.KeyRecord{KID: "kid-deep", OrgAlias: "test", PublicKeyPEM: encodePubPEM(t, pub)}); err != nil {
		t.Fatalf("SaveKeyRecord: %v", err)
	}
	file, err := AttestContentVersion(context.Background(), mapContentFetcher{"068A": "original"}, "contract.pdf", "068A")
	if err != nil {
		t.Fatalf("AttestContentVersion: %v", err)
	}
	bundle, err := Assemble(Manifest{Account: &Account{ID: "001A", Name: "Acme"}, Files: []FileRef{file}}, AssembleOptions{
		OrgID:       "00D",
		UserID:      "005",
		QueryWindow: "P90D",
		TraceID:     "trace-deep",
		SourcesUsed: []string{"composite_graph"},
	}, s)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	path := tmp + "/bundle.json"
	writeBundle(t, path, bundle)

	if _, err := VerifyBundle(path, VerifyOptions{Deep: true, FileFetcher: mapContentFetcher{"068A": "tampered"}}); err == nil || !strings.Contains(err.Error(), "FILE_BYTES_TAMPERED") {
		t.Fatalf("expected FILE_BYTES_TAMPERED, got %v", err)
	}
}
