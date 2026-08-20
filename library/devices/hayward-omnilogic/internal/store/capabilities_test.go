package store

import (
	"testing"
)

func TestSiteCapabilities_RoundTrip(t *testing.T) {
	s := openTempStore(t)

	// No row → returns nil
	got, err := s.GetSiteCapabilities(12345)
	if err != nil {
		t.Fatalf("Get with no row: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil when no row configured, got %+v", got)
	}

	// Upsert a row representing a basic chlorine-tab pool with the
	// temp-sensor-needs-flow quirk
	want := SiteCapabilities{
		SiteMspSystemID: 12345,
		HasPHSensor:     false,
		HasORPSensor:    false,
		HasSaltSensor:   false,
		TempNeedsFlow:   true,
		Notes:           "no chemistry probes; temp sensor needs flow",
	}
	if err := s.SetSiteCapabilities(want); err != nil {
		t.Fatalf("SetSiteCapabilities: %v", err)
	}

	got, err = s.GetSiteCapabilities(12345)
	if err != nil {
		t.Fatalf("Get after set: %v", err)
	}
	if got == nil {
		t.Fatalf("expected row, got nil")
	}
	if got.HasPHSensor || got.HasORPSensor || got.HasSaltSensor {
		t.Errorf("expected all sensors false, got pH=%v orp=%v salt=%v", got.HasPHSensor, got.HasORPSensor, got.HasSaltSensor)
	}
	if !got.TempNeedsFlow {
		t.Errorf("expected TempNeedsFlow=true, got false")
	}
	if got.Notes != want.Notes {
		t.Errorf("notes round-trip: want %q got %q", want.Notes, got.Notes)
	}
	if got.ConfiguredAt.IsZero() {
		t.Errorf("ConfiguredAt should be set by Set when caller leaves it zero")
	}
}

func TestSiteCapabilities_Update(t *testing.T) {
	s := openTempStore(t)
	first := SiteCapabilities{SiteMspSystemID: 1, HasPHSensor: true, HasORPSensor: true, HasSaltSensor: true}
	if err := s.SetSiteCapabilities(first); err != nil {
		t.Fatal(err)
	}
	second := SiteCapabilities{SiteMspSystemID: 1, HasPHSensor: false, HasORPSensor: false, HasSaltSensor: true}
	if err := s.SetSiteCapabilities(second); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSiteCapabilities(1)
	if got.HasPHSensor || got.HasORPSensor || !got.HasSaltSensor {
		t.Errorf("update did not apply: got pH=%v orp=%v salt=%v", got.HasPHSensor, got.HasORPSensor, got.HasSaltSensor)
	}
}

func TestSiteCapabilities_Clear(t *testing.T) {
	s := openTempStore(t)
	_ = s.SetSiteCapabilities(SiteCapabilities{SiteMspSystemID: 1, HasPHSensor: true})
	if err := s.ClearSiteCapabilities(1); err != nil {
		t.Fatal(err)
	}
	got, _ := s.GetSiteCapabilities(1)
	if got != nil {
		t.Errorf("expected nil after clear, got %+v", got)
	}
}

func TestAssumeAllEquipped(t *testing.T) {
	c := AssumeAllEquipped(42)
	if !c.HasPHSensor || !c.HasORPSensor || !c.HasSaltSensor {
		t.Errorf("default should assume all chemistry sensors equipped")
	}
	if c.TempNeedsFlow {
		t.Errorf("default should NOT assume temp-sensor-needs-flow quirk (it's installation-specific)")
	}
}
