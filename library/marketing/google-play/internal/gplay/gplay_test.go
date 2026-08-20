package gplay

import (
	"encoding/json"
	"testing"
)

func TestNormalizeCollection(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"TOP_FREE", "topselling_free", true},
		{"top_free", "topselling_free", true},
		{"TOP_PAID", "topselling_paid", true},
		{"GROSSING", "topgrossing", true},
		{"TOP_GROSSING", "topgrossing", true},
		{" grossing ", "topgrossing", true},
		{"BOGUS", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeCollection(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeCollection(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizeSort(t *testing.T) {
	cases := []struct {
		in   string
		want int
		ok   bool
	}{
		{"NEWEST", SortNewest, true},
		{"", SortNewest, true},
		{"RATING", SortRating, true},
		{"HELPFULNESS", SortHelpfulness, true},
		{"relevance", SortHelpfulness, true},
		{"nope", 0, false},
	}
	for _, c := range cases {
		got, ok := NormalizeSort(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeSort(%q) = (%d,%v), want (%d,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizePrice(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"all", PriceAll, true},
		{"", PriceAll, true},
		{"free", PriceFree, true},
		{"paid", PricePaid, true},
		{"cheap", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizePrice(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizePrice(%q) = (%q,%v), want (%q,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"Tom &amp; Jerry":           "Tom & Jerry",
		"It&#39;s here":             "It's here",
		"  spaced  ":                "spaced",
		"q\\u003dv":                 "q=v",
		"line one<br>line two":      "line one\nline two",
		"a<br/>b<br />c":            "a\nb\nc",
		"<b>bold</b> and <i>it</i>": "bold and it",
		"plain text":                "plain text",
	}
	for in, want := range cases {
		if got := cleanText(in); got != want {
			t.Errorf("cleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDevIDFromURL(t *testing.T) {
	cases := map[string]string{
		"/store/apps/developer?id=Dream+Games%2C+Ltd.":               "Dream Games, Ltd.",
		"/store/apps/dev?id=5700313618786177705":                     "5700313618786177705",
		"https://play.google.com/store/apps/developer?id=Acme&hl=en": "Acme",
		"no-id-here": "",
	}
	for in, want := range cases {
		if got := devIDFromURL(in); got != want {
			t.Errorf("devIDFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNodeNavigation(t *testing.T) {
	raw := json.RawMessage(`[["a", ["b", 3.5, true]], {"138": [10, 20]}]`)
	n := decode(raw)
	if got := n.path(0, 0).str(); got != "a" {
		t.Errorf("path(0,0).str() = %q, want a", got)
	}
	if got := n.path(0, 1, 1).float(); got != 3.5 {
		t.Errorf("path(0,1,1).float() = %v, want 3.5", got)
	}
	if got := n.path(0, 1, 2).bool(); !got {
		t.Errorf("path(0,1,2).bool() = false, want true")
	}
	// out-of-range is safe (nil node, zero values)
	if got := n.path(99, 5).str(); got != "" {
		t.Errorf("out-of-range path should be empty, got %q", got)
	}
	// negative index
	if got := n.path(0).at(-1).at(0).str(); got != "b" {
		t.Errorf("negative index nav = %q, want b", got)
	}
	// map key access
	if got := n.at(1).key("138").at(1).int(); got != 20 {
		t.Errorf("key(138).at(1).int() = %d, want 20", got)
	}
}

func TestParseBatchExecute(t *testing.T) {
	// Simulate the )]}' length-prefixed framing with a double-encoded payload.
	inner := `[["term1"],["term2"]]`
	innerJSON, _ := json.Marshal(inner)
	body := ")]}'\n\n" + `[["wrb.fr","IJ4APc",` + string(innerJSON) + `,null,null,null,"generic"],["di",7]]` + "\n"
	got, err := parseBatchExecute([]byte(body), "IJ4APc")
	if err != nil {
		t.Fatalf("parseBatchExecute error: %v", err)
	}
	if string(got) != inner {
		t.Errorf("payload = %q, want %q", string(got), inner)
	}
}

func TestParseBatchExecute_NullPayload(t *testing.T) {
	body := ")]}'\n\n" + `[["wrb.fr","oCPfdb",null,null,null,null,"generic"]]` + "\n"
	got, err := parseBatchExecute([]byte(body), "oCPfdb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("null payload should decode to nil, got %q", string(got))
	}
}

func TestParseBatchExecute_RtcChunked(t *testing.T) {
	inner := `[42]`
	innerJSON, _ := json.Marshal(inner)
	chunk := `[["wrb.fr","vyAe2",` + string(innerJSON) + `,null,null,null,"generic"]]`
	// rt=c framing: )]}' then length line then chunk then trailing length.
	body := ")]}'\n\n" + "123\n" + chunk + "\n" + "10\n" + `[["di",5]]` + "\n"
	got, err := parseBatchExecute([]byte(body), "vyAe2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != inner {
		t.Errorf("rt=c payload = %q, want %q", string(got), inner)
	}
}

func TestExtractAFData(t *testing.T) {
	html := `<html><script>AF_initDataCallback({key: 'ds:5', hash: '1', data:[1,2,["x"]], sideChannel: {}});</script>` +
		`<script>AF_initDataCallback({key: 'ds:3', hash: '2', data:["safety"], sideChannel: {}});</script></html>`
	ds, err := extractAFData(html)
	if err != nil {
		t.Fatalf("extractAFData error: %v", err)
	}
	if _, ok := ds["ds:5"]; !ok {
		t.Error("expected ds:5 to be extracted")
	}
	if got := decode(ds["ds:5"]).path(2, 0).str(); got != "x" {
		t.Errorf("ds:5 nav = %q, want x", got)
	}
	if got := decode(ds["ds:3"]).path(0).str(); got != "safety" {
		t.Errorf("ds:3 nav = %q, want safety", got)
	}
}

func TestParseChartApp(t *testing.T) {
	// Minimal chart entry: appId at [0,0,0], title at [0,3], score at [0,4,1].
	raw := json.RawMessage(`[[["com.example.app"],null,null,"Example Game",["4.5",4.53],null,null,null,[null,[[1990000,"USD"]]],[null,"Fun game"]]]`)
	la := parseChartApp(decode(raw))
	if la.AppID != "com.example.app" {
		t.Errorf("appId = %q", la.AppID)
	}
	if la.Title != "Example Game" {
		t.Errorf("title = %q", la.Title)
	}
	if la.Score != 4.53 {
		t.Errorf("score = %v", la.Score)
	}
}
