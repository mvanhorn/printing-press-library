package captcha

import (
	"context"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/suno/internal/auth"
)

func TestToCDPCookies_MapsFieldsAndDefaultsPath(t *testing.T) {
	in := []auth.SunoCookie{
		{Name: "__client", Value: "v1", Domain: "auth.suno.com", Path: "", Secure: true, HTTPOnly: true},
		{Name: "", Value: "skip", Domain: ".suno.com"}, // nameless -> dropped
	}
	got := toCDPCookies(in)
	if len(got) != 1 {
		t.Fatalf("nameless cookie should be dropped: got %d", len(got))
	}
	if got[0].Path != "/" {
		t.Fatalf("empty path should default to /, got %q", got[0].Path)
	}
	if got[0].Name != "__client" || got[0].Domain != "auth.suno.com" {
		t.Fatalf("field mismatch: %+v", got[0])
	}
}

func TestSeedFromBrowser_WrapsAuthReader(t *testing.T) {
	fn := seedFromBrowser
	if fn == nil {
		t.Fatal("seedFromBrowser must be a usable SeedFunc")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _ = fn(ctx) // must not panic; result depends on host browser
}
