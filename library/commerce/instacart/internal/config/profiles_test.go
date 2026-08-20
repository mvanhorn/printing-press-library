package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestValidProfileName(t *testing.T) {
	// Digits are intentionally allowed as the first char so address-derived
	// slugs like "1528-37th-ave-e" (produced by `profiles import`) round-trip.
	good := []string{"home", "work", "home.seattle", "a", "weekend-house", "unit_42", "abc123", "1528-37th-ave-e"}
	bad := []string{"", "Home", "-leading", ".dot", "has space", "has/slash", "way-too-long-name-that-exceeds-the-cap-by-a-lot"}
	for _, s := range good {
		if !ValidProfileName(s) {
			t.Errorf("expected %q to be valid", s)
		}
	}
	for _, s := range bad {
		if ValidProfileName(s) {
			t.Errorf("expected %q to be invalid", s)
		}
	}
}

func TestProfileCRUD(t *testing.T) {
	c := &Config{}
	if got := c.ProfileNames(); got != nil {
		t.Fatalf("ProfileNames on empty config = %v, want nil", got)
	}

	if err := c.SetProfile(Profile{Name: "home", PostalCode: "98112", Latitude: 47.63, Longitude: -122.28}); err != nil {
		t.Fatalf("SetProfile home: %v", err)
	}
	if err := c.SetProfile(Profile{Name: "work", PostalCode: "98052", Latitude: 47.67, Longitude: -122.12}); err != nil {
		t.Fatalf("SetProfile work: %v", err)
	}

	if names := c.ProfileNames(); !reflect.DeepEqual(names, []string{"home", "work"}) {
		t.Errorf("ProfileNames = %v, want [home work]", names)
	}

	if p, ok := c.GetProfile("home"); !ok || p.PostalCode != "98112" {
		t.Errorf("GetProfile(home) = (%+v, %v); want postal=98112 ok=true", p, ok)
	}
	if _, ok := c.GetProfile("missing"); ok {
		t.Errorf("GetProfile(missing) returned ok=true")
	}

	// Mutating the returned copy must not change the stored map.
	if p, _ := c.GetProfile("home"); true {
		p.PostalCode = "00000"
		_ = p
	}
	if p, _ := c.GetProfile("home"); p.PostalCode != "98112" {
		t.Errorf("GetProfile returned a live pointer; postal now %q", p.PostalCode)
	}

	if err := c.SetProfile(Profile{Name: "Invalid Name"}); err == nil {
		t.Errorf("SetProfile with invalid name should error")
	}
}

func TestUseAndApplyProfile(t *testing.T) {
	c := &Config{
		Profiles: map[string]Profile{
			"home": {Name: "home", PostalCode: "98112", AddressID: "73256642", Latitude: 47.63, Longitude: -122.28},
			"work": {Name: "work", PostalCode: "98052", AddressID: "12345", Latitude: 47.67, Longitude: -122.12, ZoneID: "42"},
		},
	}

	if err := c.UseProfile("home"); err != nil {
		t.Fatalf("UseProfile home: %v", err)
	}
	if c.ActiveProfile != "home" || c.PostalCode != "98112" || c.AddressID != "73256642" {
		t.Errorf("after UseProfile(home), config = %+v", c)
	}

	// Apply (no active-profile change).
	if err := c.ApplyProfile("work"); err != nil {
		t.Fatalf("ApplyProfile work: %v", err)
	}
	if c.ActiveProfile != "home" {
		t.Errorf("ApplyProfile should not change ActiveProfile; got %q", c.ActiveProfile)
	}
	if c.PostalCode != "98052" || c.AddressID != "12345" || c.ZoneID != "42" {
		t.Errorf("ApplyProfile(work) did not copy fields: postal=%q addr=%q zone=%q",
			c.PostalCode, c.AddressID, c.ZoneID)
	}

	if err := c.ApplyProfile("nope"); err == nil {
		t.Errorf("ApplyProfile of missing profile should error")
	}
	if err := c.UseProfile("nope"); err == nil {
		t.Errorf("UseProfile of missing profile should error")
	}
}

func TestDeleteProfile(t *testing.T) {
	c := &Config{
		Profiles:      map[string]Profile{"home": {Name: "home"}, "work": {Name: "work"}},
		ActiveProfile: "home",
	}
	if err := c.DeleteProfile("home"); err != nil {
		t.Fatalf("DeleteProfile home: %v", err)
	}
	if c.ActiveProfile != "" {
		t.Errorf("DeleteProfile of active should clear ActiveProfile; got %q", c.ActiveProfile)
	}
	if _, ok := c.GetProfile("home"); ok {
		t.Errorf("home still present after delete")
	}

	if err := c.DeleteProfile("missing"); err == nil {
		t.Errorf("DeleteProfile of missing profile should error")
	}

	// Deleting a non-active profile must leave ActiveProfile alone.
	c.ActiveProfile = "work"
	c.Profiles["spare"] = Profile{Name: "spare"}
	if err := c.DeleteProfile("spare"); err != nil {
		t.Fatalf("DeleteProfile spare: %v", err)
	}
	if c.ActiveProfile != "work" {
		t.Errorf("ActiveProfile changed by unrelated delete; got %q", c.ActiveProfile)
	}
}

func TestConfigBackwardsCompatibleJSON(t *testing.T) {
	// Pre-profile config files have no `profiles` or `active_profile` keys.
	// Loading + saving must not introduce them and must round-trip cleanly.
	old := `{"user_agent":"test","postal_code":"98112","latitude":47.63,"longitude":-122.28}`
	var c Config
	if err := json.Unmarshal([]byte(old), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Profiles != nil || c.ActiveProfile != "" {
		t.Errorf("legacy config produced profile state: profiles=%v active=%q", c.Profiles, c.ActiveProfile)
	}
	out, err := json.Marshal(&c)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if got := string(out); !jsonOmits(got, `"profiles"`) || !jsonOmits(got, `"active_profile"`) {
		t.Errorf("legacy round-trip should omit profile fields, got %s", got)
	}
}

func jsonOmits(haystack, needle string) bool {
	return !contains(haystack, needle)
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
