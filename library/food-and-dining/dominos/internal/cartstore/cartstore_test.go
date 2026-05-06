package cartstore

import (
	"errors"
	"os"
	"testing"
)

// withTempHome points cartstore at a fresh temp directory by overriding
// HOME for the duration of the test. cartstore resolves all paths
// through os.UserHomeDir, so this is the supported isolation surface.
func withTempHome(t *testing.T) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
}

func TestActiveCartRoundTrip(t *testing.T) {
	withTempHome(t)

	if _, err := LoadActive(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound on empty store, got %v", err)
	}

	cart := &Cart{
		StoreID: "1234",
		Service: "Delivery",
		Address: "1 Main St",
		Items: []CartItem{
			{Code: "12SCREEN", Qty: 1, Toppings: []string{"P:1/1:1"}},
		},
	}
	if err := SaveActive(cart); err != nil {
		t.Fatalf("SaveActive: %v", err)
	}
	got, err := LoadActive()
	if err != nil {
		t.Fatalf("LoadActive: %v", err)
	}
	if got.StoreID != "1234" || got.Service != "Delivery" || len(got.Items) != 1 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}

	if err := ClearActive(); err != nil {
		t.Fatalf("ClearActive: %v", err)
	}
	if _, err := LoadActive(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after Clear, got %v", err)
	}
	// Clearing again is a no-op, not an error.
	if err := ClearActive(); err != nil {
		t.Fatalf("ClearActive on missing cart: %v", err)
	}
}

func TestTemplateRoundTripAndList(t *testing.T) {
	withTempHome(t)

	if names, err := ListTemplates(); err != nil || len(names) != 0 {
		t.Fatalf("ListTemplates on empty store: names=%v err=%v", names, err)
	}

	t1 := &Template{StoreID: "1", Service: "Carryout"}
	t2 := &Template{StoreID: "2", Service: "Delivery"}
	if err := SaveTemplate("friday", t1); err != nil {
		t.Fatalf("SaveTemplate friday: %v", err)
	}
	if err := SaveTemplate("birthday", t2); err != nil {
		t.Fatalf("SaveTemplate birthday: %v", err)
	}

	names, err := ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(names) != 2 || names[0] != "birthday" || names[1] != "friday" {
		t.Fatalf("ListTemplates returned unexpected order: %v", names)
	}

	got, err := LoadTemplate("friday")
	if err != nil {
		t.Fatalf("LoadTemplate friday: %v", err)
	}
	if got.Name != "friday" || got.StoreID != "1" {
		t.Fatalf("LoadTemplate mismatch: %+v", got)
	}

	if err := DeleteTemplate("friday"); err != nil {
		t.Fatalf("DeleteTemplate: %v", err)
	}
	if _, err := LoadTemplate("friday"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if err := DeleteTemplate("friday"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("DeleteTemplate of missing: expected ErrNotFound, got %v", err)
	}
}

func TestValidateTemplateName(t *testing.T) {
	cases := []struct {
		name    string
		wantErr bool
	}{
		{"friday", false},
		{"my-template_v2", false},
		{"", true},
		{"foo/bar", true},
		{`foo\bar`, true},
		{".secret", true},
	}
	for _, c := range cases {
		err := validateTemplateName(c.name)
		if (err != nil) != c.wantErr {
			t.Errorf("validateTemplateName(%q): err=%v wantErr=%v", c.name, err, c.wantErr)
		}
	}
}

func TestSaveActiveRejectsNil(t *testing.T) {
	withTempHome(t)
	if err := SaveActive(nil); err == nil {
		t.Fatalf("expected error on nil cart")
	}
}

func TestSaveTemplateRejectsNil(t *testing.T) {
	withTempHome(t)
	if err := SaveTemplate("name", nil); err == nil {
		t.Fatalf("expected error on nil template")
	}
}

func TestListTemplatesIgnoresNonTomlFiles(t *testing.T) {
	withTempHome(t)
	dir, err := templatesDir()
	if err != nil {
		t.Fatalf("templatesDir: %v", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// Drop a junk file alongside a real template.
	if err := os.WriteFile(dir+"/notes.txt", []byte("ignore me"), 0o644); err != nil {
		t.Fatalf("write junk: %v", err)
	}
	if err := SaveTemplate("real", &Template{StoreID: "X"}); err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	names, err := ListTemplates()
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}
	if len(names) != 1 || names[0] != "real" {
		t.Fatalf("expected [real], got %v", names)
	}
}
