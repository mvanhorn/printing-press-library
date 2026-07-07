//go:build chrome

// Opt-in real-Chrome test. Run with: go test -tags chrome ./internal/captcha/
// Requires a local Chrome/Chromium. Not part of default CI.

package captcha

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIntegration_OpenNavigateEvaluate(t *testing.T) {
	dir := filepath.Join(os.TempDir(), "suno-captcha-itest")
	opts := Options{Profile: "itest", UserDataDir: dir, CDPPort: 9320, Interactive: false, Timeout: 30 * time.Second}
	b, err := openBrowser(context.Background(), opts, false)
	if err != nil {
		t.Fatalf("openBrowser: %v", err)
	}
	defer b.close()
	if err := b.navigate(context.Background()); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	out, err := b.evaluate(context.Background(), `(async()=>'pong')()`)
	if err != nil || out != "pong" {
		t.Fatalf("evaluate pong: out=%q err=%v", out, err)
	}
}
