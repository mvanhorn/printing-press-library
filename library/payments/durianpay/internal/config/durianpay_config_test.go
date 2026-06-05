// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored: regression test for Durianpay's blank-password Basic auth.
package config

import (
	"encoding/base64"
	"testing"
)

func TestAuthHeaderBlankPassword(t *testing.T) {
	c := &Config{DurianpayApiKey: "dp_test_key"}
	want := "Basic " + base64.StdEncoding.EncodeToString([]byte("dp_test_key:"))
	if got := c.AuthHeader(); got != want {
		t.Errorf("AuthHeader with blank password = %q, want %q", got, want)
	}
	empty := &Config{}
	if got := empty.AuthHeader(); got != "" {
		t.Errorf("AuthHeader with no key = %q, want empty", got)
	}
}
