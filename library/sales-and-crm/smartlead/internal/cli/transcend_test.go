// Copyright 2026 bossriceshark and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestDomainOf(t *testing.T) {
	// Fixtures use dotless hosts on purpose: domainOf splits on the last '@'
	// and lowercases the remainder — the dot in a real domain is irrelevant
	// to its logic, so dotless hosts exercise the same code path.
	cases := []struct {
		in, want string
	}{
		{"User@Mailbox", "mailbox"},
		{"  Lead@HostName ", "hostname"},
		{"first@middle@Final", "final"},
		{"noatsign", ""},
		{"trailing@", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := domainOf(c.in); got != c.want {
			t.Errorf("domainOf(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseSLTime(t *testing.T) {
	ok := []string{
		"2026-05-15T22:27:10.000Z",
		"2026-05-15T22:27:10Z",
		"2026-05-15 22:27:10",
		"2026-05-15",
	}
	for _, s := range ok {
		if _, parsed := parseSLTime(s); !parsed {
			t.Errorf("parseSLTime(%q) failed, expected success", s)
		}
	}
	bad := []string{"", "null", "not-a-date", "15/05/2026"}
	for _, s := range bad {
		if _, parsed := parseSLTime(s); parsed {
			t.Errorf("parseSLTime(%q) succeeded, expected failure", s)
		}
	}
}

func TestWarmupInboxRate(t *testing.T) {
	cases := []struct {
		inbox, spam int
		want        float64
	}{
		{193, 75, float64(193) / float64(268)},
		{100, 0, 1.0},
		{0, 0, 0},
		{0, 50, 0},
	}
	for _, c := range cases {
		if got := warmupInboxRate(c.inbox, c.spam); got != c.want {
			t.Errorf("warmupInboxRate(%d,%d) = %v, want %v", c.inbox, c.spam, got, c.want)
		}
	}
}

func TestRate(t *testing.T) {
	cases := []struct {
		num, den int
		want     float64
	}{
		{1, 4, 0.25},
		{1, 3, 0.3333},
		{0, 0, 0},
		{5, 0, 0},
		{3, 3, 1.0},
	}
	for _, c := range cases {
		if got := rate(c.num, c.den); got != c.want {
			t.Errorf("rate(%d,%d) = %v, want %v", c.num, c.den, got, c.want)
		}
	}
}

func TestTruthy(t *testing.T) {
	for _, s := range []string{"1", "true", "TRUE", " yes ", "t"} {
		if !truthy(s) {
			t.Errorf("truthy(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"0", "false", "", "no", "null"} {
		if truthy(s) {
			t.Errorf("truthy(%q) = true, want false", s)
		}
	}
}

func TestAsInt(t *testing.T) {
	cases := []struct {
		in   any
		want int
	}{
		{"193", 193},
		{float64(42), 42},
		{int(7), 7},
		{"  12 ", 12},
		{"notanumber", 0},
		{nil, 0},
	}
	for _, c := range cases {
		if got := asInt(c.in); got != c.want {
			t.Errorf("asInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestAsBool(t *testing.T) {
	if !asBool(true) || !asBool("true") || !asBool(float64(1)) {
		t.Error("asBool truthy cases failed")
	}
	if asBool(false) || asBool("false") || asBool(float64(0)) || asBool(nil) {
		t.Error("asBool falsy cases failed")
	}
}

func TestRound4(t *testing.T) {
	cases := []struct {
		in, want float64
	}{
		{0.123456, 0.1235},
		{-0.123456, -0.1235},
		{0, 0},
		{0.5, 0.5},
	}
	for _, c := range cases {
		if got := round4(c.in); got != c.want {
			t.Errorf("round4(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}
