// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import "testing"

func TestImageTypeForPathRejectsExtensionlessInput(t *testing.T) {
	if _, err := imageTypeForPath("scan"); err == nil {
		t.Fatal("expected extensionless image path to be rejected")
	}
	got, err := imageTypeForPath("scan.PNG")
	if err != nil {
		t.Fatalf("imageTypeForPath returned error: %v", err)
	}
	if got != "PNG" {
		t.Fatalf("image type = %q, want PNG", got)
	}
}

func TestHashBytesDeterministic(t *testing.T) {
	got := hashBytes("x.pdf", []byte("hello"))
	if got.SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("sha256=%s", got.SHA256)
	}
	if got.Size != 5 {
		t.Fatalf("size=%d", got.Size)
	}
}
func TestExtractLiteralText(t *testing.T) {
	b := []byte("BT (Hello)Tj ET BT (World) Tj ET")
	if got := extractLiteralText(b); got != "Hello\nWorld" {
		t.Fatalf("text=%q", got)
	}
}
func TestPrivacyPatternsFindExpectedValues(t *testing.T) {
	text := "email alice@example.com phone +1 555 123 4567 PAN ABCDE1234F"
	if len(privacyPatterns["email"].FindAllString(text, -1)) != 1 {
		t.Fatal("email")
	}
	if len(privacyPatterns["phone"].FindAllString(text, -1)) != 1 {
		t.Fatal("phone")
	}
	if len(privacyPatterns["pan"].FindAllString(text, -1)) != 1 {
		t.Fatal("pan")
	}
}
