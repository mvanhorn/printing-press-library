// Tests for the slug-collision lint helpers.

package cli

import "testing"

func TestIsLookalike_Pluralization(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"foo", "foos", true},         // pluralization with -s; explicit rule
		{"foo", "foo-", true},         // trailing-dash rule
		{"foo", "fooo", true},         // single insertion (edit distance 1)
		{"foo", "f0o", true},          // single substitution (edit distance 1)
		{"launch", "launchs", true},   // single insertion
		{"launch", "launches", false}, // edit distance 2 — too far for the lint heuristic
		{"foo", "FOO", false},         // case-only differs by 3 characters; flagged separately by case-collision check
		{"foo", "completely-different", false},
		{"foo", "foo", false}, // identical, not a collision
	}
	for _, c := range cases {
		got := isLookalike(c.a, c.b)
		if got != c.want {
			t.Errorf("isLookalike(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestEditDistanceLE(t *testing.T) {
	cases := []struct {
		a, b string
		max  int
		want bool
	}{
		{"abc", "abc", 0, true},
		{"abc", "abd", 1, true},
		{"abc", "ab", 1, true},
		{"abc", "abcd", 1, true},
		{"abc", "xyz", 1, false},
		{"abc", "xyz", 3, true},
		{"hello", "world", 1, false},
	}
	for _, c := range cases {
		if got := editDistanceLE(c.a, c.b, c.max); got != c.want {
			t.Errorf("editDistanceLE(%q,%q,%d) = %v, want %v", c.a, c.b, c.max, got, c.want)
		}
	}
}

func TestAbs(t *testing.T) {
	if abs(-5) != 5 {
		t.Error("abs(-5) should be 5")
	}
	if abs(5) != 5 {
		t.Error("abs(5) should be 5")
	}
	if abs(0) != 0 {
		t.Error("abs(0) should be 0")
	}
}

func TestReservedWords_KnownDangerous(t *testing.T) {
	for _, w := range []string{"admin", "api", "auth", "login", "settings", "qr"} {
		if !reservedWords[w] {
			t.Errorf("reservedWords[%q] should be true", w)
		}
	}
	if reservedWords["compound-engineering"] {
		t.Error("reservedWords[compound-engineering] should not be flagged")
	}
}
