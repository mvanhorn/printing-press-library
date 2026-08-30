// Copyright 2026 Matt Van Horn and contributors. Licensed under Apache-2.0. See LICENSE.

package dispatch

import (
	"context"
	"errors"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/source/youtube"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/transcript"
)

// fakeAdapter is a minimal source.Adapter for driving dispatchChain.
type fakeAdapter struct {
	name    string
	tier    transcript.Tier
	matches bool
	err     error
	tr      *transcript.Transcript
}

func (f *fakeAdapter) Name() string          { return f.name }
func (f *fakeAdapter) Tier() transcript.Tier { return f.tier }
func (f *fakeAdapter) Match(string) bool     { return f.matches }
func (f *fakeAdapter) Fetch(context.Context, string) (*transcript.Transcript, error) {
	return f.tr, f.err
}

func TestDispatchChain_FinalErrorUnwrapsToTypedError(t *testing.T) {
	// Regression: the chain-exhausted summary used to flatten the last
	// recoverable error into a string (%s), so errors.As could never reach
	// the typed error and the CLI's cookie-missing hint was dead code.
	cookieErr := &source.CookieMissingError{Service: "spotify", Hint: "run auth login-service"}
	chain := []source.Adapter{
		&fakeAdapter{name: "spotify", tier: transcript.TierCookie, matches: true, err: cookieErr},
	}
	_, err := dispatchChain(context.Background(), "https://open.spotify.com/episode/x", Options{}, chain)
	if err == nil {
		t.Fatal("expected an error after exhausting the chain")
	}
	var cm *source.CookieMissingError
	if !errors.As(err, &cm) {
		t.Fatalf("Dispatch error does not unwrap to *source.CookieMissingError: %v", err)
	}
	if cm.Service != "spotify" {
		t.Errorf("unwrapped service = %q, want spotify", cm.Service)
	}
}

func TestDispatchChain_NoMatchStillGeneric(t *testing.T) {
	chain := []source.Adapter{
		&fakeAdapter{name: "youtube", tier: transcript.TierFree, matches: false},
	}
	_, err := dispatchChain(context.Background(), "https://example.com/x", Options{}, chain)
	if err == nil || err.Error() != "no adapter matched https://example.com/x" {
		t.Fatalf("expected generic no-adapter error, got %v", err)
	}
}

func TestApplyOptions_LangCopiesYouTubeSingleton(t *testing.T) {
	yt := youtube.New()
	got := applyOptions(yt, Options{Lang: "it"})
	cp, ok := got.(*youtube.Adapter)
	if !ok {
		t.Fatalf("applyOptions returned %T, want *youtube.Adapter", got)
	}
	if cp == yt {
		t.Fatal("applyOptions must not return the registered singleton when Lang is set")
	}
	if cp.Lang != "it" {
		t.Errorf("copy Lang = %q, want it", cp.Lang)
	}
	if yt.Lang != "" {
		t.Errorf("singleton mutated: Lang = %q, want empty", yt.Lang)
	}
}

func TestApplyOptions_PassthroughWithoutLang(t *testing.T) {
	yt := youtube.New()
	if got := applyOptions(yt, Options{}); got != source.Adapter(yt) {
		t.Errorf("without Lang the singleton itself must be returned")
	}
	fake := &fakeAdapter{name: "spoken"}
	if got := applyOptions(fake, Options{Lang: "it"}); got != source.Adapter(fake) {
		t.Errorf("non-youtube adapters must pass through unchanged")
	}
}
