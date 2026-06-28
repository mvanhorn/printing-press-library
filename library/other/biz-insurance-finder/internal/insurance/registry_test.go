package insurance

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedRegistry_LoadsAndValidates(t *testing.T) {
	reg, err := EmbeddedRegistry()
	if err != nil {
		t.Fatalf("EmbeddedRegistry: %v", err)
	}
	if reg.Source != "embedded" {
		t.Errorf("Source = %q, want embedded", reg.Source)
	}
	if len(reg.Providers) < 15 {
		t.Errorf("expected the full seed registry, got %d providers", len(reg.Providers))
	}
	// Every provider must carry a known type and a non-empty appetite.
	validTypes := map[string]bool{
		ProviderTypeDirectCarrier: true, ProviderTypeDigitalAgency: true,
		ProviderTypeBrokerMarket: true, ProviderTypeSurplusSpecialty: true,
		ProviderTypeWholesalerAgent: true,
	}
	for _, p := range reg.Providers {
		if !validTypes[p.Type] {
			t.Errorf("provider %q has unknown type %q", p.ID, p.Type)
		}
		if p.URL == "" {
			t.Errorf("provider %q has no URL", p.ID)
		}
	}
}

func TestEmbeddedRegistry_IncludesTivlyAndSupersure(t *testing.T) {
	reg, _ := EmbeddedRegistry()
	if _, ok := reg.Get("tivly"); !ok {
		t.Errorf("registry must include Tivly")
	}
	supersure, ok := reg.Get("supersure")
	if !ok {
		t.Fatalf("registry must include Supersure")
	}
	if !supersure.Unverified {
		t.Errorf("Supersure must be flagged unverified until confirmed")
	}
}

func TestLoadRegistry_ExplicitOverride(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "providers.json")
	content := `{"providers":[{"id":"only","name":"Only Co","url":"https://x","type":"direct_carrier","quote_channel":"instant_online","appetite":{"importer":"poor","private_label":"poor","manufacturer":"poor","retail":"good","service":"good"}}]}`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	reg, err := LoadRegistry(path)
	if err != nil {
		t.Fatalf("LoadRegistry: %v", err)
	}
	if len(reg.Providers) != 1 || reg.Providers[0].ID != "only" {
		t.Errorf("override not honored: %+v", reg.Providers)
	}
	if reg.Source != path {
		t.Errorf("Source = %q, want %q", reg.Source, path)
	}
}

func TestParseRegistry_RejectsDuplicateID(t *testing.T) {
	bad := `{"providers":[{"id":"a","name":"A"},{"id":"a","name":"B"}]}`
	if _, err := parseRegistry([]byte(bad)); err == nil {
		t.Errorf("expected an error for duplicate ids")
	}
}

func TestProvider_StartURL(t *testing.T) {
	p := Provider{URL: "https://u", QuoteURL: "https://q"}
	if p.StartURL() != "https://q" {
		t.Errorf("StartURL with quote_url = %q", p.StartURL())
	}
	p2 := Provider{URL: "https://u"}
	if p2.StartURL() != "https://u" {
		t.Errorf("StartURL without quote_url = %q", p2.StartURL())
	}
}
