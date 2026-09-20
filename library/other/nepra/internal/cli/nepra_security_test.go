package cli

import "testing"

// TestSecHostBoundary pins the authority boundary. A bare prefix match
// against "https://nepra.org.pk" also matches a DIFFERENT registrable domain
// ("nepra.org.pk.evil.example") and a userinfo trick
// ("nepra.org.pk@evil.example"), either of which would have been stripped to
// a bare path and then treated as a nepra.org.pk document by every consumer
// of the sources catalogue.
func TestSecHostBoundary(t *testing.T) {
	cases := map[string]bool{
		"https://nepra.org.pk/x":              true,
		"https://nepra.org.pk":                true,
		"https://nepra.org.pk?q=1":            true,
		"https://nepra.org.pk.evil.example/x": false,
		"https://nepra.org.pk@evil.example/x": false,
		"https://nepra.org.pk-evil.example/x": false,
		"https://nepra.org.pk:8443/x":         false,
	}
	for in, want := range cases {
		got := nepraSourcesIsOnHost(in, nepraSourcesHostPrefixHTTPS)
		if got != want {
			t.Errorf("%s: on-host=%v want %v", in, got, want)
		}
	}
}

// TestSecEventScheme pins the scheme allowlist on scraped determination
// links. normalizeHTMLURL returns any ABSOLUTE url unchanged, so without this
// whatever scheme a page carried travelled out through --json as a "document
// URL". A determination row is KEPT when its link is unusable — the
// determination is still real — but the link is dropped.
func TestSecEventScheme(t *testing.T) {
	for in, want := range map[string]bool{
		"https://nepra.org.pk/a.pdf": true,
		"http://nepra.org.pk/a.pdf":  true,
		"/tariff/a.pdf":              true,
		"javascript:alert(1)":        false,
		"data:text/html,<b>x":        false,
		"file:///etc/passwd":         false,
		"":                           false,
	} {
		got := eventsFetchableURL(in)
		if got != want {
			t.Errorf("%q fetchable=%v want %v", in, got, want)
		}
	}
}

// TestSecEncodedSubPathRefusesTraversalAndNonPathBytes pins the reliability
// path validator against the three bypasses a code review reproduced against
// the shipped binary.
func TestSecEncodedSubPathRefusesTraversalAndNonPathBytes(t *testing.T) {
	for _, bad := range []string{
		// A literal ".." test is trivially bypassed by encoding it: "%2e%2e"
		// is the same path segment to any server and contains no '.' at all.
		// Before the fix this built GET /Standards/2020/%2e%2e/%2e%2e/etc/passwd
		// and exited 0.
		"2020/%2e%2e/%2e%2e/etc/passwd",
		"%2E%2E/secret",
		"2020/../etc/passwd",
		// None of these is a path byte: '?' turns the remainder into a query
		// string and '#' into a fragment the server never sees, so either
		// silently fetches a DIFFERENT document than the caller named.
		"report?x=1",
		"report#frag",
		`report\x`,
		"/absolute/path.pdf",
		"raw space.pdf",
	} {
		if _, err := replaceEncodedSubPath("/Standards/{path}", "path", bad); err == nil {
			t.Errorf("%q was accepted; it must be refused rather than building a URL that names one "+
				"document and fetches another", bad)
		}
	}
	// The real published paths must still pass, including the one whose
	// filename ends in a load-bearing trailing space.
	for _, ok := range []string{
		"2020/PER%20DISCOs%202018-19.pdf",
		"2022/NEPRA%20PER%202021%20Distribution%20Companies%20.pdf",
		"M&E/PER/Distribution/PER%202022-23%20-%20DSICOs.pdf",
	} {
		if _, err := replaceEncodedSubPath("/Standards/{path}", "path", ok); err != nil {
			t.Errorf("published path %q was refused: %v", ok, err)
		}
	}
}
