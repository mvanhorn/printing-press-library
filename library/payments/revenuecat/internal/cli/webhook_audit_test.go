// Copyright 2026 Joseph Alvin Castillo and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestClassifyHost(t *testing.T) {
	cases := []struct {
		name      string
		host      string
		wantStale bool
	}{
		{"localhost", "localhost", true},
		{"loopback ip", "127.0.0.1", true},
		{"loopback ip with port", "127.0.0.1:8080", true},
		{"all interfaces", "0.0.0.0", true},
		{"rfc1918", "10.1.2.3", true},
		{"rfc1918 192", "192.168.1.5", true},
		{"ipv6 loopback bracket", "[::1]", true},
		{"ipv6 loopback bracket port", "[::1]:9000", true},
		{"ngrok", "abcd.ngrok.io", true},
		{"ngrok free", "abcd.ngrok-free.app", true},
		{"loca.lt", "foo.loca.lt", true},
		{"trycloudflare", "x.trycloudflare.com", true},
		{".test tld", "api.test", true},
		{".local mdns", "host.local", true},
		{".internal", "svc.internal", true},
		{"public host", "hooks.example.com", false},
		{"public host with port", "hooks.example.com:443", false},
		{"public ip", "8.8.8.8", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stale, reason := classifyHost(tc.host)
			if stale != tc.wantStale {
				t.Fatalf("classifyHost(%q) stale = %v (reason %q), want %v", tc.host, stale, reason, tc.wantStale)
			}
			if stale && reason == "" {
				t.Fatalf("classifyHost(%q) flagged stale but gave empty reason", tc.host)
			}
		})
	}
}

func TestExtractHost(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"https://hooks.example.com/rc", "hooks.example.com"},
		{"https://API.Example.com:8443/x", "api.example.com:8443"},
		{"", "(no url)"},
		{"not a url", "not a url"},
	}
	for _, tc := range cases {
		if got := extractHost(tc.in); got != tc.want {
			t.Fatalf("extractHost(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildWebhookAudit(t *testing.T) {
	db := newNovelTestStore(t)

	// Two webhooks to the same prod host => duplicate.
	putResource(t, db, "integrations", "wh1", map[string]any{
		"id": "wh1", "name": "A", "url": "https://hooks.example.com/a",
		"event_types": []string{"initial_purchase", "renewal"},
	})
	putResource(t, db, "integrations", "wh2", map[string]any{
		"id": "wh2", "name": "B", "url": "https://hooks.example.com/b",
		"event_types": []string{"cancellation"},
	})
	// One stale localhost webhook.
	putResource(t, db, "integrations", "wh3", map[string]any{
		"id": "wh3", "name": "Dev", "url": "http://localhost:4000/hook",
		"event_types": []string{"expiration"},
	})

	view, err := buildWebhookAudit(db, "proj1")
	if err != nil {
		t.Fatalf("buildWebhookAudit: %v", err)
	}
	if view.TotalWebhooks != 3 {
		t.Fatalf("total webhooks = %d, want 3", view.TotalWebhooks)
	}
	if view.StaleHosts != 1 {
		t.Fatalf("stale hosts = %d, want 1", view.StaleHosts)
	}
	if view.DuplicateHosts != 1 {
		t.Fatalf("duplicate hosts = %d, want 1", view.DuplicateHosts)
	}
	// example.com group should carry 2 webhooks and 3 event subscriptions.
	var found bool
	for _, h := range view.Hosts {
		if h.Host == "hooks.example.com" {
			found = true
			if len(h.Webhooks) != 2 {
				t.Fatalf("example.com webhooks = %d, want 2", len(h.Webhooks))
			}
			if h.EventCount != 3 {
				t.Fatalf("example.com event count = %d, want 3", h.EventCount)
			}
			if !h.Duplicate {
				t.Fatal("example.com should be flagged duplicate")
			}
		}
	}
	if !found {
		t.Fatal("expected hooks.example.com host group")
	}
}
