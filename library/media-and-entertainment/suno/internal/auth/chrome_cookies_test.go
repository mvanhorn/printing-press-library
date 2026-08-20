package auth

import "testing"

// SunoCookie is the exported shape the captcha package consumes. This test only
// asserts the type/contract exists and is well-formed; reading the real browser
// store is covered by the build-tagged integration test in internal/captcha.
func TestSunoCookie_Shape(t *testing.T) {
	c := SunoCookie{Name: "__client", Value: "abc", Domain: ".suno.com", Path: "/", Secure: true, HTTPOnly: true}
	if c.Name == "" || c.Domain == "" {
		t.Fatal("SunoCookie fields must round-trip")
	}
}
