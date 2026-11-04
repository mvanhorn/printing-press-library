package cli

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/government-data/internal/config"
)

// Reuse the generated TLS/configuration seam, but only its HTTP transport.
// Authentication and redirect policies remain owned by each public source.
func govHTTPClient(f *rootFlags) (*http.Client, error) {
	cfg, err := config.Load(f.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	f.applyTLSFlags(cfg)
	c := client.New(cfg, f.timeout, f.rateLimit)
	if err := c.TLSConfigError(); err != nil {
		return nil, configErr(err)
	}
	h := *c.HTTPClient
	h.Jar = nil
	return &h, nil
}

type secSettingsKey struct{}
type secSettings struct {
	http    *http.Client
	limiter *cliutil.AdaptiveLimiter
}

func withSECSettings(ctx context.Context, selected *http.Client, rate float64) context.Context {
	h := *selected
	h.Jar = nil
	h.CheckRedirect = secRedirect
	// SEC remains conservatively capped at two requests/second; a lower
	// user ceiling is honored. Explicit zero disables local pacing.
	if rate < 0 || rate > 2 {
		rate = 2
	}
	return context.WithValue(ctx, secSettingsKey{}, secSettings{&h, cliutil.NewAdaptiveLimiter(rate)})
}
func secRedirect(r *http.Request, via []*http.Request) error {
	if len(via) >= 5 {
		return fmt.Errorf("SEC redirect limit reached")
	}
	if r.URL.Scheme != "https" || !secHost(r.URL.Hostname()) {
		return fmt.Errorf("SEC redirected outside approved SEC hosts")
	}
	return nil
}
func secRequestSettings(ctx context.Context) (*http.Client, *cliutil.AdaptiveLimiter) {
	if settings, ok := ctx.Value(secSettingsKey{}).(secSettings); ok {
		return settings.http, settings.limiter
	}
	return &http.Client{Timeout: time.Minute, CheckRedirect: secRedirect}, secLimiter
}
