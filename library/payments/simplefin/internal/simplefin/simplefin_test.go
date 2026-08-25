// Copyright 2026 Todd Dailey and contributors. Licensed under Apache-2.0. See LICENSE.

package simplefin

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestParseAmount(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{
		{"-33293.43", -33293.43, true},
		{"100.23", 100.23, true},
		{"0.00", 0, true},
		{"", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseAmount(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseAmount(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestNormalizePayee(t *testing.T) {
	cases := []struct {
		txn  Transaction
		want string
	}{
		{Transaction{Payee: "John's Fishin Shack"}, "john s fishin shack"},
		{Transaction{Description: "AMAZON.COM*A12B3 SEATTLE"}, "amazon com ab seattle"},
		{Transaction{Payee: "", Description: "Netflix #4821"}, "netflix"},
	}
	for _, c := range cases {
		if got := NormalizePayee(c.txn); got != c.want {
			t.Errorf("NormalizePayee(%+v) = %q want %q", c.txn, got, c.want)
		}
	}
}

func TestContentHashMirroredDuplicatesCollide(t *testing.T) {
	// Same charge mirrored into two accounts with different IDs must collide.
	posted := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC).Unix()
	a := Transaction{ID: "txn-A", Amount: "-3157.00", Posted: posted, Description: "Wire Transfer", Payee: "ACME"}
	b := Transaction{ID: "txn-B", Amount: "-3157.00", Posted: posted + 3600, Description: "Wire Transfer", Payee: "ACME"}
	if ContentHash(a) != ContentHash(b) {
		t.Errorf("expected mirrored duplicates to share a content hash: %s vs %s", ContentHash(a), ContentHash(b))
	}
	// A genuinely different amount must NOT collide.
	c := Transaction{ID: "txn-C", Amount: "-3158.00", Posted: posted, Description: "Wire Transfer", Payee: "ACME"}
	if ContentHash(a) == ContentHash(c) {
		t.Errorf("different amounts must not share a content hash")
	}
}

func TestParseDate(t *testing.T) {
	now := time.Date(2026, 6, 29, 0, 0, 0, 0, time.UTC)
	// Relative duration.
	got, err := ParseDate("90d", now)
	if err != nil {
		t.Fatalf("ParseDate(90d) error: %v", err)
	}
	if d := now.Sub(got); d < 89*24*time.Hour || d > 91*24*time.Hour {
		t.Errorf("ParseDate(90d) delta = %v, want ~90d", d)
	}
	// Absolute date.
	got, err = ParseDate("2026-01-15", now)
	if err != nil || got.Year() != 2026 || got.Month() != 1 || got.Day() != 15 {
		t.Errorf("ParseDate(2026-01-15) = %v, %v", got, err)
	}
	// Unix epoch.
	epoch := now.Unix()
	got, err = ParseDate(EpochParam(now), now)
	if err != nil || got.Unix() != epoch {
		t.Errorf("ParseDate(epoch) = %v, %v want %d", got, err, epoch)
	}
	// Garbage.
	if _, err := ParseDate("not-a-date", now); err == nil {
		t.Errorf("ParseDate(not-a-date) should error")
	}
}

func TestParseAccountSet(t *testing.T) {
	raw := `{"errlist":[],"connections":[{"conn_id":"C1","name":"Bank"}],"accounts":[{"id":"a1","name":"Checking","currency":"USD","balance":"100.23","balance-date":978366153,"transactions":[{"id":"t1","posted":793090572,"amount":"-33.43","description":"Bait"}],"holdings":[{"id":"h1","symbol":"AAPL","market_value":"105884.8","cost_basis":"55.00","shares":"550"}]}]}`
	set, err := ParseAccountSet([]byte(raw))
	if err != nil {
		t.Fatalf("ParseAccountSet error: %v", err)
	}
	if len(set.Accounts) != 1 || set.Accounts[0].ID != "a1" {
		t.Fatalf("expected 1 account a1, got %+v", set.Accounts)
	}
	if len(set.Accounts[0].Transactions) != 1 || set.Accounts[0].Transactions[0].Amount != "-33.43" {
		t.Errorf("transaction not parsed: %+v", set.Accounts[0].Transactions)
	}
	if len(set.Accounts[0].Holdings) != 1 || set.Accounts[0].Holdings[0].Symbol != "AAPL" {
		t.Errorf("holding not parsed: %+v", set.Accounts[0].Holdings)
	}
}

func TestDecodeSetupTokenRejectsNonHTTPS(t *testing.T) {
	// A base64 of an http:// URL must be rejected.
	bad := base64.StdEncoding.EncodeToString([]byte("http://evil.example.com/claim/x"))
	if _, err := decodeSetupToken(bad); err == nil {
		t.Errorf("decodeSetupToken should reject non-https URL")
	}
	good := base64.StdEncoding.EncodeToString([]byte("https://bridge.simplefin.org/simplefin/claim/demo"))
	got, err := decodeSetupToken(good)
	if err != nil || got != "https://bridge.simplefin.org/simplefin/claim/demo" {
		t.Errorf("decodeSetupToken(good) = %q, %v", got, err)
	}
}
