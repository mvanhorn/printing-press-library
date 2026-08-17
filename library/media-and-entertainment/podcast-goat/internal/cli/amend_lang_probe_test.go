// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the 2026-07 amend surfaces: --lang validation and cache gating,
// the Spotify cookie-missing fallback hint (shell safety included), the
// spend-free probe plumbing, doctor's env-vars verdict, and the pt_demo
// demo-key warnings.

package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

func TestValidateLangFlag(t *testing.T) {
	for _, ok := range []string{"", "en", "it", "zh-Hans"} {
		if err := validateLangFlag(ok); err != nil {
			t.Errorf("validateLangFlag(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"es,en", "it, en", "it en"} {
		if err := validateLangFlag(bad); err == nil {
			t.Errorf("validateLangFlag(%q) = nil, want single-code error", bad)
		}
	}
}

func TestCacheableLang(t *testing.T) {
	if !cacheableLang("") || !cacheableLang("en") {
		t.Error("default-language fetches must remain cacheable")
	}
	if cacheableLang("it") {
		t.Error("non-default language must not be cached (per-URL identity is language-blind)")
	}
}

func TestInAllowList(t *testing.T) {
	if !inAllowList("spoken", nil) {
		t.Error("empty allow-list must allow everything")
	}
	if !inAllowList("spoken", []string{"spoken", "youtube"}) {
		t.Error("listed provider must be allowed")
	}
	if inAllowList("spoken", []string{"youtube"}) {
		t.Error("unlisted provider must be excluded — a probe is a real network call")
	}
}

func TestShellSingleQuote_HostileTitle(t *testing.T) {
	hostile := "watch this $(touch /tmp/pwn) `id` $HOME 'quote'"
	quoted := shellSingleQuote(hostile)
	// Inside POSIX single quotes nothing expands; the only metacharacter to
	// neutralize is the single quote itself.
	if !strings.HasPrefix(quoted, "'") || !strings.HasSuffix(quoted, "'") {
		t.Fatalf("not single-quoted: %q", quoted)
	}
	if want := `'\''quote'\''`; !strings.Contains(quoted, want) {
		t.Errorf("expected %s idiom in %q", want, quoted)
	}
	// Round-trip: count of unescaped quote runs must keep the payload inert —
	// reconstructing what a POSIX shell would parse yields the original text.
	if unquoted := shellUnquoteForTest(quoted); unquoted != hostile {
		t.Errorf("shell round-trip mismatch:\n  got  %q\n  want %q", unquoted, hostile)
	}
}

// shellUnquoteForTest reverses shellSingleQuote the way a POSIX shell would:
// strip the outer quotes and collapse the '\” idiom back to a single quote.
func shellUnquoteForTest(s string) string {
	s = strings.TrimPrefix(s, "'")
	s = strings.TrimSuffix(s, "'")
	return strings.ReplaceAll(s, `'\''`, "'")
}

func TestAlternateSourceHint_NonSpotifyURL(t *testing.T) {
	if got := alternateSourceHint(context.Background(), "https://www.youtube.com/watch?v=abc"); got != "" {
		t.Errorf("non-Spotify URL must produce no hint, got %q", got)
	}
}

func withFakeOEmbed(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	oldBase, oldClient := spotifyOEmbedBase, spotifyOEmbedClient
	spotifyOEmbedBase = srv.URL
	spotifyOEmbedClient = srv.Client()
	t.Cleanup(func() { spotifyOEmbedBase, spotifyOEmbedClient = oldBase, oldClient })
}

func TestAlternateSourceHint_TitleResolved(t *testing.T) {
	withFakeOEmbed(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"title":"A Great Episode"}`)
	})
	got := alternateSourceHint(context.Background(), "https://open.spotify.com/episode/abc")
	if !strings.Contains(got, "'ytsearch1:A Great Episode'") {
		t.Errorf("hint missing single-quoted ytsearch command: %q", got)
	}
}

func TestAlternateSourceHint_HostileTitleIsNeutralized(t *testing.T) {
	withFakeOEmbed(t, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"title":"Ep $(touch /tmp/pwn)"}`)
	})
	got := alternateSourceHint(context.Background(), "https://open.spotify.com/episode/abc")
	if got == "" {
		t.Fatal("expected a hint")
	}
	// The hostile payload must sit inside single quotes, where the shell
	// performs no command substitution.
	if !strings.Contains(got, "'ytsearch1:Ep $(touch /tmp/pwn)'") {
		t.Errorf("hostile title not confined to single quotes: %q", got)
	}
	if strings.Contains(got, `"ytsearch1:`) {
		t.Errorf("hint must not use double quotes around network-sourced text: %q", got)
	}
}

func TestAlternateSourceHint_OEmbedFailureFallsBackGeneric(t *testing.T) {
	withFakeOEmbed(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	got := alternateSourceHint(context.Background(), "https://open.spotify.com/episode/abc")
	if !strings.Contains(got, "'ytsearch1:<episode title>'") {
		t.Errorf("expected the generic template hint on oEmbed failure, got %q", got)
	}
}

// fakeInfoAdapter drives probeRow's non-spoken branch.
type fakeInfoAdapter struct{}

func (fakeInfoAdapter) Name() string          { return "taddy" }
func (fakeInfoAdapter) Tier() transcript.Tier { return transcript.TierPaid }
func (fakeInfoAdapter) Match(string) bool     { return true }
func (fakeInfoAdapter) Fetch(context.Context, string) (*transcript.Transcript, error) {
	return nil, errors.New("not called")
}

func TestProbeRow_UnsupportedAdapter(t *testing.T) {
	var row infoRow
	probeRow(context.Background(), fakeInfoAdapter{}, "https://example.com/x", &row)
	if row.Probed {
		t.Error("non-spoken adapters must not report Probed")
	}
	if row.ProbeNote != "probe not supported for this source" {
		t.Errorf("ProbeNote = %q", row.ProbeNote)
	}
}

func TestProbeErrNote(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"timeout", fmt.Errorf("spoken search: %w", context.DeadlineExceeded), "probe timed out"},
		{"no results", &source.NotApplicableError{Source: "spoken", URL: "u", Reason: "no spoken.md results"}, "no results"},
		{"key missing", &source.KeyMissingError{EnvVar: "SPOKEN_API_KEY"}, "no API key"},
		{"other", errors.New("boom"), "probe failed: boom"},
	}
	for _, c := range cases {
		if got := probeErrNote(c.err); !strings.Contains(got, c.want) {
			t.Errorf("%s: probeErrNote = %q, want contains %q", c.name, got, c.want)
		}
	}
}

func TestEnvVarsSummary(t *testing.T) {
	cases := []struct {
		set, total     int
		authConfigured bool
		want           string
	}{
		{6, 6, false, "OK 6/6 paid-provider keys set"},
		{2, 6, false, "OK 2/6 paid-provider keys set (rest optional"},
		{0, 6, true, "INFO no paid-provider env keys set (optional — cookie auth is configured"},
		{0, 6, false, "INFO no paid-provider env keys set (optional — free + cookie tiers"},
	}
	for _, c := range cases {
		got := envVarsSummary(c.set, c.total, c.authConfigured)
		if !strings.HasPrefix(got, c.want) {
			t.Errorf("envVarsSummary(%d,%d,%v) = %q, want prefix %q", c.set, c.total, c.authConfigured, got, c.want)
		}
		if strings.Contains(got, "missing required") || strings.HasPrefix(got, "ERROR") {
			t.Errorf("env-vars verdict must never be an error: %q", got)
		}
	}
}

func TestDemoKeyWarning(t *testing.T) {
	if got := demoKeyWarning("spoken", "pt_demo"); !strings.Contains(got, "demo key") {
		t.Errorf("pt_demo on spoken must warn, got %q", got)
	}
	if got := demoKeyWarning("spoken", "pt_realkey123"); got != "" {
		t.Errorf("real key must not warn, got %q", got)
	}
	if got := demoKeyWarning("taddy", "pt_demo"); got != "" {
		t.Errorf("other providers must not warn, got %q", got)
	}
}

func TestDemoKeyMarker(t *testing.T) {
	if got := demoKeyMarker("spoken", "config", "pt_demo"); got != "config:demo" {
		t.Errorf("got %q, want config:demo", got)
	}
	if got := demoKeyMarker("spoken", "env", "pt_demo"); got != "env:demo" {
		t.Errorf("got %q, want env:demo", got)
	}
	if got := demoKeyMarker("spoken", "missing", ""); got != "missing" {
		t.Errorf("missing key must stay unmarked, got %q", got)
	}
	if got := demoKeyMarker("spoken", "config", "pt_real"); got != "config" {
		t.Errorf("real key must stay unmarked, got %q", got)
	}
	if got := demoKeyMarker("taddy", "config", "pt_demo"); got != "config" {
		t.Errorf("non-spoken provider must stay unmarked, got %q", got)
	}
}
