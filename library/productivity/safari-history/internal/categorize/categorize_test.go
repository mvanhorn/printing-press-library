package categorize

import "testing"

func TestClassify(t *testing.T) {
	cases := []struct {
		domain string
		bucket string
		prod   string
	}{
		{"github.com", "Coding", "productive"},
		{"docs.python.org", "Coding", "productive"},
		{"m.reddit.com", "Social", "distracting"},
		{"workspace.slack.com", "Comms", "productive"},
		{"google.com", "Search", "neutral"},
		{"claude.com", "AI", "productive"},
		{"unknown-example.invalid", "Other", "neutral"},
	}
	for _, tc := range cases {
		b, p := Classify(tc.domain)
		if b != tc.bucket || p != tc.prod {
			t.Fatalf("Classify(%q)=(%q,%q) want (%q,%q)", tc.domain, b, p, tc.bucket, tc.prod)
		}
	}
}
