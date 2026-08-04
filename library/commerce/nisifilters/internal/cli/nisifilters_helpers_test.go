// Copyright 2026 chiotas and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored tests for the nisifilters-pp-cli novel-command helpers.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFgStripHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "hello world", "hello world"},
		{"tags", "<p>hello <strong>world</strong></p>", "hello world"},
		{"entities", "Caf&eacute; &amp; tea &#8211; ok", "Café & tea – ok"},
		{"script dropped", "a<script>var x=1;</script>b", "ab"},
		{"breaks become newlines", "line1<br>line2", "line1\nline2"},
		{"empty", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fgStripHTML(tt.in); got != tt.want {
				t.Fatalf("fgStripHTML(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFgWooPrice(t *testing.T) {
	tests := []struct {
		name      string
		prices    string
		wantDisp  string
		wantValue float64
		wantOK    bool
	}{
		{"eur", `{"price":"9000","currency_minor_unit":2,"currency_code":"EUR"}`, "90.00 EUR", 90, true},
		{"calendar", `{"price":"2800","currency_minor_unit":2,"currency_code":"EUR"}`, "28.00 EUR", 28, true},
		{"no minor unit", `{"price":"500","currency_code":"USD"}`, "500.00 USD", 500, true},
		{"missing price", `{"currency_code":"EUR"}`, "", 0, false},
		{"empty", `{}`, "", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			disp, val, ok := fgWooPrice(json.RawMessage(tt.prices))
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && (disp != tt.wantDisp || val != tt.wantValue) {
				t.Fatalf("fgWooPrice = (%q, %v), want (%q, %v)", disp, val, tt.wantDisp, tt.wantValue)
			}
		})
	}
}

func TestFgRenderedAndTitle(t *testing.T) {
	obj := fgDecode(json.RawMessage(`{"title":{"rendered":"My &amp; Post"},"name":"ignored"}`))
	if got := fgPlainTitle(obj); got != "My & Post" {
		t.Fatalf("fgPlainTitle = %q, want %q", got, "My & Post")
	}
	// WooCommerce flat name fallback.
	prod := fgDecode(json.RawMessage(`{"name":"Print &#8211; Norway"}`))
	if got := fgPlainTitle(prod); got != "Print – Norway" {
		t.Fatalf("fgPlainTitle(product) = %q, want %q", got, "Print – Norway")
	}
}

func TestFgParseWPTime(t *testing.T) {
	got, err := fgParseWPTime("2025-06-05T17:19:37", time.UTC)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Year() != 2025 || got.Month() != time.June || got.Day() != 5 {
		t.Fatalf("parsed wrong date: %v", got)
	}
	if _, err := fgParseWPTime("not-a-date", time.UTC); err == nil {
		t.Fatal("expected error for malformed timestamp")
	}
}

func TestFgMediaSizes(t *testing.T) {
	media := fgDecode(json.RawMessage(`{
		"source_url":"https://example.com/full.jpg",
		"media_details":{"sizes":{
			"thumbnail":{"source_url":"https://example.com/thumb.jpg"},
			"full":{"source_url":"https://example.com/full.jpg"}
		}}
	}`))
	if got := fgMediaSourceURL(media); got != "https://example.com/full.jpg" {
		t.Fatalf("source url = %q", got)
	}
	sizes := fgMediaSizes(media)
	if sizes["thumbnail"] != "https://example.com/thumb.jpg" || sizes["full"] != "https://example.com/full.jpg" {
		t.Fatalf("sizes = %v", sizes)
	}
}
