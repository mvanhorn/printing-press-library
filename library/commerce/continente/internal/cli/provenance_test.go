package cli

import (
	"encoding/json"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/commerce/continente/internal/config"
)

func TestWrapWithProvenanceIncludesPreferredStore(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"items":[{"id":"1"}]}`)
	wrapped, err := wrapWithProvenance(data, DataProvenance{
		Source: "live",
		Store: &config.PreferredStore{
			ID:         "col-1981-store",
			Name:       "Continente Bom Dia Sao Marcos",
			City:       "Sao Marcos",
			PostalCode: "2735-529",
		},
	}, "full")
	if err != nil {
		t.Fatalf("wrapWithProvenance(...) error = %v", err)
	}

	var payload struct {
		Meta struct {
			PreferredStore *config.PreferredStore `json:"preferred_store"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(wrapped, &payload); err != nil {
		t.Fatalf("json.Unmarshal(...) error = %v", err)
	}
	if payload.Meta.PreferredStore == nil {
		t.Fatal("preferred_store missing from provenance meta")
	}
	if payload.Meta.PreferredStore.ID != "col-1981-store" {
		t.Fatalf("preferred_store.id = %q; want col-1981-store", payload.Meta.PreferredStore.ID)
	}
}

func TestWrapWithProvenanceMinimalOmitsVerboseMeta(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"items":[{"id":"1"}]}`)
	wrapped, err := wrapWithProvenance(data, DataProvenance{
		Source:       "live",
		ResourceType: "search",
		Reason:       "api_unreachable",
	}, "minimal")
	if err != nil {
		t.Fatalf("wrapWithProvenance(...) error = %v", err)
	}

	var payload struct {
		Meta map[string]any `json:"meta"`
	}
	if err := json.Unmarshal(wrapped, &payload); err != nil {
		t.Fatalf("json.Unmarshal(...) error = %v", err)
	}
	if got := payload.Meta["source"]; got != "live" {
		t.Fatalf("meta.source = %#v, want live", got)
	}
	if _, ok := payload.Meta["resource_type"]; ok {
		t.Fatalf("minimal meta should omit resource_type: %#v", payload.Meta)
	}
	if _, ok := payload.Meta["reason"]; ok {
		t.Fatalf("minimal meta should omit reason: %#v", payload.Meta)
	}
}

func TestWrapWithProvenanceNoneReturnsRawData(t *testing.T) {
	t.Parallel()

	data := json.RawMessage(`{"items":[{"id":"1"}]}`)
	wrapped, err := wrapWithProvenance(data, DataProvenance{Source: "live"}, "none")
	if err != nil {
		t.Fatalf("wrapWithProvenance(...) error = %v", err)
	}
	if string(wrapped) != string(data) {
		t.Fatalf("wrapped = %s, want raw %s", string(wrapped), string(data))
	}
}
