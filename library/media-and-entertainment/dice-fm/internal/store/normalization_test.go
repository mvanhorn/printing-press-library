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

func TestCanonicalEntityParent(t *testing.T) {
	s := openTestStore(t)
	if err := s.UpsertCanonicalEntityWithParent("venue", "venue:complex1", "northside hall", ""); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertCanonicalEntityWithParent("venue", "venue:room1", "northside hall - main room", "venue:complex1"); err != nil {
		t.Fatal(err)
	}
	var parent string
	s.DB().QueryRow(`SELECT COALESCE(parent_canonical_id,'') FROM canonical_entity WHERE canonical_id='venue:room1'`).Scan(&parent)
	if parent != "venue:complex1" {
		t.Errorf("parent = %q, want venue:complex1", parent)
	}
}

// TestEntityAttributesRoundTrip verifies UpsertEntityAttribute writes via
// writeMu, the PK upsert overwrites a re-written key, and ListEntityAttributes
// reads the rows back filtered by entity type. Synthetic fixtures only.
func TestEntityAttributesRoundTrip(t *testing.T) {
	s := openTestStore(t)

	if err := s.UpsertEntityAttribute("price_tier:aa", "price_tier", "price_band", "mid", "regex", 1); err != nil {
		t.Fatalf("upsert price_band: %v", err)
	}
	if err := s.UpsertEntityAttribute("price_tier:aa", "price_tier", "currency", "usd", "regex", 1); err != nil {
		t.Fatalf("upsert currency: %v", err)
	}
	// Re-upsert the same (canonical_id, attr_key) updates value/method.
	if err := s.UpsertEntityAttribute("price_tier:aa", "price_tier", "price_band", "high", "manual", 2); err != nil {
		t.Fatalf("re-upsert price_band: %v", err)
	}
	// A different entity type must not appear in a price_tier listing.
	if err := s.UpsertEntityAttribute("genre:bb", "genre", "family", "electronic", "regex", 1); err != nil {
		t.Fatalf("upsert genre: %v", err)
	}

	rows, err := s.ListEntityAttributes("price_tier")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 price_tier rows, got %d: %+v", len(rows), rows)
	}
	// Ordered by canonical_id, attr_key: currency before price_band.
	if rows[0].AttrKey != "currency" || rows[0].AttrValue != "usd" {
		t.Errorf("row[0] = %+v, want currency=usd", rows[0])
	}
	if rows[1].AttrKey != "price_band" || rows[1].AttrValue != "high" || rows[1].Method != "manual" || rows[1].ClassifierVersion != 2 {
		t.Errorf("row[1] = %+v, want price_band=high method=manual cv=2 (updated)", rows[1])
	}
	if rows[1].EntityType != "price_tier" {
		t.Errorf("row[1].EntityType = %q, want price_tier", rows[1].EntityType)
	}
}

// TestEntityAttributeTypeImmutableOnConflict verifies that re-upserting the same
// (canonical_id, attr_key) with a different entity_type does NOT change the
// stored entity_type (F4): the canonical_id is type-prefixed and owns exactly
// one entity type for life, so the ON CONFLICT path must not relabel it.
func TestEntityAttributeTypeImmutableOnConflict(t *testing.T) {
	s := openTestStore(t)

	if err := s.UpsertEntityAttribute("price_tier:aa", "price_tier", "price_band", "mid", "regex", 1); err != nil {
		t.Fatalf("initial upsert: %v", err)
	}
	// Re-upsert the same key but with a (wrong) different entity_type. The value
	// should update; the entity_type must stay price_tier.
	if err := s.UpsertEntityAttribute("price_tier:aa", "genre", "price_band", "high", "manual", 2); err != nil {
		t.Fatalf("conflicting-type upsert: %v", err)
	}

	rows, err := s.ListEntityAttributes("price_tier")
	if err != nil {
		t.Fatalf("list price_tier: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 price_tier row, got %d: %+v", len(rows), rows)
	}
	if rows[0].EntityType != "price_tier" {
		t.Errorf("entity_type = %q, want price_tier (must be immutable on conflict)", rows[0].EntityType)
	}
	if rows[0].AttrValue != "high" {
		t.Errorf("attr_value = %q, want high (value should still update)", rows[0].AttrValue)
	}
	// The row must NOT have moved into the "genre" listing.
	genreRows, err := s.ListEntityAttributes("genre")
	if err != nil {
		t.Fatalf("list genre: %v", err)
	}
	if len(genreRows) != 0 {
		t.Errorf("genre listing should be empty; the row must not have been relabeled: %+v", genreRows)
	}
}

// TestClearNormalizationAtomicClearsAllTables verifies the transactional clear
// (F3) removes non-manual rows from the crosswalk AND the typed/generic
// attribute tables in one shot, leaving manual rows intact.
func TestClearNormalizationAtomicClearsAllTables(t *testing.T) {
	s := openTestStore(t)

	// A non-manual ticket_type: crosswalk + tier_attributes.
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "vip lounge",
		CanonicalID: "ticket_type:vip", Method: "regex", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert non-manual crosswalk: %v", err)
	}
	if err := s.UpsertTierAttributes("ticket_type:vip", TierAttributesRow{
		CanonicalID: "ticket_type:vip", AccessClass: "vip", ClassifierVersion: 1, Method: "regex",
	}); err != nil {
		t.Fatalf("upsert tier attrs: %v", err)
	}
	// A manual ticket_type that must survive.
	if err := s.UpsertCrosswalk(CrosswalkRow{
		EntityType: "ticket_type", SourceSystem: "dice", SourceValue: "comp guest",
		CanonicalID: "ticket_type:comp", Method: "manual", ClassifierVersion: 1,
	}); err != nil {
		t.Fatalf("upsert manual crosswalk: %v", err)
	}
	if err := s.UpsertTierAttributes("ticket_type:comp", TierAttributesRow{
		CanonicalID: "ticket_type:comp", CompFlag: true, ClassifierVersion: 1, Method: "manual",
	}); err != nil {
		t.Fatalf("upsert manual tier attrs: %v", err)
	}

	if err := s.ClearNormalization("ticket_type"); err != nil {
		t.Fatalf("ClearNormalization: %v", err)
	}

	cw, err := s.ListCrosswalk("ticket_type", "dice")
	if err != nil {
		t.Fatalf("list crosswalk: %v", err)
	}
	if len(cw) != 1 || cw[0].CanonicalID != "ticket_type:comp" {
		t.Fatalf("want only the manual crosswalk row left, got %+v", cw)
	}

	tiers, err := s.ListTierAttributes("ticket_type")
	if err != nil {
		t.Fatalf("list tier attrs: %v", err)
	}
	if len(tiers) != 1 || tiers[0].CanonicalID != "ticket_type:comp" {
		t.Fatalf("want only the manual tier_attributes row left, got %+v", tiers)
	}
}

// TestClearNormalizationPreservesManualEntityAttributes verifies that
// ClearNormalization deletes non-manual entity_attributes rows for the entity
// type while preserving rows whose own method is "manual". Synthetic fixtures.
func TestClearNormalizationPreservesManualEntityAttributes(t *testing.T) {
	s := openTestStore(t)

	// A derived (regex) attribute and a manual attribute on the same entity type.
	if err := s.UpsertEntityAttribute("price_tier:derived", "price_tier", "price_band", "low", "regex", 1); err != nil {
		t.Fatalf("upsert derived: %v", err)
	}
	if err := s.UpsertEntityAttribute("price_tier:manual", "price_tier", "price_band", "high", "manual", 1); err != nil {
		t.Fatalf("upsert manual: %v", err)
	}

	if err := s.ClearNormalization("price_tier"); err != nil {
		t.Fatalf("clear: %v", err)
	}

	rows, err := s.ListEntityAttributes("price_tier")
	if err != nil {
		t.Fatalf("list after clear: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 row after clear (manual preserved, regex deleted), got %d: %+v", len(rows), rows)
	}
	if rows[0].Method != "manual" || rows[0].CanonicalID != "price_tier:manual" {
		t.Errorf("surviving row = %+v, want the manual one", rows[0])
	}
}
