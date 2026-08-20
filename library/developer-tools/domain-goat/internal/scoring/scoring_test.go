package scoring

import "testing"

func TestComputeBasic(t *testing.T) {
	cases := []struct {
		fqdn         string
		minTotal     int
		wantHack     bool
		wantDictWord bool
	}{
		{"kindred.io", 60, false, true},
		{"a.com", 30, false, false},
		{"this-is-a-very-long-domain-name.io", 0, false, false},
		{"del.icio.us", 30, true, false},
		{"abba.ai", 40, false, false},
	}
	for _, c := range cases {
		s := Compute(c.fqdn)
		if s.Total < c.minTotal {
			t.Errorf("Compute(%s).Total=%d, want >= %d", c.fqdn, s.Total, c.minTotal)
		}
		if s.HackStyle != c.wantHack {
			t.Errorf("Compute(%s).HackStyle=%v, want %v", c.fqdn, s.HackStyle, c.wantHack)
		}
		if s.DictWord != c.wantDictWord {
			t.Errorf("Compute(%s).DictWord=%v, want %v", c.fqdn, s.DictWord, c.wantDictWord)
		}
	}
}

func TestLengthQualityScore(t *testing.T) {
	if lengthQualityScore(5) <= lengthQualityScore(15) {
		t.Errorf("expected 5-char label to outscore 15-char")
	}
}

func TestPalindrome(t *testing.T) {
	s := Compute("aabaa.com")
	if !s.Palindrome {
		t.Errorf("expected palindrome=true for aabaa")
	}
	s2 := Compute("hello.com")
	if s2.Palindrome {
		t.Errorf("expected palindrome=false for hello")
	}
}

func TestCountSyllables(t *testing.T) {
	if countSyllables("hello") != 2 {
		t.Errorf("hello=%d, want 2", countSyllables("hello"))
	}
	if countSyllables("rhythm") < 1 {
		t.Errorf("rhythm=%d, want >= 1", countSyllables("rhythm"))
	}
}
