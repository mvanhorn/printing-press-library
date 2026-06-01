// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package client

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestSunoDynamicHeadersBrowserTokenShape(t *testing.T) {
	h := SunoDynamicHeaders("dev-123")

	if h["Device-Id"] != "dev-123" {
		t.Fatalf("Device-Id = %q, want dev-123", h["Device-Id"])
	}
	if h["Origin"] != "https://suno.com" {
		t.Fatalf("Origin = %q", h["Origin"])
	}
	if h["Referer"] != "https://suno.com/" {
		t.Fatalf("Referer = %q", h["Referer"])
	}

	// Browser-Token must be {"token":"<base64>"} decoding to {"timestamp":<number>}.
	var outer struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(h["Browser-Token"]), &outer); err != nil {
		t.Fatalf("Browser-Token is not JSON: %v (%s)", err, h["Browser-Token"])
	}
	if outer.Token == "" {
		t.Fatal("Browser-Token.token is empty")
	}
	decoded, err := base64.StdEncoding.DecodeString(outer.Token)
	if err != nil {
		t.Fatalf("token is not standard base64: %v", err)
	}
	var inner struct {
		Timestamp int64 `json:"timestamp"`
	}
	if err := json.Unmarshal(decoded, &inner); err != nil {
		t.Fatalf("decoded token is not {\"timestamp\":...}: %v (%s)", err, decoded)
	}
	if inner.Timestamp <= 0 {
		t.Fatalf("timestamp = %d, want positive ms-since-epoch", inner.Timestamp)
	}

	// Zero-UUID fallback when deviceID is empty.
	if SunoDynamicHeaders("")["Device-Id"] != "00000000-0000-0000-0000-000000000000" {
		t.Fatal("empty deviceID should fall back to zero UUID")
	}
}
