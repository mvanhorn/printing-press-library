package store

import (
	"context"
	"path/filepath"
	"testing"
)

// TestListVenueAttributesRoundTrip verifies that UpsertVenueAttributes followed
// by ListVenueAttributes returns the seeded row, mirroring the tier_attributes
// round-trip so normalize stats counts are symmetric.
func TestListVenueAttributesRoundTrip(t *testing.T) {
	s := openTestStore(t)

	// Seed a crosswalk row so the canonical_id is reachable via venue entity_type.
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "venue", SourceSystem: "dice", SourceValue: "northside hall",
		CanonicalID: "venue:abc", Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk: %v", err)
	}
	if err := s.UpsertVenueAttributes("venue:abc", VenueAttributesRow{
		CanonicalID: "venue:abc", Complex: "northside hall", Room: "",
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert venue attributes: %v", err)
	}

	rows, err := s.ListVenueAttributes("venue")
	if err != nil {
		t.Fatalf("ListVenueAttributes: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 venue attribute row, got %d", len(rows))
	}
	if rows[0].Complex != "northside hall" {
		t.Errorf("complex = %q, want %q", rows[0].Complex, "northside hall")
	}
}

// TestListVenueAttributesExcludesUnmatched verifies that unmatched crosswalk
// rows (no venue_attributes entry) are not counted by ListVenueAttributes,
// mirroring the ListTierAttributes join behaviour.
func TestListVenueAttributesExcludesUnmatched(t *testing.T) {
	s := openTestStore(t)

	// Matched venue.
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "venue", SourceSystem: "dice", SourceValue: "matched venue",
		CanonicalID: "venue:m1", Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk matched: %v", err)
	}
	if err := s.UpsertVenueAttributes("venue:m1", VenueAttributesRow{
		CanonicalID: "venue:m1", Complex: "matched venue",
		ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert venue attributes: %v", err)
	}
	// Unmatched venue — crosswalk row exists but no venue_attributes.
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "venue", SourceSystem: "dice", SourceValue: "unmatched venue",
		CanonicalID: "venue:u1", Method: "unmatched", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert crosswalk unmatched: %v", err)
	}

	rows, err := s.ListVenueAttributes("venue")
	if err != nil {
		t.Fatalf("ListVenueAttributes: %v", err)
	}
	if len(rows) != 1 {
		t.Errorf("want 1 matched venue attribute row (unmatched excluded), got %d", len(rows))
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenWithContext(context.Background(), filepath.Join(t.TempDir(), "data.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// TestUpsertExternalRefRoundTrip verifies that UpsertExternalRef writes via
// writeMu and can be read back from entity_external_ref.
func TestUpsertExternalRefRoundTrip(t *testing.T) {
	s := openTestStore(t)

	if err := s.UpsertExternalRef("ticket_type", "ticket_type:abc", "dice", "ext-999"); err != nil {
		t.Fatalf("UpsertExternalRef: %v", err)
	}

	// Idempotent: second call with a different external_id updates the row.
	if err := s.UpsertExternalRef("ticket_type", "ticket_type:abc", "dice", "ext-updated"); err != nil {
		t.Fatalf("UpsertExternalRef idempotent: %v", err)
	}

	var extID string
	err := s.DB().QueryRow(
		`SELECT external_id FROM entity_external_ref
		 WHERE entity_type=? AND canonical_id=? AND source_system=?`,
		"ticket_type", "ticket_type:abc", "dice",
	).Scan(&extID)
	if err != nil {
		t.Fatalf("query external_ref: %v", err)
	}
	if extID != "ext-updated" {
		t.Errorf("external_id = %q, want %q", extID, "ext-updated")
	}
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
