package drudge

import "testing"

func TestOutboundDomain(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want string
	}{
		{"https with subdomain", "https://www.nytimes.com/2026/05/21/world/asia/china-tariffs.html", "www.nytimes.com"},
		{"http upper-case host", "http://WWW.WSJ.COM/articles/foo", "www.wsj.com"},
		{"trailing whitespace", "  https://apnews.com/article/iran-policy\n", "apnews.com"},
		{"relative href", "/page", ""},
		{"empty input", "", ""},
		{"protocol-only", "https://", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := outboundDomain(tc.url)
			if got != tc.want {
				t.Fatalf("outboundDomain(%q) = %q, want %q", tc.url, got, tc.want)
			}
		})
	}
}

func TestRSSSlot(t *testing.T) {
	cases := []struct {
		desc      string
		wantSlot  Slot
		wantIndex int
	}{
		{"(Main headline, 1st story, byline)", SlotSplash, 0},
		{"(First column, 3rd story, more)", SlotColumn1, 2},
		{"(Second column, 1st story, x)", SlotColumn2, 0},
		{"(Top left, 4th story, foo)", SlotTopLeft, 3},
		{"no position label here", SlotColumn2, 0},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			slot, idx := rssSlot(tc.desc)
			if slot != tc.wantSlot || idx != tc.wantIndex {
				t.Fatalf("rssSlot(%q) = (%v, %d), want (%v, %d)", tc.desc, slot, idx, tc.wantSlot, tc.wantIndex)
			}
		})
	}
}

func TestParseHTMLBasicZones(t *testing.T) {
	body := []byte(`
<html>
<body>
<!MAIN HEADLINE>
<a href="https://www.nytimes.com/article-one"><font color="red">SPLASH STORY ONE</font></a>
<!MAIN HEADLINE END HERE>
<!TOP LEFT STARTS HERE>
<a href="https://apnews.com/topleft-a">TOPLEFT A</a>
<!TOP LEFT HEADLINES END HERE>
<!LINKS FIRST COLUMN>
<a href="https://www.wsj.com/c1-a">COLUMN ONE A</a>
<a href="https://www.washingtonpost.com/c1-b">COLUMN ONE B</a>
<!LINKS SECOND COLUMN>
<a href="https://www.reuters.com/c2-a">COLUMN TWO A</a>
</body>
</html>`)
	stories, err := ParseHTML(body)
	if err != nil {
		t.Fatalf("ParseHTML returned error: %v", err)
	}
	if len(stories) == 0 {
		t.Fatalf("ParseHTML returned 0 stories")
	}
	// Find the splash and verify its outbound domain is set.
	var sawSplash, sawTopLeft, sawColumn1, sawColumn2 bool
	for _, s := range stories {
		switch s.Slot {
		case SlotSplash:
			sawSplash = true
			if s.OutboundDomain != "www.nytimes.com" {
				t.Errorf("splash outbound = %q, want www.nytimes.com", s.OutboundDomain)
			}
			if !s.IsRed {
				t.Errorf("splash should be red")
			}
		case SlotTopLeft:
			sawTopLeft = true
		case SlotColumn1:
			sawColumn1 = true
		case SlotColumn2:
			sawColumn2 = true
		}
	}
	if !sawSplash || !sawTopLeft || !sawColumn1 || !sawColumn2 {
		t.Errorf("missing zone: splash=%v topleft=%v c1=%v c2=%v", sawSplash, sawTopLeft, sawColumn1, sawColumn2)
	}
}
