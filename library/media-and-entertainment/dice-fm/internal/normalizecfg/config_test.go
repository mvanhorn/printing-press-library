package normalizecfg

import "testing"

func TestParseConfigRoundTrip(t *testing.T) {
	src := `
version: 1
entities:
  ticket_type:
    source: tickets.ticketType.name
    shape: attributes
    attributes: [access_class, sales_stage, entry_window_type, entry_window_time, group_size, comp_flag]
    rules:
      - match: '(?i)\bvip\b'
        set: {access_class: vip}
  genre:
    source: events.genres
    shape: vocab
    vocab: [house, techno, trance]
`
	cfg, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	tt, ok := cfg.Entities["ticket_type"]
	if !ok || tt.Shape != "attributes" || tt.Source != "tickets.ticketType.name" {
		t.Fatalf("ticket_type wrong: %+v", tt)
	}
	if len(tt.Attributes) != 6 || len(tt.Rules) != 1 || tt.Rules[0].Set["access_class"] != "vip" {
		t.Fatalf("ticket_type attrs/rules wrong: %+v", tt)
	}
	if g := cfg.Entities["genre"]; g.Shape != "vocab" || len(g.Vocab) != 3 {
		t.Fatalf("genre wrong: %+v", g)
	}
}

func TestValidateRejectsUnknownShape(t *testing.T) {
	_, err := Parse([]byte("version: 1\nentities:\n  x:\n    source: a.b\n    shape: bogus\n"))
	if err == nil {
		t.Fatal("expected error for unknown shape")
	}
}

// TestStripPatternValidates verifies a valid strip_pattern parses and an
// invalid Go regexp fails Parse validation so a malformed config fails fast.
func TestStripPatternValidates(t *testing.T) {
	good := `version: 1
entities:
  genre:
    source: events.genres
    shape: alias
    strip_pattern: '^[a-z]+:'
`
	cfg, err := Parse([]byte(good))
	if err != nil {
		t.Fatalf("valid strip_pattern should parse: %v", err)
	}
	if got := cfg.Entities["genre"].StripPattern; got != "^[a-z]+:" {
		t.Errorf("strip_pattern = %q, want %q", got, "^[a-z]+:")
	}

	bad := `version: 1
entities:
  genre:
    source: events.genres
    shape: alias
    strip_pattern: '([unclosed'
`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatal("expected error for uncompilable strip_pattern regexp")
	}
}

// TestValidateRuleMatch verifies that a rule's match pattern is validated at
// parse time: a valid pattern parses, an uncompilable pattern fails the load
// (instead of silently disabling itself at classify time), and an empty match
// is rejected.
func TestValidateRuleMatch(t *testing.T) {
	good := `version: 1
entities:
  ticket_type:
    source: tickets.ticketType.name
    shape: attributes
    rules:
      - match: '(?i)\bvip\b'
        set: {access_class: vip}
`
	if _, err := Parse([]byte(good)); err != nil {
		t.Fatalf("valid rule match should parse: %v", err)
	}

	badRegexp := `version: 1
entities:
  ticket_type:
    source: tickets.ticketType.name
    shape: attributes
    rules:
      - match: '([unclosed'
        set: {access_class: vip}
`
	if _, err := Parse([]byte(badRegexp)); err == nil {
		t.Fatal("expected error for uncompilable rule match")
	}

	emptyMatch := `version: 1
entities:
  ticket_type:
    source: tickets.ticketType.name
    shape: attributes
    rules:
      - set: {access_class: vip}
`
	if _, err := Parse([]byte(emptyMatch)); err == nil {
		t.Fatal("expected error for empty rule match")
	}
}

func mustParse(t *testing.T, b []byte) *Config {
	t.Helper()
	c, err := Parse(b)
	if err != nil {
		t.Fatalf("mustParse: %v", err)
	}
	return c
}

func TestLayeredLoadMerges(t *testing.T) {
	starter := []byte("version: 1\nentities:\n  genre:\n    source: events.genres\n    shape: vocab\n    vocab: [house]\n")
	operator := []byte("version: 1\nentities:\n  genre:\n    source: events.genres\n    shape: vocab\n    vocab: [house, techno]\n  artist:\n    source: events.artists.name\n    shape: alias\n")
	cfg, err := Merge(mustParse(t, starter), mustParse(t, operator))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if len(cfg.Entities["genre"].Vocab) != 2 { // operator wins
		t.Errorf("genre vocab = %v, want operator's 2", cfg.Entities["genre"].Vocab)
	}
	if _, ok := cfg.Entities["artist"]; !ok { // operator-added entity present
		t.Error("artist (operator-only) missing")
	}
}
