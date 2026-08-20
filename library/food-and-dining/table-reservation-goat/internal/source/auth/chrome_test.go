package auth

import (
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/browserutils/kooky"
)

// TestChromeCookieCandidatePathsIn verifies the filesystem walk finds both
// Chrome 96+ (<profile>/Network/Cookies) and pre-96 (<profile>/Cookies)
// layouts even when <root>/Local State doesn't list the profile in
// info_cache. This is the bug surfaced by a user whose macOS Chrome had
// `Default/Cookies` and `Profile 1/Cookies` on disk but kooky returned
// 0 Tock cookies — symptom of info_cache not listing those profiles.
func TestChromeCookieCandidatePathsIn(t *testing.T) {
	tmp := t.TempDir()

	// Layout: a Chrome-like root with both layouts present in different profiles,
	// plus a non-profile sibling directory that must be skipped.
	mustWriteFile := func(p string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	root := filepath.Join(tmp, "Chrome")
	// Default profile: only OLD layout (the failing case from the bug report)
	defaultCookies := filepath.Join(root, "Default", "Cookies")
	mustWriteFile(defaultCookies)
	// Profile 1: only NEW layout
	profile1Network := filepath.Join(root, "Profile 1", "Network", "Cookies")
	mustWriteFile(profile1Network)
	// Profile 2: BOTH layouts (e.g., a recently migrated profile)
	profile2Network := filepath.Join(root, "Profile 2", "Network", "Cookies")
	profile2Old := filepath.Join(root, "Profile 2", "Cookies")
	mustWriteFile(profile2Network)
	mustWriteFile(profile2Old)
	// Guest Profile: covered by the allowlist
	guestCookies := filepath.Join(root, "Guest Profile", "Cookies")
	mustWriteFile(guestCookies)
	// Non-profile siblings: every browser-internal directory must be ignored
	// without us having to enumerate them by name. These three include both
	// stable Chrome internals and a future Chrome dir we don't yet know about
	// — the allowlist must reject them all.
	mustWriteFile(filepath.Join(root, "Crashpad", "Cookies"))
	mustWriteFile(filepath.Join(root, "GrShaderCache", "Cookies"))
	mustWriteFile(filepath.Join(root, "SegmentationPlatform", "Cookies"))
	mustWriteFile(filepath.Join(root, "System Profile", "Cookies"))
	// A file (not a directory) at root level: must not crash the walk.
	mustWriteFile(filepath.Join(root, "Local State"))

	got := chromeCookieCandidatePathsIn([]string{root})

	want := []string{
		defaultCookies,
		profile1Network,
		profile2Network,
		profile2Old,
		guestCookies,
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("chromeCookieCandidatePathsIn:\n got:  %v\n want: %v", got, want)
	}

	// Excluded sibling must NOT appear.
	for _, g := range got {
		for _, banned := range []string{"Crashpad", "GrShaderCache", "SegmentationPlatform", "System Profile"} {
			if strings.Contains(g, banned) {
				t.Errorf("non-profile dir %q leaked into candidates: %q", banned, g)
			}
		}
	}
}

// TestChromeCookieCandidatePathsIn_MissingRoot verifies a non-existent root
// is silently skipped rather than failing the walk — Chrome Beta / Canary
// / Brave on machines that don't have them installed must be tolerated.
func TestChromeCookieCandidatePathsIn_MissingRoot(t *testing.T) {
	got := chromeCookieCandidatePathsIn([]string{"/nonexistent-path-aaa", "/nonexistent-path-bbb"})
	if len(got) != 0 {
		t.Errorf("expected zero candidates for non-existent roots; got %v", got)
	}
}

// TestDedupeCookies_KeysOnDomainNamePath verifies the dedupe collapses
// duplicate (Domain, Name, Path) entries and prefers the entry with the
// later Expires.
func TestDedupeCookies_KeysOnDomainNamePath(t *testing.T) {
	now := time.Now()
	earlier := now.Add(1 * time.Hour)
	later := now.Add(24 * time.Hour)

	a := []*kooky.Cookie{
		{Cookie: http.Cookie{Name: "session", Domain: ".opentable.com", Path: "/", Value: "old", Expires: earlier}},
		{Cookie: http.Cookie{Name: "csrf", Domain: ".opentable.com", Path: "/", Value: "x", Expires: later}},
	}
	b := []*kooky.Cookie{
		// Same key as a's "session" cookie but with a LATER expiry — should win.
		{Cookie: http.Cookie{Name: "session", Domain: ".opentable.com", Path: "/", Value: "new", Expires: later}},
		// Distinct (different Path)
		{Cookie: http.Cookie{Name: "session", Domain: ".opentable.com", Path: "/admin", Value: "admin-only", Expires: later}},
	}

	got := dedupeCookies(a, b)
	if len(got) != 3 {
		t.Fatalf("expected 3 deduped cookies; got %d (%+v)", len(got), got)
	}
	for _, c := range got {
		if c.Name == "session" && c.Path == "/" && c.Value != "new" {
			t.Errorf("dedupeCookies kept the older session cookie value=%q; expected the later-expiring 'new' value", c.Value)
		}
	}
}

// TestDedupeCookies_PersistentBeatsSession verifies that a persistent cookie
// (concrete Expires) is preferred over a session-only cookie (zero Expires)
// for the same (Domain, Name, Path) — even when the session cookie is
// encountered later. Greptile P-finding from PR #399: prior logic short-
// circuited the Expires comparison whenever either side had a zero Expires,
// causing a stale-profile session cookie to silently overwrite a fresh
// persistent cookie from kooky's auto-discovery.
func TestDedupeCookies_PersistentBeatsSession(t *testing.T) {
	now := time.Now()
	persistent := &kooky.Cookie{Cookie: http.Cookie{
		Name: "session", Domain: ".opentable.com", Path: "/", Value: "fresh-persistent",
		Expires: now.Add(48 * time.Hour),
	}}
	sessionOnly := &kooky.Cookie{Cookie: http.Cookie{
		Name: "session", Domain: ".opentable.com", Path: "/", Value: "stale-session-only",
		// Expires zero → session cookie
	}}

	// Persistent encountered first, session-only later.
	got := dedupeCookies([]*kooky.Cookie{persistent}, []*kooky.Cookie{sessionOnly})
	if len(got) != 1 || got[0].Value != "fresh-persistent" {
		t.Errorf("session cookie replaced persistent one; got %+v", got)
	}

	// Reverse order: session-only first, persistent later. The persistent
	// one should win regardless of order.
	got = dedupeCookies([]*kooky.Cookie{sessionOnly}, []*kooky.Cookie{persistent})
	if len(got) != 1 || got[0].Value != "fresh-persistent" {
		t.Errorf("dedupe order-dependent; expected persistent to win, got %+v", got)
	}
}

// TestDedupeCookies_NilEntriesIgnored confirms nil entries (which kooky may
// occasionally emit) don't panic and don't pollute the output.
func TestDedupeCookies_NilEntriesIgnored(t *testing.T) {
	a := []*kooky.Cookie{
		nil,
		{Cookie: http.Cookie{Name: "x", Domain: "y", Path: "/"}},
		nil,
	}
	got := dedupeCookies(a)
	if len(got) != 1 {
		t.Errorf("expected nil entries to be skipped; got %d cookies", len(got))
	}
}
