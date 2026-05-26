package store

import (
	"context"
	"path/filepath"
	"testing"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestNormalizationTablesCreated(t *testing.T) {
	s := openTestStore(t)
	for _, table := range []string{"canonical_entity", "entity_external_ref", "entity_crosswalk", "tier_attributes", "venue_attributes"} {
		var name string
		err := s.DB().QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table).Scan(&name)
		if err != nil {
			t.Errorf("table %q not created: %v", table, err)
		}
	}
}

func TestCrosswalkRoundTrip(t *testing.T) {
	s := openTestStore(t)
	err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "general admission ",
		CanonicalID: "ticket_type:abc123", Method: "regex", ClassifierVersion: 1,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	// Re-upsert with method=manual must overwrite (manual wins on re-run).
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "general admission ",
		CanonicalID: "ticket_type:abc123", Method: "manual", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	rows, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].Method != "manual" {
		t.Fatalf("want 1 row method=manual, got %+v", rows)
	}
}

func TestClearNormalizationPreservesManual(t *testing.T) {
	s := openTestStore(t)

	// Insert a regex row and a manual row for the same entity_type.
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "general admission",
		CanonicalID: "ticket_type:aa", Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert regex: %v", err)
	}
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "vip experience",
		CanonicalID: "ticket_type:bb", Method: "manual", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert manual: %v", err)
	}

	// Insert tier_attributes for both.
	if err := s.UpsertTierAttributes("ticket_type:aa", TierAttributesRow{
		AccessClass: "ga", ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs regex: %v", err)
	}
	if err := s.UpsertTierAttributes("ticket_type:bb", TierAttributesRow{
		AccessClass: "vip", ClassifierVersion: 1, Method: "manual",
	}); err != nil {
		t.Fatalf("upsert tier attrs manual: %v", err)
	}

	if err := s.ClearNormalization("ticket_type"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	rows, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(rows) != 1 || rows[0].Method != "manual" {
		t.Fatalf("want 1 manual row after clear, got %+v", rows)
	}

	attrs, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("list tier attrs after clear: %v", err)
	}
	if len(attrs) != 1 || attrs[0].Method != "manual" {
		t.Fatalf("want 1 manual tier attr after clear, got %+v", attrs)
	}
}
