package roadside

import "testing"

const sampleAttrList = `<ul class="attrlist">
<li id="attr-2055-li">
<img border="0" style="float:left" src="/map/image/pin.png" alt="1"><a class="attractname" href="javascript:openInfo(2055);">Swampy: World&#039;s Largest Alligator</a><div class="attritem" id="attr-2055-addr">
<div class="street">26205 E. Colonial Drive</div>
<div class="cityState">Christmas, FL</div>
<div class="location">
        (&lt;1 mi. away)
      </div>
<a class="mapmorelink" href="/tip/2055">More...</a>
</div>
<br clear="left">
</li>
<li id="attr-40689-li">
<img border="0" src="x" alt="2"><a class="attractname" href="javascript:openInfo(40689);">Statue of the Cannon Lady</a><div class="attritem" id="attr-40689-addr">
<div class="street">N. Congress Ave.</div>
<div class="cityState">Austin, TX</div>
<div class="location">
        (3 mi. away)
      </div>
<a class="mapmorelink" href="/story/40689">More...</a>
</div>
</li>
</ul>`

func TestParseAttrList(t *testing.T) {
	got := ParseAttrList(sampleAttrList)
	if len(got) != 2 {
		t.Fatalf("expected 2 attractions, got %d", len(got))
	}
	a := got[0]
	if a.ID != "2055" {
		t.Errorf("id: want 2055, got %q", a.ID)
	}
	if a.Name != "Swampy: World's Largest Alligator" {
		t.Errorf("name (entities should decode): got %q", a.Name)
	}
	if a.City != "Christmas" || a.State != "FL" {
		t.Errorf("city/state: got %q / %q", a.City, a.State)
	}
	if a.DistanceMi != 0.5 {
		t.Errorf("distance_mi for '<1': want 0.5, got %v", a.DistanceMi)
	}
	if a.DetailPath != "/tip/2055" {
		t.Errorf("detail_path: got %q", a.DetailPath)
	}
	if a.SourceURL != "https://www.roadsideamerica.com/tip/2055" {
		t.Errorf("source_url: got %q", a.SourceURL)
	}
	if !contains(a.Categories, "biggest") || !contains(a.Categories, "animals") {
		t.Errorf("categories: want biggest+animals, got %v", a.Categories)
	}
	b := got[1]
	if b.ID != "40689" || b.City != "Austin" || b.State != "TX" {
		t.Errorf("second item: got id=%q city=%q state=%q", b.ID, b.City, b.State)
	}
	if b.DistanceMi != 3 {
		t.Errorf("distance_mi: want 3, got %v", b.DistanceMi)
	}
	if b.DetailPath != "/story/40689" {
		t.Errorf("detail_path (story): got %q", b.DetailPath)
	}
}

const sampleDetail = `<html><head>
<meta property="og:title" content="Christmas, FL - Swampy: World's Largest Alligator"/>
<meta property="og:description" content="Visit reports, news, maps, directions and info on Swampy: World&#039;s Largest Alligator in Christmas, Florida."/>
<meta property="og:url" content="https://www.roadsideamerica.com/tip/2055"/>
<meta property="og:image" content="https://www.roadsideamerica.com/x.jpg"/>
<title>Christmas, FL - Swampy: World's Largest Alligator</title>
</head><body>
<dl>
<dt>Address:</dt><dd><div class="mapIcon"><a href="/map/2055"><img src="x"/></a></div></dd><dd><a href="/map/2055">26205 E. Colonial Drive, Christmas, FL</a></dd><dt>Directions:</dt><dd>Jungle Adventures. I-95 exit 215.</dd>
</dl>
<p>A 200-foot-long alligator-shaped building named "Swampy" serves as the entrance to a Florida gator attraction.</p>
<p>Reports and tips from RoadsideAmerica.com visitors.</p>
</body></html>`

func TestParseDetail(t *testing.T) {
	d := ParseDetail("2055", "/tip/2055", sampleDetail)
	if d.Name != "Swampy: World's Largest Alligator" {
		t.Errorf("name: got %q", d.Name)
	}
	if d.City != "Christmas" || d.State != "FL" {
		t.Errorf("city/state: got %q / %q", d.City, d.State)
	}
	if d.Street == "" || !containsStr(d.Street, "Colonial Drive") {
		t.Errorf("street: got %q", d.Street)
	}
	if !containsStr(d.Directions, "Jungle Adventures") {
		t.Errorf("directions: got %q", d.Directions)
	}
	if !containsStr(d.Summary, "Visit reports") {
		t.Errorf("summary: got %q", d.Summary)
	}
	if !containsStr(d.Writeup, "200-foot-long") {
		t.Errorf("writeup should be the editorial paragraph, got %q", d.Writeup)
	}
	if d.SourceURL != "https://www.roadsideamerica.com/tip/2055" {
		t.Errorf("source_url: got %q", d.SourceURL)
	}
}

func TestSplitCityState(t *testing.T) {
	cases := []struct{ in, city, state string }{
		{"Christmas, FL", "Christmas", "FL"},
		{"Austin, TX", "Austin", "TX"},
		{"Truth or Consequences, NM", "Truth or Consequences", "NM"},
		{"Nowhere", "Nowhere", ""},
	}
	for _, c := range cases {
		city, state := SplitCityState(c.in)
		if city != c.city || state != c.state {
			t.Errorf("SplitCityState(%q) = %q/%q, want %q/%q", c.in, city, state, c.city, c.state)
		}
	}
}

func TestParseDistanceMiles(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"<1 mi. away", 0.5},
		{"3 mi. away", 3},
		{"12.4 mi. away", 12.4},
		{"", 0},
		{"nearby", 0},
	}
	for _, c := range cases {
		if got := ParseDistanceMiles(c.in); got != c.want {
			t.Errorf("ParseDistanceMiles(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestNormalizeDistanceLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"~26 mi. away", "~26 mi. away"},
		{"<1 mi. away", "<1 mi. away"},
		{"3 mi. away", "3 mi. away"},
		// The "Location Approximate" blob carries a leading prefix and an
		// embedded newline; normalize to just the distance phrase.
		{"- Location Approximate - \n        (~26 mi. away", "~26 mi. away"},
		{"- Location Approximate -\n(~1 mi. away", "~1 mi. away"},
		// No distance phrase: collapse whitespace, leave content otherwise.
		{"  nearby  ", "nearby"},
	}
	for _, c := range cases {
		if got := normalizeDistanceLabel(c.in); got != c.want {
			t.Errorf("normalizeDistanceLabel(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func contains(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func containsStr(haystack, needle string) bool {
	return len(haystack) >= len(needle) && indexOf(haystack, needle) >= 0
}

func indexOf(h, n string) int {
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
