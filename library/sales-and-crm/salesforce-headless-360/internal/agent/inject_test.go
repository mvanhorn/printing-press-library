package agent

import (
	"crypto/ed25519"
	"crypto/rand"
	"os"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

type fakeSlackPoster struct {
	posted int
	text   string
}

func (f *fakeSlackPoster) PostMessage(_ string, text string) error {
	f.posted++
	f.text = text
	return nil
}
func (f *fakeSlackPoster) PostEphemeral(_, _, text string) error {
	f.posted++
	f.text = text
	return nil
}
func (f *fakeSlackPoster) UploadFile(_, _ string) error { return nil }

func TestInjectBundleVerifiesIntersectsAndPosts(t *testing.T) {
	path := signedInjectBundle(t)
	poster := &fakeSlackPoster{}
	result, err := InjectBundle(InjectOptions{
		BundlePath: path,
		Channel:    "C123",
		Members: []AudienceMember{
			{SalesforceUserID: "005A", Fields: map[string][]string{"Account": {"Name", "Industry"}, "Opportunity": {"Name"}}},
			{SalesforceUserID: "005B", Fields: map[string][]string{"Account": {"Name"}, "Opportunity": {"Name"}}},
		},
		Slack: poster,
	})
	if err != nil {
		t.Fatalf("InjectBundle: %v", err)
	}
	if !result.InjectAudienceIntersected || poster.posted != 1 {
		t.Fatalf("inject did not post/intersect: result=%+v posted=%d", result, poster.posted)
	}
	if contains(poster.text, "Manufacturing") {
		t.Fatalf("posted field outside intersection:\n%s", poster.text)
	}
}

func TestInjectBundleRejectsBadSignatureBeforePost(t *testing.T) {
	path := signedInjectBundle(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read bundle: %v", err)
	}
	if err := os.WriteFile(path, append(data, []byte("x")...), 0o600); err != nil {
		t.Fatalf("tamper bundle: %v", err)
	}
	poster := &fakeSlackPoster{}
	if _, err := InjectBundle(InjectOptions{BundlePath: path, Channel: "C123", Slack: poster}); err == nil {
		t.Fatal("expected bad signature to reject")
	}
	if poster.posted != 0 {
		t.Fatalf("posted before verification: %d", poster.posted)
	}
}

func signedInjectBundle(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	s := fixedSigner{priv: priv, pub: pub, kid: "kid-inject"}
	if err := trust.SaveKeyRecord(trust.KeyRecord{KID: "kid-inject", OrgAlias: "test", PublicKeyPEM: encodePubPEM(t, pub)}); err != nil {
		t.Fatalf("SaveKeyRecord: %v", err)
	}
	bundle, err := Assemble(Manifest{
		Account:       &Account{ID: "001A", Name: "Acme", Industry: "Manufacturing"},
		Opportunities: []Opportunity{{ID: "006A", Name: "Renewal", Amount: 1000}},
	}, AssembleOptions{
		OrgID:       "00D",
		UserID:      "005",
		QueryWindow: "P90D",
		TraceID:     "trace-inject",
		SourcesUsed: []string{"local"},
	}, s)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	path := tmp + "/bundle.json"
	writeBundle(t, path, bundle)
	return path
}
