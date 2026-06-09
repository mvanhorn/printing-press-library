// Copyright 2026 Brian Wishan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestParseCurlSessionBash(t *testing.T) {
	raw := `curl 'https://www.amazon.ca/your-orders/orders?timeFilter=months-3' \
  -H 'accept: text/html' \
  -H 'cookie: session-id=143-8440915-8787300; ubid-acbca=135-1; at-acbca=Atza|abc' \
  -H 'user-agent: Mozilla/5.0' \
  --compressed`
	cookies, origin, err := parseCurlSession(raw)
	if err != nil {
		t.Fatalf("parseCurlSession: %v", err)
	}
	if want := "session-id=143-8440915-8787300; ubid-acbca=135-1; at-acbca=Atza|abc"; cookies != want {
		t.Errorf("cookies = %q, want %q", cookies, want)
	}
	if origin != "https://www.amazon.ca" {
		t.Errorf("origin = %q, want https://www.amazon.ca", origin)
	}
}

func TestParseCurlSessionWindowsCmd(t *testing.T) {
	// Windows "Copy as cURL (cmd)" uses double quotes and ^ line continuations.
	raw := "curl \"https://www.amazon.com/your-orders/orders\" ^\n" +
		"  -H \"accept: text/html\" ^\n" +
		"  -H \"cookie: session-id=111-2; x=\\\"q\\\"\" ^\n" +
		"  --compressed"
	cookies, origin, err := parseCurlSession(raw)
	if err != nil {
		t.Fatalf("parseCurlSession: %v", err)
	}
	if want := `session-id=111-2; x="q"`; cookies != want {
		t.Errorf("cookies = %q, want %q", cookies, want)
	}
	if origin != "https://www.amazon.com" {
		t.Errorf("origin = %q, want https://www.amazon.com", origin)
	}
}

func TestParseCurlSessionDashB(t *testing.T) {
	raw := `curl https://www.amazon.ca/your-orders/orders -b 'session-id=143-1; ubid-acbca=9'`
	cookies, origin, err := parseCurlSession(raw)
	if err != nil {
		t.Fatalf("parseCurlSession: %v", err)
	}
	if cookies != "session-id=143-1; ubid-acbca=9" {
		t.Errorf("cookies = %q", cookies)
	}
	if origin != "https://www.amazon.ca" {
		t.Errorf("origin = %q", origin)
	}
}

func TestParseCurlSessionNoCookie(t *testing.T) {
	raw := `curl 'https://www.amazon.ca/your-orders/orders' -H 'accept: text/html'`
	if _, _, err := parseCurlSession(raw); err == nil {
		t.Fatal("expected error when no Cookie header is present")
	}
}
