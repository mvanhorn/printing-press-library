// Copyright 2026 horknfbr. Licensed under Apache-2.0. See LICENSE.

package auth

import (
	"strings"
	"testing"
)

func TestStudioCookieHeader(t *testing.T) {
	in := []SunoCookie{
		{Name: "_ga", Value: "g1", Domain: ".suno.com"},
		{Name: "suno_device_id", Value: "d1", Domain: "suno.com"},
		{Name: "edge", Value: "e1", Domain: "studio-api-prod.suno.com"},
		{Name: "__client", Value: "secret", Domain: "auth.suno.com"},             // other subdomain -> drop
		{Name: "hmt_id", Value: "h1", Domain: "hcaptcha-endpoint-prod.suno.com"}, // drop
		{Name: "", Value: "x", Domain: ".suno.com"},                              // nameless -> skip
	}
	got := studioCookieHeader(in)
	want := "_ga=g1; suno_device_id=d1; edge=e1"
	if got != want {
		t.Fatalf("studioCookieHeader =\n  %q\nwant\n  %q", got, want)
	}
}

func TestStudioCookieHeaderEmpty(t *testing.T) {
	if got := studioCookieHeader(nil); got != "" {
		t.Fatalf("empty input -> %q, want \"\"", got)
	}
	if got := studioCookieHeader([]SunoCookie{{Name: "__client", Domain: "auth.suno.com"}}); got != "" {
		t.Fatalf("only-other-subdomain -> %q, want \"\"", got)
	}
}

func TestStudioCookieHeaderDropsSession(t *testing.T) {
	in := []SunoCookie{
		{Name: "suno_auth", Value: "a", Domain: "suno.com"},
		{Name: "__session", Value: "rot", Domain: "suno.com"},
		{Name: "__session_Ab12", Value: "rot2", Domain: "suno.com"},
		{Name: "__client_uat", Value: "u", Domain: ".suno.com"},
	}
	got := studioCookieHeader(in)
	if strings.Contains(got, "__session") {
		t.Fatalf("studioCookieHeader kept a __session cookie: %q", got)
	}
	if !strings.Contains(got, "suno_auth=a") || !strings.Contains(got, "__client_uat=u") {
		t.Fatalf("studioCookieHeader dropped a long-lived cookie: %q", got)
	}
}
