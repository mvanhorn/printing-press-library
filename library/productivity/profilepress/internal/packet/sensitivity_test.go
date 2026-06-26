package packet

import "testing"

func TestSensitiveChanges(t *testing.T) {
	changes := []Change{{Section: "Headline"}, {Section: "about"}, {Section: "role-description"}, {Section: "company"}}
	got := SensitiveChanges(changes)
	if len(got) != 3 {
		t.Fatalf("sensitive count=%d", len(got))
	}
}

func TestNonSensitive(t *testing.T) {
	if IsSensitiveSection("about") {
		t.Fatal("about should not be sensitive by default")
	}
}
